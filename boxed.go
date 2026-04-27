package foxpro

import "github.com/gdamore/tcell/v2"

// BoxedProvider wraps an inner ContentProvider with a single-line
// border (┌─┐│└┘) and an internal vertical scrollbar drawn on the
// right column when the inner is Scrollable and overflows.
//
// The box owns its own scrollbar — it does NOT expose Scrollable to
// the outside — so it can be safely composed inside containers like
// PaneProvider without competing with the framework's chrome bars or
// the container's own scrollbar drawing. WheelHandler routes mouse
// wheel from the framework to the wrapped provider's Scrollable.
type BoxedProvider struct {
	Inner ContentProvider
	Title string // optional; centred in the top border in the accent colour

	lastOuter Rect

	// Thumb-drag state. Captured by the framework via the
	// MouseDragHandler interface during a thumb press; updated on
	// HandleMouseMotion; cleared on HandleMouseRelease.
	dragKind   boxedDragKind
	dragStart  int // V: track top y; H: track left x
	dragEnd    int // V: track bot y; H: track right x
	dragMaxScr int // max scroll value along the dragging axis
}

type boxedDragKind int

const (
	boxedDragNone boxedDragKind = iota
	boxedDragVThumb
	boxedDragHThumb
)

// NewBoxedProvider wraps inner in a bordered box with the given title.
// Pass "" for no title.
func NewBoxedProvider(inner ContentProvider, title string) *BoxedProvider {
	return &BoxedProvider{Inner: inner, Title: title}
}

// hasScrollbar reports whether the inner is Scrollable (so the right
// border column will host scrollbar arrows + thumb when overflowing).
// The scrollbar overlays the right border itself — content width
// doesn't change between scrollable and non-scrollable inners.
func (b *BoxedProvider) hasScrollbar() bool {
	_, ok := b.Inner.(Scrollable)
	return ok
}

// innerRect returns the rect handed to the wrapped provider —
// inside the border on every side. The scrollbar (when present) is
// drawn ON the right border column rather than reserving its own
// inner column, so content width stays the same regardless.
func (b *BoxedProvider) innerRect(outer Rect) Rect {
	w := outer.W - 2
	if w < 0 {
		w = 0
	}
	h := outer.H - 2
	if h < 0 {
		h = 0
	}
	return Rect{X: outer.X + 1, Y: outer.Y + 1, W: w, H: h}
}

func (b *BoxedProvider) Draw(screen tcell.Screen, outer Rect, theme Theme, focused bool) {
	b.lastOuter = outer
	if outer.W < 3 || outer.H < 3 {
		return
	}
	// Inner area: window background.
	fillRect(screen, outer, theme.WindowBG)

	// Border layout depends on whether the inner is Scrollable:
	//
	//   Scrollable: top + bottom borders extend straight across the
	//     full width (no ┐/┘ corners). The right column between them
	//     becomes the scrollbar — that's the FoxPro "half box" look:
	//     horizontal lines cap an open-sided scroll strip.
	//
	//   Non-scrollable: classic ┌─┐│└─┘ rectangle with `│` on the
	//     right.
	border := theme.Border
	botY := outer.Y + outer.H - 1
	rightX := outer.X + outer.W - 1

	// Left column + its two corners — same in both cases.
	for y := outer.Y + 1; y < botY; y++ {
		screen.SetContent(outer.X, y, '│', nil, border)
	}
	screen.SetContent(outer.X, outer.Y, '┌', nil, border)
	screen.SetContent(outer.X, botY, '└', nil, border)

	if b.hasScrollbar() {
		// Top horizontal line runs all the way to the right edge.
		for x := outer.X + 1; x <= rightX; x++ {
			screen.SetContent(x, outer.Y, '─', nil, border)
		}
		// Bottom horizontal line stops one cell short — the
		// bottom-right corner is left as a blank cell in the
		// scrollbar background, which is where the V and H bars
		// would otherwise visually collide.
		for x := outer.X + 1; x < rightX; x++ {
			screen.SetContent(x, botY, '─', nil, border)
		}
		screen.SetContent(rightX, botY, ' ', nil, theme.Scrollbar)
	} else {
		for x := outer.X + 1; x < rightX; x++ {
			screen.SetContent(x, outer.Y, '─', nil, border)
			screen.SetContent(x, botY, '─', nil, border)
		}
		for y := outer.Y + 1; y < botY; y++ {
			screen.SetContent(rightX, y, '│', nil, border)
		}
		screen.SetContent(rightX, outer.Y, '┐', nil, border)
		screen.SetContent(rightX, botY, '┘', nil, border)
	}

	// Title overlaid on the top border in accent colour.
	if b.Title != "" && outer.W > 4 {
		title := " " + b.Title + " "
		if len(title) > outer.W-2 {
			title = title[:outer.W-2]
		}
		tx := outer.X + (outer.W-len(title))/2
		drawString(screen, tx, outer.Y, title, border.Foreground(theme.TitleAccent))
	}

	inner := b.innerRect(outer)
	if b.hasScrollbar() {
		b.drawScrollbar(screen, outer, inner, theme)
		b.drawHScrollbar(screen, outer, inner, theme)
	}
	if inner.W > 0 && inner.H > 0 && b.Inner != nil {
		b.Inner.Draw(screen, inner, theme, focused)
	}
}

