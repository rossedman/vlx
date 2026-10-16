package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/cmdline"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/schema"
)

type callFlags struct {
	file       string
	more       bool
	oneway     bool
	maxReplies int
	dryRun     bool
	yes        bool
	noValidate bool
}

func newCallCommand(a *app) *cobra.Command {
	var f callFlags
	cmd := &cobra.Command{
		Use:     "call SERVICE METHOD [PARAMS | NAME=VALUE...]",
		GroupID: "call",
		Short:   "Call a method and print the reply",
		Long: `Call METHOD on SERVICE. Give parameters as NAME=VALUE pairs (the explorer's
command-line syntax: strings are taken literally, other values are JSON) or as
one JSON object ('-' reads stdin, -f reads a file, and nothing means {}).
Parameters are checked against the method's IDL before anything is sent.

Safety: methods are classified READ, WRITE or DANGER. vlx only sends READ
methods unless you pass --allow-writes. WRITE calls then ask for confirmation
and DANGER calls ask you to type the method name; in scripts, --yes confirms.

Output: each reply is printed as JSON, pretty on a terminal and one compact
object per line otherwise (or with -o json). Streams (--more) print every
reply as it arrives. Exit status is 3 when the service returns an error,
4 when vlx refuses a call, 5 when the service cannot be found.`,
		Example: `  vlx call hostname io.systemd.Hostname.Describe
  vlx call resolve ResolveHostname name=example.com family=2
  vlx call resolve ResolveHostname '{"name":"example.com"}'
  echo '{"name":"example.com"}' | vlx call resolve ResolveHostname -
  vlx call multiplexer GetUserRecord '{"service":"io.systemd.Multiplexer"}' --more
  vlx call demo SetDefaultLanguage '{"language":"german"}' --allow-writes --yes`,
		Args:              usageArgs(cobra.MinimumNArgs(2)),
		ValidArgsFunction: completeService(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCall(cmd.Context(), a, f, args)
		},
	}
	fl := cmd.Flags()
	fl.StringVarP(&f.file, "file", "f", "", "read PARAMS from a file ('-' for stdin)")
	fl.BoolVar(&f.more, "more", false, "ask for a stream of replies (required by some methods)")
	fl.BoolVar(&f.oneway, "oneway", false, "send without waiting for a reply")
	fl.IntVar(&f.maxReplies, "max-replies", 0, "stop a stream after N replies (0: no limit; implies --more when supported)")
	fl.BoolVar(&f.dryRun, "dry-run", false, "validate and print the message without sending it")
	fl.BoolVarP(&f.yes, "yes", "y", false, "confirm WRITE and DANGER calls without prompting")
	fl.BoolVar(&f.noValidate, "no-validate", false, "skip introspection and parameter checks (needs a qualified METHOD; safety still applies)")
	cmd.MarkFlagsMutuallyExclusive("more", "oneway")
	cmd.MarkFlagsMutuallyExclusive("file", "dry-run")
	return cmd
}

// kvArgs returns the NAME=VALUE arguments after METHOD, or nil when the
// parameters are given as JSON (or not at all).
func kvArgs(args []string) ([]cmdline.Arg, error) {
	rest := args[2:]
	if len(rest) == 0 || len(rest) == 1 && !looksLikeArg(rest[0]) {
		return nil, nil
	}
	l, err := cmdline.Parse("x y " + quoteAll(rest))
	if err != nil {
		return nil, usageError(fmt.Errorf("parameters: %w (give one JSON object, or NAME=VALUE pairs)", err))
	}
	return l.Args, nil
}

func looksLikeArg(s string) bool {
	name, _, ok := strings.Cut(s, "=")
	return ok && name != "" && !strings.ContainsAny(name, "{[\" \t")
}

func quoteAll(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = cmdline.Quote(w)
	}
	return strings.Join(q, " ")
}

func readParams(a *app, f callFlags, args []string) ([]byte, error) {
	src := ""
	if len(args) == 3 {
		src = args[2]
	}
	if f.file != "" {
		if src != "" {
			return nil, usageError(errors.New("give PARAMS or --file, not both"))
		}
		if f.file == "-" {
			return io.ReadAll(io.LimitReader(a.in, 16<<20))
		}
		return os.ReadFile(f.file)
	}
	if src == "-" {
		return io.ReadAll(io.LimitReader(a.in, 16<<20))
	}
	return []byte(src), nil
}

