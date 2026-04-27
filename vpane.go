package foxpro

import "github.com/gdamore/tcell/v2"

// VPaneProvider stacks two ContentProviders vertically (top + bottom).
// Mirror of PaneProvider but along the Y axis. Useful for "input at
// top, list below" layouts and any other top/bottom split.
//
// Tab / Shift+Tab cycle focus. Mouse click in either region focuses
// it and forwards to that region's MouseHandler. Optional interfaces
// (StatusHinter, MouseDragHandler, WheelHandler, Scrollable) are
// forwarded to the focused pane.
type VPaneProvider struct {
	Top       ContentProvider
	Bottom    ContentProvider
	TopHeight int

	bottomFocused bool
}

// NewVPaneProvider constructs a vertical pane container. topHeight is
// the cells reserved for the top pane (no divider — top sits directly
// above bottom).
func NewVPaneProvider(top, bottom ContentProvider, topHeight int) *VPaneProvider {
	return &VPaneProvider{Top: top, Bottom: bottom, TopHeight: topHeight}
}

func (p *VPaneProvider) topRect(inner Rect) Rect {
	h := p.TopHeight
	if h > inner.H-1 {
		h = inner.H - 1
	}
	if h < 1 {
		h = 1
	}
	return Rect{X: inner.X, Y: inner.Y, W: inner.W, H: h}
}

func (p *VPaneProvider) bottomRect(inner Rect) Rect {
	th := p.topRect(inner).H
	bh := inner.H - th
	if bh < 0 {
		bh = 0
	}
	return Rect{X: inner.X, Y: inner.Y + th, W: inner.W, H: bh}
}

func (p *VPaneProvider) focusedPane() ContentProvider {
	if p.bottomFocused {
		return p.Bottom
	}
	return p.Top
}

func (p *VPaneProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	topR := p.topRect(inner)
	botR := p.bottomRect(inner)
	if p.Top != nil {
		p.Top.Draw(screen, topR, theme, focused && !p.bottomFocused)
	}
	if p.Bottom != nil {
		p.Bottom.Draw(screen, botR, theme, focused && p.bottomFocused)
	}
}

func (p *VPaneProvider) HandleKey(ev *tcell.EventKey) bool {
	// Tab / Shift+Tab: forward to the focused child first so its own
	// internal cycling has a chance. If the child doesn't consume
	// the key, advance focus across panes; if we're already at the
	// boundary, return false so the next-outer container can take it.
	if ev.Key() == tcell.KeyTab || ev.Key() == tcell.KeyBacktab {
		if target := p.focusedPane(); target != nil && target.HandleKey(ev) {
			return true
		}
		if ev.Key() == tcell.KeyTab && !p.bottomFocused {
			p.bottomFocused = true
			return true
		}
		if ev.Key() == tcell.KeyBacktab && p.bottomFocused {
			p.bottomFocused = false
			return true
		}
		return false
	}
	if target := p.focusedPane(); target != nil {
		return target.HandleKey(ev)
	}
	return false
}

func (p *VPaneProvider) HandleMouse(ev *tcell.EventMouse, inner Rect) bool {
	_, my := ev.Position()
	topR := p.topRect(inner)
	botR := p.bottomRect(inner)
	if my < topR.Y+topR.H {
		p.bottomFocused = false
		if mh, ok := p.Top.(MouseHandler); ok {
			return mh.HandleMouse(ev, topR)
		}
		return true
	}
	p.bottomFocused = true
	if mh, ok := p.Bottom.(MouseHandler); ok {
		return mh.HandleMouse(ev, botR)
	}
	return true
}

// HandleMouseMotion forwards drags to the focused pane.
func (p *VPaneProvider) HandleMouseMotion(ev *tcell.EventMouse, inner Rect) {
	pane := p.focusedPane()
	if pane == nil {
		return
	}
	mdh, ok := pane.(MouseDragHandler)
	if !ok {
		return
	}
	rect := p.topRect(inner)
	if p.bottomFocused {
		rect = p.bottomRect(inner)
	}
	mdh.HandleMouseMotion(ev, rect)
}

func (p *VPaneProvider) HandleMouseRelease(ev *tcell.EventMouse, inner Rect) {
	pane := p.focusedPane()
	if pane == nil {
		return
	}
	mdh, ok := pane.(MouseDragHandler)
	if !ok {
		return
	}
	rect := p.topRect(inner)
	if p.bottomFocused {
		rect = p.bottomRect(inner)
	}
	mdh.HandleMouseRelease(ev, rect)
}

// HandleWheel routes the wheel to whichever pane the cursor is over.
func (p *VPaneProvider) HandleWheel(ev *tcell.EventMouse, inner Rect) bool {
	_, my := ev.Position()
	topR := p.topRect(inner)
	botR := p.bottomRect(inner)
	var pane ContentProvider
	var rect Rect
	if my < topR.Y+topR.H {
		pane = p.Top
		rect = topR
	} else {
		pane = p.Bottom
		rect = botR
	}
	if pane == nil {
		return false
	}
	if wh, ok := pane.(WheelHandler); ok {
		return wh.HandleWheel(ev, rect)
	}
	if scr, ok := pane.(Scrollable); ok {
		const step = 3
		btn := ev.Buttons()
		sx, sy := scr.ScrollOffset()
		switch {
		case btn&tcell.WheelUp != 0:
			scr.SetScrollOffset(sx, sy-step)
		case btn&tcell.WheelDown != 0:
			scr.SetScrollOffset(sx, sy+step)
		case btn&tcell.WheelLeft != 0:
			scr.SetScrollOffset(sx-step, sy)
		case btn&tcell.WheelRight != 0:
			scr.SetScrollOffset(sx+step, sy)
		}
		return true
	}
	return false
}

// StatusHint forwards to the focused pane.
func (p *VPaneProvider) StatusHint() string {
	if h, ok := p.focusedPane().(StatusHinter); ok {
		return h.StatusHint()
	}
	return ""
}

// Scrollable forwards to the bottom pane (the one usually filling
// the larger area; matches PaneProvider's "right pane gets the
// chrome bar" choice).
func (p *VPaneProvider) ContentSize() (int, int) {
	if scr, ok := p.Bottom.(Scrollable); ok {
		return scr.ContentSize()
	}
	return 0, 0
}

func (p *VPaneProvider) ScrollOffset() (int, int) {
	if scr, ok := p.Bottom.(Scrollable); ok {
		return scr.ScrollOffset()
	}
	return 0, 0
}

func (p *VPaneProvider) SetScrollOffset(x, y int) {
	if scr, ok := p.Bottom.(Scrollable); ok {
		scr.SetScrollOffset(x, y)
	}
}
