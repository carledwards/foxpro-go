// Package widgets provides stateless drawing primitives for composing
// FoxPro-style screens inside a foxpro.ContentProvider.
//
// The package intentionally has no dependency on the core foxpro package
// — widget functions take tcell.Style values (and an accent tcell.Color
// where needed) directly, so callers decide which theme slots to feed in.
// This keeps the dependency arrow one-way (core may import widgets;
// widgets never imports core) and prevents cycles.
package widgets

import "github.com/gdamore/tcell/v2"

// CheckboxGlyph picks the box character pair for a checkbox.
func CheckboxGlyph(checked bool) string {
	if checked {
		return "[X]"
	}
	return "[ ]"
}

// RadioGlyph picks the box character pair for a radio button.
func RadioGlyph(selected bool) string {
	if selected {
		return "(•)"
	}
	return "( )"
}

// DrawCheckbox renders "[X] Label" or "[ ] Label" at (x,y).
// When highlighted, the entire row out to width is filled with the
// highlight style so the user can see which row is focused. The label
// is truncated to fit within width — nothing is drawn past x+width-1.
func DrawCheckbox(screen tcell.Screen, x, y, width int, checked bool, label string, highlighted bool, normal, highlight tcell.Style) {
	if width <= 0 {
		return
	}
	style := normal
	if highlighted {
		style = highlight
		fillSpan(screen, x, y, width, style)
	}
	drawClipped(screen, x, y, width, CheckboxGlyph(checked), style)
	if width > 4 {
		drawClipped(screen, x+4, y, width-4, label, style)
	}
}

// DrawRadio renders "(•) Label" or "( ) Label" at (x,y), clipped to width.
func DrawRadio(screen tcell.Screen, x, y, width int, selected bool, label string, highlighted bool, normal, highlight tcell.Style) {
	if width <= 0 {
		return
	}
	style := normal
	if highlighted {
		style = highlight
		fillSpan(screen, x, y, width, style)
	}
	drawClipped(screen, x, y, width, RadioGlyph(selected), style)
	if width > 4 {
		drawClipped(screen, x+4, y, width-4, label, style)
	}
}

// DrawListRow renders one row of a selectable list, clipped to width.
func DrawListRow(screen tcell.Screen, x, y, width int, label string, highlighted bool, normal, highlight tcell.Style) {
	if width <= 0 {
		return
	}
	style := normal
	if highlighted {
		style = highlight
	}
	fillSpan(screen, x, y, width, style)
	if width > 1 {
		drawClipped(screen, x+1, y, width-1, label, style)
	}
}

// DrawButton renders "[ Label ]" inside a single-line bordered box at
// (x,y) with size (w,h). The label gets the accent foreground when
// focused; otherwise it uses bg's foreground.
func DrawButton(screen tcell.Screen, x, y, w, h int, label string, focused bool, bg tcell.Style, accent tcell.Color) {
	fillRect(screen, x, y, w, h, bg)
	for cx := x + 1; cx < x+w-1; cx++ {
		screen.SetContent(cx, y, '─', nil, bg)
		screen.SetContent(cx, y+h-1, '─', nil, bg)
	}
	for cy := y + 1; cy < y+h-1; cy++ {
		screen.SetContent(x, cy, '│', nil, bg)
		screen.SetContent(x+w-1, cy, '│', nil, bg)
	}
	screen.SetContent(x, y, '┌', nil, bg)
	screen.SetContent(x+w-1, y, '┐', nil, bg)
	screen.SetContent(x, y+h-1, '└', nil, bg)
	screen.SetContent(x+w-1, y+h-1, '┘', nil, bg)

	labelStyle := bg
	if focused {
		labelStyle = bg.Foreground(accent)
	}
	if h >= 1 && len(label) <= w-2 {
		midY := y + h/2
		midX := x + (w-len(label))/2
		drawString(screen, midX, midY, label, labelStyle)
	}
}

// SeparatorRow draws a horizontal `─` divider of the given width.
func SeparatorRow(screen tcell.Screen, x, y, width int, style tcell.Style) {
	for i := 0; i < width; i++ {
		screen.SetContent(x+i, y, '─', nil, style)
	}
}

// drawString writes s starting at (x,y) in the given style.
func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for i, r := range s {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

// drawClipped writes s starting at (x,y), stopping at maxWidth runes so
// nothing renders past x+maxWidth-1.
func drawClipped(screen tcell.Screen, x, y, maxWidth int, s string, style tcell.Style) {
	if maxWidth <= 0 {
		return
	}
	count := 0
	for _, r := range s {
		if count >= maxWidth {
			return
		}
		screen.SetContent(x+count, y, r, nil, style)
		count++
	}
}

// fillSpan fills `width` cells starting at (x,y) with spaces in style.
func fillSpan(screen tcell.Screen, x, y, width int, style tcell.Style) {
	for i := 0; i < width; i++ {
		screen.SetContent(x+i, y, ' ', nil, style)
	}
}

// fillRect fills the (w,h) rect at (x,y) with spaces in style.
func fillRect(screen tcell.Screen, x, y, w, h int, style tcell.Style) {
	for cy := y; cy < y+h; cy++ {
		for cx := x; cx < x+w; cx++ {
			screen.SetContent(cx, cy, ' ', nil, style)
		}
	}
}
