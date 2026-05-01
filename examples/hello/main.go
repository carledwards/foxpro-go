package main

import (
	"fmt"
	"os"

	foxpro "github.com/carledwards/foxpro-go"
)

func main() {
	app, err := foxpro.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	defer app.Close()

	// Opt in to the framework's standard CLEAR / HELP / QUIT / VER
	// command-window commands. Terminal hosts want these; browser
	// hosts can skip the call (QUIT would brick the wasm runtime).
	foxpro.RegisterBuiltinCommands(app)

	greeting := foxpro.NewTextProvider([]string{
		"Welcome to foxpro-go.",
		"",
		"Drag this window by its title bar.",
		"Resize from the bottom-right corner.",
		"Click another window to bring it forward.",
		"",
		"F10 opens the menu bar, Alt+letter for accelerators.",
	})
	app.Manager.Add(foxpro.NewWindow(
		"Hello",
		foxpro.Rect{X: 4, Y: 3, W: 50, H: 10},
		greeting,
	))

	scroller := foxpro.NewTextProvider(makeLines(80))
	app.Manager.Add(foxpro.NewWindow(
		"Scrollback",
		foxpro.Rect{X: 22, Y: 9, W: 52, H: 14},
		scroller,
	))

	status := foxpro.NewTextProvider([]string{"(no action yet)"})
	statusWin := foxpro.NewWindow(
		"Status",
		foxpro.Rect{X: 8, Y: 18, W: 60, H: 5},
		status,
	)
	app.Manager.Add(statusWin)

	setStatus := func(msg string) {
		status.Lines = []string{msg}
	}

	// Demonstrate registering a custom command on the framework's
	// built-in registry. Type "ECHO hello world" in the command window.
	app.Commands.Register("ECHO", "Print arguments back", func(cp *foxpro.CommandProvider, args string) {
		cp.Print(args)
	})

	app.MenuBar = foxpro.NewMenuBar([]foxpro.Menu{
		{
			Label: "&File",
			Items: []foxpro.MenuItem{
				{Label: "&New Window", OnSelect: func() {
					n := len(app.Manager.AllWindows()) + 1
					app.Manager.Add(foxpro.NewWindow(
						fmt.Sprintf("Window %d", n),
						foxpro.Rect{X: 6 + n*2, Y: 4 + n, W: 30, H: 7},
						foxpro.NewTextProvider([]string{fmt.Sprintf("This is window #%d.", n)}),
					))
					setStatus(fmt.Sprintf("opened window #%d", n))
				}},
				{Label: "&Settings...", OnSelect: func() {
					app.Manager.Add(foxpro.NewSettingsWindow(app))
					setStatus("opened settings")
				}},
				{Label: "&Command Window", Hotkey: "F2", OnSelect: app.ToggleCommandWindow},
				{Separator: true},
				{Label: "E&xit", Hotkey: "Esc", OnSelect: app.Quit},
			},
		},
		{
			Label: "&Edit",
			Items: []foxpro.MenuItem{
				{Label: "&Cut", Hotkey: "Ctrl+X", OnSelect: func() { setStatus("Edit > Cut") }},
				{Label: "C&opy", Hotkey: "Ctrl+C", OnSelect: func() { setStatus("Edit > Copy") }},
				{Label: "&Paste", Hotkey: "Ctrl+V", OnSelect: func() { setStatus("Edit > Paste") }},
			},
		},
		{
			Label: "&Window",
			Items: []foxpro.MenuItem{
				{Label: "&Next", Hotkey: "F6", OnSelect: app.Manager.FocusNext},
				{Label: "&Close Active", OnSelect: func() {
					if w := app.Manager.Active(); w != nil {
						app.Manager.Remove(w)
						setStatus("closed active window")
					}
				}},
			},
		},
		{
			Label: "&Help",
			Items: []foxpro.MenuItem{
				{Label: "&About", OnSelect: func() { setStatus("foxpro-go — DOS-style TUI for Go") }},
			},
		},
	})

	app.Run()
}

func makeLines(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = fmt.Sprintf("line %02d  arrow keys / PgUp / PgDn / Home / End", i+1)
	}
	return out
}
