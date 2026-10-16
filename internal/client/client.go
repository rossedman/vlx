// Package client wraps sdvarlink for vlx: it introspects a service into a
// browsable model and runs calls with vlx's limits applied.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rossedman/sdvarlink/idl"
	"github.com/rossedman/sdvarlink/varlink"
)

// Options are the limits applied to every connection.
type Options struct {
	// Timeout bounds dialing, introspection and single-reply calls. Streams
	// are bounded only by their context.
	Timeout time.Duration
	// MaxFrameSize is the largest message accepted from the service.
	MaxFrameSize int
}

func (o Options) dialOpts() []varlink.Option {
	var opts []varlink.Option
	if o.MaxFrameSize > 0 {
		opts = append(opts, varlink.WithMaxFrameSize(o.MaxFrameSize))
	}
	return opts
}

func (o Options) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 10 * time.Second
	}
	return o.Timeout
}

// Service is the introspected model of one socket.
type Service struct {
	Path       string
	Info       varlink.Info
	Interfaces []*Interface
}

// Interface is one interface a service implements.
type Interface struct {
	Name string
	// Description is the IDL text as returned by the service.
	Description string
	// Parsed is nil if the description could not be fetched or parsed;
	// Err says why.
	Parsed *idl.Interface
	Err    error
}

// Introspect connects to the socket at path, asks for its info and every
// interface description, and parses them. A broken interface does not fail
// the whole service: its Err is set instead.
func Introspect(ctx context.Context, path string, opts Options) (*Service, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout())
	defer cancel()
	conn, err := varlink.Dial(ctx, path, opts.dialOpts()...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	info, err := varlink.GetInfo(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("GetInfo: %w", err)
	}
	svc := &Service{Path: path, Info: *info}
	names := append([]string(nil), info.Interfaces...)
	sort.Strings(names)
	for _, name := range names {
		iface := &Interface{Name: name}
		desc, err := varlink.GetInterfaceDescription(ctx, conn, name)
		if err != nil {
			iface.Err = err
		} else {
			iface.Description = desc
			iface.Parsed, iface.Err = idl.Parse(desc)
		}
		svc.Interfaces = append(svc.Interfaces, iface)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return svc, nil
}

// Info asks the socket at path for its GetInfo reply only.
func Info(ctx context.Context, path string, opts Options) (*varlink.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout())
	defer cancel()
	conn, err := varlink.Dial(ctx, path, opts.dialOpts()...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return varlink.GetInfo(ctx, conn)
}

// Interface returns the named interface, or nil.
func (s *Service) Interface(name string) *Interface {
	for _, i := range s.Interfaces {
		if i.Name == name {
			return i
		}
	}
	return nil
}

// FindMethod resolves a method name, qualified ("io.systemd.Hostname.Describe")
// or bare ("Describe") when exactly one interface defines it.
func (s *Service) FindMethod(name string) (*Interface, *idl.Method, error) {
	if strings.Contains(name, ".") {
		ifaceName, method, err := varlink.SplitMethod(name)
		if err != nil {
			return nil, nil, err
		}
		iface := s.Interface(ifaceName)
		if iface == nil {
			return nil, nil, fmt.Errorf("%s does not implement interface %s", s.Path, ifaceName)
		}
		if iface.Parsed == nil {
			return nil, nil, fmt.Errorf("interface %s: %w", ifaceName, iface.Err)
		}
		m := iface.Parsed.Method(method)
		if m == nil {
			return nil, nil, fmt.Errorf("interface %s has no method %s", ifaceName, method)
		}
		return iface, m, nil
	}
	var (
		found  *Interface
		method *idl.Method
		where  []string
	)
	for _, iface := range s.Interfaces {
		if iface.Parsed == nil {
			continue
		}
		if m := iface.Parsed.Method(name); m != nil {
			found, method = iface, m
			where = append(where, iface.Name+"."+name)
		}
	}
	switch len(where) {
	case 0:
		return nil, nil, fmt.Errorf("no interface on %s has a method %s", s.Path, name)
	case 1:
		return found, method, nil
	default:
		return nil, nil, fmt.Errorf("method %s is ambiguous, use one of: %s", name, strings.Join(where, ", "))
	}
}

// Request is one call to make.
type Request struct {
	Method string
	Params json.RawMessage
	More   bool
	Oneway bool
}

// Reply is one reply received.
type Reply struct {
	Params    json.RawMessage
	Continues bool
	// Files is the number of file descriptors that came with the reply.
	// vlx closes them immediately.
	Files   int
	Elapsed time.Duration
}

// ErrStop can be returned by a reply callback to end a stream early.
var ErrStop = errors.New("stop")

// Compact returns params as compact JSON, or {} when empty.
func Compact(params []byte) (json.RawMessage, error) {
	params = bytes.TrimSpace(params)
	if len(params) == 0 {
		return json.RawMessage("{}"), nil
	}
	var b bytes.Buffer
	if err := json.Compact(&b, params); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Call dials the socket at path, sends req and hands every reply to fn.
// Single-reply calls are bounded by opts.Timeout; streams run until they
// end, ctx is cancelled or fn returns an error (ErrStop ends quietly).
// Varlink error replies are returned as *varlink.Error or
// *varlink.SystemError.
func Call(ctx context.Context, path string, req Request, opts Options, fn func(Reply) error) error {
	params, err := Compact(req.Params)
	if err != nil {
		return fmt.Errorf("parameters: %w", err)
	}
	dialCtx, cancelDial := context.WithTimeout(ctx, opts.timeout())
	conn, err := varlink.Dial(dialCtx, path, opts.dialOpts()...)
	cancelDial()
	if err != nil {
		return err
	}
	defer conn.Close()

	start := time.Now()
	switch {
	case req.Oneway:
		ctx, cancel := context.WithTimeout(ctx, opts.timeout())
		defer cancel()
		return conn.Oneway(ctx, req.Method, params)
	case req.More:
		for r, err := range conn.Stream(ctx, req.Method, params) {
			if err != nil {
				return err
			}
			rep := toReply(r, start)
			if err := fn(rep); err != nil {
				if errors.Is(err, ErrStop) {
					return nil
				}
				return err
			}
		}
		return nil
	default:
		ctx, cancel := context.WithTimeout(ctx, opts.timeout())
		defer cancel()
		r, err := conn.CallReply(ctx, req.Method, params)
		if err != nil {
			return err
		}
		err = fn(toReply(r, start))
		if errors.Is(err, ErrStop) {
			return nil
		}
		return err
	}
}

func toReply(r *varlink.Reply, start time.Time) Reply {
	rep := Reply{
		Params:    append(json.RawMessage(nil), r.Parameters...),
		Continues: r.Continues,
		Files:     len(r.Files),
		Elapsed:   time.Since(start),
	}
	r.Close() // vlx never keeps received descriptors
	if len(rep.Params) == 0 {
		rep.Params = json.RawMessage("{}")
	}
	return rep
}

// ErrorDetail describes a Varlink error reply for display.
type ErrorDetail struct {
	Name       string          `json:"error"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// AsVarlinkError extracts the error name and parameters from err, if it is
// a Varlink error reply.
func AsVarlinkError(err error) (ErrorDetail, bool) {
	var ve *varlink.Error
	if errors.As(err, &ve) {
		return ErrorDetail{Name: ve.Name, Parameters: ve.Parameters}, true
	}
	var se *varlink.SystemError
	if errors.As(err, &se) {
		params, _ := json.Marshal(se)
		return ErrorDetail{Name: se.VarlinkError(), Parameters: params}, true
	}
	return ErrorDetail{}, false
}
