package foxpro

import (
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// doubleClickWindow is the time window inside which two mouse presses at
// the same cell on the same window count as a double-click.
const doubleClickWindow = 500 * time.Millisecond

// App ties together the tcell screen, theme, window manager, and optional
// menu bar. Callers populate Manager (and MenuBar) before calling Run.
type App struct {
	Screen   tcell.Screen
	Theme    Theme
	Settings Settings
	Manager  *WindowManager
	MenuBar  *MenuBar         // optional; reserves row 0 when set
	Commands *CommandRegistry // commands available in the command window

	// OnKey, when non-nil, sees every key event before built-in handling.
	// Return true to consume.
	OnKey func(ev *tcell.EventKey) bool

	quit bool
	drag dragState
	prevButtons tcell.ButtonMask

	// Mouse cursor state. The cursor is drawn as an inverted-style cell
	// at (mouseX, mouseY) whenever mouseVisible is true. It appears on
	// any mouse event and hides on the next real keyboard input.
	mouseX, mouseY int
	mouseVisible   bool

	// Double-click tracking — see doubleClickWindow.
	lastClickAt time.Time
	lastClickX  int
	lastClickY  int
	lastClickW  *Window

	// Command window reference, kept so F2 / ToggleCommandWindow can
	// find it across opens and closes.
	cmdWindow *Window

	// Mouse capture — when a MouseHandler press returns true and the
	// provider also implements MouseDragHandler, subsequent
	// motion-while-held + the release fire on this window's provider
	// instead of routing through hit-testing.
	capturedWindow *Window
}

type dragKind int

const (
	dragNone dragKind = iota
	dragMove
	dragResize
	dragVThumb
	dragHThumb
)

type dragState struct {
	kind   dragKind
	window *Window
	offX   int // for move: click X minus window X
	offY   int
	// Thumb-drag fields: track the scrollbar geometry captured at the
	// moment of grab, so continueDrag can map mouse motion to scroll.
	trackStart int
	trackEnd   int
	maxScroll  int
}

// NewApp initialises tcell. Caller must defer Close (or Screen.Fini).
func NewApp() (*App, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := s.Init(); err != nil {
		return nil, err
	}
	s.EnableMouse()
	app := &App{
		Screen:   s,
		Theme:    DefaultTheme(),
		Settings: DefaultSettings(),
		Manager:  NewWindowManager(),
		Commands: NewCommandRegistry(),
	}
	registerBuiltinCommands(app, app.Commands)
	return app, nil
}

// Close tears down the terminal.
func (a *App) Close() { a.Screen.Fini() }

// Quit asks the event loop to exit after the current iteration.
func (a *App) Quit() { a.quit = true }

// Run renders and dispatches events until Quit is called.
// Built-ins: Esc / Ctrl+Q quit, Tab cycles focus, F10 + Alt+letter open menus.
func (a *App) Run() {
	for !a.quit {
		a.draw()
		ev := a.Screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			a.Screen.Sync()
		case *tcell.EventKey:
			a.dispatchKey(ev)
		case *tcell.EventMouse:
			a.dispatchMouse(ev)
		case *tcell.EventInterrupt:
			// Posted from a goroutine via Post / Tick. Run the
			// callback on the main loop, then fall through to the
			// implicit redraw at the top of the next iteration.
			if fn, ok := ev.Data().(func()); ok && fn != nil {
				fn()
			}
		}
	}
}

// Post schedules fn to run on the main UI goroutine. Safe to call
// from any goroutine. The next event-loop iteration runs fn and then
// redraws. Pass nil to wake the loop for a redraw without other work
// (useful when you've changed state that a Compute callback reads).
func (a *App) Post(fn func()) {
	_ = a.Screen.PostEvent(tcell.NewEventInterrupt(fn))
}

// Tick fires fn on the main UI goroutine every d. Returns a stop
// function. fn may be nil — useful for a steady redraw cadence so
// tray Compute callbacks (animated spinner, clock) update on time.
func (a *App) Tick(d time.Duration, fn func()) (stop func()) {
	stopCh := make(chan struct{})
	go func() {
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-t.C:
				a.Post(fn)
			}
		}
	}()
	return func() { close(stopCh) }
}

