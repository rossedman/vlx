package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
	"github.com/rossedman/vlx/internal/schema"
)

// probeConcurrency bounds how many sockets list_services probes at once.
const probeConcurrency = 8

// addTools registers the generic tools.
func (s *Server) addTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:  "list_services",
		Title: "List Varlink services",
		Description: "List the Varlink sockets on this host: name, path and whether you may connect. " +
			"With probe, also connect to each accessible socket and report its vendor, product, version and interfaces.",
		Annotations: readOnly("List Varlink services"),
	}, s.listServices)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:  "describe",
		Title: "Describe a Varlink service",
		Description: "Show a service's interfaces with their methods, types and errors. Each method has its doc comment, " +
			"safety class (READ, WRITE or DANGER), streaming mode, a one-line signature and whether this server will call it. " +
			"Give interface to describe only one.",
		Annotations: readOnly("Describe a Varlink service"),
	}, s.describe)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:  "method_schema",
		Title: "Get a method's schema",
		Description: "JSON Schema for a method's parameters and reply, an example parameter object with every required field, " +
			"and the optional fields. Use it before call when you are unsure of the parameters.",
		Annotations: readOnly("Get a method's schema"),
	}, s.methodSchema)
	s.mcp.AddTool(&mcp.Tool{
		Name:  "call",
		Title: "Call a Varlink method",
		Description: "Call a method and return its replies. Parameters are checked against the method's IDL first, and every " +
			"problem is reported at once. Only READ methods are sent. Methods that stream return up to stream.max_replies " +
			"replies within stream.max_seconds; continues is true when there was more.",
		InputSchema: callInputSchema(),
		Annotations: &mcp.ToolAnnotations{Title: "Call a Varlink method", ReadOnlyHint: false, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)},
	}, s.callTool)
}

// readOnly returns the annotations of a tool that only reads.
func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)}
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// ListServicesInput is the input of list_services.
type ListServicesInput struct {
	Probe bool `json:"probe,omitempty" jsonschema:"also connect to each accessible socket and report its vendor, product, version and interfaces"`
}

// ServiceEntry is one socket in list_services.
type ServiceEntry struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Accessible bool     `json:"accessible"`
	Vendor     string   `json:"vendor,omitempty"`
	Product    string   `json:"product,omitempty"`
	Version    string   `json:"version,omitempty"`
	URL        string   `json:"url,omitempty"`
	Interfaces []string `json:"interfaces,omitempty"`
	// Error says why probing failed.
	Error string `json:"error,omitempty"`
}

// ListServicesOutput is the result of list_services.
type ListServicesOutput struct {
	Services []ServiceEntry `json:"services"`
	Notice   string         `json:"notice"`
}

// listServices handles list_services.
func (s *Server) listServices(ctx context.Context, _ *mcp.CallToolRequest, in ListServicesInput) (*mcp.CallToolResult, ListServicesOutput, error) {
	socks := s.sockets()
	out := ListServicesOutput{Services: make([]ServiceEntry, len(socks)), Notice: Notice}
	for i, sock := range socks {
		out.Services[i] = ServiceEntry{Name: sanitize.Line(sock.Name), Path: sanitize.Line(sock.Path), Accessible: sock.Accessible}
	}
	if in.Probe {
		s.probe(ctx, socks, out.Services)
	}
	return nil, out, nil
}

// probe fills in GetInfo details for every accessible socket, a few at a
// time.
func (s *Server) probe(ctx context.Context, socks []discovery.Socket, entries []ServiceEntry) {
	sem := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup
	for i, sock := range socks {
		if !sock.Accessible {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			info, err := client.Info(ctx, sock.Path, s.opts.Client)
			e := &entries[i]
			if err != nil {
				e.Error = s.callError(err, nil).Message
				return
			}
			e.Vendor, e.Product = sanitize.Line(info.Vendor), sanitize.Line(info.Product)
			e.Version, e.URL = sanitize.Line(info.Version), sanitize.Line(info.URL)
			for _, name := range info.Interfaces {
				e.Interfaces = append(e.Interfaces, sanitize.Line(name))
			}
		}()
	}
	wg.Wait()
}

// DescribeInput is the input of describe.
type DescribeInput struct {
	Service   string `json:"service" jsonschema:"a short name (resolve), a full name (io.systemd.Resolve) or a socket path"`
	Interface string `json:"interface,omitempty" jsonschema:"describe only this interface, by full name or its last part"`
}

// DescribeOutput is the result of describe.
type DescribeOutput struct {
	Path       string            `json:"path"`
	Vendor     string            `json:"vendor,omitempty"`
	Product    string            `json:"product,omitempty"`
	Version    string            `json:"version,omitempty"`
	Interfaces []InterfaceDetail `json:"interfaces"`
	Notice     string            `json:"notice"`
}

// InterfaceDetail is one interface in describe.
type InterfaceDetail struct {
	Name    string         `json:"name"`
	Doc     string         `json:"doc,omitempty"`
	Error   string         `json:"error,omitempty"`
	Methods []MethodDetail `json:"methods,omitempty"`
	Types   []TypeDetail   `json:"types,omitempty"`
	Errors  []ErrorDetail  `json:"errors,omitempty"`
}

