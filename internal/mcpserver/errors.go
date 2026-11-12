package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"syscall"

	"github.com/rossedman/sdvarlink/idl"
	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/sanitize"
	"github.com/rossedman/vlx/internal/schema"
)

// Error kinds, the error.kind field of a failed tool result.
const (
	// KindVarlink: the service replied with a Varlink error.
	KindVarlink = "varlink"
	// KindErrno: the service replied with io.systemd.System and an errno.
	KindErrno = "errno"
	// KindRefused: the safety policy refused the call. Retrying will not help.
	KindRefused = "refused"
	// KindInvalidParams: the parameters do not match the method's IDL.
	KindInvalidParams = "invalid_params"
	// KindNotFound: the service, interface or method does not exist, or a
	// name is ambiguous.
	KindNotFound = "not_found"
	// KindAccess: the user may not use the socket.
	KindAccess = "access"
	// KindLimit: a timeout or size limit was reached.
	KindLimit = "limit"
	// KindUnavailable: the service could not be reached for another reason.
	KindUnavailable = "unavailable"
)

// ToolError is a failure reported to the model as a tool result with
// isError set, so it can read it and recover. Its Error method returns the
// JSON the model sees.
type ToolError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	// Hint is advice on what to do next.
	Hint string `json:"hint,omitempty"`
	// Retry says whether trying again (with changes) can succeed.
	Retry bool `json:"retry"`
	// Candidates are the services an ambiguous name matched.
	Candidates []string `json:"candidates,omitempty"`
	// Params lists every parameter problem, by JSON path.
	Params []ParamError `json:"params,omitempty"`
	// Class is the method's safety class, for refusals.
	Class string `json:"class,omitempty"`
	// Varlink holds the error reply for KindVarlink and KindErrno.
	Varlink *VarlinkError `json:"varlink,omitempty"`
	// Notice marks text that came from the service.
	Notice string `json:"notice,omitempty"`
}

// ParamError is one parameter problem.
type ParamError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// VarlinkError describes an error reply from a service.
type VarlinkError struct {
	Name       string `json:"name"`
	Parameters any    `json:"parameters,omitempty"`
	// Doc is the error's doc comment from the interface, when it declares
	// the error.
	Doc string `json:"doc,omitempty"`
	// Errno and ErrnoName are set for io.systemd.System errors.
	Errno     int    `json:"errno,omitempty"`
	ErrnoName string `json:"errnoName,omitempty"`
}

// Error returns {"error": {...}}, the text of the failed tool result.
func (e *ToolError) Error() string {
	b, err := json.Marshal(map[string]*ToolError{"error": e})
	if err != nil {
		return e.Kind + ": " + e.Message
	}
	return string(b)
}

// notFound reports a service that could not be resolved, listing the
// candidates when the name was ambiguous.
func notFound(service string, err error) *ToolError {
	te := &ToolError{Kind: KindNotFound, Message: sanitize.Line(err.Error()), Retry: true,
		Hint: "Call list_services to see what this host has."}
	var amb *discovery.AmbiguousError
	if errors.As(err, &amb) {
		te.Message = "service name " + sanitize.Line(service) + " is ambiguous"
		te.Hint = "Use one of the candidates' full names."
		for _, c := range amb.Candidates {
			te.Candidates = append(te.Candidates, sanitize.Line(c.Name))
		}
	}
	return te
}

// methodNotFound reports a method or interface that FindMethod could not
// resolve.
func methodNotFound(err error) *ToolError {
	return &ToolError{Kind: KindNotFound, Message: sanitize.Line(err.Error()), Retry: true,
		Hint: "Call describe on the service to see its methods."}
}

// invalidParams reports parameters that failed schema.Validate, listing
// every problem so the model can fix them in one retry.
func invalidParams(method string, err error) *ToolError {
	te := &ToolError{Kind: KindInvalidParams, Message: "parameters for " + method + " do not match its IDL", Retry: true,
		Hint: "Call method_schema for the exact input schema and an example."}
	var errs schema.Errors
	var one *schema.Error
	switch {
	case errors.As(err, &errs):
		for _, e := range errs {
			te.Params = append(te.Params, ParamError{Path: e.Path, Message: e.Msg})
		}
	case errors.As(err, &one):
		te.Params = []ParamError{{Path: one.Path, Message: one.Msg}}
	default:
		te.Params = []ParamError{{Message: err.Error()}}
	}
	return te
}

// callError turns an error from introspecting or calling a service into a
// ToolError, attaching the error's IDL doc comment when iface declares it.
func (s *Server) callError(err error, iface *idl.Interface) *ToolError {
	var te *ToolError
	if errors.As(err, &te) {
		return te
	}
	hint := client.Hint(err)
	var se *varlink.SystemError
	var ve *varlink.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &ToolError{Kind: KindLimit, Message: "the service did not answer in time", Hint: hint, Retry: true}
	case errors.Is(err, varlink.ErrFrameTooLarge):
		return &ToolError{Kind: KindLimit, Message: "the reply was larger than max_frame_size", Hint: hint}
	case errors.As(err, &se):
		return &ToolError{Kind: KindErrno, Message: sanitize.Line(se.Error()), Hint: hint, Notice: Notice,
			Varlink: &VarlinkError{Name: se.VarlinkError(), Errno: se.Errno, ErrnoName: sanitize.Line(se.ErrnoName)}}
	case errors.Is(err, varlink.ErrPermissionDenied), errors.Is(err, syscall.EACCES), errors.Is(err, os.ErrPermission):
		return &ToolError{Kind: KindAccess, Message: "permission denied", Hint: hint}
	case errors.As(err, &ve):
		d := &VarlinkError{Name: sanitize.Line(ve.Name), Parameters: s.cleanValue(ve.Parameters)}
		if iface != nil {
			if _, short, ok := cutLast(ve.Name); ok {
				if e := iface.Error(short); e != nil && iface.Name+"."+short == ve.Name {
					d.Doc = sanitize.Text(e.Doc)
				}
			}
		}
		return &ToolError{Kind: KindVarlink, Message: "the service returned " + d.Name, Hint: hint, Retry: true, Varlink: d, Notice: Notice}
	}
	return &ToolError{Kind: KindUnavailable, Message: sanitize.Line(err.Error()), Hint: hint}
}

// cutLast splits a qualified name at its last dot.
func cutLast(name string) (prefix, last string, ok bool) {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[:i], name[i+1:], true
		}
	}
	return "", name, false
}
