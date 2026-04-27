package foxpro

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// MenuItem is a single row inside a Menu's drop-down.
// Use Separator: true for a divider line.
// Label uses '&' to mark the accelerator letter, e.g. "&Open".
//
// Enable/disable rules:
//   - Disabled is a hard flag (use for items that aren't implemented
//     yet, or section-header rows that should never be invokable).
//   - EnabledIf is an optional dynamic check called every time the
//     menu is drawn or invoked — useful for selection-driven enabling
//     (e.g. "Show Pod Details" only enabled when a pod is selected).
//   - An item is enabled iff Disabled == false AND (EnabledIf == nil
//     OR EnabledIf() == true).
//   - Disabled items render in Theme.MenuDisabled, are skipped during
//     keyboard navigation, and ignore OnSelect.
type MenuItem struct {
	Label     string
	Hotkey    string
	OnSelect  func()
	Separator bool
	Disabled  bool
	EnabledIf func() bool
}

// IsEnabled reports whether the item is currently invokable.
func (it MenuItem) IsEnabled() bool {
	if it.Separator || it.Disabled {
		return false
	}
	if it.EnabledIf != nil {
		return it.EnabledIf()
	}
	return true
}

// Menu is one top-level entry in the bar plus its drop-down items.
type Menu struct {
	Label string // "&File"
	Items []MenuItem
}

// TrayItem is a label rendered flush-right on the menu bar (think
// macOS menu bar status items). Use Text for static labels or Compute
// for ones that change between frames (clocks, refresh state, etc.).
// OnClick, when non-nil, fires when the user clicks the item.
type TrayItem struct {
	Text    string
	Compute func() string
	OnClick func()
}

func (t TrayItem) display() string {
	if t.Compute != nil {
		return t.Compute()
	}
	return t.Text
}

// MenuBar lives at row 0. It tracks which top-level menu is open and which
// item is currently highlighted in the drop-down. Tray items render
// flush-right on the same row.
type MenuBar struct {
	Menus []Menu
	Tray  []TrayItem

	activeRoot  int // -1 = bar inactive
	open        bool
	activeChild int

	// trayHits caches the screen X-range of each tray item from the
	// last Draw, so HandleMousePress can route clicks back to OnClick.
	trayHits []trayHit
}

type trayHit struct {
	startX, endX int
	item         int
}

// NewMenuBar returns a fresh, inactive menu bar.
func NewMenuBar(menus []Menu) *MenuBar {
	return &MenuBar{Menus: menus, activeRoot: -1, activeChild: -1}
}

// IsActive reports whether the menu bar is currently capturing input.
func (mb *MenuBar) IsActive() bool { return mb.activeRoot >= 0 }

// Activate opens the bar with menu index `root` selected.
func (mb *MenuBar) Activate(root int) {
	if root < 0 || root >= len(mb.Menus) {
		return
	}
	mb.activeRoot = root
	mb.open = true
	mb.activeChild = mb.firstSelectable(root)
}

// Deactivate closes any open menu and releases input capture.
func (mb *MenuBar) Deactivate() {
	mb.activeRoot = -1
	mb.open = false
	mb.activeChild = -1
}

