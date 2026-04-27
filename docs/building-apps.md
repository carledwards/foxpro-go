# Building Apps on foxpro-go

This guide is for developers consuming the framework — building a TUI app
on top of `foxpro-go` rather than working on the framework itself. It
documents the supported surface area, common patterns, and the line
between "consumer code" and "framework internals."

For the full behavior spec see `foundation-ui-spec.md`. For what's
implemented today see `CHANGELOG.md`.

## Quick Start

```go
package main

import (
	"fmt"
	"os"

	foxpro "github.com/carledwards/foxpro-go"
)

func main() {
	app, err := foxpro.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer app.Close()

	app.Manager.Add(foxpro.NewWindow(
		"Hello",
		foxpro.Rect{X: 4, Y: 3, W: 40, H: 8},
		foxpro.NewTextProvider([]string{"Hello, foxpro-go."}),
	))

	app.Run()
}
```

That's it. `NewApp` initialises the terminal, sets the default theme and
settings, and creates an empty `WindowManager`. `Run` blocks until
`Quit` is called or the user hits Esc / Ctrl+Q.

## Anatomy of an App

An `App` owns five things you'll touch from your code:

| Field | Purpose |
| --- | --- |
| `Screen` | tcell screen — usually you don't touch it directly |
| `Theme` | active styles (built from a `Palette`) |
| `Settings` | runtime UI preferences (`ShowShadows`, etc.) |
| `Manager` | window list, focus, z-order |
| `MenuBar` | optional menu bar (set to enable) |
| `OnKey` | optional pre-handler for raw key events |

The framework owns the event loop, drawing pipeline, mouse handling,
and the chrome you see on every window. Your app contributes:

- Windows — each backed by a `ContentProvider` you implement
- An optional `MenuBar` and its callbacks
- A theme and settings, if you want to deviate from defaults

## Building a Window

The shape of any window is a `*Window` plus a `ContentProvider`. The
provider draws the inside and handles input directed at the window.

```go
type ContentProvider interface {
	Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool)
	HandleKey(ev *tcell.EventKey) bool
}
```

`inner` is the content area — the framework already drew the chrome
ring, so just paint into the rect you're given.

`HandleKey` returns `true` if it consumed the event. Return `false` to
let the event keep bubbling (currently nothing else handles it; this
is set up for a future event chain).

### Built-in providers

- `NewTextProvider(lines)` — scrollable read-only lines (arrow keys,
  PgUp/PgDn, Home/End). Implements `Scrollable`.
- `NewSettingsProvider(app)` — Apple-style settings page with mouse +
  keyboard support; see `NewSettingsWindow` to mount.
- `NewCommandProvider(registry)` — DOS-style command line with input,
  history, and live tail. Implements `Scrollable`. See
  `NewCommandWindow(app)` for the standard window factory.
- `NewTreeView(root)` — hierarchical `*TreeNode` browser with
  expand/collapse, lazy children (`TreeNode.Loader`), live `OnSelect`
  callback, scrollbars (V + H), mouse, and keyboard. Implements
  `Scrollable`, `MouseHandler`, `StatusHinter`.
- `NewPaneProvider(left, right, leftWidth)` — split-window container
  that runs two providers side by side, with Tab/Shift+Tab focus
  cycling. Forwards optional interfaces to the focused pane so the
  framework's scrollbars and status hints track the user.
- `NewBoxedProvider(inner, title)` — wraps any provider with a
  single-line border + internal scrollbar. Doesn't expose
  `Scrollable` outward (owns its own bar) so it composes safely
  inside containers without doubled chrome bars.

### Optional interfaces

A provider may also implement any of these. Each is opt-in via a
type assertion at the call site, so unrelated providers are unaffected.

