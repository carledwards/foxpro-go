// Package dialog hosts the standard FoxPro-style modal dialogs.
//
// Picker is a "choose one of N options" dialog: a radio group on top,
// a multi-line cyan description box that updates as the user
// navigates, and an OK / Cancel button row at the bottom. Same shape
// as the FoxPro printer-selection / find-replace dialogs.
//
// Use NewWindow to get a ready-to-add *foxpro.Window. The framework
// already enforces modality on Window.Dialog=true, so callers don't
// need to block input themselves.
package dialog
import (
	"github.com/gdamore/tcell/v2"

	foxpro "github.com/carledwards/foxpro-go"
	"github.com/carledwards/foxpro-go/widgets"
)

// Option describes one choice in a Picker.
type Option struct {
	// Name is the host-side identifier passed to OnConfirm. The
	// host uses it to dispatch (e.g. "interp" / "netsim", or a
	// demo's filename).
	Name string
	// Label is the human-readable list entry.
	Label string
	// Description is the multi-line teaching blurb shown in the
	// cyan box when this option is highlighted. Each entry is one
	// display line; the box truncates beyond DescLines.
	Description []string
}

// DescLines is the height of the description text-box's interior
// (excluding its own border). Three lines fits a tight blurb without
// dominating the dialog.
const DescLines = 3

type focusZone int

const (
	zoneList focusZone = iota
	zoneOK
	zoneCancel
)

// Picker is a foxpro.ContentProvider that renders the choose-one
// dialog. Don't construct directly — call NewWindow.
type Picker struct {
	options []Option
	sel     int
	zone    focusZone

	okRect     foxpro.Rect
	cancelRect foxpro.Rect
	rowRects   []foxpro.Rect

	onConfirm func(name string)
	onCancel  func()
	close     func()
}

// New builds a Picker with the given options. current names the
// option that should be pre-highlighted at open (matched by
// Option.Name); pass "" to start at the first option. onConfirm
// fires when the user confirms a selection; onCancel fires on Esc /
// Cancel. closeFn is called after either to remove the window from
// the manager — typically a small closure capturing the window and
// app.
func New(opts []Option, current string, onConfirm func(name string), onCancel func(), closeFn func()) *Picker {
	p := &Picker{
		options:   opts,
		onConfirm: onConfirm,
		onCancel:  onCancel,
		close:     closeFn,
		zone:      zoneList,
		rowRects:  make([]foxpro.Rect, len(opts)),
	}
	for i, o := range opts {
		if o.Name == current {
			p.sel = i
			break
		}
	}
	return p
}

// NewWindow wraps Picker in a sized, centered, modal Window ready
// for app.Manager.Add. Centering uses screenW × screenH — pull these
// from app.Screen.Size(). title appears centered on the top body row
// in the dialog's yellow accent.
//
// Layout adapts to the options: if at least one Option has a non-
// empty Description, the dialog renders the cyan description box
// and reserves 70×(rows+desc) cells. If every Description is empty
// the dialog collapses to a tighter 50-cell-wide box just big
// enough for the radios + buttons.
func NewWindow(title string, opts []Option, current string, onConfirm func(name string), onCancel func(), screenW, screenH int) *foxpro.Window {
	width, height := pickerSize(opts)
	var w *foxpro.Window
	closeFn := func() {
		if w != nil && w.OnClose != nil {
			w.OnClose()
		}
	}
	p := New(opts, current, onConfirm, onCancel, closeFn)
	w = foxpro.NewWindow(title, foxpro.Rect{X: 0, Y: 0, W: width, H: height}, p)
	w.Dialog = true
	w.Closable = false
	w.Zoomable = false
	w.Center(screenW, screenH)
	return w
}

