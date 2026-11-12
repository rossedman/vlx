package mcpserver

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/sanitize"
)

// ResourceScheme is the URI scheme of interface resources:
// varlink://<socket name>/<interface>.
const ResourceScheme = "varlink://"

// idlMIMEType is the MIME type of an interface's IDL text.
const idlMIMEType = "text/plain"

// addResources registers the interface template and the middleware that
// answers resources/list from discovery.
func (s *Server) addResources() {
	s.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "varlink-interface",
		Title:       "Varlink interface IDL",
		Description: "The IDL of one interface a Varlink service implements. " + Notice,
		MIMEType:    idlMIMEType,
		URITemplate: ResourceScheme + "{service}/{interface}",
	}, s.readInterface)
	s.mcp.AddReceivingMiddleware(s.listResourcesMiddleware)
}

// listResourcesMiddleware answers resources/list with one resource per
// interface of every accessible socket, keeping the SDK's own result
// fields, and passes every other request on.
func (s *Server) listResourcesMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "resources/list" {
			return next(ctx, method, req)
		}
		res, err := next(ctx, method, req)
		if lr, ok := res.(*mcp.ListResourcesResult); ok && err == nil {
			lr.Resources, lr.NextCursor = s.listInterfaces(ctx), ""
		}
		return res, err
	}
}

// listInterfaces introspects the accessible sockets, a few at a time, and
// returns their interfaces as resources sorted by URI. Sockets that fail
// to answer are left out.
func (s *Server) listInterfaces(ctx context.Context) []*mcp.Resource {
	socks := s.sockets()
	sem := make(chan struct{}, probeConcurrency)
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		out = []*mcp.Resource{}
	)
	for _, sock := range socks {
		if !sock.Accessible {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(sock discovery.Socket) {
			defer wg.Done()
			defer func() { <-sem }()
			svc, err := s.introspect(ctx, sock.Path)
			if err != nil {
				return
			}
			var found []*mcp.Resource
			for _, iface := range svc.Interfaces {
				found = append(found, &mcp.Resource{
					URI:         ResourceURI(sock.Name, iface.Name),
					Name:        sanitize.Line(iface.Name),
					Description: "IDL of " + sanitize.Line(iface.Name) + " on " + sanitize.Line(sock.Name) + ". " + Notice,
					MIMEType:    idlMIMEType,
				})
			}
			mu.Lock()
			out = append(out, found...)
			mu.Unlock()
		}(sock)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].URI < out[j].URI })
	return out
}

// ResourceURI returns the URI of an interface on the socket called service.
func ResourceURI(service, iface string) string {
	return ResourceScheme + url.PathEscape(service) + "/" + url.PathEscape(iface)
}

// parseResourceURI splits a resource URI into its socket name and
// interface.
func parseResourceURI(uri string) (service, iface string, ok bool) {
	rest, ok := strings.CutPrefix(uri, ResourceScheme)
	if !ok {
		return "", "", false
	}
	svc, ifc, ok := strings.Cut(rest, "/")
	if !ok || svc == "" || ifc == "" || strings.Contains(ifc, "/") {
		return "", "", false
	}
	svc, err1 := url.PathUnescape(svc)
	ifc, err2 := url.PathUnescape(ifc)
	if err1 != nil || err2 != nil || strings.Contains(svc, "/") {
		return "", "", false
	}
	return svc, ifc, true
}

// readInterface handles resources/read for an interface URI, returning its
// IDL text sanitised and prefixed with the data notice.
func (s *Server) readInterface(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	service, name, ok := parseResourceURI(uri)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	path, err := s.resolve(service)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	svc, err := s.introspect(ctx, path)
	if err != nil {
		return nil, s.callError(err, nil)
	}
	iface := svc.Interface(name)
	if iface == nil || iface.Description == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI: uri, MIMEType: idlMIMEType,
		Text: "# " + Notice + "\n" + sanitize.Text(iface.Description),
	}}}, nil
}