func runCall(ctx context.Context, a *app, f callFlags, args []string) error {
	path, err := a.resolve(args[0])
	if err != nil {
		return err
	}
	kv, err := kvArgs(args)
	if err != nil {
		return err
	}
	var raw []byte
	switch {
	case kv != nil && f.file != "":
		return usageError(errors.New("give NAME=VALUE pairs or --file, not both"))
	case kv != nil && f.noValidate:
		return usageError(errors.New("NAME=VALUE needs the method's IDL to convert values; drop --no-validate or pass a JSON object"))
	case kv == nil && len(args) > 3:
		return usageError(errors.New("give one JSON object, or NAME=VALUE pairs"))
	case kv == nil:
		if raw, err = readParams(a, f, args); err != nil {
			return err
		}
	}
	method := args[1]
	more := f.more

	if !f.noValidate {
		svc, err := client.Introspect(ctx, path, a.clientOptions())
		if err != nil {
			return err
		}
		iface, m, err := svc.FindMethod(method)
		if err != nil {
			return &ExitError{Code: ExitNotFound, Err: err}
		}
		method = iface.Name + "." + m.Name
		if kv != nil {
			if raw, err = cmdline.Params(iface.Parsed, m.In, kv); err != nil {
				return usageError(fmt.Errorf("parameters for %s: %w", method, err))
			}
		}
		if err := schema.Validate(iface.Parsed, m.In, raw); err != nil {
			return usageError(fmt.Errorf("parameters for %s: %w\n(run 'vlx template %s %s' for a starting point)", method, err, args[0], method))
		}
		switch {
		case m.More == idl.MoreSupported && f.maxReplies > 0 && !f.oneway:
			more = true
		case m.More == idl.MoreRequired && !more && !f.oneway:
			if !f.dryRun {
				fmt.Fprintf(a.errOut, "note: %s streams; calling with --more\n", method)
			}
			more = true
		case m.More == idl.MoreNone && more:
			return usageError(fmt.Errorf("%s does not support --more", method))
		}
	} else if !strings.Contains(method, ".") {
		return usageError(errors.New("--no-validate needs a qualified method name, like io.systemd.Hostname.Describe"))
	}

	params, err := client.Compact(raw)
	if err != nil {
		return usageError(fmt.Errorf("parameters: %w", err))
	}

	if f.dryRun {
		msg := map[string]any{"method": method, "parameters": params}
		if more {
			msg["more"] = true
		}
		if f.oneway {
			msg["oneway"] = true
		}
		v := safety.NewClassifier(a.cfg.Safety).ClassifyCall(method, f.oneway)
		fmt.Fprintf(a.errOut, "dry run: %s is %s (%s); nothing was sent to %s\n", method, v.Class, v.Reason, path)
		return writeJSON(a.out, msg)
	}

	if err := a.checkSafety(method, path, string(params), f); err != nil {
		return err
	}

	ctx, stop := signalContext(ctx)
	defer stop()

	n := 0
	err = client.Call(ctx, path, client.Request{Method: method, Params: params, More: more, Oneway: f.oneway}, a.clientOptions(),
		func(r client.Reply) error {
			n++
			if r.Files > 0 {
				fmt.Fprintf(a.errOut, "note: reply %d carried %d file descriptor(s); vlx closed them\n", n, r.Files)
			}
			if err := a.printReply(r.Params); err != nil {
				return err
			}
			if f.maxReplies > 0 && n >= f.maxReplies {
				return client.ErrStop
			}
			return nil
		})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &ExitError{Code: ExitInterrupted, Err: errors.New("interrupted")}
		}
		if d, ok := client.AsVarlinkError(err); ok {
			if a.json() {
				_ = writeJSONCompact(a.out, d)
			}
			return &ExitError{Code: ExitVarlink, Err: fmt.Errorf("%s returned %s%s", method, d.Name, paramSuffix(d.Parameters))}
		}
		return err
	}
	if f.oneway {
		fmt.Fprintln(a.errOut, "sent (oneway: no reply)")
	}
	return nil
}

func paramSuffix(p []byte) string {
	if len(p) == 0 || string(p) == "{}" {
		return ""
	}
	return " " + string(p)
}

func (a *app) printReply(params []byte) error {
	if a.json() || !a.outIsTTY() {
		_, err := fmt.Fprintln(a.out, compactLine(params))
		return err
	}
	_, err := fmt.Fprintln(a.out, render.JSON(params, render.JSONOptions{Color: a.color()}))
	return err
}

func compactLine(p []byte) string {
	c, err := client.Compact(p)
	if err != nil {
		return string(p)
	}
	return string(c)
}

func writeJSONCompact(w io.Writer, v any) error {
	enc := newCompactEncoder(w)
	return enc.Encode(v)
}

// checkSafety applies the policy, prompting on a terminal when needed.
func (a *app) checkSafety(method, path, params string, f callFlags) error {
	v := safety.NewClassifier(a.cfg.Safety).ClassifyCall(method, f.oneway)
	d := safety.Policy{AllowWrites: a.cfg.AllowWrites}.Decide(method, v)
	if !d.Allowed {
		return &ExitError{Code: ExitRefused, Err: &safety.RefusedError{Method: method, Class: v.Class}}
	}
	if d.Confirm == safety.ConfirmNone || f.yes {
		return nil
	}
	if !a.inIsTTY() || !a.errIsTTY() {
		return &ExitError{Code: ExitRefused, Err: fmt.Errorf("%s is a %s method; confirm with --yes when not running interactively", method, v.Class)}
	}
	fmt.Fprintf(a.errOut, "%s %s call\n  method  %s\n  socket  %s\n  params  %s\n  reason  %s\n",
		classLabel(v.Class, a.color()), "", method, path, truncate(params, 200), v.Reason)
	if os.Geteuid() == 0 {
		fmt.Fprintln(a.errOut, "  as      root")
	}
	r := bufio.NewReader(a.in)
	short := method[strings.LastIndexByte(method, '.')+1:]
	if d.Confirm == safety.ConfirmTyped {
		fmt.Fprintf(a.errOut, "This is a DANGER method. Type %s to send it: ", short)
	} else {
		fmt.Fprint(a.errOut, "Send it? [y/N] ")
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	ok := d.Confirm == safety.ConfirmTyped && line == short ||
		d.Confirm == safety.ConfirmYes && (line == "y" || line == "Y" || line == "yes")
	if !ok {
		return &ExitError{Code: ExitRefused, Err: errors.New("not confirmed; nothing was sent")}
	}
	return nil
}

func classLabel(c safety.Class, color bool) string {
	if !color {
		return c.String()
	}
	switch c {
	case safety.Danger:
		return errStyle.Render(c.String())
	default:
		return warnStyle.Bold(true).Render(c.String())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
