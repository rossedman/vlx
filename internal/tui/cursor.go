package tui

import "github.com/charmbracelet/bubbles/cursor"

// noBlink turns cursor blinking off. Tests set it so that blink timers do
// not slow down the synchronous test harness.
var noBlink bool

func staticCursorInTests(c *cursor.Model) {
	if noBlink {
		c.SetMode(cursor.CursorStatic)
	}
}