// pickerSize returns the (W, H) bounding box for a Picker with the
// given options. Width is fixed at 70 (with description) or 50
// (compact). Height grows with the number of radio rows; with a
// description the cyan box adds DescLines+2 rows plus a label row.
func pickerSize(opts []Option) (w, h int) {
	hasDesc := false
	for _, o := range opts {
		if len(o.Description) > 0 {
			hasDesc = true
			break
		}
	}
	// Fixed rows: top border, title, blank, [radio rows], blank,
	// button, blank, bottom border = 7. With description add a
	// label row + the cyan box (DescLines + 2) = DescLines + 3.
	base := 7 + len(opts)
	if hasDesc {
		return 70, base + DescLines + 3
	}
	return 50, base
}

func (p *Picker) Draw(screen tcell.Screen, inner foxpro.Rect, theme foxpro.Theme, focused bool) {
	scheme := theme.Dialog
	body := scheme.NormalText
	hi := scheme.SelectedItem
	textBox := scheme.TextBox

	rowOpt := inner.Y + 1
	rowDescLabel := rowOpt + len(p.options) + 1
	rowDescBoxTop := rowDescLabel + 1
	rowBtn := inner.Y + inner.H - 1

	// Radio group. FoxPro convention: in a radio group the dot
	// follows highlight — Up/Down moves both, no separate
	// "highlighted but unselected" state. Stripe is just wide
	// enough to wrap the marker + longest label with one cell of
	// padding on each side.
	indent := 6
	maxLabel := 0
	for _, o := range p.options {
		if n := runeLen(o.Label); n > maxLabel {
			maxLabel = n
		}
	}
	const markerW = 3 // "(•)" / "( )"
	const gap = 1
	hiW := markerW + gap + maxLabel + 2
	hiX := inner.X + indent - 1
	if hiX < inner.X+1 {
		hiX = inner.X + 1
	}
	if hiX+hiW > inner.X+inner.W-1 {
		hiW = inner.X + inner.W - 1 - hiX
	}
	for i, o := range p.options {
		ly := rowOpt + i
		if ly >= rowDescLabel {
			break
		}
		marker := "( )"
		if i == p.sel {
			marker = "(•)"
		}
		st := body
		if p.zone == zoneList && i == p.sel {
			st = hi
			fillSpan(screen, hiX, ly, hiW, st)
		}
		drawAt(screen, inner.X+indent, ly, marker, st)
		drawAt(screen, inner.X+indent+markerW+gap, ly, clip(o.Label, hiW-markerW-gap-1), st)
		p.rowRects[i] = foxpro.Rect{X: hiX, Y: ly, W: hiW, H: 1}
	}

	hasDesc := false
	for _, o := range p.options {
		if len(o.Description) > 0 {
			hasDesc = true
			break
		}
	}
	if hasDesc && rowDescLabel < rowBtn-1 {
		drawAt(screen, inner.X+indent, rowDescLabel, "Description:", body)
		boxX := inner.X + indent
		boxW := inner.W - 2*indent
		boxH := DescLines + 2
		boxBot := rowDescBoxTop + boxH - 1
		if boxBot < rowBtn-1 && boxW >= 4 {
			drawDescBox(screen, boxX, rowDescBoxTop, boxW, boxH, textBox)
			if p.sel >= 0 && p.sel < len(p.options) {
				lines := p.options[p.sel].Description
				for i := 0; i < DescLines && i < len(lines); i++ {
					drawAt(screen, boxX+2, rowDescBoxTop+1+i, clip(lines[i], boxW-4), textBox)
				}
			}
		}
	}

	// Buttons. OK pads to match Cancel's width so both rendered
	// chevron-buttons end up the same 10-cell width.
	okLabel := "  OK  "
	cancelLabel := "Cancel"
	cancelW := len("< ") + runeLen(cancelLabel) + len(" >")
	okX := inner.X + inner.W/4
	cancelX := inner.X + 3*inner.W/4 - cancelW
	okW := widgets.DrawDialogButton(screen, okX, rowBtn, okLabel, true, p.zone == zoneOK, scheme.EnabledCtrl, scheme.SelectedItem)
	widgets.DrawDialogButton(screen, cancelX, rowBtn, cancelLabel, false, p.zone == zoneCancel, scheme.EnabledCtrl, scheme.SelectedItem)
	p.okRect = foxpro.Rect{X: okX, Y: rowBtn, W: okW, H: 1}
	p.cancelRect = foxpro.Rect{X: cancelX, Y: rowBtn, W: cancelW, H: 1}
}