| Interface | Method(s) | Triggers |
| --- | --- | --- |
| `StatusHinter` | `StatusHint() string` | Right side of the status bar while the window is active. Empty string = no hint. The hint is recomputed each frame, so it can vary with internal state. |
| `MouseHandler` | `HandleMouse(ev *tcell.EventMouse, inner Rect) bool` | Button-down events that land in the window body (HitBody zone). The event arrives with absolute screen coordinates; compare against the supplied `inner` rect. |
| `Scrollable` | `ContentSize() (w, h int)`, `ScrollOffset() (x, y int)`, `SetScrollOffset(x, y int)` | The framework draws scrollbars when content extent exceeds the visible viewport, and routes mouse-wheel, scrollbar-arrow, track-click, and thumb-drag events into `SetScrollOffset`. Implementations should clamp `(x, y)` to the valid range. |
| `WheelHandler` | `HandleWheel(ev *tcell.EventMouse, inner Rect) bool` | Wheel events with positional context. Useful for split layouts that need to scroll different children based on where the cursor is. The framework calls this *before* falling back to the top-level `Scrollable`. |

#### Implementing `Scrollable` cleanly

Two gotchas the built-in providers handle:

1. **Don't trust your stored offset until Draw**. The visible viewport
   height isn't known until the framework calls `Draw(inner)`. Cache
   `inner.H` (and `inner.W` if horizontal) into a `lastVisibleH` field
   each Draw, then clamp the offset against `len(content) - lastVisibleH`.
   `CommandProvider` shows the pattern.
2. **Auto-tail vs. user scroll** — for log-style providers, set
   `scrollY = len(content)` after appending and let `clampScroll()`
   pull it back to the new bottom on next Draw. If the user manually
   scrolled up, your stored offset stays untouched.

### Hooks

- `Window.OnClose func()` — runs when the close (`■`) is clicked.
  Default: removes the window from the manager.
- `Window.OnZoom func()` — runs when the maximize (`≡`) is clicked.
  Default: maximize/restore against the screen.

Set either to override default behavior (e.g. to show a "save first?"
dialog before closing).

## Menus

```go
app.MenuBar = foxpro.NewMenuBar([]foxpro.Menu{
	{
		Label: "&File",
		Items: []foxpro.MenuItem{
			{Label: "&New", Hotkey: "Ctrl+N", OnSelect: doNew},
			{Separator: true},
			{Label: "E&xit", OnSelect: app.Quit},
		},
	},
})
```

- `&` in a label marks the access-key letter (drawn in the accent
  color; `Alt+letter` opens the menu / triggers the item).
- `Hotkey` is right-aligned hint text only — the framework does **not**
  bind global shortcuts for you. Wire those up in `App.OnKey`.
- `Separator: true` draws a divider row.

### Menu Bar Tray (macOS-style status items)

Right-aligned items on the menu bar row. Use `Text` for static labels
or `Compute func() string` for live ones (the function runs each
frame, so the value can change without explicit redraws):

```go
app.MenuBar.Tray = []foxpro.TrayItem{
    {Compute: func() string {
        if a.isRefreshing.Load() { return "● refreshing" }
        return "● ready"
    }},
    {Text: "k8s: " + clusterName},
    {Text: "GCP: " + project, OnClick: openProjectPicker},
}
```

Items drop right-to-left when the terminal is too narrow, so put the
most important item last in the slice.

## Themes

Use a built-in:

```go
app.Theme = foxpro.ThemeFromPalette(foxpro.DraculaPalette())
```

Theme slots worth knowing about (full list in `style.go`):

| Slot | Used for |
| --- | --- |
| `WindowBG` | Window content background |
| `TitleActive` / `TitleInactive` | Top-row frame on focused / unfocused windows |
| `TitleAccent` (a `tcell.Color`) | Title text + close/zoom/resize chrome |
| `Focus` | Currently focused row/control inside a content provider — the brown FoxPro "selected button" look |
| `MenuBar` / `MenuBarActive` / `MenuAccel` | Menu bar |
| `Shadow` | Drop shadow (dark gray on black, dims runes underneath) |

