package cmdline

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rossedman/sdvarlink/idl"
)

func TestSplit(t *testing.T) {
	tests := map[string][]string{
		``:                     nil,
		`  a  b `:              {"a", "b"},
		`a 'b c' "d e"`:        {"a", "b c", "d e"},
		`name='it'\''s'`:       {"name=it's"},
		`x="a \"q\" \\ b"`:     {`x=a "q" \ b`},
		`a\ b`:                 {"a b"},
		`k=''`:                 {"k="},
		`json='{"a": [1, 2]}'`: {`json={"a": [1, 2]}`},
	}
	for in, want := range tests {
		got, err := Split(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Split(`a 'b`); !errors.Is(err, ErrUnterminated) {
		t.Errorf("unterminated: %v", err)
	}
}

func TestParse(t *testing.T) {
	l, err := Parse(`resolve ResolveHostname name=github.com family=2 `)
	if err != nil {
		t.Fatal(err)
	}
	want := Line{Service: "resolve", Method: "ResolveHostname", Args: []Arg{{"name", "github.com"}, {"family", "2"}}, Trailing: true}
	if !reflect.DeepEqual(l, want) {
		t.Errorf("got %+v", l)
	}
	if l, _ := Parse("res"); l.Service != "res" || l.Method != "" || l.Trailing {
		t.Errorf("got %+v", l)
	}
	// '=' inside a value, and a word that only looks like an arg.
	if l, _ := Parse(`svc M expr=a=b`); l.Args[0].Value != "a=b" {
		t.Errorf("got %+v", l)
	}
	if l, _ := Parse(`unix:/run/x=y M`); l.Service != "unix:/run/x=y" {
		t.Errorf("non-name before '=' should be a word: %+v", l)
	}
	for _, bad := range []string{`svc M a=1 stray`, `svc M extra`, `svc 'x`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) succeeded", bad)
		}
	}
}

func TestFormatRoundTrip(t *testing.T) {
	args := []Arg{{"name", "it's here"}, {"n", "1"}, {"e", ""}, {"j", `{"a":1}`}}
	line := Format("resolve", "ResolveHostname", args)
	if line != `resolve ResolveHostname name='it'\''s here' n=1 e='' j='{"a":1}'` {
		t.Errorf("Format = %s", line)
	}
	l, err := Parse(line)
	if err != nil || !reflect.DeepEqual(l.Args, args) {
		t.Errorf("round trip: %+v %v", l.Args, err)
	}
	if Format("", "", nil) != "" {
		t.Error("empty format")
	}
}

const testIDL = `
interface com.example.T
type Color (red, green)
type P (x: int)
method M(name: string, n: ?int, ok: ?bool, color: ?Color, p: ?P, tags: ?[]string, alias: ?Alias) -> ()
type Alias (s: string)
method None() -> ()
`

func TestParamsAndArgs(t *testing.T) {
	iface, err := idl.Parse(testIDL)
	if err != nil {
		t.Fatal(err)
	}
	in := iface.Method("M").In
	got, err := Params(iface, in, []Arg{{"name", "123"}, {"n", "5"}, {"ok", "true"}, {"color", "green"}, {"p", `{"x":1}`}, {"tags", `["a"]`}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"name":"123","n":5,"ok":true,"color":"green","p":{"x":1},"tags":["a"]}` {
		t.Errorf("Params = %s", got)
	}
	back, err := Args(iface, in, got)
	if err != nil {
		t.Fatal(err)
	}
	want := []Arg{{"name", "123"}, {"n", "5"}, {"ok", "true"}, {"color", "green"}, {"p", `{"x":1}`}, {"tags", `["a"]`}}
	if !reflect.DeepEqual(back, want) {
		t.Errorf("Args = %+v", back)
	}
	for _, tc := range []struct {
		args []Arg
		want string
	}{
		{[]Arg{{"Name", "x"}}, `did you mean "name"`},
		{[]Arg{{"zzz", "x"}}, "parameters: name, n"},
		{[]Arg{{"n", "five"}}, `n: "five" is not a valid int`},
		{[]Arg{{"name", "a"}, {"name", "b"}}, "given twice"},
	} {
		if _, err := Params(iface, in, tc.args); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Params(%v) err = %v, want %q", tc.args, err, tc.want)
		}
	}
	if _, err := Params(iface, iface.Method("None").In, []Arg{{"x", "1"}}); err == nil || !strings.Contains(err.Error(), "no parameters") {
		t.Errorf("no-parameter method: %v", err)
	}
	if got, _ := Params(nil, nil, nil); string(got) != "{}" {
		t.Errorf("empty = %s", got)
	}
	if a, err := Args(iface, in, nil); a != nil || err != nil {
		t.Error("Args(nil)")
	}
	if _, err := Args(iface, in, []byte("[")); err == nil {
		t.Error("Args invalid JSON")
	}
	if a, _ := Args(iface, in, []byte(`{"name":null,"n":2}`)); len(a) != 1 || a[0].Name != "n" {
		t.Errorf("null skipped: %+v", a)
	}
	if compact([]byte("{bad")) != "{bad" {
		t.Error("compact invalid")
	}
}