func (a *App) dispatchKey(ev *tcell.EventKey) {
	// Any real key input hides the mouse cursor (FoxPro behaviour).
	// Terminals don't deliver modifier-only events, so every EventKey
	// we see qualifies as input.
	a.mouseVisible = false

	if a.OnKey != nil && a.OnKey(ev) {
		return
	}
	if a.MenuBar != nil && a.MenuBar.IsActive() {
		a.MenuBar.HandleKey(ev)
		return
	}
	if a.handleBuiltinKey(ev) {
		return
	}
	a.Manager.HandleKey(ev)
}

func (a *App) handleBuiltinKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlQ:
		a.Quit()
		return true
	case tcell.KeyF1:
		// F1 toggles the status bar. Contextual hints (Space: toggle,
		// etc.) live on the bar's right side, so apps that want a
		// minimal look can hide the whole strip.
		a.Settings.ShowStatusBar = !a.Settings.ShowStatusBar
		return true
	case tcell.KeyF2:
		a.ToggleCommandWindow()
		return true
	case tcell.KeyF6:
		// F6 cycles windows. Shift+F6 cycles backward.
		// Tab is intentionally NOT bound here so ContentProviders can
		// use it to move focus between their own controls.
		if ev.Modifiers()&tcell.ModShift != 0 {
			a.Manager.FocusPrev()
		} else {
			a.Manager.FocusNext()
		}
		return true
	case tcell.KeyF10:
		if a.MenuBar != nil && len(a.MenuBar.Menus) > 0 {
			a.MenuBar.Activate(0)
			return true
		}
	case tcell.KeyRune:
		if a.MenuBar != nil && ev.Modifiers()&tcell.ModAlt != 0 {
			ch := unicode.ToLower(ev.Rune())
			for i, m := range a.MenuBar.Menus {
				d, accel := parseLabel(m.Label)
				if accel >= 0 && accel < len(d) && unicode.ToLower(rune(d[accel])) == ch {
					a.MenuBar.Activate(i)
					return true
				}
			}
		}
	}
	return false
}

func (a *App) dispatchMouse(ev *tcell.EventMouse) {
	mx, my := ev.Position()
	a.mouseX, a.mouseY = mx, my
	a.mouseVisible = true
	btn := ev.Buttons()
	pressed := btn&tcell.Button1 != 0 && a.prevButtons&tcell.Button1 == 0
	released := btn&tcell.Button1 == 0 && a.prevButtons&tcell.Button1 != 0
	wheelMask := tcell.WheelUp | tcell.WheelDown | tcell.WheelLeft | tcell.WheelRight
	a.prevButtons = btn

	// Wheel events: ask the provider's WheelHandler first (so split
	// layouts can route by cursor position), then fall back to the
	// top-level Scrollable. Three lines per tick is the conventional
	// default. Wheel is dispatched before press/release tracking so
	// it works even mid-drag.
	if btn&wheelMask != 0 {
		const step = 3
		if w, _ := a.Manager.HitTest(mx, my); w != nil {
			consumed := false
			if w.Content != nil {
				if wh, ok := w.Content.(WheelHandler); ok {
					consumed = wh.HandleWheel(ev, w.Bounds.Inner())
				}
			}
			if !consumed {
				switch {
				case btn&tcell.WheelUp != 0:
					a.scrollContent(w, 0, -step)
				case btn&tcell.WheelDown != 0:
					a.scrollContent(w, 0, step)
				case btn&tcell.WheelLeft != 0:
					a.scrollContent(w, -step, 0)
				case btn&tcell.WheelRight != 0:
					a.scrollContent(w, step, 0)
				}
			}
		}
		return
	}

	if released {
		a.drag = dragState{}
		if a.capturedWindow != nil {
			if mdh, ok := a.capturedWindow.Content.(MouseDragHandler); ok {
				mdh.HandleMouseRelease(ev, a.capturedWindow.Bounds.Inner())
			}
			a.capturedWindow = nil
		}
		return
	}

	// Provider mouse capture: while a MouseDragHandler is captured,
	// motion-while-held is delivered to it directly.
	if a.capturedWindow != nil && btn&tcell.Button1 != 0 {
		if mdh, ok := a.capturedWindow.Content.(MouseDragHandler); ok {
			mdh.HandleMouseMotion(ev, a.capturedWindow.Bounds.Inner())
		}
		return
	}

	// Drag-in-progress takes priority (window-chrome drags).
	if a.drag.kind != dragNone && btn&tcell.Button1 != 0 {
		a.continueDrag(mx, my)
		return
	}

	if !pressed {
		return
	}

	// Menu bar gets first crack at presses.
	if a.MenuBar != nil && a.MenuBar.HandleMousePress(mx, my) {
		return
	}

	// Otherwise: hit-test windows.
	w, zone := a.Manager.HitTest(mx, my)
	if w == nil {
		return
	}
	a.Manager.Raise(w)
	switch zone {
	case HitTitle:
		// Double-click on the title bar toggles window-shade.
		if a.isDoubleClick(mx, my, w) {
			a.shadeWindow(w)
			a.recordClick(mx, my, nil) // consume so triple-click won't re-fire
			return
		}
		a.recordClick(mx, my, w)
		a.drag = dragState{kind: dragMove, window: w, offX: mx - w.Bounds.X, offY: my - w.Bounds.Y}
	case HitResize:
		a.recordClick(mx, my, w)
		a.drag = dragState{kind: dragResize, window: w}
	case HitClose:
		a.recordClick(mx, my, nil)
		a.closeWindow(w)
	case HitZoom:
		a.recordClick(mx, my, nil)
		a.zoomWindow(w)
	case HitScrollUp:
		a.recordClick(mx, my, w)
		a.scrollContent(w, 0, -1)
	case HitScrollDown:
		a.recordClick(mx, my, w)
		a.scrollContent(w, 0, 1)
	case HitScrollLeft:
		a.recordClick(mx, my, w)
		a.scrollContent(w, -1, 0)
	case HitScrollRight:
		a.recordClick(mx, my, w)
		a.scrollContent(w, 1, 0)
	case HitScrollVTrack:
		a.recordClick(mx, my, w)
		a.handleVTrackClick(w, my)
	case HitScrollHTrack:
		a.recordClick(mx, my, w)
		a.handleHTrackClick(w, mx)
	default:
		// HitBody — forward the press to the content provider if it
		// implements MouseHandler. If the provider also implements
		// MouseDragHandler and consumed the press, capture further
		// mouse events for the duration of the drag.
		a.recordClick(mx, my, w)
		if w.Content != nil {
			if mh, ok := w.Content.(MouseHandler); ok {
				if mh.HandleMouse(ev, w.Bounds.Inner()) {
					if _, ok := w.Content.(MouseDragHandler); ok {
						a.capturedWindow = w
					}
				}
			}
		}
	}
}