Or define your own palette by constructing a `Palette` literal with all
16 slots filled in, then `ThemeFromPalette(yourPalette)`.

To register a palette in the **Settings → Appearance** picker, append
to `foxpro.ThemePresets` before opening the settings window:

```go
foxpro.ThemePresets = append(foxpro.ThemePresets, foxpro.ThemePreset{
	Name:    "Solarized",
	Palette: solarizedPalette,
})
```

## Settings

Read or write `app.Settings.*` directly — they're plain fields and
take effect on the next draw. The framework reacts immediately.

Add your own toggles by extending `SettingsProvider` (currently
requires a fork or wrapper provider; first-class extension is on the
roadmap).

## Widget Helpers

Inside a `ContentProvider.Draw` you can compose with the stateless
helpers in the `widgets/` subpackage:

```go
import "github.com/carledwards/foxpro-go/widgets"

widgets.DrawCheckbox(screen, x, y, width, checked, label, highlighted,
    theme.WindowBG, theme.MenuBarActive)
```

| Helper | Renders |
| --- | --- |
| `DrawCheckbox` | `[X] Label` row |
| `DrawRadio` | `(•) Label` row |
| `DrawListRow` | a selectable line with optional full-width highlight |
| `DrawButton` | `[ Label ]` in a single-line border |

These take explicit `tcell.Style` values rather than a `Theme`, so the
widgets package stays decoupled from the core. Pull the styles you
want out of your `Theme` at the call site. The settings window is the
worked example.

## Command Window

Apps register commands on `app.Commands`:

```go
app.Commands.Register("ECHO", "Print arguments back",
    func(cp *foxpro.CommandProvider, args string) {
        cp.Print(args)
    })
```

The handler can call `cp.Print(text)` (multi-line ok) and `cp.Clear()`.
The framework provides `HELP`, `CLEAR`, `CLS`, `QUIT`, and `VER` out
of the box. `F2` toggles the window; `app.ToggleCommandWindow()` does
the same imperatively.

## Keyboard Reservations

Bound at the framework level (your provider sees them only if you set
`App.OnKey` and consume first):

| Key | Action |
| --- | --- |
| `F1` | Toggle status bar |
| `F2` | Toggle command window |
| `F6` / `Shift+F6` | Cycle window focus |
| `F10` | Open menu bar |
| `Alt+<letter>` | Open menu by accelerator |
| `Esc` / `Ctrl+Q` | Quit app |

Free for your providers (NOT bound globally):

- `Tab` — intentionally left for in-content focus cycling.
- All other function keys, plain letters, navigation keys.

## Public API Surface

If you import a name from `foxpro` that isn't in this list, you're
probably reaching past the supported boundary. Open an issue / extend
the framework instead of depending on it.

Stable today:

```
App, NewApp                      Window, NewWindow
WindowManager (via App.Manager)  ContentProvider
  .Add .Remove .Raise .Active    StatusHinter (optional)
  .FocusNext .FocusPrev          MouseHandler (optional)
  .AllWindows .Contains          Scrollable (optional)
MenuBar, NewMenuBar              Menu, MenuItem, TrayItem
Theme, ThemeFromPalette          Palette, ClassicPalette,
DefaultTheme                       DraculaPalette,
                                   MonochromePalette,
Settings, DefaultSettings          RetroGreenPalette,
ThemePreset, ThemePresets          RetroAmberPalette
NewTextProvider                  Rect (X, Y, W, H, Contains, Inner)
NewSettingsProvider              CGABlack ... CGAWhite (palette
NewSettingsWindow                  slot accessors, back-compat)
NewCommandProvider               ShadedWidth (constant)
NewCommandWindow
CommandRegistry, CommandFunc
App.ToggleCommandWindow
App.Commands
App.Post, App.Tick
TreeNode, NewTreeView
PaneProvider, NewPaneProvider
BoxedProvider, NewBoxedProvider
WheelHandler (optional interface)
```

In `github.com/carledwards/foxpro-go/widgets`:

