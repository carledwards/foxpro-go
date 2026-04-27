package foxpro

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// CommandFunc is the signature an app uses to handle a registered command.
// args is the rest of the input line after the command name. cp is the
// command-window provider so handlers can call cp.Print / cp.Clear and
// reach the registry for HELP-style introspection.
type CommandFunc func(cp *CommandProvider, args string)

// CommandRegistry maps uppercase command names to handlers. Apps register
// their commands here (typically once during startup) and the command
// window dispatches incoming lines against it.
type CommandRegistry struct {
	entries map[string]commandEntry
}

type commandEntry struct {
	Name    string
	Help    string
	Handler CommandFunc
}

// NewCommandRegistry returns an empty registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{entries: map[string]commandEntry{}}
}

// Register adds (or replaces) a command. name is matched case-insensitively.
func (r *CommandRegistry) Register(name, help string, handler CommandFunc) {
	up := strings.ToUpper(name)
	r.entries[up] = commandEntry{Name: up, Help: help, Handler: handler}
}

func (r *CommandRegistry) lookup(name string) (commandEntry, bool) {
	e, ok := r.entries[strings.ToUpper(name)]
	return e, ok
}

func (r *CommandRegistry) sorted() []commandEntry {
	out := make([]commandEntry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CommandProvider is the ContentProvider for the command window. It
// keeps a scrolling history of output lines plus an input line at the
// bottom; pressing Enter dispatches the input to the registry. Also
// implements Scrollable so the framework can draw a scrollbar and
// route arrow / wheel clicks for history navigation.
type CommandProvider struct {
	registry     *CommandRegistry
	history      []string
	input        []rune
	prompt       string
	scrollY      int // top visible history line index
	lastVisibleH int // height of the history viewport, set in Draw
}

// NewCommandProvider constructs a command provider against the given
// registry. The prompt defaults to "> ".
func NewCommandProvider(registry *CommandRegistry) *CommandProvider {
	return &CommandProvider{registry: registry, prompt: "> "}
}

// NewCommandWindow returns a non-modal command window for the given
// app, wired up to app.Commands. Add it to the manager to show.
func NewCommandWindow(app *App) *Window {
	return NewWindow("Command", Rect{X: 4, Y: 6, W: 60, H: 12}, NewCommandProvider(app.Commands))
}

// Print appends text to the history. Newlines split into separate rows.
// New output snaps the viewport back to the bottom (tail behaviour).
func (cp *CommandProvider) Print(text string) {
	for _, ln := range strings.Split(text, "\n") {
		cp.history = append(cp.history, ln)
	}
	cp.scrollY = len(cp.history) // clamped on next Draw
	cp.clampScroll()
}

// Clear empties the history buffer and resets the scroll position.
func (cp *CommandProvider) Clear() {
	cp.history = nil
	cp.scrollY = 0
}

func (cp *CommandProvider) clampScroll() {
	maxTop := len(cp.history) - cp.lastVisibleH
	if maxTop < 0 {
		maxTop = 0
	}
	if cp.scrollY > maxTop {
		cp.scrollY = maxTop
	}
	if cp.scrollY < 0 {
		cp.scrollY = 0
	}
}

// ContentSize returns the longest history line and the number of lines.
func (cp *CommandProvider) ContentSize() (int, int) {
	w := 0
	for _, line := range cp.history {
		if len(line) > w {
			w = len(line)
		}
	}
	return w, len(cp.history)
}

// ScrollOffset returns (0, scrollY); horizontal scrolling is not yet
// supported by the command window.
func (cp *CommandProvider) ScrollOffset() (int, int) {
	cp.clampScroll()
	return 0, cp.scrollY
}

// SetScrollOffset clamps y into the valid range and updates the
// viewport top. X is ignored.
func (cp *CommandProvider) SetScrollOffset(x, y int) {
	cp.scrollY = y
	cp.clampScroll()
}

// Registry returns the registry the provider dispatches against.
func (cp *CommandProvider) Registry() *CommandRegistry { return cp.registry }

func (cp *CommandProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	style := theme.WindowBG
	historyH := inner.H - 1
	if historyH < 0 {
		historyH = 0
	}
	cp.lastVisibleH = historyH
	cp.clampScroll()
	for row := 0; row < historyH && cp.scrollY+row < len(cp.history); row++ {
		line := cp.history[cp.scrollY+row]
		if len(line) > inner.W {
			line = line[:inner.W]
		}
		drawString(screen, inner.X, inner.Y+row, line, style)
	}
	// Input row at the very bottom.
	inputY := inner.Y + inner.H - 1
	line := cp.prompt + string(cp.input)
	if focused {
		line += "█"
	}
	if len(line) > inner.W {
		line = line[len(line)-inner.W:]
	}
	drawString(screen, inner.X, inputY, line, style)
}

func (cp *CommandProvider) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter:
		cp.execute()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(cp.input) > 0 {
			cp.input = cp.input[:len(cp.input)-1]
		}
		return true
	case tcell.KeyUp:
		cp.SetScrollOffset(0, cp.scrollY-1)
		return true
	case tcell.KeyDown:
		cp.SetScrollOffset(0, cp.scrollY+1)
		return true
	case tcell.KeyPgUp:
		step := cp.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		cp.SetScrollOffset(0, cp.scrollY-step)
		return true
	case tcell.KeyPgDn:
		step := cp.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		cp.SetScrollOffset(0, cp.scrollY+step)
		return true
	case tcell.KeyRune:
		cp.input = append(cp.input, ev.Rune())
		return true
	}
	return false
}

// StatusHint returns the contextual hint shown on the status bar while
// the command window is active.
func (cp *CommandProvider) StatusHint() string {
	return "Enter: run  HELP: commands "
}

func (cp *CommandProvider) execute() {
	line := strings.TrimSpace(string(cp.input))
	cp.input = cp.input[:0]
	if line == "" {
		return
	}
	cp.Print(cp.prompt + line)

	parts := strings.SplitN(line, " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	if entry, ok := cp.registry.lookup(name); ok {
		entry.Handler(cp, args)
	} else {
		cp.Print("? Unknown command: " + strings.ToUpper(name))
	}
}

// registerBuiltinCommands installs the framework-level commands on r.
// Called once by NewApp; apps can add their own with Register afterwards.
func registerBuiltinCommands(app *App, r *CommandRegistry) {
	r.Register("CLEAR", "Clear the command window", func(cp *CommandProvider, args string) {
		cp.Clear()
	})
	r.Register("CLS", "Clear the command window", func(cp *CommandProvider, args string) {
		cp.Clear()
	})
	r.Register("HELP", "List registered commands", func(cp *CommandProvider, args string) {
		for _, e := range cp.Registry().sorted() {
			cp.Print(fmt.Sprintf("  %-10s  %s", e.Name, e.Help))
		}
	})
	r.Register("QUIT", "Quit the application", func(cp *CommandProvider, args string) {
		app.Quit()
	})
	r.Register("VER", "Show framework version", func(cp *CommandProvider, args string) {
		cp.Print("foxpro-go (development)")
	})
}