// MethodDetail is one method in describe.
type MethodDetail struct {
	Name        string `json:"name"`
	Qualified   string `json:"qualified"`
	Doc         string `json:"doc,omitempty"`
	Class       string `json:"class"`
	ClassReason string `json:"classReason"`
	Streaming   string `json:"streaming" jsonschema:"single, supports-more or requires-more"`
	Signature   string `json:"signature"`
	// Callable says whether call will send this method; Refusal says why
	// not.
	Callable bool   `json:"callable"`
	Refusal  string `json:"refusal,omitempty"`
}

// TypeDetail is one named type in describe.
type TypeDetail struct {
	Name       string `json:"name"`
	Doc        string `json:"doc,omitempty"`
	Definition string `json:"definition"`
}

// ErrorDetail is one error in describe.
type ErrorDetail struct {
	Name       string `json:"name"`
	Doc        string `json:"doc,omitempty"`
	Parameters string `json:"parameters"`
}

// describe handles describe.
func (s *Server) describe(ctx context.Context, _ *mcp.CallToolRequest, in DescribeInput) (*mcp.CallToolResult, DescribeOutput, error) {
	path, err := s.resolve(in.Service)
	if err != nil {
		return nil, DescribeOutput{}, err
	}
	svc, err := s.introspect(ctx, path)
	if err != nil {
		return nil, DescribeOutput{}, s.callError(err, nil)
	}
	ifaces := svc.Interfaces
	if in.Interface != "" {
		iface, err := findInterface(svc, in.Interface)
		if err != nil {
			return nil, DescribeOutput{}, err
		}
		ifaces = []*client.Interface{iface}
	}
	out := DescribeOutput{
		Path: sanitize.Line(path), Vendor: sanitize.Line(svc.Info.Vendor), Product: sanitize.Line(svc.Info.Product),
		Version: sanitize.Line(svc.Info.Version), Interfaces: []InterfaceDetail{}, Notice: Notice,
	}
	for _, iface := range ifaces {
		out.Interfaces = append(out.Interfaces, s.interfaceDetail(iface))
	}
	return nil, out, nil
}

