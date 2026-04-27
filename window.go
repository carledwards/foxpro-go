package foxpro

import "github.com/gdamore/tcell/v2"

// ContentProvider draws the inside of a window and handles input directed at it.
// Implementations own their own scroll position and any per-window state.
type ContentProvider interface {
	// Draw renders into the inner rect (border-free area).
	Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool)
	// HandleKey returns true if it consumed the event.
	HandleKey(ev *tcell.EventKey) bool
}

// StatusHinter is an optional interface a ContentProvider may implement
// to contribute a contextual hint to the right side of the status bar.
// Empty string means "no hint".
type StatusHinter interface {
	StatusHint() string
}

// MouseHandler is an optional interface a ContentProvider may implement
// to receive mouse press events that land inside the window's body
// (HitBody zone). The framework calls HandleMouse with the event and
// the window's content rect (border-stripped). Return true if consumed.
//
// HandleMouse fires on button-down only; drags and motion-only events
// are not forwarded unless the provider also implements MouseDragHandler.
type MouseHandler interface {
	HandleMouse(ev *tcell.EventMouse, inner Rect) bool
}

// MouseDragHandler is an optional companion to MouseHandler. When a
// MouseHandler.HandleMouse press returns true and the provider also
// implements this interface, the framework captures further mouse
// events for that window: motion-while-held is delivered via
// HandleMouseMotion, and the eventual button release via
// HandleMouseRelease. Useful for scrollbar thumbs, selection
// dragging, and any other drag-style interaction inside a provider.
type MouseDragHandler interface {
	HandleMouseMotion(ev *tcell.EventMouse, inner Rect)
	HandleMouseRelease(ev *tcell.EventMouse, inner Rect)
}

// WheelHandler is an optional interface a ContentProvider may
// implement to receive mouse-wheel events with positional context.
// Useful for split layouts (PaneProvider) that need to route the
// wheel based on which child the cursor is over rather than to the
// single Scrollable the window exposes. Return true if consumed; if
// not, the framework falls back to scrolling the top-level
// Scrollable, if any.
type WheelHandler interface {
	HandleWheel(ev *tcell.EventMouse, inner Rect) bool
}

// Scrollable is an optional interface a ContentProvider may implement
// to opt into framework-drawn scrollbars and click-to-scroll mouse
// support. Width/height are total content extents in cells; offset is
// the top-left visible coordinate. SetScrollOffset is called when the
// user clicks a scroll arrow or thumb; implementations should clamp
// to a valid range (e.g. 0 .. max(contentH - viewportH, 0)).
//
// The framework shows the vertical bar when ContentSize().h exceeds
// the visible inner height, and the horizontal bar when the width
// exceeds the inner width. Either or both can be active.
type Scrollable interface {
	ContentSize() (w, h int)
	ScrollOffset() (x, y int)
	SetScrollOffset(x, y int)
}

// Window is a movable, focusable rectangle owning a ContentProvider.
//
// Closable / Zoomable control whether the close (■) and zoom (▲) glyphs
// render in the title bar and respond to clicks. OnClose / OnZoom let the
// caller override the default actions (remove from manager / toggle
// maximize against the screen).
type Window struct {
	Title    string
	Bounds   Rect
	Content  ContentProvider
	Closable bool
	Zoomable bool
	OnClose  func()
	OnZoom   func()

	// MinW / MinH are optional minimum dimensions enforced during
	// drag-resize. Either may be zero (unset) — the framework's
	// default floor (8 × 3) still applies. Set these to keep the
	// content provider's required layout visible.
	MinW int
	MinH int

	// Internal zoom state — set when the window is currently maximized
	// and restored on a second zoom click.
	maximized  bool
	prevBounds Rect

	// Internal shade state — when shaded, the window collapses to a
	// 1-row title bar and shadeBounds remembers the prior size.
	shaded      bool
	shadeBounds Rect
}

// NewWindow constructs a window with the given title, bounds, and content.
// Closable and Zoomable default to true.
func NewWindow(title string, bounds Rect, content ContentProvider) *Window {
	return &Window{Title: title, Bounds: bounds, Content: content, Closable: true, Zoomable: true}
}

// ShadedWidth is the fixed width a window collapses to when shaded.
// Wide enough to comfortably show a short title between the chrome glyphs;
// uniform across windows so shaded titles stack neatly.
const ShadedWidth = 32

// Shaded reports whether the window is currently in window-shade mode
// (collapsed to a 1-row title bar).
func (w *Window) Shaded() bool { return w.shaded }