// parseLabel strips the '&' marker and returns the visible label plus the
// rune-index of the accelerator (or -1 if none).
func parseLabel(label string) (string, int) {
	var sb strings.Builder
	accel := -1
	skipNext := false
	for i, r := range label {
		_ = i
		if skipNext {
			skipNext = false
			sb.WriteRune(r)
			continue
		}
		if r == '&' {
			accel = sb.Len()
			skipNext = true
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String(), accel
}

// itemStartX returns the x-coordinate where menu i begins on the bar.
func (mb *MenuBar) itemStartX(i int) int {
	x := 0
	for j := 0; j < i; j++ {
		d, _ := parseLabel(mb.Menus[j].Label)
		x += len(d) + 2
	}
	return x
}

func (mb *MenuBar) itemAt(mx int) int {
	x := 0
	for i, m := range mb.Menus {
		d, _ := parseLabel(m.Label)
		w := len(d) + 2
		if mx >= x && mx < x+w {
			return i
		}
		x += w
		_ = i
	}
	return -1
}

func (mb *MenuBar) popupRect() Rect {
	if mb.activeRoot < 0 {
		return Rect{}
	}
	items := mb.Menus[mb.activeRoot].Items
	w := 0
	for _, it := range items {
		d, _ := parseLabel(it.Label)
		ln := len(d)
		if it.Hotkey != "" {
			ln += len(it.Hotkey) + 2
		}
		if ln > w {
			w = ln
		}
	}
	w += 4 // 1-cell border + 1-cell pad on each side
	if w < 14 {
		w = 14
	}
	h := len(items) + 2
	return Rect{X: mb.itemStartX(mb.activeRoot), Y: 1, W: w, H: h}
}

func (mb *MenuBar) firstSelectable(root int) int {
	for i, it := range mb.Menus[root].Items {
		if it.IsEnabled() {
			return i
		}
	}
	return -1
}

func (mb *MenuBar) nextSelectable(root, from int) int {
	items := mb.Menus[root].Items
	for off := 1; off <= len(items); off++ {
		i := (from + off) % len(items)
		if items[i].IsEnabled() {
			return i
		}
	}
	return from
}

func (mb *MenuBar) prevSelectable(root, from int) int {
	items := mb.Menus[root].Items
	for off := 1; off <= len(items); off++ {
		i := (from - off + len(items)) % len(items)
		if items[i].IsEnabled() {
			return i
		}
	}
	return from
}

// Draw renders the bar across the top and the popup if open.
func (mb *MenuBar) Draw(s tcell.Screen, theme Theme, screenW int) {
	fillRect(s, Rect{X: 0, Y: 0, W: screenW, H: 1}, theme.MenuBar)
	x := 0
	for i, m := range mb.Menus {
		active := i == mb.activeRoot
		x = mb.drawBarItem(s, x, m.Label, active, theme)
	}
	mb.drawTray(s, theme, screenW, x)
	if mb.open {
		mb.drawPopup(s, theme)
	}
}

// drawTray paints the tray items flush right on row 0 with a vertical
// divider between them. Items are dropped left-to-right when they
// don't fit, so the leftmost (most important) survives. menuEnd is
// the column where the menu items finished — tray will not encroach.
func (mb *MenuBar) drawTray(s tcell.Screen, theme Theme, screenW, menuEnd int) {
	mb.trayHits = mb.trayHits[:0]
	if len(mb.Tray) == 0 {
		return
	}
	// Compose item texts now so Compute funcs see one stable frame.
	texts := make([]string, len(mb.Tray))
	for i, t := range mb.Tray {
		texts[i] = t.display()
	}
	const sep = " │ "
	// Build right-to-left so the rightmost item always shows.
	type span struct {
		start, end, idx int
	}
	avail := screenW - menuEnd - 1
	x := screenW - 1
	spans := make([]span, 0, len(texts))
	for i := len(texts) - 1; i >= 0; i-- {
		t := texts[i]
		if t == "" {
			continue
		}
		need := len(t) + 2 // 1 space pad each side
		if i > 0 {
			need += len(sep)
		}
		if need > avail {
			break
		}
		end := x - 1
		start := end - len(t) - 1
		spans = append(spans, span{start: start, end: end, idx: i})
		x = start - len(sep)
		avail -= need
	}
	// Paint left-to-right for natural cursor / overdraw order.
	for j := len(spans) - 1; j >= 0; j-- {
		sp := spans[j]
		t := texts[sp.idx]
		drawString(s, sp.start, 0, " "+t+" ", theme.MenuBar)
		// Separator before this item (when not first visible).
		if j < len(spans)-1 {
			drawString(s, sp.start-len(sep), 0, sep, theme.MenuBar)
		}
		mb.trayHits = append(mb.trayHits, trayHit{startX: sp.start, endX: sp.end, item: sp.idx})
	}
}

func (mb *MenuBar) drawBarItem(s tcell.Screen, x int, label string, active bool, theme Theme) int {
	d, accel := parseLabel(label)
	style := theme.MenuBar
	accelStyle := theme.MenuAccel
	if active {
		style = theme.MenuBarActive
		accelStyle = theme.MenuAccelActive
	}
	s.SetContent(x, 0, ' ', nil, style)
	for i, r := range d {
		s.SetContent(x+1+i, 0, r, nil, style)
	}
	s.SetContent(x+1+len(d), 0, ' ', nil, style)
	if accel >= 0 && accel < len(d) {
		s.SetContent(x+1+accel, 0, rune(d[accel]), nil, accelStyle)
	}
	return x + len(d) + 2
}

func (mb *MenuBar) drawPopup(s tcell.Screen, theme Theme) {
	p := mb.popupRect()
	items := mb.Menus[mb.activeRoot].Items

	// Drop shadow — same geometry and treatment as window shadows: a
	// popup-sized rect at (+2, +1) offset, cells covered by the popup
	// itself are skipped, and the shadow keeps each rune underneath
	// while replacing fg+bg with theme.Shadow.
	const sox, soy = 2, 1
	for y := p.Y + soy; y < p.Y+soy+p.H; y++ {
		for x := p.X + sox; x < p.X+sox+p.W; x++ {
			if x < p.X+p.W && y < p.Y+p.H {
				continue // covered by the popup itself
			}
			shadeCell(s, x, y, theme.Shadow)
		}
	}

	fillRect(s, p, theme.MenuBar)
	border := theme.MenuBar
	for x := p.X + 1; x < p.X+p.W-1; x++ {
		s.SetContent(x, p.Y, '─', nil, border)
		s.SetContent(x, p.Y+p.H-1, '─', nil, border)
	}
	for y := p.Y + 1; y < p.Y+p.H-1; y++ {
		s.SetContent(p.X, y, '│', nil, border)
		s.SetContent(p.X+p.W-1, y, '│', nil, border)
	}
	s.SetContent(p.X, p.Y, '┌', nil, border)
	s.SetContent(p.X+p.W-1, p.Y, '┐', nil, border)
	s.SetContent(p.X, p.Y+p.H-1, '└', nil, border)
	s.SetContent(p.X+p.W-1, p.Y+p.H-1, '┘', nil, border)

	for i, it := range items {
		rowY := p.Y + 1 + i
		if it.Separator {
			for x := p.X + 1; x < p.X+p.W-1; x++ {
				s.SetContent(x, rowY, '─', nil, border)
			}
			s.SetContent(p.X, rowY, '├', nil, border)
			s.SetContent(p.X+p.W-1, rowY, '┤', nil, border)
			continue
		}
		rowStyle := theme.MenuBar
		accelStyle := theme.MenuAccel
		if i == mb.activeChild {
			rowStyle = theme.MenuBarActive
			accelStyle = theme.MenuAccelActive
		} else if !it.IsEnabled() {
			rowStyle = theme.MenuDisabled
			accelStyle = theme.MenuDisabled
		}
		for x := p.X + 1; x < p.X+p.W-1; x++ {
			s.SetContent(x, rowY, ' ', nil, rowStyle)
		}
		d, accel := parseLabel(it.Label)
		drawString(s, p.X+2, rowY, d, rowStyle)
		if accel >= 0 && accel < len(d) {
			s.SetContent(p.X+2+accel, rowY, rune(d[accel]), nil, accelStyle)
		}
		if it.Hotkey != "" {
			drawString(s, p.X+p.W-2-len(it.Hotkey), rowY, it.Hotkey, rowStyle)
		}
	}
}

// HandleKey processes input while the bar is active. Returns nothing because
// the bar consumes every key while open.
func (mb *MenuBar) HandleKey(ev *tcell.EventKey) {
	if mb.activeRoot < 0 {
		return
	}
	items := mb.Menus[mb.activeRoot].Items
	switch ev.Key() {
	case tcell.KeyEscape:
		mb.Deactivate()
	case tcell.KeyLeft:
		mb.activeRoot = (mb.activeRoot - 1 + len(mb.Menus)) % len(mb.Menus)
		mb.activeChild = mb.firstSelectable(mb.activeRoot)
	case tcell.KeyRight:
		mb.activeRoot = (mb.activeRoot + 1) % len(mb.Menus)
		mb.activeChild = mb.firstSelectable(mb.activeRoot)
	case tcell.KeyUp:
		if mb.activeChild >= 0 {
			mb.activeChild = mb.prevSelectable(mb.activeRoot, mb.activeChild)
		}
	case tcell.KeyDown:
		if mb.activeChild >= 0 {
			mb.activeChild = mb.nextSelectable(mb.activeRoot, mb.activeChild)
		}
	case tcell.KeyEnter:
		mb.invokeChild(items)
	case tcell.KeyRune:
		ch := unicode.ToLower(ev.Rune())
		for i, it := range items {
			if !it.IsEnabled() {
				continue
			}
			d, accel := parseLabel(it.Label)
			if accel >= 0 && accel < len(d) && unicode.ToLower(rune(d[accel])) == ch {
				mb.activeChild = i
				mb.invokeChild(items)
				return
			}
		}
	}
}

func (mb *MenuBar) invokeChild(items []MenuItem) {
	if mb.activeChild < 0 || mb.activeChild >= len(items) {
		return
	}
	it := items[mb.activeChild]
	if !it.IsEnabled() {
		return // disabled — don't dismiss, just no-op
	}
	cb := it.OnSelect
	mb.Deactivate()
	if cb != nil {
		cb()
	}
}

// HandleMousePress routes a button-down at (mx,my) to the bar.
// Returns true if the bar consumed the click.
func (mb *MenuBar) HandleMousePress(mx, my int) bool {
	if my == 0 {
		i := mb.itemAt(mx)
		if i >= 0 {
			if mb.activeRoot == i && mb.open {
				mb.Deactivate()
			} else {
				mb.Activate(i)
			}
			return true
		}
		// Tray item click — only fires for items with an OnClick.
		for _, h := range mb.trayHits {
			if mx >= h.startX && mx <= h.endX && h.item < len(mb.Tray) {
				if cb := mb.Tray[h.item].OnClick; cb != nil {
					cb()
					return true
				}
			}
		}
		// Click on empty bar area while inactive: do nothing.
		if mb.activeRoot >= 0 {
			mb.Deactivate()
			return true
		}
		return false
	}
	if !mb.open {
		return false
	}
	p := mb.popupRect()
	if !p.Contains(mx, my) {
		mb.Deactivate()
		return true
	}
	// Inside popup: select & invoke if it's an item row.
	row := my - p.Y - 1
	items := mb.Menus[mb.activeRoot].Items
	if row >= 0 && row < len(items) && !items[row].Separator {
		mb.activeChild = row
		mb.invokeChild(items)
	}
	return true
}
