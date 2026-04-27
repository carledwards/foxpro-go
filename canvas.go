package foxpro

import "github.com/gdamore/tcell/v2"

// Canvas is a clipped, scrollable drawing surface bound to a window's
// inner rect.
//
// Providers issue Put / Set / Fill calls in *logical* coordinates
// (origin 0,0). Canvas translates them by the current scroll offset
// and discards anything that falls outside the bound inner rect. As a
// side effect it tracks the bounding rect of every write — Natural()
// returns it, suitable as the answer to Scrollable.ContentSize.
//
// Typical use inside a ContentProvider.Draw:
//
//	func (p *Provider) Draw(screen, inner, theme, focused) {
//	    c := foxpro.NewCanvas(screen, inner, &p.ScrollState)
//	    c.Put(0, 0, "Hello", theme.WindowBG)
//	    c.Put(0, 1, "World", theme.WindowBG)
//	}
//
// Embed ScrollState in the provider to satisfy Scrollable from the
// canvas's bookkeeping for free.
type Canvas struct {
	screen tcell.Screen
	inner  Rect
	scroll *ScrollState
}

// NewCanvas binds the screen + inner rect. If state is non-nil it is
// used to read the current scroll offset and to record the natural
// extents observed during this Draw — pass a stable *ScrollState
// embedded in the provider so its Scrollable methods see the result.
func NewCanvas(screen tcell.Screen, inner Rect, state *ScrollState) *Canvas {
	if state != nil {
		state.lastInnerW = inner.W
		state.lastInnerH = inner.H
		state.naturalW = 0
		state.naturalH = 0
	}
	return &Canvas{screen: screen, inner: inner, scroll: state}
}

// Set writes a single rune+style at logical (x, y). Clipped if the
// translated cell falls outside the inner rect.
func (c *Canvas) Set(x, y int, ch rune, st tcell.Style) {
	if c.scroll != nil {
		if x+1 > c.scroll.naturalW {
			c.scroll.naturalW = x + 1
		}
		if y+1 > c.scroll.naturalH {
			c.scroll.naturalH = y + 1
		}
	}
	sx, sy := 0, 0
	if c.scroll != nil {
		sx, sy = c.scroll.X, c.scroll.Y
	}
	px := x - sx
	py := y - sy
	if px < 0 || py < 0 || px >= c.inner.W || py >= c.inner.H {
		return
	}
	c.screen.SetContent(c.inner.X+px, c.inner.Y+py, ch, nil, st)
}

// Put writes a string starting at logical (x, y), advancing one cell
// per rune. Returns the next logical x for chaining.
func (c *Canvas) Put(x, y int, s string, st tcell.Style) int {
	for _, r := range s {
		c.Set(x, y, r, st)
		x++
	}
	return x
}

// Fill writes ch+st across every cell of logical rect r.
func (c *Canvas) Fill(r Rect, ch rune, st tcell.Style) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			c.Set(x, y, ch, st)
		}
	}
}

// Inner returns the visible viewport rect (the bound inner).
func (c *Canvas) Inner() Rect { return c.inner }

// ScrollState carries a provider's current scroll offset and the
// natural extents observed by Canvas in the most recent Draw.
//
// Embed it in a provider:
//
//	type Provider struct {
//	    foxpro.ScrollState
//	    // ... other fields
//	}
//
// The promoted methods (ContentSize, ScrollOffset, SetScrollOffset)
// satisfy the Scrollable interface — the framework will draw and
// route scrollbars automatically. Read X / Y in HandleKey to drive
// keyboard scrolling; call SetScrollOffset to clamp and update.
type ScrollState struct {
	X, Y                   int
	naturalW, naturalH     int
	lastInnerW, lastInnerH int
}

// ContentSize returns the natural extents observed in the last Draw.
// Reports zero until at least one Draw has happened.
func (s *ScrollState) ContentSize() (int, int) {
	return s.naturalW, s.naturalH
}

// ScrollOffset returns the current scroll offset.
func (s *ScrollState) ScrollOffset() (int, int) { return s.X, s.Y }

// SetScrollOffset clamps and stores a new offset. The clamp uses the
// inner size and natural size from the most recent Draw, so calling
// SetScrollOffset before the first Draw resolves to (0, 0).
func (s *ScrollState) SetScrollOffset(x, y int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if mx := s.naturalW - s.lastInnerW; mx >= 0 && x > mx {
		x = mx
	}
	if my := s.naturalH - s.lastInnerH; my >= 0 && y > my {
		y = my
	}
	s.X, s.Y = x, y
}

// LastViewport returns the inner rect dimensions from the most
// recent Draw. Useful for HandleKey when paging.
func (s *ScrollState) LastViewport() (w, h int) {
	return s.lastInnerW, s.lastInnerH
}
