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

// drawString writes s starting at (x,y). No clipping past screen edge.
func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for i, r := range s {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

// shadeCell re-paints the cell at (x,y) with its existing rune but the
// given style (both fg and bg replaced). Used for the drop shadow, which
// dims whatever character is underneath to a fixed darkgray-on-black look.
func shadeCell(screen tcell.Screen, x, y int, style tcell.Style) {
	mainc, combc, _, _ := screen.GetContent(x, y)
	if mainc == 0 {
		mainc = ' '
	}
	screen.SetContent(x, y, mainc, combc, style)
}

// drawWindow renders a window's drop shadow, frame, title bar, and delegates
// the inside to its ContentProvider.
func drawWindow(screen tcell.Screen, w *Window, theme Theme, settings Settings, focused bool) {
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
