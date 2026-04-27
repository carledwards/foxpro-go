# Framework Wishlist

Place to record patterns that should land in `foxpro-go` but haven't
yet. Add an entry the moment you'd otherwise be tempted to "just do
it in the app" — see *Contributing Patterns Back* in
`building-apps.md`.

Entries are not commitments — they're a memory aid. Each one is one
of:

- **\[blocker]** an app needs this to ship cleanly
- **\[lift]** an app already built it; it needs to be lifted into
  the framework before the next app re-invents it
- **\[idea]** worth doing, no current driver

## Format

```
### Title
- **Status:** blocker | lift | idea
- **Proposed by:** name / app
- **Use case:** one sentence
- **Sketch:** rough API surface — types, methods, files
- **Notes:** open questions, alternatives considered
```

---

## Open

### Modal dialogs
- **Status:** idea
- **Proposed by:** foundation
- **Use case:** confirm/cancel actions, input prompts, "save first?"
  on close.
- **Sketch:** `Dialog` window subtype with double-line border, Tab
  cycles controls, Esc cancels, Enter triggers default button. Modal
  while open: blocks input to other windows.
- **Notes:** see `foundation-ui-spec.md` "Dialog Boxes" section.

### Persistence
- **Status:** idea
- **Use case:** save/load `Settings` and window layout across runs.
- **Sketch:** `App.LoadState(path)` / `App.SaveState(path)` —
  caller chooses location, framework handles serialization of the
  in-memory structs.

### Wheel scrolling on widgets that aren't full-window Scrollables
- **Status:** idea
- **Use case:** a settings page with many rows could want wheel scroll
  inside the right panel without making the whole provider Scrollable.
- **Sketch:** maybe nested Scrollable regions, or a viewport widget.

### Stateful Button widget with press/drag/release semantics
- **Status:** lift
- **Proposed by:** 6502-sim-tui
- **Use case:** `widgets.DrawButton` is stateless — fine for static
  rendering but doesn't handle the standard "press → drag-off cancels
  → drag-back arms → release-inside fires" UX every clickable button
  needs. 6502-sim-tui's display window has implemented the state
  machine inline by combining `MouseHandler` + `MouseDragHandler`,
  but it's the kind of micro-protocol every clickable should reuse.
- **Sketch:**
  ```go
  type Button struct {
      Label   string
      Style   tcell.Style   // resting
      Armed   tcell.Style   // pressed-and-still-inside
      OnClick func()
  }

  // Place inside a Canvas at logical coords; renders the label.
  func (b *Button) Draw(c *foxpro.Canvas, x, y int)

  // HandleMouse / HandleMouseMotion / HandleMouseRelease implement
  // the standard armed-on-press, fire-on-release-inside pattern.
  // Provider just forwards events to the button.
  func (b *Button) HandleMouse(ev *tcell.EventMouse, inner Rect, scrollX, scrollY int) (consumed, capture bool)
  func (b *Button) HandleMouseMotion(ev *tcell.EventMouse, inner Rect, scrollX, scrollY int)
  func (b *Button) HandleMouseRelease(ev *tcell.EventMouse, inner Rect, scrollX, scrollY int)
  ```
- **Notes:** Could grow to handle keyboard activation (Space/Enter
  when focused) and a default-action variant (the boxed FoxPro "See
  Also" look). The Canvas/scroll translation in the API is awkward —
  maybe Button operates entirely in logical coords and providers do
  the screen→logical translation before forwarding.

### Up/Down arrow command-history recall in `CommandProvider`
- **Status:** idea
- **Use case:** type a command, press Up to recall the previous one.
- **Sketch:** keep a `submitted []string` ring; intercept Up/Down
  before scroll dispatch when the input line is empty or matches the
  current recall.

## Done

### Canvas + ScrollState (clipped, scrollable drawing surface)
- **Landed:** `Canvas` and `ScrollState` in `canvas.go`.
- **Use case:** Eliminates the put-with-clip pattern that was
  hand-rolled in every content provider. Provider gets a `*Canvas`
  bound to its inner rect, draws in *logical* coordinates (origin
  0,0), and the canvas auto-translates by the current scroll offset
  and clips at the inner rect. Natural extents are tracked as a side
  effect of every Put / Set.
- **Pattern:**
  ```go
  type Provider struct {
      foxpro.ScrollState  // satisfies Scrollable for free
      // ...
  }
  func (p *Provider) Draw(screen, inner, theme, focused) {
      c := foxpro.NewCanvas(screen, inner, &p.ScrollState)
      c.Put(0, 0, "Hello", theme.WindowBG)
  }
  ```
- Used by 6502-sim-tui's cpuwin, clockwin, ramwin, debugwin — all
  five floating windows scroll cleanly when resized below their
  natural content size, and clip without artifacts.
- **Future:** Optional overflow indicator (`▶` or `…`) on lines that
  have content beyond the right edge — shown automatically when
  `naturalW > inner.W`.

### Per-window minimum size
- **Landed:** `Window.MinW` / `Window.MinH` in `window.go`; enforced
  in the drag-resize handler in `app.go`.
- **Use case:** Providers know how small their layout can shrink
  before becoming useless. Setting MinW/MinH stops drag-resize at
  that floor instead of falling through to the framework's default
  (8 × 3). With Canvas's scroll fallback, content below the min is
  scrollable rather than hidden.
- Used by 6502-sim-tui — each provider exports `MinW` / `MinH`
  constants and the wire-up in `main.go` plumbs them through.

### Periodic redraw / ticker
- **Landed:** `App.Post(fn)` and `App.Tick(d, fn)` in `app.go`.
- Posts a `tcell.EventInterrupt` from any goroutine; the main loop
  runs the callback (if any) and redraws on the next iteration.
- Used by kubism for the tray spinner (100 ms tick) and to rebuild
  the cluster tree after a background refresh completes.

### Pane container (left-selector / right-detail)
- **Landed:** `PaneProvider{Left, Right, LeftWidth}` in `pane.go`.
- Two `ContentProvider`s share one window, divided by a 1-cell
  vertical bar. Tab / Shift+Tab swap focus.
- Forwards `MouseHandler`, `StatusHinter`, and `Scrollable` to the
  *focused* pane so the framework's scrollbars and status hints
  follow the user.
- Used by kubism's cluster window (tree + detail).
- Future: settings window should also adopt this; it predates the
  primitive and still hand-rolls a similar layout.
