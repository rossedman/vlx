// Package safety decides which Varlink methods vlx may send, and how much
// confirmation each one needs.
//
// Every method gets a Class: Read, Write or Danger. The class comes from user
// overrides, then a built-in table of systemd methods, then the method's
// verb; anything still unknown is Write, so an unfamiliar method is never
// assumed to be harmless. A Policy then turns a class into a Decision.
package safety

import (
	"fmt"
	"path"
	"strings"
)

// Class is how risky a method is to call.
type Class int

const (
	// Read methods have no side effects.
	Read Class = iota
	// Write methods change state. Unknown methods are Write.
	Write
	// Danger methods are disruptive, hard to undo or security-sensitive.
	Danger
)

// String returns the label shown in the UI: READ, WRITE or DANGER.
func (c Class) String() string {
	switch c {
	case Read:
		return "READ"
	case Write:
		return "WRITE"
	default:
		return "DANGER"
	}
}

// Verdict is a classification with the reason for it.
type Verdict struct {
	Class  Class
	Reason string
}

// Overrides are user-configured glob patterns over qualified method names
// ("io.systemd.Unit.*"), matched with path.Match semantics where '.' is an
// ordinary character. The most severe matching class wins.
type Overrides struct {
	Read   []string `mapstructure:"read"`
	Write  []string `mapstructure:"write"`
	Danger []string `mapstructure:"danger"`
}

// Classifier assigns classes to methods.
type Classifier struct {
	overrides Overrides
}

// NewClassifier returns a classifier that applies o before the built-in
// rules.
func NewClassifier(o Overrides) *Classifier {
	return &Classifier{overrides: o}
}

// Validate reports malformed override patterns.
func (o Overrides) Validate() error {
	for _, group := range [][]string{o.Read, o.Write, o.Danger} {
		for _, p := range group {
			if _, err := path.Match(p, ""); err != nil {
				return fmt.Errorf("safety pattern %q: %w", p, err)
			}
		}
	}
	return nil
}

// Classify returns the class of a qualified method name such as
// "io.systemd.Hostname.Describe".
func (c *Classifier) Classify(qualified string) Verdict {
	if v, ok := c.override(qualified); ok {
		return v
	}
	if cls, ok := builtin[qualified]; ok {
		return Verdict{cls, "built-in rule for " + qualified}
	}
	return heuristic(qualified)
}

// ClassifyCall is Classify for a call with the given flags: a oneway call
// is at least Write, since nothing about it can be observed.
func (c *Classifier) ClassifyCall(qualified string, oneway bool) Verdict {
	v := c.Classify(qualified)
	if oneway && v.Class == Read {
		return Verdict{Write, "oneway calls are treated as writes"}
	}
	return v
}

// override returns the class from the first matching config pattern,
// checking Danger, then Write, then Read.
func (c *Classifier) override(qualified string) (Verdict, bool) {
	groups := []struct {
		class    Class
		patterns []string
	}{
		{Danger, c.overrides.Danger},
		{Write, c.overrides.Write},
		{Read, c.overrides.Read},
	}
	for _, g := range groups {
		for _, p := range g.patterns {
			if ok, _ := path.Match(p, qualified); ok {
				return Verdict{g.class, "config rule " + p}, true
			}
		}
	}
	return Verdict{}, false
}

// Verbs that make a method Danger or Read when it starts with them. Verbs
// match whole words in CamelCase: "Is" matches "IsActive", not "Issue".
var (
	dangerVerbs = []string{
		"Kill", "Terminate", "Remove", "Delete", "Destroy", "Wipe", "Erase",
		"Format", "Reset", "FactoryReset", "Reboot", "PowerOff", "Poweroff",
		"Halt", "Shutdown", "Reexecute", "Purge", "Revoke", "Exit", "Abort",
	}
	readVerbs = []string{
		"Get", "List", "Describe", "Resolve", "Query", "Lookup", "Browse",
		"Inspect", "Introspect", "Monitor", "Subscribe", "Watch", "Dump",
		"Can", "Is", "Has", "Ping", "Read", "Show", "Status", "Find", "Search",
		"Peek", "Count", "Check", "Validate", "Echo",
	}
)

// heuristic classifies by the method's leading verb: danger verbs first,
// then read verbs, else Write.
func heuristic(qualified string) Verdict {
	method := qualified
	if i := strings.LastIndexByte(qualified, '.'); i >= 0 {
		method = qualified[i+1:]
	}
	for _, v := range dangerVerbs {
		if hasVerb(method, v) {
			return Verdict{Danger, fmt.Sprintf("name starts with %q", v)}
		}
	}
	for _, v := range readVerbs {
		if hasVerb(method, v) {
			return Verdict{Read, fmt.Sprintf("name starts with %q", v)}
		}
	}
	return Verdict{Write, "unknown methods are treated as writes"}
}

// hasVerb reports whether method is verb or starts with verb followed by an
// upper-case letter, digit or underscore.
func hasVerb(method, verb string) bool {
	if !strings.HasPrefix(method, verb) {
		return false
	}
	if len(method) == len(verb) {
		return true
	}
	next := method[len(verb)]
	return next >= 'A' && next <= 'Z' || next >= '0' && next <= '9' || next == '_'
}
