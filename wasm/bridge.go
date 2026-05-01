//go:build js && wasm

// Package wasm exposes a foxpro App to a browser host.
//
// The App keeps using a tcell.Screen for rendering — but instead of a
// terminal-backed screen, the caller supplies a tcell.SimulationScreen
// (a pure-Go cell buffer with no syscalls). Install registers a small
// JS bridge under `window.foxproWasm` that:
//
//   - lets JS pull a packed snapshot of the cell grid for canvas-side
//     rendering,
//   - injects browser keyboard/mouse events as tcell events,
//   - exposes tcell key/mod/button constants so the JS side has no
//     magic numbers,
//   - allows the host to resize the grid and quit the app.
//
// Demos are free to add their own JS-callable functions on top
// (e.g. to wire DOM buttons to foxpro state). Use App.Post from inside
// any custom js.Func so that mutations land on foxpro's UI goroutine,
// not on whatever goroutine the JS callback ran on.
//
// Cell packing: 16 bytes per cell, row-major.
//
//	0..3   uint32 LE  rune (BMP only; surrogate pairs collapse to first rune)
//	4..7   uint32 LE  fg color (0xRRGGBB; DefaultColor sentinel = unset)
//	8..11  uint32 LE  bg color (same encoding)
//	12..15 uint32 LE  tcell.AttrMask (bold/underline/reverse/...)
package wasm

import (
	"encoding/binary"
	"syscall/js"

	foxpro "github.com/carledwards/foxpro-go"
	"github.com/gdamore/tcell/v2"
)

// DefaultColor marks a cell color as "unset" in a snapshot. Browser
// renderers should fall back to a theme-appropriate default when they
// see this value (e.g. white on blue for the foxpro desktop).
const DefaultColor uint32 = 0xFF000000

// BridgeName is the key on window where the bridge is published.
const BridgeName = "foxproWasm"

// ReadyCallback is the name of an optional global JS function. If
// defined when Install runs, it is invoked once the bridge is live —
// the JS side typically uses it to start its render loop.
const ReadyCallback = "onFoxproReady"

// Install publishes the bridge on window[BridgeName]. Call after the
// screen has been Init'd and sized but before app.Run. Safe to call
// once per app.
func Install(app *foxpro.App, screen tcell.SimulationScreen) {
	b := &bridge{app: app, screen: screen}

	js.Global().Set(BridgeName, js.ValueOf(map[string]any{
		"size":         js.FuncOf(b.size),
		"snapshot":     js.FuncOf(b.snapshot),
		"injectKey":    js.FuncOf(b.injectKey),
		"injectMouse":  js.FuncOf(b.injectMouse),
		"resize":       js.FuncOf(b.resize),
		"quit":         js.FuncOf(b.quit),
		"keys":         keysMap(),
		"mods":         modsMap(),
		"buttons":      buttonsMap(),
		"defaultColor": int(DefaultColor),
	}))

	if cb := js.Global().Get(ReadyCallback); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
}

// Run is a convenience wrapper: Install, then app.Run (which blocks
// until app.Quit).
func Run(app *foxpro.App, screen tcell.SimulationScreen) {
	Install(app, screen)
	app.Run()
}

type bridge struct {
	app    *foxpro.App
	screen tcell.SimulationScreen
}

func (b *bridge) size(this js.Value, args []js.Value) any {
	w, h := b.screen.Size()
	return []any{w, h}
}

func (b *bridge) snapshot(this js.Value, args []js.Value) any {
	cells, w, h := b.screen.GetContents()
	n := w * h
	buf := make([]byte, 16*n)
	for i := 0; i < n; i++ {
		c := cells[i]
		ch := uint32(' ')
		if len(c.Runes) > 0 {
			ch = uint32(c.Runes[0])
		}
		fg, bg, attrs := c.Style.Decompose()
		off := 16 * i
		binary.LittleEndian.PutUint32(buf[off+0:], ch)
		binary.LittleEndian.PutUint32(buf[off+4:], encodeColor(fg))
		binary.LittleEndian.PutUint32(buf[off+8:], encodeColor(bg))
		binary.LittleEndian.PutUint32(buf[off+12:], uint32(attrs))
	}
	if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
		js.CopyBytesToJS(args[0], buf)
	}
	return []any{w, h}
}

