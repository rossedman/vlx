package safety

import "fmt"

// Policy decides whether a call of a given class may be sent.
type Policy struct {
	// AllowWrites enables Write and Danger calls (after confirmation).
	AllowWrites bool
}

// Confirm is the confirmation a permitted call needs.
type Confirm int

const (
	// ConfirmNone: send immediately.
	ConfirmNone Confirm = iota
	// ConfirmYes: ask yes/no.
	ConfirmYes
	// ConfirmTyped: ask the user to type the method name.
	ConfirmTyped
)

// Decision is the policy's answer for one call.
type Decision struct {
	Allowed bool
	Confirm Confirm
	// Why explains a refusal, ready to show to the user.
	Why string
}

// RefusedError is returned when the policy refuses a call.
type RefusedError struct {
	Method string
	Class  Class
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("%s is a %s method and vlx is read-only; rerun with --allow-writes to enable it", e.Method, e.Class)
}

// Decide applies the policy to a classified method.
func (p Policy) Decide(method string, v Verdict) Decision {
	switch {
	case v.Class == Read:
		return Decision{Allowed: true, Confirm: ConfirmNone}
	case !p.AllowWrites:
		return Decision{Why: (&RefusedError{method, v.Class}).Error()}
	case v.Class == Danger:
		return Decision{Allowed: true, Confirm: ConfirmTyped}
	default:
		return Decision{Allowed: true, Confirm: ConfirmYes}
	}
}
