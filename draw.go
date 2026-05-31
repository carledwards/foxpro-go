package foxpro

import "github.com/gdamore/tcell/v2"

// fillRect paints every cell of r with the given style and a single space.
func fillRect(screen tcell.Screen, r Rect, style tcell.Style) {
	fillRectRune(screen, r, ' ', style)
}

// fillRectRune paints every cell of r with rune ch and the given style.
// Used for the hatched desktop background.
func fillRectRune(screen tcell.Screen, r Rect, ch rune, style tcell.Style) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			screen.SetContent(x, y, ch, nil, style)
		}
	}
}

// drawString writes s starting at (x,y), advancing one cell per
// RUNE. No clipping past screen edge. `for i, r := range s` would
// advance i by the rune's UTF-8 byte count, which is fine for ASCII
// but puts multi-byte runes (e.g. "│") at the wrong column and
// leaves gaps for the bytes that "weren't" used. Counting runes
// ourselves keeps a 3-rune separator " │ " exactly 3 cells wide.
func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	col := 0
	for _, r := range s {
		screen.SetContent(x+col, y, r, nil, style)
		col++
	}
}

// shadeCell turns the cell at (x,y) into part of a drop shadow. It keeps the
// rune underneath but *darkens its existing colors* to a fraction of their
// brightness — a translucent-looking veil rather than a flat block — and tags
// the cell AttrDim so a pixel-layer host (the wasm canvas) can veil its own
// pixels to match. Because the shadow scales whatever is beneath it, it reads
// consistently over text, window chrome, and the pixel Screen alike. The style
// argument is ignored (kept for call-site compatibility).
func shadeCell(screen tcell.Screen, x, y int, _ tcell.Style) {
	mainc, combc, st, _ := screen.GetContent(x, y)
	if mainc == 0 {
		mainc = ' '
	}
	fg, bg, attrs := st.Decompose()
	screen.SetContent(x, y, mainc, combc, tcell.StyleDefault.
		Foreground(darkenColor(fg)).
		Background(darkenColor(bg)).
		Attributes(attrs|tcell.AttrDim))
}

// shadowBrightness is the percentage of its original brightness a drop-shadow
// cell keeps. ~30% matches the wasm host's translucent pixel veil
// (rgba(0,0,0,0.7) lets 30% of the canvas show through), so a single shadow
// reads the same over character cells and over a pixel layer.
const shadowBrightness = 30

// darkenColor scales a color toward black for drop-shadow veiling. Unset or
// invalid colors collapse to black.
func darkenColor(c tcell.Color) tcell.Color {
	if c == tcell.ColorDefault {
		return tcell.ColorBlack
	}
	r, g, b := c.RGB()
	if r < 0 {
		return tcell.ColorBlack
	}
	return tcell.NewRGBColor(r*shadowBrightness/100, g*shadowBrightness/100, b*shadowBrightness/100)
}

