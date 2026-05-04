package foxpro

import "github.com/gdamore/tcell/v2"

// HitZone identifies which part of a window the mouse landed on.
type HitZone int

const (
	HitNone HitZone = iota
	HitTitle
	HitBody
	HitResize
	HitClose
	HitZoom
	HitScrollUp
	HitScrollDown
	HitScrollLeft
	HitScrollRight
	HitScrollVTrack // anywhere on the vertical track between the arrows
	HitScrollHTrack // anywhere on the horizontal track between the arrows
)

// WindowManager owns the z-ordered list of windows.
// Index 0 is back-most; len-1 is front-most and active.
type WindowManager struct {
	windows []*Window
	active  int
}

// NewWindowManager returns an empty manager with no focus.
func NewWindowManager() *WindowManager {
	return &WindowManager{active: -1}
}

// Add appends w to the top of the z-order and gives it focus.
func (m *WindowManager) Add(w *Window) {
	m.windows = append(m.windows, w)
	m.active = len(m.windows) - 1
}

// Remove drops w. If it was focused, focus moves to the new top window.
func (m *WindowManager) Remove(w *Window) {
	for i, win := range m.windows {
		if win == w {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			if m.active >= len(m.windows) {
				m.active = len(m.windows) - 1
			}
			return
		}
	}
}

// AllWindows returns the current z-ordered slice (back-to-front).
func (m *WindowManager) AllWindows() []*Window { return m.windows }

// Contains reports whether w is currently mounted on this manager.
func (m *WindowManager) Contains(w *Window) bool {
	for _, x := range m.windows {
		if x == w {
			return true
		}
	}
	return false
}

// Active returns the focused window, or nil.
func (m *WindowManager) Active() *Window {
	if m.active < 0 || m.active >= len(m.windows) {
		return nil
	}
	return m.windows[m.active]
}

// ActiveDialog returns the topmost window if it is currently a
// Dialog (modal), else nil. While a dialog is active the framework
// blocks focus changes and click-to-raise on every other window —
// see App.dispatchMouse and the FocusNext/FocusPrev guards below.
func (m *WindowManager) ActiveDialog() *Window {
	w := m.Active()
	if w != nil && w.Dialog {
		return w
	}
	return nil
}

// Raise moves w to the top of the z-order and focuses it. Blocked
// when a different window is the active dialog: a modal must stay
// on top until it dismisses itself.
func (m *WindowManager) Raise(w *Window) {
	if d := m.ActiveDialog(); d != nil && d != w {
		return
	}
	for i, win := range m.windows {
		if win == w {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			m.windows = append(m.windows, w)
			m.active = len(m.windows) - 1
			return
		}
	}
}

// FocusNext cycles focus forward, raising the newly focused window.
// No-op while a dialog is active — the dialog must stay on top
// until it closes.
func (m *WindowManager) FocusNext() {
	if len(m.windows) == 0 || m.ActiveDialog() != nil {
		return
	}
	m.active = (m.active + 1) % len(m.windows)
	w := m.windows[m.active]
	m.Raise(w)
}

// FocusPrev cycles focus backward, raising the newly focused window.
// No-op while a dialog is active.
func (m *WindowManager) FocusPrev() {
	if len(m.windows) == 0 || m.ActiveDialog() != nil {
		return
	}
	idx := m.active - 1
	if idx < 0 {
		idx = len(m.windows) - 1
	}
	m.active = idx
	w := m.windows[m.active]
	m.Raise(w)
}

// HitTest returns the topmost window containing (x,y) and which zone was
// hit. Close, zoom, resize, and scroll-arrow zones are only reported for
// the *active* (topmost) window — clicking those positions on an inactive
// window just raises it.
func (m *WindowManager) HitTest(x, y int) (*Window, HitZone) {
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		b := w.Bounds
		if !b.Contains(x, y) {
			continue
		}
		// Borderless windows have no title / chrome / resize zones —
		// the entire surface is body. Reporting HitTitle here would
		// let the user drag a popup by its top row, which defeats
		// the "popup is anchored to its trigger" UX.
		if w.Borderless {
			return w, HitBody
		}
		isActive := i == len(m.windows)-1
		// Title row.
		if y == b.Y {
			if isActive && w.Closable && b.W >= 4 && x == b.X {
				return w, HitClose
			}
			if isActive && w.Zoomable && b.W >= 4 && x == b.X+b.W-1 {
				return w, HitZoom
			}
			return w, HitTitle
		}
		// Bottom-right corner is the resize grip.
		if isActive && x == b.X+b.W-1 && y == b.Y+b.H-1 {
			return w, HitResize
		}
		// Vertical scrollbar in the right column. Top/bottom rows are
		// the arrow buttons; everything in between is the track.
		if isActive && x == b.X+b.W-1 {
			if y == b.Y+1 {
				return w, HitScrollUp
			}
			if y == b.Y+b.H-2 {
				return w, HitScrollDown
			}
			if y > b.Y+1 && y < b.Y+b.H-2 {
				return w, HitScrollVTrack
			}
		}
		// Horizontal scrollbar in the bottom row.
		if isActive && y == b.Y+b.H-1 {
			if x == b.X+1 {
				return w, HitScrollLeft
			}
			if x == b.X+b.W-2 {
				return w, HitScrollRight
			}
			if x > b.X+1 && x < b.X+b.W-2 {
				return w, HitScrollHTrack
			}
		}
		return w, HitBody
	}
	return nil, HitNone
}

// Draw renders every window back-to-front so the active one ends up on top.
func (m *WindowManager) Draw(screen tcell.Screen, theme Theme, settings Settings) {
	for i, w := range m.windows {
		drawWindow(screen, w, theme, settings, i == m.active)
	}
}

// HandleKey routes the event to the active window's content provider.
func (m *WindowManager) HandleKey(ev *tcell.EventKey) bool {
	w := m.Active()
	if w == nil || w.Content == nil {
		return false
	}
	return w.Content.HandleKey(ev)
}
