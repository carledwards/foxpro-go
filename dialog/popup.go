// popup.go: a small modal "choose one" overlay anchored under a
// trigger control. Different from Picker (the full magenta dialog
// with description box and OK / Cancel buttons): Popup is a
// borderless cyan box with a thin frame, the current row in
// theme.Focus (brown), and only the bare-minimum interaction —
// click / Enter to commit, Esc / click-outside to dismiss.
//
// Visual model: the FoxPro spinner-style dropdown that appears
// under a "field" widget when the user clicks it (image 5 in the
// design notes). Cyan body, thin white frame, drop shadow.

package dialog

import (
	"github.com/gdamore/tcell/v2"

	foxpro "github.com/carledwards/foxpro-go"
)

// Popup is a foxpro.ContentProvider that renders the dropdown.
// Don't construct directly — call NewPopupWindow.
type Popup struct {
	options  []string
	selected int

	rowRects []foxpro.Rect

	onPick func(idx int)
	close  func()
}

// NewPopupWindow returns a sized, modal, borderless Window ready
// for app.Manager.Add. anchorX / anchorY position the popup's
// top-left corner on screen — usually just below the trigger
// control, in screen coords from app.Screen.Size().
//
// The popup draws its own cyan body + thin ┌──┐ frame and looks
// like a small dialog overlay. It is modal (Dialog=true blocks
// input to other windows) but cannot be moved (Borderless=true
// suppresses the title-bar drag zone — there is no title row).
//
// onPick is called with the chosen index when the user confirms.
// The window is closed (removed from the manager via OnClose) on
// either confirm or cancel — caller wires that up in main.go after
// NewPopupWindow returns.
func NewPopupWindow(opts []string, current, anchorX, anchorY int, onPick func(idx int)) *foxpro.Window {
	w, h := popupSize(opts)
	var win *foxpro.Window
	closeFn := func() {
		if win != nil && win.OnClose != nil {
			win.OnClose()
		}
	}
	p := &Popup{
		options:  opts,
		selected: current,
		onPick:   onPick,
		close:    closeFn,
		rowRects: make([]foxpro.Rect, len(opts)),
	}
	if p.selected < 0 || p.selected >= len(opts) {
		p.selected = 0
	}
	win = foxpro.NewWindow("", foxpro.Rect{X: anchorX, Y: anchorY, W: w, H: h}, p)
	win.Dialog = true     // modal: clicks outside don't raise other windows
	win.Borderless = true // provider paints frame + body itself
	win.Closable = false
	win.Zoomable = false
	return win
}

// popupSize computes the (W, H) bounding box for the popup. Inner
// width = longest option rune count + 2 cells of padding (1 on each
// side, so the longest option doesn't touch the border). Total width
// adds 2 cells for the frame. Height = rows + 2 cells of frame.
func popupSize(opts []string) (w, h int) {
	maxLen := 0
	for _, o := range opts {
		if n := runeLen(o); n > maxLen {
			maxLen = n
		}
	}
	w = maxLen + 4 // 1 frame + 1 pad + maxLen + 1 pad + 1 frame
	h = len(opts) + 2
	return
}

func (p *Popup) Draw(screen tcell.Screen, bounds foxpro.Rect, theme foxpro.Theme, focused bool) {
	body := theme.WindowBG
	hi := theme.Focus
	frame := theme.Border

	// Fill the body.
	for y := 0; y < bounds.H; y++ {
		fillRow(screen, bounds.X, bounds.Y+y, bounds.W, body)
	}

	// Single-line ┌──┐ border — popups read as a thin overlay over
	// the underlying surface. The 3D "shadow" mixed-line border
	// (┌──╖ / ╘══╝) is reserved for clickable picker fields on
	// the underlying chrome.
	right := bounds.X + bounds.W - 1
	bot := bounds.Y + bounds.H - 1
	for x := bounds.X + 1; x < right; x++ {
		screen.SetContent(x, bounds.Y, '─', nil, frame)
		screen.SetContent(x, bot, '─', nil, frame)
	}
	for y := bounds.Y + 1; y < bot; y++ {
		screen.SetContent(bounds.X, y, '│', nil, frame)
		screen.SetContent(right, y, '│', nil, frame)
	}
	screen.SetContent(bounds.X, bounds.Y, '┌', nil, frame)
	screen.SetContent(right, bounds.Y, '┐', nil, frame)
	screen.SetContent(bounds.X, bot, '└', nil, frame)
	screen.SetContent(right, bot, '┘', nil, frame)

	// Inner area for option rows.
	rowY0 := bounds.Y + 1
	rowX := bounds.X + 1
	rowW := bounds.W - 2

	// Rows are left-justified with one cell of leading padding —
	// FoxPro popups read like a stacked menu, not a centered list.
	// The highlight bar still spans the full inner width so the
	// selection looks like a row, not a tag.
	for i, opt := range p.options {
		ly := rowY0 + i
		if ly >= bot {
			break
		}
		st := body
		if i == p.selected {
			st = hi
			fillRow(screen, rowX, ly, rowW, hi)
		}
		drawRunes(screen, rowX+1, ly, opt, st)
		p.rowRects[i] = foxpro.Rect{X: rowX, Y: ly, W: rowW, H: 1}
	}
}

func (p *Popup) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if p.selected > 0 {
			p.selected--
		}
		return true
	case tcell.KeyDown:
		if p.selected < len(p.options)-1 {
			p.selected++
		}
		return true
	case tcell.KeyEnter:
		p.confirm()
		return true
	case tcell.KeyEscape:
		p.cancel()
		return true
	case tcell.KeyRune:
		if ev.Rune() == ' ' {
			p.confirm()
			return true
		}
	}
	return false
}

func (p *Popup) HandleMouse(ev *tcell.EventMouse, _ foxpro.Rect) bool {
	if ev.Buttons()&tcell.Button1 == 0 {
		return false
	}
	mx, my := ev.Position()
	for i, r := range p.rowRects {
		if r.Contains(mx, my) {
			p.selected = i
			p.confirm()
			return true
		}
	}
	// Click outside the popup body cancels — typical dropdown UX.
	p.cancel()
	return true
}

func (p *Popup) confirm() {
	if p.onPick != nil && p.selected >= 0 && p.selected < len(p.options) {
		p.onPick(p.selected)
	}
	if p.close != nil {
		p.close()
	}
}

func (p *Popup) cancel() {
	if p.close != nil {
		p.close()
	}
}

func fillRow(screen tcell.Screen, x, y, w int, st tcell.Style) {
	for i := 0; i < w; i++ {
		screen.SetContent(x+i, y, ' ', nil, st)
	}
}

func drawRunes(screen tcell.Screen, x, y int, s string, st tcell.Style) {
	col := 0
	for _, r := range s {
		screen.SetContent(x+col, y, r, nil, st)
		col++
	}
}