// drawWindow renders a window's drop shadow, frame, title bar, and delegates
// the inside to its ContentProvider.
func drawWindow(screen tcell.Screen, w *Window, theme Theme, settings Settings, focused bool) {
	if w.Borderless {
		// Borderless: provider paints frame + body itself. The
		// framework still draws the drop shadow so borderless
		// overlays (popups, tooltips) read as floating above
		// the desktop with the same visual cue as a regular
		// window. Disable globally via Settings.ShowShadows.
		if !w.shaded && settings.ShowShadows {
			b := w.Bounds
			const sox, soy = 2, 1
			for y := b.Y + soy; y < b.Y+soy+b.H; y++ {
				for x := b.X + sox; x < b.X+sox+b.W; x++ {
					if x < b.X+b.W && y < b.Y+b.H {
						continue
					}
					shadeCell(screen, x, y, theme.Shadow)
				}
			}
		}
		if w.Content != nil && w.Bounds.W > 0 && w.Bounds.H > 0 {
			w.Content.Draw(screen, w.Bounds, theme, focused)
		}
		return
	}
	if w.Dialog {
		drawDialogWindow(screen, w, theme, settings, focused)
		return
	}
	b := w.Bounds

	// Drop shadow: a full window-sized rect offset by (+2, +1). The window
	// itself will paint over the overlap area, so the visible shadow ends
	// up as a 2-column right strip + 1-row bottom strip — offset so the
	// top-right and bottom-left corners are clear. Each shadow cell keeps
	// the underlying rune but takes the theme.Shadow style, dimming any
	// chars underneath to a fixed darkgray-on-black look.
	// Shaded windows skip the shadow so they sit flat on the desktop.
	// Settings.ShowShadows can also disable shadows globally.
	if !w.shaded && settings.ShowShadows {
		const sox, soy = 2, 1
		for y := b.Y + soy; y < b.Y+soy+b.H; y++ {
			for x := b.X + sox; x < b.X+sox+b.W; x++ {
				if x < b.X+b.W && y < b.Y+b.H {
					continue // covered by the window itself
				}
				shadeCell(screen, x, y, theme.Shadow)
			}
		}
	}

	// Window background — fills the full bounds; the frame and controls
	// overpaint the perimeter cells. Skipped for shaded windows (H==1)
	// since they have no body to fill.
	if b.H >= 2 {
		fillRect(screen, b, theme.WindowBG)
	}

	// Frame: 1-cell ring around the content. Drawn as plain spaces in the
	// frame style (no box-drawing glyphs — FoxPro floating windows have no
	// visible border lines, just a coloured chrome strip).
	frameStyle := theme.TitleInactive
	if focused {
		frameStyle = theme.TitleActive
	}
	// Top row (always; shaded windows are this row only).
	for x := b.X; x < b.X+b.W; x++ {
		screen.SetContent(x, b.Y, ' ', nil, frameStyle)
	}
	// Side columns (only when there's room between top and bottom).
	if b.H >= 3 {
		for y := b.Y + 1; y < b.Y+b.H-1; y++ {
			screen.SetContent(b.X, y, ' ', nil, frameStyle)
			screen.SetContent(b.X+b.W-1, y, ' ', nil, frameStyle)
		}
	}
	// Bottom row (only when distinct from the top row).
	if b.H >= 2 {
		for x := b.X; x < b.X+b.W; x++ {
			screen.SetContent(x, b.Y+b.H-1, ' ', nil, frameStyle)
		}
	}

	// Active windows get yellow chrome (close, maximize, resize) and a
	// yellow title; inactive windows show no chrome and use the frame's
	// own foreground (dark gray by default) for the title text.
	leftSafe := 0
	rightSafe := b.W
	titleTextStyle := frameStyle
	if focused {
		accent := frameStyle.Foreground(theme.TitleAccent)
		titleTextStyle = accent
		if w.Closable && b.W >= 4 {
			screen.SetContent(b.X, b.Y, '■', nil, accent)
			leftSafe = 1
		}
		if w.Zoomable && b.W >= 4 {
			screen.SetContent(b.X+b.W-1, b.Y, '≡', nil, accent)
			rightSafe = b.W - 1
		}
		// Resize handle only on windows tall enough to have a distinct
		// bottom row (i.e. not shaded).
		if b.H >= 2 {
			screen.SetContent(b.X+b.W-1, b.Y+b.H-1, '.', nil, accent)
		}
	}

	// Title text — centred in the row, between any chrome controls.
	if w.Title != "" {
		avail := rightSafe - leftSafe
		title := w.Title
		if len(title) > avail {
			title = title[:avail]
		}
		if title != "" {
			tx := b.X + leftSafe + (avail-len(title))/2
			drawString(screen, tx, b.Y, title, titleTextStyle)
		}
	}

	// Inside (skipped when there's no body — shaded or 2-row windows).
	if w.Content != nil && b.H >= 3 && b.W >= 3 {
		w.Content.Draw(screen, b.Inner(), theme, focused)
	}

	// Scrollbars overlay the right column / bottom row when the content
	// provider implements Scrollable and the content extent exceeds the
	// visible viewport. Drawn on any window that has scrollable content
	// (not gated on focus) so they stay visible during transient
	// states like resize drags. Skipped when the window has no distinct
	// body — the horizontal bar's row would otherwise collide with the
	// title chrome on a shaded (1-row) window.
	if w.Content != nil && b.H >= 3 {
		if scr, ok := w.Content.(Scrollable); ok {
			drawScrollbars(screen, b, scr, frameStyle, theme.TitleAccent)
		}
	}
}

// drawScrollbars paints the optional vertical (right column) and
// horizontal (bottom row) scrollbars based on the Scrollable's reported
// content size and offset. The track itself is left as plain frame
// cells — only the arrows and thumb are drawn, so the bar blends with
// the rest of the chrome.
func drawScrollbars(screen tcell.Screen, b Rect, scr Scrollable, frame tcell.Style, accent tcell.Color) {
	_ = frame
	cw, ch := scr.ContentSize()
	sx, sy := scr.ScrollOffset()
	innerW := b.W - 2
	innerH := b.H - 2
	arrow := tcell.StyleDefault.Background(frameBG(frame)).Foreground(accent)

	// Vertical bar — column b.X+b.W-1, rows b.Y+1 .. b.Y+b.H-2.
	if ch > innerH && b.H >= 4 {
		col := b.X + b.W - 1
		topY := b.Y + 1
		botY := b.Y + b.H - 2
		screen.SetContent(col, topY, '▲', nil, arrow)
		screen.SetContent(col, botY, '▼', nil, arrow)
		trackTop := topY + 1
		trackBot := botY - 1
		trackH := trackBot - trackTop + 1
		if trackH > 0 {
			maxScroll := ch - innerH
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
			screen.SetContent(col, thumbY, '◆', nil, arrow)
		}
	}

	// Horizontal bar — row b.Y+b.H-1, cols b.X+1 .. b.X+b.W-2.
	if cw > innerW && b.W >= 4 {
		row := b.Y + b.H - 1
		leftX := b.X + 1
		rightX := b.X + b.W - 2
		screen.SetContent(leftX, row, '◄', nil, arrow)
		screen.SetContent(rightX, row, '►', nil, arrow)
		trackL := leftX + 1
		trackR := rightX - 1
		trackW := trackR - trackL + 1
		if trackW > 0 {
			maxScroll := cw - innerW
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
			screen.SetContent(thumbX, row, '◆', nil, arrow)
		}
	}
}

