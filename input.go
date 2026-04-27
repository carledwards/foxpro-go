package foxpro

import "github.com/gdamore/tcell/v2"

// InputProvider is a single-line text input rendered as a flat gray
// strip (Theme.Input). The buffer scrolls horizontally so the cursor
// stays inside the visible window even with long text.
//
// Standard editing: Left/Right move the cursor, Home/End jump,
// Backspace and Delete edit, printable runes insert at the cursor.
// Enter fires OnSubmit; Esc clears the buffer (and only consumes the
// event when there was text to clear, so an empty Esc still
// propagates to app shortcuts). OnChange fires after every text
// mutation.
type InputProvider struct {
	Text        string
	Placeholder string
	OnChange    func(text string)
	OnSubmit    func(text string)

	cursorPos int
	scrollX   int
}

// NewInputProvider returns an empty input with the given placeholder
// (drawn dimly when the buffer is empty and the input is not focused).
func NewInputProvider(placeholder string) *InputProvider {
	return &InputProvider{Placeholder: placeholder}
}

// SetText replaces the buffer (and resets the cursor to the end).
// Does NOT fire OnChange — caller can call it explicitly if needed.
func (p *InputProvider) SetText(text string) {
	p.Text = text
	p.cursorPos = len(p.Text)
}

func (p *InputProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	if inner.W <= 0 || inner.H <= 0 {
		return
	}
	style := theme.Input

	// Adjust horizontal scroll so the cursor is visible.
	if p.cursorPos < p.scrollX {
		p.scrollX = p.cursorPos
	} else if p.cursorPos >= p.scrollX+inner.W {
		p.scrollX = p.cursorPos - inner.W + 1
	}
	if p.scrollX < 0 {
		p.scrollX = 0
	}

	// Fill the visible row with the input background.
	for x := inner.X; x < inner.X+inner.W; x++ {
		screen.SetContent(x, inner.Y, ' ', nil, style)
	}

	// Render either the visible window of the buffer or the
	// dimmed placeholder when empty + unfocused.
	if p.Text == "" && !focused && p.Placeholder != "" {
		ph := p.Placeholder
		if len(ph) > inner.W {
			ph = ph[:inner.W]
		}
		dim := style.Foreground(theme.Palette.DarkGray)
		for i, r := range ph {
			screen.SetContent(inner.X+i, inner.Y, r, nil, dim)
		}
	} else {
		visible := p.Text
		if p.scrollX < len(visible) {
			visible = visible[p.scrollX:]
		} else {
			visible = ""
		}
		if len(visible) > inner.W {
			visible = visible[:inner.W]
		}
		for i, r := range visible {
			screen.SetContent(inner.X+i, inner.Y, r, nil, style)
		}
	}

	if focused {
		cx := inner.X + (p.cursorPos - p.scrollX)
		if cx < inner.X {
			cx = inner.X
		}
		if cx >= inner.X+inner.W {
			cx = inner.X + inner.W - 1
		}
		// Blinking block cursor — terminals fall back to whatever
		// cursor style they support if styling is unsupported.
		screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
		screen.ShowCursor(cx, inner.Y)
	}
}

func (p *InputProvider) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter:
		if p.OnSubmit != nil {
			p.OnSubmit(p.Text)
		}
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if p.cursorPos > 0 {
			p.Text = p.Text[:p.cursorPos-1] + p.Text[p.cursorPos:]
			p.cursorPos--
			p.fireChange()
		}
		return true
	case tcell.KeyDelete:
		if p.cursorPos < len(p.Text) {
			p.Text = p.Text[:p.cursorPos] + p.Text[p.cursorPos+1:]
			p.fireChange()
		}
		return true
	case tcell.KeyLeft:
		if p.cursorPos > 0 {
			p.cursorPos--
		}
		return true
	case tcell.KeyRight:
		if p.cursorPos < len(p.Text) {
			p.cursorPos++
		}
		return true
	case tcell.KeyHome:
		p.cursorPos = 0
		return true
	case tcell.KeyEnd:
		p.cursorPos = len(p.Text)
		return true
	case tcell.KeyEscape:
		if p.Text != "" {
			p.Text = ""
			p.cursorPos = 0
			p.scrollX = 0
			p.fireChange()
			return true
		}
		return false
	case tcell.KeyRune:
		ch := string(ev.Rune())
		p.Text = p.Text[:p.cursorPos] + ch + p.Text[p.cursorPos:]
		p.cursorPos += len(ch)
		p.fireChange()
		return true
	}
	return false
}

func (p *InputProvider) fireChange() {
	if p.OnChange != nil {
		p.OnChange(p.Text)
	}
}

// StatusHint contributes a contextual key hint to the bottom status bar.
func (p *InputProvider) StatusHint() string {
	return "type to filter  Enter: submit  Esc: clear "
}