func (a *App) isDoubleClick(mx, my int, w *Window) bool {
	if a.lastClickW != w {
		return false
	}
	if mx != a.lastClickX || my != a.lastClickY {
		return false
	}
	return time.Since(a.lastClickAt) <= doubleClickWindow
}

func (a *App) recordClick(mx, my int, w *Window) {
	a.lastClickAt = time.Now()
	a.lastClickX = mx
	a.lastClickY = my
	a.lastClickW = w
}

// scrollContent nudges a Scrollable provider's offset by (dx, dy) cells.
// No-op if the active provider isn't Scrollable.
func (a *App) scrollContent(w *Window, dx, dy int) {
	if w == nil || w.Content == nil {
		return
	}
	scr, ok := w.Content.(Scrollable)
	if !ok {
		return
	}
	x, y := scr.ScrollOffset()
	scr.SetScrollOffset(x+dx, y+dy)
}

// handleVTrackClick handles a press inside the vertical scrollbar
// track (the cells between the up and down arrows). Above the thumb
// pages up; below pages down; on the thumb starts a drag that maps
// mouse Y to scroll position proportionally.
func (a *App) handleVTrackClick(w *Window, my int) {
	scr, ok := w.Content.(Scrollable)
	if !ok {
		return
	}
	_, ch := scr.ContentSize()
	_, sy := scr.ScrollOffset()
	innerH := w.Bounds.H - 2
	maxScroll := ch - innerH
	if maxScroll <= 0 {
		return
	}
	trackTop := w.Bounds.Y + 2
	trackBot := w.Bounds.Y + w.Bounds.H - 3
	trackH := trackBot - trackTop + 1
	if trackH <= 0 {
		return
	}
	thumbY := trackTop + (sy*(trackH-1))/maxScroll
	page := innerH - 1
	if page < 1 {
		page = 1
	}
	switch {
	case my < thumbY:
		a.scrollContent(w, 0, -page)
	case my > thumbY:
		a.scrollContent(w, 0, page)
	default:
		// On the thumb: start a proportional drag.
		a.drag = dragState{
			kind:       dragVThumb,
			window:     w,
			trackStart: trackTop,
			trackEnd:   trackBot,
			maxScroll:  maxScroll,
		}
	}
}