// findInterface returns the interface named name, or the only one whose
// last part is name.
func findInterface(svc *client.Service, name string) (*client.Interface, error) {
	if iface := svc.Interface(name); iface != nil {
		return iface, nil
	}
	var found []*client.Interface
	for _, iface := range svc.Interfaces {
		if _, last, _ := cutLast(iface.Name); strings.EqualFold(last, name) {
			found = append(found, iface)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	te := &ToolError{Kind: KindNotFound, Message: "the service has no interface " + sanitize.Line(name), Retry: true,
		Hint: "Call describe without interface to list them."}
	for _, iface := range found {
		te.Candidates = append(te.Candidates, sanitize.Line(iface.Name))
	}
	if len(found) > 1 {
		te.Message = "interface name " + sanitize.Line(name) + " is ambiguous"
	}
	return nil, te
}

// interfaceDetail describes one interface, sanitising everything the
// service wrote.
func (s *Server) interfaceDetail(iface *client.Interface) InterfaceDetail {
	d := InterfaceDetail{Name: sanitize.Line(iface.Name)}
	if iface.Parsed == nil {
		if iface.Err != nil {
			d.Error = sanitize.Line(iface.Err.Error())
		}
		return d
	}
	p := iface.Parsed
	d.Doc = sanitize.Text(p.Doc)
	for _, m := range p.Methods {
		d.Methods = append(d.Methods, s.methodDetail(p, m))
	}
	for _, t := range p.Types {
		d.Types = append(d.Types, TypeDetail{Name: t.Name, Doc: sanitize.Text(t.Doc), Definition: typeString(t.Type)})
	}
	for _, e := range p.Errors {
		d.Errors = append(d.Errors, ErrorDetail{Name: e.Name, Doc: sanitize.Text(e.Doc), Parameters: typeString(e.Params)})
	}
	return d
}

// methodDetail describes one method, with its class and whether call will
// send it.
func (s *Server) methodDetail(iface *idl.Interface, m *idl.Method) MethodDetail {
	q := iface.Name + "." + m.Name
	v := s.classifier.Classify(q)
	refusal := s.refusal(iface, m, q, v)
	return MethodDetail{
		Name: m.Name, Qualified: q, Doc: sanitize.Text(m.Doc),
		Class: v.Class.String(), ClassReason: sanitize.Line(v.Reason), Streaming: m.More.String(),
		Signature: m.Name + typeString(m.In) + " -> " + typeString(m.Out),
		Callable:  refusal == "", Refusal: refusal,
	}
}

// typeString formats t on one line, or () when it is missing.
func typeString(t *idl.Type) string {
	if t == nil {
		return "()"
	}
	return sanitize.Line(t.String())
}

// refusal returns why call will not send method q, or "" when it will.
func (s *Server) refusal(iface *idl.Interface, m *idl.Method, q string, v safety.Verdict) string {
	switch {
	case v.Class != safety.Read:
		d := safety.Policy{AllowWrites: s.opts.AllowWrites}.Decide(q, v)
		if !d.Allowed {
			return q + " is a " + v.Class.String() + " method and vlx mcp is read-only. " +
				"Sending it needs a person to start vlx mcp with --allow-writes and to confirm the call, which this version of vlx mcp does not support yet. Nothing was sent."
		}
		return q + " is a " + v.Class.String() + " method. It needs a person's confirmation, which this version of vlx mcp does not support yet. Nothing was sent."
	case takesFiles(iface, m.In):
		return q + " takes file descriptors, which vlx mcp does not pass. Nothing was sent."
	}
	return ""
}

// takesFiles reports whether t has a field that carries a file descriptor:
// an int field whose name ends in FileDescriptor or FileDescriptorIndex,
// the convention systemd's interfaces use, at any depth.
func takesFiles(iface *idl.Interface, t *idl.Type) bool {
	return fdWalk(iface, t, map[string]bool{}, 0)
}

// fdWalk does the work of takesFiles, visiting each named type once.
func fdWalk(iface *idl.Interface, t *idl.Type, seen map[string]bool, depth int) bool {
	if t == nil || depth > maxNesting {
		return false
	}
	switch t.Kind {
	case idl.Nullable, idl.Array, idl.Map:
		return fdWalk(iface, t.Elem, seen, depth+1)
	case idl.Named:
		if seen[t.Name] {
			return false
		}
		seen[t.Name] = true
		if def := iface.Type(t.Name); def != nil {
			return fdWalk(iface, def.Type, seen, depth+1)
		}
	case idl.Struct:
		for _, f := range t.Fields {
			if isFDField(f) || fdWalk(iface, f.Type, seen, depth+1) {
				return true
			}
		}
	}
	return false
}

// isFDField reports whether f looks like a file-descriptor index.
func isFDField(f *idl.Field) bool {
	n := strings.ToLower(f.Name)
	if !strings.HasSuffix(n, "filedescriptor") && !strings.HasSuffix(n, "filedescriptorindex") {
		return false
	}
	t := f.Type
	for t.Kind == idl.Nullable || t.Kind == idl.Array {
		t = t.Elem
	}
	return t.Kind == idl.Int
}

// MethodInput names a method on a service.
type MethodInput struct {
	Service string `json:"service" jsonschema:"a short name (resolve), a full name (io.systemd.Resolve) or a socket path"`
	Method  string `json:"method" jsonschema:"qualified (io.systemd.Resolve.ResolveHostname) or bare when only one interface has it"`
}

// MethodSchemaOutput is the result of method_schema.
type MethodSchemaOutput struct {
	Method    string `json:"method"`
	Doc       string `json:"doc,omitempty"`
	Class     string `json:"class"`
	Streaming string `json:"streaming"`
	Callable  bool   `json:"callable"`
	Refusal   string `json:"refusal,omitempty"`
	// InputSchema and OutputSchema are JSON Schema (draft 2020-12).
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
	// Example is a parameter object with every required field set.
	Example any `json:"example"`
	// Optional lists the optional parameters as "name: type".
	Optional []string `json:"optional,omitempty"`
	Notice   string   `json:"notice"`
}

// methodSchema handles method_schema.
func (s *Server) methodSchema(ctx context.Context, _ *mcp.CallToolRequest, in MethodInput) (*mcp.CallToolResult, MethodSchemaOutput, error) {
	_, iface, m, err := s.findMethod(ctx, in.Service, in.Method)
	if err != nil {
		return nil, MethodSchemaOutput{}, err
	}
	p := iface.Parsed
	d := s.methodDetail(p, m)
	example, optional := schema.Skeleton(p, m.In)
	var ex any
	if err := json.Unmarshal([]byte(example), &ex); err != nil {
		ex = example
	}
	for i, o := range optional {
		optional[i] = sanitize.Line(o)
	}
	return nil, MethodSchemaOutput{
		Method: d.Qualified, Doc: d.Doc, Class: d.Class, Streaming: d.Streaming, Callable: d.Callable, Refusal: d.Refusal,
		InputSchema: JSONSchema(p, m.In), OutputSchema: JSONSchema(p, m.Out),
		Example: ex, Optional: optional, Notice: Notice,
	}, nil
}

// findMethod resolves the service, introspects it and finds the method.
func (s *Server) findMethod(ctx context.Context, service, method string) (string, *client.Interface, *idl.Method, error) {
	path, err := s.resolve(service)
	if err != nil {
		return "", nil, nil, err
	}
	svc, err := s.introspect(ctx, path)
	if err != nil {
		return "", nil, nil, s.callError(err, nil)
	}
	iface, m, err := svc.FindMethod(method)
	if err != nil {
		return "", nil, nil, methodNotFound(err)
	}
	return path, iface, m, nil
}