// frameBG extracts the background colour of a style for use when
// recomposing chrome glyphs (so they sit on the same colour as the
// frame they overlay).
func frameBG(s tcell.Style) tcell.Color {
	_, bg, _ := s.Decompose()
	return bg
}

// drawDialogWindow renders a Window flagged as Dialog: a double-line
// ╔═══╗ border around a magenta body using theme.Dialog. The window's
// title (if non-empty) is embedded in the top border as " Title ",
// centered, in yellow-on-magenta when focused.
//
// Content area (what gets passed to Content.Draw) is the full rect
// strictly inside the border (DialogInner) — the title lives in the
// border, so the provider lays out from y=0 with no title row to skip.
//
// Dialogs ignore w.Closable / w.Zoomable / shading / scrollbars —
// any chrome embedded in the dialog (OK button, Cancel button) is
// the provider's job. Resize handle is also suppressed.
func drawDialogWindow(screen tcell.Screen, w *Window, theme Theme, settings Settings, focused bool) {
	b := w.Bounds
	scheme := theme.Dialog

	if scheme.CastsShadow && settings.ShowShadows {
		const sox, soy = 2, 1
		for y := b.Y + soy; y < b.Y+soy+b.H; y++ {
			for x := b.X + sox; x < b.X+sox+b.W; x++ {
				if x < b.X+b.W && y < b.Y+b.H {
					continue
				}
				shadeCell(screen, x, y, scheme.Shadow)
			}
		}
	}

	// Magenta body (full bounds) — border overpaints the perimeter.
	fillRect(screen, b, scheme.NormalText)

	// Single-line ┌───┐ border at the perimeter.
	if b.W >= 2 && b.H >= 2 {
		drawBoxBorder(screen, b, scheme.Border, dialogBorderBox)
	}

	// Title — embedded in the top border as " Title ", centered, in
	// TitleActive style when focused, TitleIdle when not. Skipped when
	// w.Title is empty. The content rect is the full interior (the title
	// sits in the border, not on a body row).
	if w.Title != "" && b.W > 4 {
		titleStyle := scheme.TitleIdle
		if focused {
			titleStyle = scheme.TitleActive
		}
		title := " " + w.Title + " "
		if len(title) > b.W-2 {
			title = title[:b.W-2]
		}
		tx := b.X + 1 + (b.W-2-len(title))/2
		drawString(screen, tx, b.Y, title, titleStyle)
	}

	if w.Content != nil {
		ci := DialogInner(b)
		if ci.W >= 1 && ci.H >= 1 {
			w.Content.Draw(screen, ci, theme, focused)
		}
	}
}

// DialogInner returns the content rect of a Dialog window — the area
// strictly inside the ╔═══╗ border (1 cell in from b on every side).
// The title is drawn in the top border, so the full interior is the
// content area.
func DialogInner(b Rect) Rect {
	return Rect{X: b.X + 1, Y: b.Y + 1, W: b.W - 2, H: b.H - 2}
}

// dialogBorderBox is the dialog-border glyph set — double line for the
// classic DOS / FoxPro dialog look. (Some bitmap/canvas fonts render the
// double-line runes with small vertical gaps between cell rows; that's a
// known cosmetic trade-off on the wasm canvas.)
var dialogBorderBox = boxGlyphs{tl: '╔', tr: '╗', bl: '╚', br: '╝', h: '═', v: '║'}

// boxGlyphs bundles the six runes that make up a rectangular border.
type boxGlyphs struct{ tl, tr, bl, br, h, v rune }

// drawBoxBorder paints a 1-cell border around r using the given
// style and glyph set. r.W and r.H must be ≥ 2.
func drawBoxBorder(screen tcell.Screen, r Rect, style tcell.Style, g boxGlyphs) {
	right := r.X + r.W - 1
	bot := r.Y + r.H - 1
	// Top + bottom edges.
	for x := r.X + 1; x < right; x++ {
		screen.SetContent(x, r.Y, g.h, nil, style)
		screen.SetContent(x, bot, g.h, nil, style)
	}
	// Left + right edges.
	for y := r.Y + 1; y < bot; y++ {
		screen.SetContent(r.X, y, g.v, nil, style)
		screen.SetContent(right, y, g.v, nil, style)
	}
	// Corners.
	screen.SetContent(r.X, r.Y, g.tl, nil, style)
	screen.SetContent(right, r.Y, g.tr, nil, style)
	screen.SetContent(r.X, bot, g.bl, nil, style)
	screen.SetContent(right, bot, g.br, nil, style)
}
