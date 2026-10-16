package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// signalContext cancels ctx on SIGINT or SIGTERM, so streams end cleanly.
func signalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}

func newCompactEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
