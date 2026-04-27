package foxpro

import "github.com/gdamore/tcell/v2"

// PaneProvider splits its inner area into a left + right pane, each
// running its own ContentProvider. The divider is one cell wide and
// doubles as the left pane's vertical scrollbar when the left pane
// has more content than fits.
//
// Focus behaviour:
//   - Tab / Shift+Tab cycle focus between the two panes
//   - Mouse click on a pane gives it focus and forwards the press to
//     that pane's MouseHandler if it implements one
//
// Scrollbar geometry — each pane gets its own visible bar:
//   - Right pane: window-chrome bar on the right edge (drawn by the
//     framework via Scrollable forwarding from PaneProvider → Right)
//   - Left pane: bar overlaid on the divider column itself, drawn by
//     PaneProvider directly
//
// Mouse wheel routes to whichever pane the cursor is over (via the
// WheelHandler interface), so each side scrolls independently.
type PaneProvider struct {
	Left      ContentProvider
	Right     ContentProvider
	LeftWidth int

	rightFocused bool
	lastInner    Rect
}

// NewPaneProvider constructs a pane container.
func NewPaneProvider(left, right ContentProvider, leftWidth int) *PaneProvider {
	return &PaneProvider{Left: left, Right: right, LeftWidth: leftWidth}
}

func (p *PaneProvider) leftRect(inner Rect) Rect {
	w := p.LeftWidth
	if w > inner.W-2 {
		w = inner.W - 2
	}
	if w < 1 {
		w = 1
	}
	return Rect{X: inner.X, Y: inner.Y, W: w, H: inner.H}
}

func (p *PaneProvider) rightRect(inner Rect) Rect {
	lw := p.leftRect(inner).W
	rw := inner.W - lw - 1
	if rw < 0 {
		rw = 0
	}
	return Rect{X: inner.X + lw + 1, Y: inner.Y, W: rw, H: inner.H}
}

// focusedPane returns the currently focused child provider.
func (p *PaneProvider) focusedPane() ContentProvider {
	if p.rightFocused {
		return p.Right
	}
	return p.Left
}

func (p *PaneProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	p.lastInner = inner
	leftR := p.leftRect(inner)
	rightR := p.rightRect(inner)

	if p.Left != nil {
		p.Left.Draw(screen, leftR, theme, focused && !p.rightFocused)
	}
	if p.Right != nil {
		p.Right.Draw(screen, rightR, theme, focused && p.rightFocused)
	}

	// Divider with optional left-pane scrollbar overlay.
	divX := inner.X + leftR.W
	if divX < inner.X+inner.W {
		p.drawDivider(screen, divX, inner.Y, inner.H, theme)
	}
}

// drawDivider paints the central divider column. If the left pane is
// Scrollable and overflows, the divider doubles as its scrollbar.
func (p *PaneProvider) drawDivider(screen tcell.Screen, x, yTop, height int, theme Theme) {
	style := theme.WindowBG
	for y := yTop; y < yTop+height; y++ {
		screen.SetContent(x, y, '│', nil, style)
	}
	scr, ok := p.Left.(Scrollable)
	if !ok {
		return
	}
	_, ch := scr.ContentSize()
	_, sy := scr.ScrollOffset()
	if ch <= height || height < 4 {
		return
	}
	arrow := style.Foreground(theme.TitleAccent)
	screen.SetContent(x, yTop, '▲', nil, arrow)
	screen.SetContent(x, yTop+height-1, '▼', nil, arrow)
	trackTop := yTop + 1
	trackBot := yTop + height - 2
	trackH := trackBot - trackTop + 1
	if trackH <= 0 {
		return
	}
	maxScroll := ch - height
	thumbOff := 0
	if maxScroll > 0 && trackH > 1 {
		thumbOff = (sy * (trackH - 1)) / maxScroll
	}
	thumbY := trackTop + thumbOff
	if thumbY < trackTop {
		thumbY = trackTop
	}
	if thumbY > trackBot {
		thumbY = trackBot
	}
	screen.SetContent(x, thumbY, '◆', nil, arrow)
}

func (p *PaneProvider) HandleKey(ev *tcell.EventKey) bool {
	// Tab / Shift+Tab cycle nested controls bottom-up: forward to
	// the focused child first; only advance our own focus if the
	// child didn't consume; bubble to the outer container at the
	// boundary so deeply nested layouts cycle the way users expect.
	if ev.Key() == tcell.KeyTab || ev.Key() == tcell.KeyBacktab {
		if target := p.focusedPane(); target != nil && target.HandleKey(ev) {
			return true
		}
		if ev.Key() == tcell.KeyTab && !p.rightFocused {
			p.rightFocused = true
			return true
		}
		if ev.Key() == tcell.KeyBacktab && p.rightFocused {
			p.rightFocused = false
			return true
		}
		return false
	}
	if target := p.focusedPane(); target != nil {
		return target.HandleKey(ev)
	}
	return false
}