func (b *bridge) injectKey(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return false
	}
	key := tcell.Key(args[0].Int())
	ch := rune(args[1].Int())
	mod := tcell.ModMask(args[2].Int())
	b.screen.InjectKey(key, ch, mod)
	return true
}

func (b *bridge) injectMouse(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return false
	}
	x := args[0].Int()
	y := args[1].Int()
	btn := tcell.ButtonMask(args[2].Int())
	mod := tcell.ModMask(0)
	if len(args) >= 4 {
		mod = tcell.ModMask(args[3].Int())
	}
	b.screen.InjectMouse(x, y, btn, mod)
	return true
}

func (b *bridge) resize(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return false
	}
	w, h := args[0].Int(), args[1].Int()
	b.screen.SetSize(w, h)
	// Wake the event loop so widgets reflow on the next draw.
	_ = b.screen.PostEvent(tcell.NewEventResize(w, h))
	return true
}

func (b *bridge) quit(this js.Value, args []js.Value) any {
	b.app.Quit()
	return nil
}

func encodeColor(c tcell.Color) uint32 {
	if c == tcell.ColorDefault {
		return DefaultColor
	}
	h := c.Hex()
	if h < 0 {
		return DefaultColor
	}
	return uint32(h)
}

func keysMap() map[string]any {
	return map[string]any{
		"Rune":       int(tcell.KeyRune),
		"Enter":      int(tcell.KeyEnter),
		"Tab":        int(tcell.KeyTab),
		"Backtab":    int(tcell.KeyBacktab),
		"Backspace":  int(tcell.KeyBackspace),
		"Backspace2": int(tcell.KeyBackspace2),
		"Esc":        int(tcell.KeyEscape),
		"Up":         int(tcell.KeyUp),
		"Down":       int(tcell.KeyDown),
		"Left":       int(tcell.KeyLeft),
		"Right":      int(tcell.KeyRight),
		"Home":       int(tcell.KeyHome),
		"End":        int(tcell.KeyEnd),
		"PgUp":       int(tcell.KeyPgUp),
		"PgDn":       int(tcell.KeyPgDn),
		"Insert":     int(tcell.KeyInsert),
		"Delete":     int(tcell.KeyDelete),
		"F1":         int(tcell.KeyF1),
		"F2":         int(tcell.KeyF2),
		"F3":         int(tcell.KeyF3),
		"F4":         int(tcell.KeyF4),
		"F5":         int(tcell.KeyF5),
		"F6":         int(tcell.KeyF6),
		"F7":         int(tcell.KeyF7),
		"F8":         int(tcell.KeyF8),
		"F9":         int(tcell.KeyF9),
		"F10":        int(tcell.KeyF10),
		"F11":        int(tcell.KeyF11),
		"F12":        int(tcell.KeyF12),
		"CtrlQ":      int(tcell.KeyCtrlQ),
		"CtrlC":      int(tcell.KeyCtrlC),
	}
}

func modsMap() map[string]any {
	return map[string]any{
		"None":  int(tcell.ModNone),
		"Shift": int(tcell.ModShift),
		"Ctrl":  int(tcell.ModCtrl),
		"Alt":   int(tcell.ModAlt),
		"Meta": int(tcell.ModMeta),
	}
}

func buttonsMap() map[string]any {
	return map[string]any{
		"None":       int(tcell.ButtonNone),
		"Primary":    int(tcell.Button1),
		"Secondary":  int(tcell.Button2),
		"Middle":     int(tcell.Button3),
		"WheelUp":    int(tcell.WheelUp),
		"WheelDown":  int(tcell.WheelDown),
		"WheelLeft":  int(tcell.WheelLeft),
		"WheelRight": int(tcell.WheelRight),
	}
}
