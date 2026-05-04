//go:build js && wasm

// wasm-hello is a browser port of the foxpro-go hello example.
//
// All bridge plumbing — SimulationScreen wiring, JS-side cell snapshots,
// keyboard/mouse injection, exported tcell constants — lives in
// github.com/carledwards/foxpro-go/wasm. This file only owns the
// app-specific bits: which windows to show, the menu structure, and
// the boot sequence.
//
// To add DOM ↔ foxpro interop (e.g. an HTML button that controls a
// foxpro window), register your own js.FuncOf callback alongside the
// bridge — wrap mutations in app.Post so they land on foxpro's UI
// goroutine rather than the JS callback's goroutine.
package main

import (
	"fmt"
	"syscall/js"

	foxpro "github.com/carledwards/foxpro-go"
	"github.com/carledwards/foxpro-go/wasm"
	"github.com/gdamore/tcell/v2"
)

func main() {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		panic(err)
	}
	s.SetSize(120, 36)
	s.EnableMouse()

	app := foxpro.NewAppWithScreen(s)

	// Disable the built-in quit chord. In a browser, App.Quit terminates
	// the Go runtime and locks the page — users close the tab instead.
	// Esc still reaches the menu bar (to close popups) because that
	// path runs before handleBuiltinKey in dispatchKey.
	app.Settings.QuitKeys = nil

	// In a browser, right-click usually means "context menu" and is
	// unreliable as a window-drag gesture. Drop the right-click chord
	// and keep only Shift+left-click for background drag.
	app.Settings.BackgroundDragChords = []foxpro.BackgroundDragChord{
		{Button: tcell.Button1, Mods: tcell.ModShift},
	}

	// Opt in to CLEAR / CLS / HELP / VER. Skipping QUIT itself isn't
	// possible without splitting the call — but its OnSelect calls
	// app.Quit which is harmless here since we route the menu's Quit
	// item through location.reload() instead. The command is still in
	// the registry; users typing QUIT in the command window will brick
	// the page. Worth a separate fix when we revisit the command set.
	foxpro.RegisterBuiltinCommands(app)

	setupDemo(app)
	wasm.Run(app, s) // blocks until app.Quit
}

func setupDemo(a *foxpro.App) {
	greeting := foxpro.NewTextProvider([]string{
		"Welcome to foxpro-go in the browser.",
		"",
		"Drag this window by its title bar.",
		"Resize from the bottom-right corner.",
		"Click another window to bring it forward.",
		"",
		"F10 opens the menu bar; Alt+letter for accelerators.",
		"Ctrl+F2 toggles the command window.",
	})
	a.Manager.Add(foxpro.NewWindow(
		"Hello",
		foxpro.Rect{X: 4, Y: 3, W: 56, H: 11},
		greeting,
	))

	scroller := foxpro.NewTextProvider(makeLines(80))
	a.Manager.Add(foxpro.NewWindow(
		"Scrollback",
		foxpro.Rect{X: 30, Y: 10, W: 52, H: 14},
		scroller,
	))

	status := foxpro.NewTextProvider([]string{"(no action yet)"})
	a.Manager.Add(foxpro.NewWindow(
		"Status",
		foxpro.Rect{X: 8, Y: 22, W: 72, H: 5},
		status,
	))

	setStatus := func(msg string) { status.Lines = []string{msg} }

	a.Commands.Register("ECHO", "Print arguments back", func(cp *foxpro.CommandProvider, args string) {
		cp.Print(args)
	})

	a.MenuBar = foxpro.NewMenuBar([]foxpro.Menu{
		{
			Label: "&File",
			Items: []foxpro.MenuItem{
				{Label: "&New Window", OnSelect: func() {
					n := len(a.Manager.AllWindows()) + 1
					a.Manager.Add(foxpro.NewWindow(
						fmt.Sprintf("Window %d", n),
						foxpro.Rect{X: 6 + n*2, Y: 4 + n, W: 30, H: 7},
						foxpro.NewTextProvider([]string{fmt.Sprintf("This is window #%d.", n)}),
					))
					setStatus(fmt.Sprintf("opened window #%d", n))
				}},
				{Label: "&Command Window", Hotkey: "Ctrl+F2", OnSelect: a.ToggleCommandWindow},
				{Separator: true},
				// "Quit" reloads the page rather than calling app.Quit.
				// In a browser, app.Quit ends the Go runtime and bricks
				// the page; reloading restarts the demo cleanly, which
				// matches what users expect from a Quit-and-relaunch.
				{Label: "&Quit", OnSelect: func() {
					js.Global().Get("location").Call("reload")
				}},
			},
		},
		{
			Label: "&Help",
			Items: []foxpro.MenuItem{
				{Label: "&About", OnSelect: func() {
					setStatus("foxpro-go running in WebAssembly via tcell.SimulationScreen")
				}},
			},
		},
	})
}

func makeLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Line %02d  —  scroll with the wheel or PgUp/PgDn", i+1)
	}
	return out
}