// handleHTrackClick is the horizontal counterpart to handleVTrackClick.
func (a *App) handleHTrackClick(w *Window, mx int) {
	scr, ok := w.Content.(Scrollable)
	if !ok {
		return
	}
	cw, _ := scr.ContentSize()
	sx, _ := scr.ScrollOffset()
	innerW := w.Bounds.W - 2
	maxScroll := cw - innerW
	if maxScroll <= 0 {
		return
	}
	trackL := w.Bounds.X + 2
	trackR := w.Bounds.X + w.Bounds.W - 3
	trackW := trackR - trackL + 1
	if trackW <= 0 {
		return
	}
	thumbX := trackL + (sx*(trackW-1))/maxScroll
	page := innerW - 1
	if page < 1 {
		page = 1
	}
	switch {
	case mx < thumbX:
		a.scrollContent(w, -page, 0)
	case mx > thumbX:
		a.scrollContent(w, page, 0)
	default:
		a.drag = dragState{
			kind:       dragHThumb,
			window:     w,
			trackStart: trackL,
			trackEnd:   trackR,
			maxScroll:  maxScroll,
		}
	}
}

// closeWindow runs the window's OnClose callback if set, otherwise removes
// the window from the manager.
func (a *App) closeWindow(w *Window) {
	if w.OnClose != nil {
		w.OnClose()
		return
	}
	a.Manager.Remove(w)
}

// zoomWindow runs OnZoom if set, otherwise toggles maximize: the window
// expands to fill the full screen below the menu bar and above the hint
// row, all the way to the right and bottom edges (the drop shadow is
// allowed to clip off-screen). A second click restores the prior bounds.
// If the window is currently shaded, zoom unshades first.
func (a *App) zoomWindow(w *Window) {
	if w.OnZoom != nil {
		w.OnZoom()
		return
	}
	if w.shaded {
		w.Bounds = w.shadeBounds
		w.shaded = false
		return
	}
	if w.maximized {
		w.Bounds = w.prevBounds
		w.maximized = false
		return
	}
	sw, sh := a.Screen.Size()
	top := 0
	if a.MenuBar != nil {
		top = 1
	}
	w.prevBounds = w.Bounds
	w.Bounds = Rect{X: 0, Y: top, W: sw, H: sh - top - 1}
	w.maximized = true
}

// ToggleCommandWindow shows the command window if hidden, or removes
// it from the manager if currently shown. The Window instance itself
// is created once and reused so the command history, input buffer,
// and last position survive across hide/show cycles. Bound to F2.
func (a *App) ToggleCommandWindow() {
	if a.cmdWindow == nil {
		a.cmdWindow = NewCommandWindow(a)
		w := a.cmdWindow
		w.OnClose = func() {
			a.Manager.Remove(w) // hide; keep state for next reopen
		}
	}
	if a.Manager.Contains(a.cmdWindow) {
		a.Manager.Remove(a.cmdWindow)
	} else {
		a.Manager.Add(a.cmdWindow)
	}
}

// shadeWindow toggles window-shade mode. When shaded, the window
// collapses to a 1-row title bar of fixed width (ShadedWidth) so shaded
// titles stack neatly. Unshade restores the prior bounds.
func (a *App) shadeWindow(w *Window) {
	if w.shaded {
		w.Bounds = w.shadeBounds
		w.shaded = false
		return
	}
	w.shadeBounds = w.Bounds
	w.Bounds.H = 1
	w.Bounds.W = ShadedWidth
	w.shaded = true
}