// drawScrollbar paints the right column between the top and bottom
// borders as the scrollbar. The corner cells (outer.Y, outer.Y+H-1)
// are NOT touched here — they belong to the horizontal borders.
// Arrows and gutter are always shown when the inner is Scrollable;
// the thumb only appears when the inner actually overflows.
func (b *BoxedProvider) drawScrollbar(screen tcell.Screen, outer, inner Rect, theme Theme) {
	barX := outer.X + outer.W - 1
	sbar := theme.Scrollbar
	// Arrow + thumb foreground = the content background colour, so
	// the chrome glyphs read as cut-outs of the same hue as the
	// inner pane behind them.
	_, contentBG, _ := theme.WindowBG.Decompose()
	arrow := sbar.Foreground(contentBG)

	yTop := outer.Y + 1
	yBot := outer.Y + outer.H - 2
	if yBot < yTop {
		return // not enough height for any scrollbar cells
	}

	if yBot == yTop {
		// One cell — just gutter.
		screen.SetContent(barX, yTop, ' ', nil, sbar)
		return
	}

	// Arrows + blank gutter — always shown when scrollable.
	screen.SetContent(barX, yTop, '▲', nil, arrow)
	screen.SetContent(barX, yBot, '▼', nil, arrow)
	for y := yTop + 1; y < yBot; y++ {
		screen.SetContent(barX, y, ' ', nil, sbar)
	}

	// Thumb only when the inner actually overflows.
	scr, _ := b.Inner.(Scrollable)
	_, ch := scr.ContentSize()
	_, sy := scr.ScrollOffset()
	if ch <= inner.H {
		return
	}
	trackTop := yTop + 1
	trackBot := yBot - 1
	trackH := trackBot - trackTop + 1
	if trackH <= 0 {
		return
	}
	maxScroll := ch - inner.H
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
	screen.SetContent(barX, thumbY, '◆', nil, arrow)
}

// drawHScrollbar mirrors drawScrollbar on the bottom row: ◄ + blank
// gutter + ► always shown when scrollable, ◆ thumb only when the
// inner overflows horizontally. Corner cells (col 0 and col W-1)
// belong to the left/right borders and are left untouched.
func (b *BoxedProvider) drawHScrollbar(screen tcell.Screen, outer, inner Rect, theme Theme) {
	barY := outer.Y + outer.H - 1
	sbar := theme.Scrollbar
	// Arrow + thumb foreground = the content background colour, so
	// the chrome glyphs read as cut-outs of the same hue as the
	// inner pane behind them.
	_, contentBG, _ := theme.WindowBG.Decompose()
	arrow := sbar.Foreground(contentBG)

	xLeft := outer.X + 1
	xRight := outer.X + outer.W - 2
	if xRight < xLeft {
		return // not enough width for any scrollbar cells
	}
	if xLeft == xRight {
		screen.SetContent(xLeft, barY, ' ', nil, sbar)
		return
	}

	screen.SetContent(xLeft, barY, '◄', nil, arrow)
	screen.SetContent(xRight, barY, '►', nil, arrow)
	for x := xLeft + 1; x < xRight; x++ {
		screen.SetContent(x, barY, ' ', nil, sbar)
	}

	scr, _ := b.Inner.(Scrollable)
	cw, _ := scr.ContentSize()
	sx, _ := scr.ScrollOffset()
	if cw <= inner.W {
		return
	}
	trackL := xLeft + 1
	trackR := xRight - 1
	trackW := trackR - trackL + 1
	if trackW <= 0 {
		return
	}
	maxScroll := cw - inner.W
	thumbOff := 0
	if maxScroll > 0 && trackW > 1 {
		thumbOff = (sx * (trackW - 1)) / maxScroll
	}
	thumbX := trackL + thumbOff
	if thumbX < trackL {
		thumbX = trackL
	}
	if thumbX > trackR {
		thumbX = trackR
	}
	screen.SetContent(thumbX, barY, '◆', nil, arrow)
}

func (b *BoxedProvider) HandleKey(ev *tcell.EventKey) bool {
	if b.Inner == nil {
		return false
	}
	return b.Inner.HandleKey(ev)
}

// HandleMouse: clicks on the V or H scrollbar run the box's own
// scroll logic; clicks inside the inner content area forward to the
// wrapped provider's MouseHandler.
func (b *BoxedProvider) HandleMouse(ev *tcell.EventMouse, _ Rect) bool {
	mx, my := ev.Position()
	outer := b.lastOuter
	if !outer.Contains(mx, my) {
		return false
	}
	inner := b.innerRect(outer)
	if b.hasScrollbar() {
		if mx == outer.X+outer.W-1 {
			return b.handleScrollbarClick(my, outer, inner)
		}
		if my == outer.Y+outer.H-1 {
			return b.handleHScrollbarClick(mx, outer, inner)
		}
	}
	if inner.Contains(mx, my) {
		if mh, ok := b.Inner.(MouseHandler); ok {
			return mh.HandleMouse(ev, inner)
		}
	}
	return true
}

