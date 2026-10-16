// Package demo is a small, self-contained Varlink service that vlx can
// serve with 'vlx demo', so people can learn vlx and Varlink on any machine
// without systemd. It keeps its state in memory and touches nothing else.
package demo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/schema"
)

// maxMessage bounds the size of a call the demo accepts.
const maxMessage = 1 << 20

// Server is a running demo service.
type Server struct {
	path     string
	listener net.Listener
	ifaces   map[string]*idl.Interface
	descs    map[string]string
	names    []string

	mu       sync.Mutex
	lang     string
	items    map[int64]item
	nextID   int64
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
	closed   bool
	closeErr error
}

// Listen starts a demo service on a Unix socket at path. The caller must
// Close it.
func Listen(path string) (*Server, error) {
	s := &Server{
		path:   path,
		ifaces: map[string]*idl.Interface{},
		descs:  map[string]string{},
		conns:  map[net.Conn]struct{}{},
	}
	for _, desc := range []string{ServiceIDL, GreeterIDL, InventoryIDL} {
		iface, err := idl.Parse(desc)
		if err != nil {
			return nil, fmt.Errorf("demo IDL: %w", err)
		}
		s.ifaces[iface.Name] = iface
		s.descs[iface.Name] = desc
		s.names = append(s.names, iface.Name)
	}
	sort.Strings(s.names)
	s.reset()
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	s.listener = l
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

// ListenTemp starts a demo service on a socket in a new private temporary
// directory, which Close removes.
func ListenTemp() (*Server, error) {
	dir, err := os.MkdirTemp("", "vlx-demo-")
	if err != nil {
		return nil, err
	}
	s, err := Listen(filepath.Join(dir, "io.vlx.demo"))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return s, nil
}

// ListenUser starts a demo service at $XDG_RUNTIME_DIR/vlx/io.vlx.demo,
// where 'vlx list' and SERVICE names find it, falling back to ListenTemp
// when that is unavailable or another demo is already running there.
func ListenUser() (*Server, error) {
	if p := UserSocketPath(); p != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err == nil {
			removeStaleSocket(p)
			if s, err := Listen(p); err == nil {
				return s, nil
			}
		}
	}
	return ListenTemp()
}

// UserSocketPath is where ListenUser listens, or "" without
// XDG_RUNTIME_DIR.
func UserSocketPath() string {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg == "" || !filepath.IsAbs(xdg) {
		return ""
	}
	return filepath.Join(xdg, "vlx", "io.vlx.demo")
}

// removeStaleSocket removes a socket left behind by a demo that crashed. It
// leaves anything that is not a socket, and sockets that still accept.
func removeStaleSocket(p string) {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode().Type() != os.ModeSocket {
		return
	}
	if c, err := net.DialTimeout("unix", p, time.Second); err == nil {
		c.Close()
		return
	}
	os.Remove(p)
}

// Path returns the socket path.
func (s *Server) Path() string { return s.path }

// Close stops the service, closes every connection and removes the socket
// (and its directory, if ListenTemp created it).
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return s.closeErr
	}
	s.closed = true
	s.closeErr = s.listener.Close()
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	os.Remove(s.path)
	if dir := filepath.Dir(s.path); strings.HasPrefix(filepath.Base(dir), "vlx-demo-") {
		os.Remove(dir)
	}
	return s.closeErr
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		c, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			c.Close()
			return
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(c)
	}
}

type call struct {
	Method     string          `json:"method"`
	Parameters json.RawMessage `json:"parameters"`
	More       bool            `json:"more"`
	Oneway     bool            `json:"oneway"`
}

