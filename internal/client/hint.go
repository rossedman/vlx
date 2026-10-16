package client

import (
	"context"
	"errors"
	"os"
	"syscall"

	"github.com/rossedman/sdvarlink/varlink"
)

// Hint suggests what to do about err, or returns "".
func Hint(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, syscall.EACCES), errors.Is(err, os.ErrPermission), errors.Is(err, varlink.ErrPermissionDenied):
		if os.Geteuid() != 0 {
			return "This service only talks to privileged users. Try again with sudo, and keep vlx read-only unless you need writes."
		}
		return "Permission denied, even as root: the service's own policy (polkit or a socket mode) refused the call."
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, varlink.ErrUnsupported):
		return "Nothing is listening on this socket. The service may be stopped or socket-activated only by name; check 'systemctl status'."
	case errors.Is(err, syscall.ENOENT):
		return "The socket does not exist. Run 'vlx list' to see what is available."
	case errors.Is(err, context.DeadlineExceeded):
		return "The service did not answer in time. Raise --timeout, or check whether it is stuck."
	case errors.Is(err, varlink.ErrMethodNotFound), errors.Is(err, varlink.ErrInterfaceNotFound):
		return "The service does not implement this. Its systemd version may be older than the interface."
	case errors.Is(err, varlink.ErrInvalidParameter):
		return "The service rejected a parameter. Compare the parameters with the method signature."
	case errors.Is(err, varlink.ErrExpectedMore):
		return "This method streams: call it with 'more' (ctrl+t in the TUI, --more on the command line)."
	case errors.Is(err, varlink.ErrFrameTooLarge):
		return "The reply exceeded max_frame_size. Raise --max-frame-size if you trust this service."
	}
	return ""
}