func (b *BoxedProvider) handleScrollbarClick(my int, outer, inner Rect) bool {
	scr, ok := b.Inner.(Scrollable)
	if !ok {
		return true
	}
	// Corner cells (`─` cap rows) are non-interactive.
	if my == outer.Y || my == outer.Y+outer.H-1 {
		return true
	}
	_, ch := scr.ContentSize()
	_, sy := scr.ScrollOffset()
	if ch <= inner.H {
		return true
	}
	yTop := outer.Y + 1
	yBot := outer.Y + outer.H - 2
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
			maxScroll := ch - inner.H
			thumbY := trackTop
			if maxScroll > 0 && trackH > 1 {
				thumbY += (sy * (trackH - 1)) / maxScroll
			}
			page := inner.H - 1
			if page < 1 {
				page = 1
			}
			switch {
			case my < thumbY:
				scr.SetScrollOffset(0, sy-page)
			case my > thumbY:
				scr.SetScrollOffset(0, sy+page)
			default:
				// On the thumb — start a vertical drag. The
				// framework will deliver subsequent motion +
				// release events via MouseDragHandler.
				b.dragKind = boxedDragVThumb
				b.dragStart = trackTop
				b.dragEnd = trackBot
				b.dragMaxScr = maxScroll
			}
		}
	}
	return true
}

func (b *BoxedProvider) handleHScrollbarClick(mx int, outer, inner Rect) bool {
	scr, ok := b.Inner.(Scrollable)
	if !ok {
		return true
	}
	// Corner cells belong to the left/right borders.
	if mx == outer.X || mx == outer.X+outer.W-1 {
		return true
	}
	cw, _ := scr.ContentSize()
	sx, sy := scr.ScrollOffset()
	if cw <= inner.W {
		return true
	}
	xLeft := outer.X + 1
	xRight := outer.X + outer.W - 2
	switch {
	case mx == xLeft:
		scr.SetScrollOffset(sx-1, sy)
	case mx == xRight:
		scr.SetScrollOffset(sx+1, sy)
	default:
		trackL := xLeft + 1
		trackR := xRight - 1
		trackW := trackR - trackL + 1
		if trackW > 0 {
			maxScroll := cw - inner.W
			thumbX := trackL
			if maxScroll > 0 && trackW > 1 {
				thumbX += (sx * (trackW - 1)) / maxScroll
			}
			page := inner.W - 1
			if page < 1 {
				page = 1
			}
			switch {
			case mx < thumbX:
				scr.SetScrollOffset(sx-page, sy)
			case mx > thumbX:
				scr.SetScrollOffset(sx+page, sy)
			default:
				// On the thumb — start a horizontal drag.
				b.dragKind = boxedDragHThumb
				b.dragStart = trackL
				b.dragEnd = trackR
				b.dragMaxScr = maxScroll
			}
		}
	}
	return true
}

// HandleMouseMotion (MouseDragHandler) — updates the scroll offset
// proportionally as the captured thumb drag continues.
func (b *BoxedProvider) HandleMouseMotion(ev *tcell.EventMouse, _ Rect) {
	if b.dragKind == boxedDragNone {
		return
	}
	scr, ok := b.Inner.(Scrollable)
	if !ok {
		return
	}
	mx, my := ev.Position()
	sx, sy := scr.ScrollOffset()
	switch b.dragKind {
	case boxedDragVThumb:
		trackH := b.dragEnd - b.dragStart + 1
		if trackH <= 1 {
			return
		}
		rel := my - b.dragStart
		if rel < 0 {
			rel = 0
		}
		if rel > trackH-1 {
			rel = trackH - 1
		}
		newSy := (rel * b.dragMaxScr) / (trackH - 1)
		scr.SetScrollOffset(sx, newSy)
	case boxedDragHThumb:
		trackW := b.dragEnd - b.dragStart + 1
		if trackW <= 1 {
			return
		}
		rel := mx - b.dragStart
		if rel < 0 {
			rel = 0
		}
		if rel > trackW-1 {
			rel = trackW - 1
		}
		newSx := (rel * b.dragMaxScr) / (trackW - 1)
		scr.SetScrollOffset(newSx, sy)
	}
}

// HandleMouseRelease (MouseDragHandler) — clears the drag state.
func (b *BoxedProvider) HandleMouseRelease(ev *tcell.EventMouse, _ Rect) {
	b.dragKind = boxedDragNone
}

// HandleWheel forwards mouse-wheel events to the wrapped provider's
// Scrollable, since the box absorbs Scrollable from the outside.
func (b *BoxedProvider) HandleWheel(ev *tcell.EventMouse, _ Rect) bool {
	scr, ok := b.Inner.(Scrollable)
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

// StatusHint forwards to the wrapped provider.
func (b *BoxedProvider) StatusHint() string {
	if h, ok := b.Inner.(StatusHinter); ok {
		return h.StatusHint()
	}
	return ""
}