```
DrawCheckbox     CheckboxGlyph
DrawRadio        RadioGlyph
DrawListRow      SeparatorRow
DrawButton
```

Internal (don't depend on):

- Functions starting with a lowercase letter
- `boolSetting`, `radioSetting`, `choiceSetting`, `settingsCategory`
- `dragState`, `dragKind`, anything in `app.go`'s drag handling
- The internal `cgaComplement` map (use `Theme.InvertColor` instead)

## Contributing Patterns Back to the Framework

The foundation only stays coherent if reusable patterns flow back into
it instead of fragmenting across apps. **Treat this as a hard rule,
not a suggestion.** Ship-day pressure is the #1 source of framework
rot — give yourself permission to slow the app down so the framework
keeps pace.

### Triggers that REQUIRE you to stop

Pause and propose a framework change before continuing if any of these
are true:

1. You're copying code out of `foxpro-go` into your app and modifying it.
2. You're importing or otherwise reaching for an unexported symbol
   (lowercase identifier).
3. You're vendoring or forking a framework file to patch it.
4. You're defining a new optional interface (à la `StatusHinter`,
   `MouseHandler`, `Scrollable`). Optional interfaces are by
   definition a framework concern — apps can't make them work
   themselves, only the framework's dispatch can.
5. You're adding window chrome (close/zoom/scrollbar variants, title
   styling, drop-shadow tweaks, palette extensions).
6. You're building a widget another app would plausibly want
   (button variant, tree, table, dropdown, tabs).
7. Two or more apps have independently grown the same workaround.

If you ship past one of these, the framework will diverge silently
and the next refactor gets twice as expensive.

### Triggers that SHOULD prompt a conversation

Worth a discussion even if you decide it stays app-side:

- A new widget helper that's clean (no app coupling).
- A new content provider that does non-trivial input/scroll handling.
- A new color slot or theme convention.
- A wrapper that adapts framework APIs into something your app prefers.

### Workflow

When a trigger fires:

1. **Notice the moment.** Stop typing the app code.
2. **Sketch the pattern** in `docs/wishlist.md` of this repo with
   - the use case (a sentence)
   - the proposed API (rough)
   - whether it's a hard blocker or a nice-to-have
3. **Build the framework piece first.** Land it in `foxpro-go`,
   then have your app compose it.
4. **Don't merge the app feature** until the framework piece exists.
5. **Keep the wishlist alive** — even items you won't act on this week
   are worth recording so we don't forget the pattern.

### Anti-patterns to refuse outright

- **Vendoring** the framework to patch internals
- **"Just for this app"** — that's how reusable patterns get
  permanently buried in app repos
- **Parallel widget systems** built around framework primitives
- **Stuffing domain logic** into would-be reusable widgets so they
  can't be lifted out later
- **Re-implementing dispatch** (mouse routing, scrollbar geometry,
  shadow rendering) inside a content provider

### Pre-ship checklist for app authors

Before you tag a release of any app built on foxpro-go:

- [ ] No internal (lowercase) framework symbols accessed
- [ ] No copy-paste from framework files
- [ ] No vendored/forked `foxpro-go` packages in the app repo
- [ ] Every reusable pattern has either landed in `foxpro-go` or has
      a `docs/wishlist.md` entry with a name on it
- [ ] Optional interface implementations live in your provider, not
      via framework hacks

Skipping this checklist is how the framework dies.

## Where Does My Code Belong?

Use this rule of thumb when deciding if something goes in the framework
or in your app:

| Goes in the framework | Goes in your app |
| --- | --- |
| Window chrome, menus, themes | Menu **structure** and callbacks |
| Drop shadows, scrollbars, dialogs | Domain windows + their providers |
| `StatusHinter`, `ContentProvider` shapes | Implementations of those |
| Reusable widget primitives | Composed screens (settings, list views, forms) |
| Keyboard / mouse routing | Keys consumed inside your provider |
| Mouse cursor inversion | App-specific cursor behavior (none yet) |

