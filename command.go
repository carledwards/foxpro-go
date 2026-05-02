package foxpro

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// cursorBlinkPeriodMs is the on/off cadence of the rendered cursor —
// half the period visible, half hidden, so a full cycle is twice this
// value. Hosts that want the cursor to actually animate must drive
// periodic redraws (App.Tick at any interval ≤ this, typically equal).
const cursorBlinkPeriodMs = 500

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

// CommandProvider is the ContentProvider for the command window.
//
// Modeled after FoxPro for DOS's Command Window: there is no separation
// between an "output area" and an "input line" — the entire window is
// one editable buffer. The cursor can be anywhere; pressing Enter on
// any line dispatches that line through the registry. Output from the
// handler appends at the bottom, and the cursor lands on a fresh
// blank line ready for the next command.
//
// Implements Scrollable so the framework draws scrollbars and routes
// arrow / wheel / track clicks for vertical and horizontal navigation.
type CommandProvider struct {
	registry     *CommandRegistry
	lines        [][]rune // every line, all editable
	cy, cx       int      // cursor row/col within lines[cy]
	scrollY      int
	scrollX      int
	lastVisibleH int // viewport height, set in Draw
	lastVisibleW int // viewport width, set in Draw
	longestLine  int // cached width of the widest line
}

// NewCommandProvider constructs a command provider against the given
// registry. The buffer starts as one empty editable line with the
// cursor at its origin.
func NewCommandProvider(registry *CommandRegistry) *CommandProvider {
	return &CommandProvider{
		registry: registry,
		lines:    [][]rune{nil},
	}
}

// NewCommandWindow returns a non-modal command window for the given
// app, wired up to app.Commands. Add it to the manager to show.
func NewCommandWindow(app *App) *Window {
	return NewWindow("Command", Rect{X: 4, Y: 6, W: 60, H: 12}, NewCommandProvider(app.Commands))
}

// Print appends one or more lines at the bottom of the buffer. Multi-line
// text is split on '\n'. The cursor position is not changed — callers
// (Enter handlers, app init) decide when to land the cursor on a fresh
// blank line via AppendInputLine.
//
// As a small ergonomic shortcut, if the buffer is exactly the initial
// single-empty-line state (e.g. just constructed or just Cleared) and
// the cursor still sits at (0,0), the first printed line replaces that
// placeholder rather than sitting below it. This avoids a leading
// blank line when an app pre-populates a freshly-opened command window.
func (cp *CommandProvider) Print(text string) {
	out := strings.Split(text, "\n")
	if len(out) == 0 {
		return
	}
	freshBuffer := len(cp.lines) == 1 && len(cp.lines[0]) == 0 && cp.cy == 0 && cp.cx == 0
	if freshBuffer {
		cp.lines[0] = []rune(out[0])
		if l := len(cp.lines[0]); l > cp.longestLine {
			cp.longestLine = l
		}
		out = out[1:]
	}
	for _, ln := range out {
		r := []rune(ln)
		cp.lines = append(cp.lines, r)
		if len(r) > cp.longestLine {
			cp.longestLine = len(r)
		}
	}
	cp.scrollX = 0
}

// AppendInputLine ensures there's a blank line at the bottom of the
// buffer and parks the cursor at its start. Call after a batch of
// Prints (welcome echo, command output) so the next user keystroke
// lands on a clean line.
func (cp *CommandProvider) AppendInputLine() {
	if len(cp.lines) == 0 || len(cp.lines[len(cp.lines)-1]) != 0 {
		cp.lines = append(cp.lines, nil)
	}
	cp.cy = len(cp.lines) - 1
	cp.cx = 0
	cp.scrollX = 0
	cp.ensureCursorVisible()
}

// Clear empties the buffer and parks the cursor on a single blank
// line at the origin. The longest-line cache is reset.
func (cp *CommandProvider) Clear() {
	cp.lines = [][]rune{nil}
	cp.cy = 0
	cp.cx = 0
	cp.scrollY = 0
	cp.scrollX = 0
	cp.longestLine = 0
}

// ContentSize returns the longest line width and the line count.
func (cp *CommandProvider) ContentSize() (int, int) {
	return cp.longestLine, len(cp.lines)
}

// ScrollOffset returns the current viewport origin within the buffer.
func (cp *CommandProvider) ScrollOffset() (int, int) {
	cp.clampScroll()
	return cp.scrollX, cp.scrollY
}

