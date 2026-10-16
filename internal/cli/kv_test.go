package cli

import (
	"strings"
	"testing"
)

func TestCallNameValue(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "GetGreeting", "name=Ada Lovelace", "language=german")
	if r.code != 0 || !strings.Contains(r.out, "Hallo, Ada Lovelace!") {
		t.Fatalf("%+v", r)
	}
	r = run(t, "", "call", sock, "GetItem", "id=1")
	if r.code != 0 || !strings.Contains(r.out, `"id":1`) {
		t.Fatalf("int: %+v", r)
	}
	r = run(t, "", "call", sock, "--dry-run", "WatchCountdown", "from=2", "intervalMilliseconds=50")
	if r.code != 0 || !strings.Contains(r.out, `"intervalMilliseconds": 50`) {
		t.Fatalf("dry run: %+v", r)
	}
	for _, tc := range []struct {
		args []string
		msg  string
	}{
		{[]string{"GetItem", "id=x"}, "not a valid int"},
		{[]string{"GetGreeting", "nam=Ada"}, `unknown parameter "nam" (parameters: name, language)`},
		{[]string{"GetGreeting", "Name=Ada"}, `did you mean "name"?`},
		{[]string{"ListLanguages", "x=1"}, "takes no parameters"},
		{[]string{"GetGreeting", "name=a", "name=b"}, "given twice"},
		{[]string{"GetGreeting", "name=a", "stray"}, "parameters"},
		{[]string{"GetGreeting", "name=a", "-f", "/dev/null"}, "NAME=VALUE"},
		{[]string{"io.vlx.demo.Greeter.GetGreeting", "name=a", "--no-validate"}, "NAME=VALUE"},
		{[]string{"GetGreeting", "{}", "{}"}, ""},
	} {
		r := run(t, "", append([]string{"call", sock}, tc.args...)...)
		if r.code != ExitUsage || !strings.Contains(r.err, tc.msg) {
			t.Errorf("%v: %+v", tc.args, r)
		}
	}
}
