package tui

import "github.com/charmbracelet/bubbles/cursor"

// noBlink turns cursor blinking off. Tests set it so that blink timers do
// not slow down the synchronous test harness.
var noBlink bool

// staticCursorInTests stops c from blinking when noBlink is set.
func staticCursorInTests(c *cursor.Model) {
	if noBlink {
		c.SetMode(cursor.CursorStatic)
	}
}
