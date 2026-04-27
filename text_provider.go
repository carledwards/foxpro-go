package foxpro

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// TextProvider is a trivial ContentProvider: a slice of lines drawn
// top-down with vertical and horizontal scroll offsets. Implements
// Scrollable so the framework auto-renders scrollbars and routes
// wheel / arrow / track / thumb interactions.
type TextProvider struct {
	Lines []string

	offsetY, offsetX int
	lastVH, lastVW   int
}

// NewTextProvider wraps the given lines.
func NewTextProvider(lines []string) *TextProvider {
	return &TextProvider{Lines: lines}
}

// Append adds a line to the end (handy for log-style windows).
func (p *TextProvider) Append(line string) {
	p.Lines = append(p.Lines, line)
}

func (p *TextProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	style := theme.WindowBG
	p.lastVW = inner.W
	p.lastVH = inner.H
	p.clamp()
	for row := 0; row < inner.H && p.offsetY+row < len(p.Lines); row++ {
		runes := []rune(p.Lines[p.offsetY+row])
		if p.offsetX >= len(runes) {
			continue
		}
		visible := runes[p.offsetX:]
		for i, r := range visible {
			if i >= inner.W {
				break
			}
			screen.SetContent(inner.X+i, inner.Y+row, r, nil, style)
		}
	}
}

// ContentSize returns the longest line (in runes) and total line count.
func (p *TextProvider) ContentSize() (int, int) {
	w := 0
	for _, line := range p.Lines {
		ln := utf8.RuneCountInString(line)
		if ln > w {
			w = ln
		}
	}
	return w, len(p.Lines)
}

// ScrollOffset returns (horizontal, vertical) offsets in cells.
func (p *TextProvider) ScrollOffset() (int, int) {
	p.clamp()
	return p.offsetX, p.offsetY
}

// SetScrollOffset clamps both axes into their valid ranges.
func (p *TextProvider) SetScrollOffset(x, y int) {
	p.offsetX = x
	p.offsetY = y
	p.clamp()
}

func (p *TextProvider) clamp() {
	cw, ch := p.ContentSize()
	if maxY := ch - p.lastVH; maxY >= 0 && p.offsetY > maxY {
		p.offsetY = maxY
	}
	if p.offsetY < 0 {
		p.offsetY = 0
	}
	if maxX := cw - p.lastVW; maxX >= 0 && p.offsetX > maxX {
		p.offsetX = maxX
	}
	if p.offsetX < 0 {
		p.offsetX = 0
	}
}

func (p *TextProvider) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.offsetY--
		p.clamp()
		return true
	case tcell.KeyDown:
		p.offsetY++
		p.clamp()
		return true
	case tcell.KeyLeft:
		p.offsetX--
		p.clamp()
		return true
	case tcell.KeyRight:
		p.offsetX++
		p.clamp()
		return true
	case tcell.KeyPgUp:
		step := p.lastVH - 1
		if step < 1 {
			step = 1
		}
		p.offsetY -= step
		p.clamp()
		return true
	case tcell.KeyPgDn:
		step := p.lastVH - 1
		if step < 1 {
			step = 1
		}
		p.offsetY += step
		p.clamp()
		return true
	case tcell.KeyHome:
		p.offsetY = 0
		p.offsetX = 0
		return true
	case tcell.KeyEnd:
		if len(p.Lines) > 0 {
			p.offsetY = len(p.Lines)
		}
		p.clamp()
		return true
	}
	return false
}