// HandleMouse routes the press. Three regions:
//   - Left half of inner: focuses + forwards to Left
//   - Divider column: left-pane scrollbar (if Scrollable + overflow)
//   - Right half: focuses + forwards to Right
func (p *PaneProvider) HandleMouse(ev *tcell.EventMouse, inner Rect) bool {
	mx, my := ev.Position()
	leftR := p.leftRect(inner)
	rightR := p.rightRect(inner)
	divX := inner.X + leftR.W

	if mx == divX {
		return p.handleDividerClick(my, inner.Y, inner.H)
	}
	if mx < divX {
		p.rightFocused = false
		if mh, ok := p.Left.(MouseHandler); ok {
			return mh.HandleMouse(ev, leftR)
		}
		return true
	}
	if mx >= rightR.X {
		p.rightFocused = true
		if mh, ok := p.Right.(MouseHandler); ok {
			return mh.HandleMouse(ev, rightR)
		}
		return true
	}
	return false
}

func (p *PaneProvider) handleDividerClick(my, yTop, height int) bool {
	scr, ok := p.Left.(Scrollable)
	if !ok {
		return true
	}
	_, ch := scr.ContentSize()
	_, sy := scr.ScrollOffset()
	if ch <= height || height < 4 {
		return true
	}
	yBot := yTop + height - 1
	switch {
	case my == yTop:
		scr.SetScrollOffset(0, sy-1)
	case my == yBot:
		scr.SetScrollOffset(0, sy+1)
	default:
		trackTop := yTop + 1
		trackBot := yBot - 1
		trackH := trackBot - trackTop + 1
		if trackH > 0 {
			maxScroll := ch - height
			thumbY := trackTop
			if maxScroll > 0 && trackH > 1 {
				thumbY += (sy * (trackH - 1)) / maxScroll
			}
			page := height - 1
			if page < 1 {
				page = 1
			}
			switch {
			case my < thumbY:
				scr.SetScrollOffset(0, sy-page)
			case my > thumbY:
				scr.SetScrollOffset(0, sy+page)
				// on thumb: drag would go here (deferred)
			}
		}
	}
	p.rightFocused = false
	return true
}

// HandleMouseMotion (MouseDragHandler) — forward motion to the
// focused pane if it implements MouseDragHandler. Drag was started
// by a press inside that pane, so subsequent motion belongs there.
func (p *PaneProvider) HandleMouseMotion(ev *tcell.EventMouse, inner Rect) {
	pane := p.focusedPane()
	if pane == nil {
		return
	}
	mdh, ok := pane.(MouseDragHandler)
	if !ok {
		return
	}
	rect := p.leftRect(inner)
	if p.rightFocused {
		rect = p.rightRect(inner)
	}
	mdh.HandleMouseMotion(ev, rect)
}

// HandleMouseRelease (MouseDragHandler) — same forwarding as motion.
func (p *PaneProvider) HandleMouseRelease(ev *tcell.EventMouse, inner Rect) {
	pane := p.focusedPane()
	if pane == nil {
		return
	}
	mdh, ok := pane.(MouseDragHandler)
	if !ok {
		return
	}
	rect := p.leftRect(inner)
	if p.rightFocused {
		rect = p.rightRect(inner)
	}
	mdh.HandleMouseRelease(ev, rect)
}

// HandleWheel routes wheel events to the pane the cursor is over,
// independent of which pane has keyboard focus. Tries each pane's
// WheelHandler first (so nested split layouts and BoxedProviders
// can route further), then falls back to its Scrollable.
func (p *PaneProvider) HandleWheel(ev *tcell.EventMouse, inner Rect) bool {
	mx, _ := ev.Position()
	leftR := p.leftRect(inner)
	rightR := p.rightRect(inner)
	var pane ContentProvider
	var paneRect Rect
	switch {
	case mx < inner.X+leftR.W:
		pane = p.Left
		paneRect = leftR
	case mx >= rightR.X:
		pane = p.Right
		paneRect = rightR
	}
	if pane == nil {
		return false
	}
	if wh, ok := pane.(WheelHandler); ok {
		return wh.HandleWheel(ev, paneRect)
	}
	scr, ok := pane.(Scrollable)
	if !ok {
		return false
	}
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

// StatusHint forwards to the focused pane.
func (p *PaneProvider) StatusHint() string {
	if h, ok := p.focusedPane().(StatusHinter); ok {
		return h.StatusHint()
	}
	return ""
}

// Scrollable: forwards to the *right* pane (which sits against the
// window's right chrome). The chrome scrollbar therefore always
// reflects the right pane. The left pane gets its own scrollbar
// drawn directly on the divider.
func (p *PaneProvider) ContentSize() (int, int) {
	if scr, ok := p.Right.(Scrollable); ok {
		return scr.ContentSize()
	}
	return 0, 0
}

func (p *PaneProvider) ScrollOffset() (int, int) {
	if scr, ok := p.Right.(Scrollable); ok {
		return scr.ScrollOffset()
	}
	return 0, 0
}

func (p *PaneProvider) SetScrollOffset(x, y int) {
	if scr, ok := p.Right.(Scrollable); ok {
		scr.SetScrollOffset(x, y)
	}
}
