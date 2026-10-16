package safety

import (
	"testing"

	"github.com/rossedman/sdvarlink/idl"
)

func TestBuiltinCoversSystemd(t *testing.T) {
	seen := 0
	for _, desc := range systemdIDL {
		iface, err := idl.Parse(desc)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range iface.Methods {
			name := iface.Name + "." + m.Name
			seen++
			if _, ok := builtin[name]; !ok {
				t.Errorf("no built-in safety rule for %s (heuristic says %v)", name, heuristic(name).Class)
			}
		}
	}
	if seen < 100 {
		t.Fatalf("only %d systemd methods found; is systemd_idl_test.go stale?", seen)
	}
}

func TestClassify(t *testing.T) {
	c := NewClassifier(Overrides{
		Read:   []string{"com.example.Thing.Peek*"},
		Danger: []string{"com.example.*.Explode"},
	})
	tests := []struct {
		method string
		want   Class
	}{
		{"io.systemd.Hostname.Describe", Read},
		{"io.systemd.Manager.Reexecute", Danger},
		{"io.systemd.Unit.SetProperties", Write},
		{"org.varlink.service.GetInfo", Read},
		// Heuristics.
		{"com.example.Thing.GetWidget", Read},
		{"com.example.Thing.ListAll", Read},
		{"com.example.Thing.List", Read},
		{"com.example.Thing.IsReady", Read},
		{"com.example.Thing.Issue", Write}, // "Is" must be a whole word
		{"com.example.Thing.Getaway", Write},
		{"com.example.Thing.KillAll", Danger},
		{"com.example.Thing.Resetting", Write},
		{"com.example.Thing.Frobnicate", Write},
		{"Bare", Write},
		// Overrides.
		{"com.example.Thing.PeekQueue", Read},
		{"com.example.Thing.Explode", Danger},
		{"com.example.Other.Explode", Danger},
	}
	for _, tt := range tests {
		if got := c.Classify(tt.method); got.Class != tt.want {
			t.Errorf("Classify(%s) = %v (%s), want %v", tt.method, got.Class, got.Reason, tt.want)
		}
	}
}

func TestOverridesBeatBuiltin(t *testing.T) {
	c := NewClassifier(Overrides{Danger: []string{"io.systemd.Hostname.*"}})
	if got := c.Classify("io.systemd.Hostname.Describe"); got.Class != Danger {
		t.Fatalf("got %v, want DANGER", got.Class)
	}
}

func TestOnewayIsWrite(t *testing.T) {
	c := NewClassifier(Overrides{})
	if got := c.ClassifyCall("io.systemd.service.Ping", true); got.Class != Write {
		t.Fatalf("oneway Ping = %v, want WRITE", got.Class)
	}
	if got := c.ClassifyCall("io.systemd.Machine.Kill", true); got.Class != Danger {
		t.Fatalf("oneway Kill = %v, want DANGER", got.Class)
	}
}

func TestPolicy(t *testing.T) {
	ro := Policy{}
	rw := Policy{AllowWrites: true}
	for _, tt := range []struct {
		p       Policy
		class   Class
		allowed bool
		confirm Confirm
	}{
		{ro, Read, true, ConfirmNone},
		{ro, Write, false, ConfirmNone},
		{ro, Danger, false, ConfirmNone},
		{rw, Read, true, ConfirmNone},
		{rw, Write, true, ConfirmYes},
		{rw, Danger, true, ConfirmTyped},
	} {
		d := tt.p.Decide("x.Y", Verdict{Class: tt.class})
		if d.Allowed != tt.allowed || d.Confirm != tt.confirm {
			t.Errorf("%+v %v: got %+v", tt.p, tt.class, d)
		}
		if !d.Allowed && d.Why == "" {
			t.Errorf("refusal without reason")
		}
	}
}

func TestValidateOverrides(t *testing.T) {
	if err := (Overrides{Read: []string{"["}}).Validate(); err == nil {
		t.Fatal("expected error for bad pattern")
	}
}