// drawDescBox draws a single-line ┌──┐ frame at (x,y,w,h) in style
// st with the interior filled in the same style.
func drawDescBox(screen tcell.Screen, x, y, w, h int, st tcell.Style) {
	right := x + w - 1
	bot := y + h - 1
	for cy := y; cy <= bot; cy++ {
		for cx := x; cx <= right; cx++ {
			screen.SetContent(cx, cy, ' ', nil, st)
		}
	}
	for cx := x + 1; cx < right; cx++ {
		screen.SetContent(cx, y, '─', nil, st)
		screen.SetContent(cx, bot, '─', nil, st)
	}
	for cy := y + 1; cy < bot; cy++ {
		screen.SetContent(x, cy, '│', nil, st)
		screen.SetContent(right, cy, '│', nil, st)
	}
	screen.SetContent(x, y, '┌', nil, st)
	screen.SetContent(right, y, '┐', nil, st)
	screen.SetContent(x, bot, '└', nil, st)
	screen.SetContent(right, bot, '┘', nil, st)
}

func (p *Picker) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyTab:
		p.zone = (p.zone + 1) % 3
		return true
	case tcell.KeyBacktab:
		p.zone = (p.zone + 2) % 3
		return true
	case tcell.KeyUp:
		if p.zone == zoneList && p.sel > 0 {
			p.sel--
		}
		return true
	case tcell.KeyDown:
		if p.zone == zoneList && p.sel < len(p.options)-1 {
			p.sel++
		}
		return true
	case tcell.KeyEnter:
		if p.zone == zoneCancel {
			p.cancel()
		} else {
			p.confirm()
		}
		return true
	case tcell.KeyEscape:
		p.cancel()
		return true
	case tcell.KeyRune:
		if ev.Rune() == ' ' {
			if p.zone == zoneCancel {
				p.cancel()
			} else {
				p.confirm()
			}
			return true
		}
	}
	return false
}

// HandleMouse: clicking a radio row jumps the highlight (and the
// dot) there; clicking a button fires it.
func (p *Picker) HandleMouse(ev *tcell.EventMouse, _ foxpro.Rect) bool {
	if ev.Buttons()&tcell.Button1 == 0 {
		return false
	}
	mx, my := ev.Position()
	for i, r := range p.rowRects {
		if r.Contains(mx, my) {
			p.zone = zoneList
			p.sel = i
			return true
		}
	}
	if p.okRect.Contains(mx, my) {
		p.zone = zoneOK
		p.confirm()
		return true
	}
	if p.cancelRect.Contains(mx, my) {
		p.zone = zoneCancel
		p.cancel()
		return true
	}
	return false
}

func (p *Picker) confirm() {
	if p.sel < 0 || p.sel >= len(p.options) {
		return
	}
	if p.onConfirm != nil {
		p.onConfirm(p.options[p.sel].Name)
	}
	if p.close != nil {
		p.close()
	}
}

func (p *Picker) cancel() {
	if p.onCancel != nil {
		p.onCancel()
	}
	if p.close != nil {
		p.close()
	}
}

func (p *Picker) StatusHint() string {
	return "↑↓: choose  Tab: next  Enter: OK  Esc: cancel"
}

// ─── helpers ────────────────────────────────────────────────────

func drawAt(screen tcell.Screen, x, y int, s string, st tcell.Style) {
	col := 0
	for _, r := range s {
		screen.SetContent(x+col, y, r, nil, st)
		col++
	}
}

func fillSpan(screen tcell.Screen, x, y, w int, st tcell.Style) {
	for i := 0; i < w; i++ {
		screen.SetContent(x+i, y, ' ', nil, st)
	}
}

func clip(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runeLen(s) <= max {
		return s
	}
	out := make([]rune, 0, max)
	for _, r := range s {
		if len(out) >= max {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
