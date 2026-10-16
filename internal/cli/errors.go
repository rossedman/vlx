package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/safety"
)

// Exit codes, documented in docs/reference/exit-codes.md.
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitVarlink     = 3
	ExitRefused     = 4
	ExitNotFound    = 5
	ExitInterrupted = 130
)

// ExitError carries an exit code with an error.
type ExitError struct {
	Code int
	Err  error
	// Silent means the error was already reported.
	Silent bool
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

func usageError(err error) error {
	return &ExitError{Code: ExitUsage, Err: err}
}

// ExitCode maps any error returned by a command to a process exit code.
func ExitCode(err error) int {
	var ee *ExitError
	var refused *safety.RefusedError
	switch {
	case err == nil:
		return ExitOK
	case errors.As(err, &ee):
		return ee.Code
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	case errors.As(err, &refused):
		return ExitRefused
	case errors.Is(err, discovery.ErrNotFound), errors.Is(err, varlink.ErrUnsupported):
		return ExitNotFound
	}
	if _, ok := client.AsVarlinkError(err); ok {
		return ExitVarlink
	}
	return ExitFailure
}

// IsSilent reports whether err was already printed.
func IsSilent(err error) bool {
	var ee *ExitError
	return errors.As(err, &ee) && ee.Silent
}

// Hint returns advice for err, for main to print after the error.
func Hint(err error) string {
	return client.Hint(err)
}