If you find yourself patching framework internals to make your app
work, that's a signal to either:

1. **Add a hook** to the framework (callback, optional interface,
   exported field) and contribute it back, or
2. **Compose** — write your own provider/window that does what you
   need on top of the existing primitives.

Avoid forking individual files into your repo to tweak them — the
framework is small enough that PRs land easily.

## Common Patterns

### Non-modal floating window with fresh state

```go
w := foxpro.NewWindow("My Tool", foxpro.Rect{X: 6, Y: 4, W: 50, H: 14}, &myProvider{})
app.Manager.Add(w)
```

`Add` puts it on top and focuses it. Your provider's struct holds any
per-instance state.

### Window that survives close (singleton)

Keep the `*Window` reference across hide/show cycles so its content
provider's state (history, scroll position, input buffer) persists.

```go
var debugWin *foxpro.Window

func toggleDebug() {
    if debugWin == nil {
        debugWin = foxpro.NewWindow("Debug", foxpro.Rect{...}, &debugProvider{})
        debugWin.OnClose = func() {
            app.Manager.Remove(debugWin) // hide; do NOT clear debugWin
        }
    }
    if app.Manager.Contains(debugWin) {
        app.Manager.Remove(debugWin)
    } else {
        app.Manager.Add(debugWin)
    }
}
```

The framework's command window uses exactly this pattern — see
`App.ToggleCommandWindow` for the worked example.

### Pre-handling a key before any window sees it

```go
app.OnKey = func(ev *tcell.EventKey) bool {
    if ev.Key() == tcell.KeyCtrlN {
        openNewDocument()
        return true // consume
    }
    return false // pass through to built-ins / windows
}
```

### Contextual status hint

```go
type myProvider struct{ /* ... */ }

func (p *myProvider) StatusHint() string {
    return "Enter: open  Del: remove  /: search"
}
```

The framework auto-detects the interface and shows the hint while your
window is active.

### Scrollable list with mouse + wheel

Implement `Scrollable` and the framework draws the scrollbar, handles
arrow / track / thumb clicks, and routes mouse-wheel events to your
provider — no extra wiring on your side.

```go
func (p *myList) ContentSize() (int, int) { return p.maxLineWidth, len(p.rows) }
func (p *myList) ScrollOffset() (int, int) { return 0, p.scrollY }
func (p *myList) SetScrollOffset(x, y int) {
    if y < 0 { y = 0 }
    if max := len(p.rows) - p.lastVisibleH; y > max { y = max }
    p.scrollY = y
}
```

If you also implement `MouseHandler`, body clicks (selection) work
alongside scrollbar interaction (which the framework owns).

### Updating the UI from a goroutine

Background work (data refresh, file watch, tail) lives in a goroutine
but UI mutations must happen on the main loop. Use `App.Post` to
hand work back over:

```go
go func() {
    rows, err := refreshFromAPI()
    app.Post(func() {
        if err != nil { /* surface */ ; return }
        view.SetRoot(buildTree(rows))
    })
}()
```

For a steady cadence (spinners, clocks, time-based refresh):

```go
stop := app.Tick(100*time.Millisecond, func() {
    if running.Load() { spinner.Add(1) }
})
defer stop()
```

`Tick` with `nil` fn just wakes the loop for a redraw — useful when
your tray uses `Compute` and you need it to update on a steady tempo.

### Defensive layout

Provider `Draw` gets a fresh `inner Rect` every frame — never cache
dimensions across calls (windows resize, get shaded, get maximized).
Cap row counts and string lengths against `inner.W` / `inner.H`
yourself; widget helpers in `widgets/` already clip to their `width`
parameter.

## What's Coming

Track `foundation-ui-spec.md` for items marked `[planned]` /
`[future]`. The big ones: dialogs, scrollbars, mouse routing into
content providers, command window, persistence. When they land they'll
be added to the surface table above.
