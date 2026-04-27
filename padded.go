package foxpro

import "github.com/gdamore/tcell/v2"

// PaddedProvider wraps a ContentProvider with cell margins on each
// side. Useful for breathing room around inset widgets without
// touching the wrapped provider's own layout.
//
// PaddedProvider does not paint its margin cells — it inherits
// whatever's behind. Forwards every optional interface to the
// wrapped provider with the inset rect.
type PaddedProvider struct {
	Inner                     ContentProvider
	Top, Right, Bottom, Left  int
}

// NewPaddedProvider wraps inner with the given margins (in cells).
func NewPaddedProvider(inner ContentProvider, top, right, bottom, left int) *PaddedProvider {
	return &PaddedProvider{Inner: inner, Top: top, Right: right, Bottom: bottom, Left: left}
}

func (p *PaddedProvider) inset(outer Rect) Rect {
	w := outer.W - p.Left - p.Right
	if w < 0 {
		w = 0
	}
	h := outer.H - p.Top - p.Bottom
	if h < 0 {
		h = 0
	}
	return Rect{X: outer.X + p.Left, Y: outer.Y + p.Top, W: w, H: h}
}

func (p *PaddedProvider) Draw(screen tcell.Screen, outer Rect, theme Theme, focused bool) {
	if p.Inner == nil {
		return
	}
	inner := p.inset(outer)
	if inner.W > 0 && inner.H > 0 {
		p.Inner.Draw(screen, inner, theme, focused)
	}
}

func (p *PaddedProvider) HandleKey(ev *tcell.EventKey) bool {
	if p.Inner == nil {
		return false
	}
	return p.Inner.HandleKey(ev)
}

func (p *PaddedProvider) HandleMouse(ev *tcell.EventMouse, outer Rect) bool {
	if mh, ok := p.Inner.(MouseHandler); ok {
		return mh.HandleMouse(ev, p.inset(outer))
	}
	return false
}

func (p *PaddedProvider) HandleMouseMotion(ev *tcell.EventMouse, outer Rect) {
	if mdh, ok := p.Inner.(MouseDragHandler); ok {
		mdh.HandleMouseMotion(ev, p.inset(outer))
	}
}

func (p *PaddedProvider) HandleMouseRelease(ev *tcell.EventMouse, outer Rect) {
	if mdh, ok := p.Inner.(MouseDragHandler); ok {
		mdh.HandleMouseRelease(ev, p.inset(outer))
	}
}

func (p *PaddedProvider) HandleWheel(ev *tcell.EventMouse, outer Rect) bool {
	if wh, ok := p.Inner.(WheelHandler); ok {
		return wh.HandleWheel(ev, p.inset(outer))
	}
	return false
}

func (p *PaddedProvider) StatusHint() string {
	if h, ok := p.Inner.(StatusHinter); ok {
		return h.StatusHint()
	}
	return ""
}

// Scrollable forwards to the wrapped provider, since margin doesn't
// affect content extents.
func (p *PaddedProvider) ContentSize() (int, int) {
	if scr, ok := p.Inner.(Scrollable); ok {
		return scr.ContentSize()
	}
	return 0, 0
}

func (p *PaddedProvider) ScrollOffset() (int, int) {
	if scr, ok := p.Inner.(Scrollable); ok {
		return scr.ScrollOffset()
	}
	return 0, 0
}

func (p *PaddedProvider) SetScrollOffset(x, y int) {
	if scr, ok := p.Inner.(Scrollable); ok {
		scr.SetScrollOffset(x, y)
	}
}