// reply is one message to send. A non-empty Error makes it an error reply.
type reply struct {
	Parameters any    `json:"parameters,omitempty"`
	Continues  bool   `json:"continues,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (s *Server) serve(c net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		c.Close()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := bufio.NewReader(c)
	for {
		msg, err := readMessage(r)
		if err != nil {
			return
		}
		var in call
		if err := json.Unmarshal(msg, &in); err != nil {
			return // protocol error: drop the connection
		}
		send := func(rep reply) bool {
			if in.Oneway {
				return true
			}
			data, err := json.Marshal(rep)
			if err != nil {
				return false
			}
			_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, err = c.Write(append(data, 0))
			return err == nil
		}
		if !s.dispatch(ctx, in, send) {
			return
		}
	}
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		chunk, err := r.ReadSlice(0)
		buf.Write(chunk)
		if buf.Len() > maxMessage {
			return nil, errors.New("message too large")
		}
		if err == nil {
			return bytes.TrimSuffix(buf.Bytes(), []byte{0}), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func errorReply(name string, params any) reply {
	return reply{Error: name, Parameters: params}
}

// dispatch handles one call. It returns false if the connection should be
// closed.
func (s *Server) dispatch(ctx context.Context, in call, send func(reply) bool) bool {
	i := strings.LastIndexByte(in.Method, '.')
	if i <= 0 {
		return send(errorReply("org.varlink.service.InterfaceNotFound", map[string]string{"interface": in.Method}))
	}
	ifaceName, methodName := in.Method[:i], in.Method[i+1:]
	iface, ok := s.ifaces[ifaceName]
	if !ok {
		return send(errorReply("org.varlink.service.InterfaceNotFound", map[string]string{"interface": ifaceName}))
	}
	m := iface.Method(methodName)
	if m == nil {
		return send(errorReply("org.varlink.service.MethodNotFound", map[string]string{"method": in.Method}))
	}
	if len(in.Parameters) == 0 || string(in.Parameters) == "null" {
		in.Parameters = json.RawMessage("{}")
	}
	if err := schema.Validate(iface, m.In, in.Parameters); err != nil {
		return send(errorReply("org.varlink.service.InvalidParameter", map[string]string{"parameter": invalidParam(err)}))
	}
	if m.More == idl.MoreRequired && !in.More {
		return send(errorReply("org.varlink.service.ExpectedMore", nil))
	}
	switch in.Method {
	case "org.varlink.service.GetInfo":
		return send(reply{Parameters: map[string]any{
			"vendor":     "vlx",
			"product":    "vlx demo service",
			"version":    "1",
			"url":        "https://github.com/rossedman/vlx",
			"interfaces": s.names,
		}})
	case "org.varlink.service.GetInterfaceDescription":
		var p struct {
			Interface string `json:"interface"`
		}
		_ = json.Unmarshal(in.Parameters, &p)
		desc, ok := s.descs[p.Interface]
		if !ok {
			return send(errorReply("org.varlink.service.InterfaceNotFound", map[string]string{"interface": p.Interface}))
		}
		return send(reply{Parameters: map[string]string{"description": desc}})
	case "io.vlx.demo.Greeter.WatchCountdown":
		return s.countdown(ctx, in, send)
	case "io.vlx.demo.Inventory.ListItems":
		return s.listItems(in, send)
	}
	params, errName, errParams := s.handle(in)
	if errName != "" {
		return send(errorReply(errName, errParams))
	}
	return send(reply{Parameters: params})
}

func invalidParam(err error) string {
	var es schema.Errors
	if errors.As(err, &es) && len(es) > 0 {
		return strings.TrimPrefix(es[0].Path, ".")
	}
	return ""
}

var greetings = map[string]string{
	"english":  "Hello, %s!",
	"spanish":  "¡Hola, %s!",
	"german":   "Hallo, %s!",
	"japanese": "こんにちは、%sさん！",
}

type item struct {
	ID         int64             `json:"id"`
	Name       string            `json:"name"`
	Quantity   int64             `json:"quantity"`
	Tags       []string          `json:"tags"`
	Dimensions *dimensions       `json:"dimensions,omitempty"`
	Attributes map[string]string `json:"attributes"`
	supplier   string
}

type supplier struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	APIToken string `json:"apiToken"`
}

type dimensions struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Depth  float64 `json:"depth"`
}

func (s *Server) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lang = "english"
	s.items = map[int64]item{
		1: {ID: 1, Name: "Socket wrench", Quantity: 12, Tags: []string{"tools"},
			Dimensions: &dimensions{Width: 4.5, Height: 2, Depth: 25},
			Attributes: map[string]string{"material": "steel"}, supplier: "Acme Tools"},
		2: {ID: 2, Name: "Unix socket (assorted)", Quantity: 404, Tags: []string{"sockets", "ipc"},
			Attributes: map[string]string{"family": "AF_UNIX", "type": "SOCK_STREAM"}, supplier: "Bell Labs Surplus"},
		3: {ID: 3, Name: "NUL byte", Quantity: 1 << 20, Tags: []string{"framing"},
			Attributes: map[string]string{"value": "0x00"}, supplier: "Varlink Supply Co."},
		4: {ID: 4, Name: "Rubber duck", Quantity: 3, Tags: []string{"debugging", "tools"},
			Dimensions: &dimensions{Width: 8, Height: 9, Depth: 10},
			Attributes: map[string]string{"color": "yellow"}, supplier: "Acme Tools"},
	}
	s.nextID = 5
}

func (s *Server) handle(in call) (params any, errName string, errParams any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch in.Method {
	case "io.vlx.demo.Greeter.GetGreeting":
		var p struct {
			Name     string  `json:"name"`
			Language *string `json:"language"`
		}
		_ = json.Unmarshal(in.Parameters, &p)
		if strings.TrimSpace(p.Name) == "" {
			return nil, "io.vlx.demo.Greeter.EmptyName", nil
		}
		lang := s.lang
		if p.Language != nil {
			lang = *p.Language
		}
		return map[string]string{"greeting": fmt.Sprintf(greetings[lang], p.Name), "language": lang}, "", nil
	case "io.vlx.demo.Greeter.ListLanguages":
		return map[string]any{"languages": []string{"english", "spanish", "german", "japanese"}, "default": s.lang}, "", nil
	case "io.vlx.demo.Greeter.SetDefaultLanguage":
		var p struct {
			Language string `json:"language"`
		}
		_ = json.Unmarshal(in.Parameters, &p)
		s.lang = p.Language
		return struct{}{}, "", nil
	case "io.vlx.demo.Inventory.GetItem", "io.vlx.demo.Inventory.GetSupplier", "io.vlx.demo.Inventory.RemoveItem":
		var p struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(in.Parameters, &p)
		it, ok := s.items[p.ID]
		if !ok {
			return nil, "io.vlx.demo.Inventory.NoSuchItem", map[string]int64{"id": p.ID}
		}
		switch in.Method {
		case "io.vlx.demo.Inventory.GetItem":
			return map[string]any{"item": it}, "", nil
		case "io.vlx.demo.Inventory.GetSupplier":
			slug := strings.ToLower(strings.ReplaceAll(it.supplier, " ", "-"))
			return map[string]any{"supplier": supplier{
				Name:     it.supplier,
				Email:    "orders@" + strings.Trim(slug, ".") + ".example",
				APIToken: fmt.Sprintf("demo-%x", len(it.supplier)*7919),
			}}, "", nil
		default:
			delete(s.items, p.ID)
			return struct{}{}, "", nil
		}
	case "io.vlx.demo.Inventory.AddItem":
		var p struct {
			Name       string            `json:"name"`
			Quantity   int64             `json:"quantity"`
			Tags       []string          `json:"tags"`
			Attributes map[string]string `json:"attributes"`
		}
		_ = json.Unmarshal(in.Parameters, &p)
		if p.Tags == nil {
			p.Tags = []string{}
		}
		if p.Attributes == nil {
			p.Attributes = map[string]string{}
		}
		it := item{ID: s.nextID, Name: p.Name, Quantity: p.Quantity, Tags: p.Tags, Attributes: p.Attributes, supplier: "You"}
		s.items[it.ID] = it
		s.nextID++
		return map[string]any{"item": it}, "", nil
	case "io.vlx.demo.Inventory.Reset":
		s.mu.Unlock()
		s.reset()
		s.mu.Lock()
		return struct{}{}, "", nil
	}
	return nil, "org.varlink.service.MethodNotImplemented", map[string]string{"method": in.Method}
}

func (s *Server) listItems(in call, send func(reply) bool) bool {
	var p struct {
		Tag *string `json:"tag"`
	}
	_ = json.Unmarshal(in.Parameters, &p)
	s.mu.Lock()
	var items []item
	for _, it := range s.items {
		if p.Tag == nil || contains(it.Tags, *p.Tag) {
			items = append(items, it)
		}
	}
	s.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) == 0 {
		return send(errorReply("io.vlx.demo.Inventory.NoSuchItem", map[string]int64{"id": 0}))
	}
	if !in.More {
		return send(reply{Parameters: map[string]any{"item": items[0]}})
	}
	for i, it := range items {
		if !send(reply{Parameters: map[string]any{"item": it}, Continues: i < len(items)-1}) {
			return false
		}
	}
	return true
}

func (s *Server) countdown(ctx context.Context, in call, send func(reply) bool) bool {
	var p struct {
		From     int64  `json:"from"`
		Interval *int64 `json:"intervalMilliseconds"`
	}
	_ = json.Unmarshal(in.Parameters, &p)
	if p.From < 0 || p.From > 100 {
		return send(errorReply("org.varlink.service.InvalidParameter", map[string]string{"parameter": "from"}))
	}
	interval := int64(500)
	if p.Interval != nil {
		if *p.Interval < 50 || *p.Interval > 5000 {
			return send(errorReply("org.varlink.service.InvalidParameter", map[string]string{"parameter": "intervalMilliseconds"}))
		}
		interval = *p.Interval
	}
	t := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer t.Stop()
	for n := p.From; n >= 0; n-- {
		if !send(reply{Parameters: map[string]int64{"remaining": n}, Continues: n > 0}) {
			return false
		}
		if n == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