// SetScrollOffset clamps (x, y) into the valid range and moves the
// viewport. Cursor is not moved — keyboard navigation does that.
func (cp *CommandProvider) SetScrollOffset(x, y int) {
	cp.scrollX = x
	cp.scrollY = y
	cp.clampScroll()
}

// Registry returns the registry the provider dispatches against.
func (cp *CommandProvider) Registry() *CommandRegistry { return cp.registry }

func (cp *CommandProvider) clampScroll() {
	maxTop := len(cp.lines) - cp.lastVisibleH
	if maxTop < 0 {
		maxTop = 0
	}
	if cp.scrollY > maxTop {
		cp.scrollY = maxTop
	}
	if cp.scrollY < 0 {
		cp.scrollY = 0
	}
	maxLeft := cp.longestLine - cp.lastVisibleW
	if maxLeft < 0 {
		maxLeft = 0
	}
	if cp.scrollX > maxLeft {
		cp.scrollX = maxLeft
	}
	if cp.scrollX < 0 {
		cp.scrollX = 0
	}
}

func (cp *CommandProvider) ensureCursorVisible() {
	if cp.lastVisibleH > 0 {
		if cp.cy < cp.scrollY {
			cp.scrollY = cp.cy
		} else if cp.cy >= cp.scrollY+cp.lastVisibleH {
			cp.scrollY = cp.cy - cp.lastVisibleH + 1
		}
	}
	if cp.lastVisibleW > 0 {
		if cp.cx < cp.scrollX {
			cp.scrollX = cp.cx
		} else if cp.cx >= cp.scrollX+cp.lastVisibleW {
			cp.scrollX = cp.cx - cp.lastVisibleW + 1
		}
	}
	cp.clampScroll()
}

func (cp *CommandProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	style := theme.WindowBG
	cp.lastVisibleH = inner.H
	cp.lastVisibleW = inner.W
	cp.clampScroll()

	for row := 0; row < inner.H && cp.scrollY+row < len(cp.lines); row++ {
		line := cp.lines[cp.scrollY+row]
		if cp.scrollX < len(line) {
			visible := line[cp.scrollX:]
			if len(visible) > inner.W {
				visible = visible[:inner.W]
			}
			drawString(screen, inner.X, inner.Y+row, string(visible), style)
		}
	}

	// Cursor — render the cell at (cy, cx) with fg and bg swapped so
	// it pops against the WindowBG style. Done as an explicit swap
	// rather than tcell.Style.Reverse so renderers that don't honour
	// the Reverse attribute (e.g. the wasm canvas, which only reads
	// fg/bg from the snapshot) still draw a visible cursor block.
	//
	// Blinks at cursorBlinkPeriodMs intervals. The blink only animates
	// if the host triggers periodic redraws (App.Tick); without that,
	// the cursor freezes in whatever phase the most recent draw caught.
	// Skipped when the window isn't focused so an inactive command
	// window doesn't keep showing a stale cursor.
	cursorVisible := (time.Now().UnixMilli()/cursorBlinkPeriodMs)%2 == 0
	if focused && cursorVisible && cp.cy >= 0 && cp.cy < len(cp.lines) {
		cy := cp.cy - cp.scrollY
		cx := cp.cx - cp.scrollX
		if cy >= 0 && cy < inner.H && cx >= 0 && cx < inner.W {
			ch := ' '
			if cp.cx < len(cp.lines[cp.cy]) {
				ch = cp.lines[cp.cy][cp.cx]
			}
			fg, bg, _ := style.Decompose()
			cursor := style.Foreground(bg).Background(fg)
			screen.SetContent(inner.X+cx, inner.Y+cy, ch, nil, cursor)
		}
	}
}

// StatusHint returns the contextual hint shown on the status bar while
// the command window is active.
func (cp *CommandProvider) StatusHint() string {
	return "Enter: run line  ↑↓←→: navigate  HELP: commands "
}

