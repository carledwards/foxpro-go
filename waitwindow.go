package foxpro

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

// WaitMode controls how a WaitWindow is dismissed.
//
// The zero value is WaitTimeout, the most forgiving mode — dismisses
// on input *or* after a timer, so a misconfigured one never sticks.
type WaitMode int

const (
	// WaitTimeout dismisses on a key press, a mouse press, OR after
	// the configured Timeout has elapsed. Default mode.
	WaitTimeout WaitMode = iota
	// WaitNoWait dismisses on the next key press OR mouse press.
	// No automatic timeout.
	WaitNoWait
	// WaitForKey dismisses only on a key press. Mouse is ignored.
	WaitForKey
)

const (
	waitDefaultTimeout = 3 * time.Second
	waitShadowOffsetX  = 2
	waitShadowOffsetY  = 1
)

// WaitWindow is a transient toast-style notification — a small
// bordered box with a one-line message and a drop shadow, anchored to
// the upper-right of the screen. It dismisses on user input and/or a
// timer, depending on Mode.
//
// FoxPro for DOS exposed the same idea via WAIT WINDOW with NOWAIT and
// TIMEOUT clauses; this is the Go-on-tcell equivalent.
//
// Show one with App.ShowWaitWindow. Only one wait window is active at
// a time — calling Show again replaces any in-flight one without
// firing its OnDismiss callback.
type WaitWindow struct {
	// Message is the single-line text shown inside the box. It will be
	// padded with one space on each side.
	Message string

	// Mode controls dismissal. Default WaitTimeout.
	Mode WaitMode

	// Timeout is how long until automatic dismissal in WaitTimeout mode.
	// Zero means use the default of 3 seconds. Ignored for other modes.
	Timeout time.Duration

	// Row is the y-coordinate to anchor the top border at. Zero means
	// use the default of 1 (one row below a typical menu bar).
	Row int

	// Optional explicit foreground/background. If UseColors is false,
	// the framework uses theme.WaitWindow (white-on-magenta dialog
	// look by default; swap palette/theme to retheme everywhere).
	// Set UseColors=true and supply Foreground+Background to override
	// for one-off styling — e.g. red bg for an "error" toast.
	Foreground tcell.Color
	Background tcell.Color
	UseColors  bool

	// OnDismiss runs when the wait window is dismissed for any reason
	// (key, mouse, timeout). Optional.
	OnDismiss func()

	// internal
	appearedAt time.Time
	paddedMsg  []rune
	boxWidth   int
	boxHeight  int
}

// NewWaitWindow returns a WaitWindow with sensible defaults: timeout
// mode, 3-second timer, anchored at row 2 (one row below a typical
// menu bar with breathing room).
func NewWaitWindow(message string) *WaitWindow {
	return &WaitWindow{
		Message: message,
		Mode:    WaitTimeout,
		Timeout: waitDefaultTimeout,
		Row:     2,
	}
}

// prepare snaps message-derived dimensions before the first draw.
// Idempotent.
func (w *WaitWindow) prepare() {
	w.paddedMsg = []rune(" " + w.Message + " ")
	w.boxWidth = len(w.paddedMsg) + 2 // borders
	w.boxHeight = 3
}

// timedOut reports whether a WaitTimeout window has overstayed its
// configured Timeout. Always false for other modes.
func (w *WaitWindow) timedOut() bool {
	if w.Mode != WaitTimeout {
		return false
	}
	return time.Since(w.appearedAt) >= w.Timeout
}

// dismissesOnMouse reports whether a mouse press in this mode
// dismisses the wait window.
func (w *WaitWindow) dismissesOnMouse() bool {
	return w.Mode == WaitNoWait || w.Mode == WaitTimeout
}