func (a *App) continueDrag(mx, my int) {
	sw, sh := a.Screen.Size()
	topReserved := 0
	if a.MenuBar != nil {
		topReserved = 1
	}
	switch a.drag.kind {
	case dragMove:
		w := a.drag.window
		w.Bounds.X = mx - a.drag.offX
		w.Bounds.Y = my - a.drag.offY
		// Keep at least one column / row of title bar reachable on screen.
		if w.Bounds.X < 1-w.Bounds.W+5 {
			w.Bounds.X = 1 - w.Bounds.W + 5
		}
		if w.Bounds.X > sw-5 {
			w.Bounds.X = sw - 5
		}
		if w.Bounds.Y < topReserved {
			w.Bounds.Y = topReserved
		}
		if w.Bounds.Y > sh-2 {
			w.Bounds.Y = sh - 2
		}
	case dragResize:
		w := a.drag.window
		nw := mx - w.Bounds.X + 1
		nh := my - w.Bounds.Y + 1
		minW := 8
		if w.MinW > minW {
			minW = w.MinW
		}
		minH := 3
		if w.MinH > minH {
			minH = w.MinH
		}
		if nw < minW {
			nw = minW
		}
		if nh < minH {
			nh = minH
		}
		if w.Bounds.X+nw > sw {
			nw = sw - w.Bounds.X
		}
		if w.Bounds.Y+nh > sh-1 {
			nh = sh - 1 - w.Bounds.Y
		}
		w.Bounds.W = nw
		w.Bounds.H = nh
	case dragVThumb:
		w := a.drag.window
		scr, ok := w.Content.(Scrollable)
		if !ok {
			return
		}
		trackH := a.drag.trackEnd - a.drag.trackStart + 1
		if trackH <= 1 {
			return
		}
		rel := my - a.drag.trackStart
		if rel < 0 {
			rel = 0
		}
		if rel > trackH-1 {
			rel = trackH - 1
		}
		newSy := (rel * a.drag.maxScroll) / (trackH - 1)
		sx, _ := scr.ScrollOffset()
		scr.SetScrollOffset(sx, newSy)
	case dragHThumb:
		w := a.drag.window
		scr, ok := w.Content.(Scrollable)
		if !ok {
			return
		}
		trackW := a.drag.trackEnd - a.drag.trackStart + 1
		if trackW <= 1 {
			return
		}
		rel := mx - a.drag.trackStart
		if rel < 0 {
			rel = 0
		}
		if rel > trackW-1 {
			rel = trackW - 1
		}
		newSx := (rel * a.drag.maxScroll) / (trackW - 1)
		_, sy := scr.ScrollOffset()
		scr.SetScrollOffset(newSx, sy)
	}
}

func (a *App) draw() {
	// Hide the terminal cursor by default each frame; providers
	// that want it visible (e.g. InputProvider) call ShowCursor in
	// their Draw and override this for the next paint.
	a.Screen.HideCursor()
	w, h := a.Screen.Size()
	topReserved := 0
	if a.MenuBar != nil {
		topReserved = 1
	}
	bottomReserved := 0
	if a.Settings.ShowStatusBar {
		bottomReserved = 1
	}
	// Desktop fill (below the menu bar, above the status bar).
	fillRectRune(a.Screen,
		Rect{X: 0, Y: topReserved, W: w, H: h - topReserved - bottomReserved},
		a.Theme.DesktopRune, a.Theme.Desktop)

	if a.Settings.ShowStatusBar {
		hintStyle := a.Theme.MenuBar
		fillRect(a.Screen, Rect{X: 0, Y: h - 1, W: w, H: 1}, hintStyle)

		// Left: built-in app hints.
		left := " F1: status  F2: cmd  F10: menu  F6: next window  Esc: quit "
		drawString(a.Screen, 0, h-1, left, hintStyle)

		// Right: contextual hint from the active window's content
		// provider, if it implements StatusHinter. Drawn flush right
		// with one trailing space, only if it doesn't overlap left.
		if active := a.Manager.Active(); active != nil && active.Content != nil {
			if hinter, ok := active.Content.(StatusHinter); ok {
				hint := hinter.StatusHint()
				if hint != "" {
					rx := w - len(hint) - 1
					if rx > len(left) {
						drawString(a.Screen, rx, h-1, hint, hintStyle)
					}
				}
			}
		}
	}

	a.Manager.Draw(a.Screen, a.Theme, a.Settings)

	// Menu bar drawn last so its popup paints over everything.
	if a.MenuBar != nil {
		a.MenuBar.Draw(a.Screen, a.Theme, w)
	}

	// Mouse cursor: read the cell under the pointer and redraw it with
	// each colour replaced by the theme palette's complement. Same
	// character, same attributes — only the colours change. Matches FoxPro
	// for DOS: blue→brown on the desktop, cyan→red over windows, etc.
	if a.mouseVisible && a.mouseX >= 0 && a.mouseY >= 0 && a.mouseX < w && a.mouseY < h {
		mainc, combc, st, _ := a.Screen.GetContent(a.mouseX, a.mouseY)
		if mainc == 0 {
			mainc = ' '
		}
		fg, bg, attr := st.Decompose()
		inv := tcell.StyleDefault.
			Foreground(a.Theme.InvertColor(fg)).
			Background(a.Theme.InvertColor(bg)).
			Attributes(attr)
		a.Screen.SetContent(a.mouseX, a.mouseY, mainc, combc, inv)
	}

	a.Screen.Show()
}