func (cp *CommandProvider) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter:
		cp.execute()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		cp.backspace()
		return true
	case tcell.KeyDelete:
		cp.deleteForward()
		return true
	case tcell.KeyUp:
		if cp.cy > 0 {
			cp.cy--
			if cp.cx > len(cp.lines[cp.cy]) {
				cp.cx = len(cp.lines[cp.cy])
			}
			cp.ensureCursorVisible()
		}
		return true
	case tcell.KeyDown:
		if cp.cy < len(cp.lines)-1 {
			cp.cy++
			if cp.cx > len(cp.lines[cp.cy]) {
				cp.cx = len(cp.lines[cp.cy])
			}
			cp.ensureCursorVisible()
		}
		return true
	case tcell.KeyLeft:
		if cp.cx > 0 {
			cp.cx--
		} else if cp.cy > 0 {
			cp.cy--
			cp.cx = len(cp.lines[cp.cy])
		}
		cp.ensureCursorVisible()
		return true
	case tcell.KeyRight:
		if cp.cx < len(cp.lines[cp.cy]) {
			cp.cx++
		} else if cp.cy < len(cp.lines)-1 {
			cp.cy++
			cp.cx = 0
		}
		cp.ensureCursorVisible()
		return true
	case tcell.KeyHome:
		cp.cx = 0
		cp.ensureCursorVisible()
		return true
	case tcell.KeyEnd:
		cp.cx = len(cp.lines[cp.cy])
		cp.ensureCursorVisible()
		return true
	case tcell.KeyPgUp:
		step := cp.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		cp.cy -= step
		if cp.cy < 0 {
			cp.cy = 0
		}
		if cp.cx > len(cp.lines[cp.cy]) {
			cp.cx = len(cp.lines[cp.cy])
		}
		cp.ensureCursorVisible()
		return true
	case tcell.KeyPgDn:
		step := cp.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		cp.cy += step
		if cp.cy >= len(cp.lines) {
			cp.cy = len(cp.lines) - 1
		}
		if cp.cx > len(cp.lines[cp.cy]) {
			cp.cx = len(cp.lines[cp.cy])
		}
		cp.ensureCursorVisible()
		return true
	case tcell.KeyRune:
		cp.insertRune(ev.Rune())
		return true
	}
	return false
}

func (cp *CommandProvider) insertRune(r rune) {
	line := cp.lines[cp.cy]
	newLine := make([]rune, 0, len(line)+1)
	newLine = append(newLine, line[:cp.cx]...)
	newLine = append(newLine, r)
	newLine = append(newLine, line[cp.cx:]...)
	cp.lines[cp.cy] = newLine
	cp.cx++
	if len(newLine) > cp.longestLine {
		cp.longestLine = len(newLine)
	}
	cp.ensureCursorVisible()
}

func (cp *CommandProvider) backspace() {
	if cp.cx > 0 {
		line := cp.lines[cp.cy]
		cp.lines[cp.cy] = append(line[:cp.cx-1], line[cp.cx:]...)
		cp.cx--
	} else if cp.cy > 0 {
		// Merge current line into previous: cursor lands at the join.
		prev := cp.lines[cp.cy-1]
		cur := cp.lines[cp.cy]
		joinAt := len(prev)
		cp.lines[cp.cy-1] = append(prev, cur...)
		cp.lines = append(cp.lines[:cp.cy], cp.lines[cp.cy+1:]...)
		cp.cy--
		cp.cx = joinAt
		if l := len(cp.lines[cp.cy]); l > cp.longestLine {
			cp.longestLine = l
		}
	}
	cp.ensureCursorVisible()
}

func (cp *CommandProvider) deleteForward() {
	line := cp.lines[cp.cy]
	if cp.cx < len(line) {
		cp.lines[cp.cy] = append(line[:cp.cx], line[cp.cx+1:]...)
	} else if cp.cy < len(cp.lines)-1 {
		// Pull the next line up onto this one.
		next := cp.lines[cp.cy+1]
		cp.lines[cp.cy] = append(cp.lines[cp.cy], next...)
		cp.lines = append(cp.lines[:cp.cy+1], cp.lines[cp.cy+2:]...)
		if l := len(cp.lines[cp.cy]); l > cp.longestLine {
			cp.longestLine = l
		}
	}
	cp.ensureCursorVisible()
}

func (cp *CommandProvider) execute() {
	line := strings.TrimSpace(string(cp.lines[cp.cy]))
	if line == "" {
		// Empty Enter just adds a fresh blank line below — no command
		// dispatch, no error message. Mirrors a text editor.
		cp.AppendInputLine()
		return
	}
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
	cp.AppendInputLine()
}

// RegisterBuiltinCommands installs framework-level commands on the
// app's CommandRegistry: CLEAR/CLS, HELP, QUIT, VER.
//
// This is opt-in. Apps that want a familiar command-window vocabulary
// call it explicitly:
//
//	app := foxpro.NewApp() // or NewAppWithScreen
//	foxpro.RegisterBuiltinCommands(app)
//
// Hosts that don't want a particular builtin (e.g. browser hosts where
// QUIT terminates the wasm runtime and locks the page) can skip the
// call and register only the subset they want — via app.Commands.Register
// directly, or by composing their own helper packages.
func RegisterBuiltinCommands(app *App) {
	r := app.Commands
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
