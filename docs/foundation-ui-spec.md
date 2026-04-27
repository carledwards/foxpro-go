# Foundation UI Spec

This document defines the reusable terminal UI behavior provided by the
`foxpro-go` framework. The primary interaction and visual influence is the
FoxPro for DOS application model.

Status legend: **[done]** implemented today, **[planned]** intended next,
**[future]** longer term.

## Rendering Foundation

- **Backend**: tcell (`github.com/gdamore/tcell/v2`) — direct cell-grid
  control, no MVU/widget framework on top.
- **Coordinate system**: top-left origin, integer cells. `Rect{X,Y,W,H}` is
  the universal bounding box.
- **Truecolor**: every theme color is specified as 24-bit RGB via
  `tcell.NewRGBColor` so the host terminal's palette/theme cannot recolor
  the UI. **[done]**

## Theme vs Palette

- **Palette**: concrete RGB values for the 16 IBM CGA color slots. Exposed
  as `CGABlack`, `CGABlue`, ... `CGAWhite`.
- **Theme**: maps semantic UI roles (`Desktop`, `WindowBG`, `TitleActive`,
  `MenuBar`, `Shadow`, ...) to `tcell.Style` values built from the palette.
- **Relationship**:
  - Theme defines **what** each UI element should look like.
  - Palette defines **how those colors render** on screen.
- **Default theme**: `DefaultTheme()` returns the FoxPro classic look —
  royal-blue desktop, cyan window bodies with white text, light-gray
  active title bars, red menu accelerators, dark-gray-on-black shadows.

### Theme Responsibilities

- Desktop background (`Desktop`, `DesktopRune`)
- Menu bar (`MenuBar`, `MenuBarActive`, `MenuAccel`, `MenuAccelActive`,
  `MenuDisabled`, `MenuHotkey`)
- Window chrome (`WindowBG`, `Border`, `TitleActive`, `TitleInactive`,
  `TitleAccent`, `Shadow`)
- Future: dialog colors (`DialogBG`, `Button`, `ButtonFocused`,
  `Input`, `InputFocused`) **[planned]**
- Future: scrollbar glyphs and colors **[planned]**

### Palette Responsibilities

- Provide authentic CGA RGB values regardless of host terminal theme.
- Allow alternative palettes without changing UI logic. **[done]**
- Built-in presets in `ThemePresets`: Classic FoxPro, Dracula,
  Monochrome, Retro Green, Retro Amber. Apps may extend the list.

## Settings **[done]**

The `Settings` struct holds runtime UI preferences (in-memory only for
now; persistence is planned):

- `ShowShadows` — toggle window drop shadows globally.
- `ShowStatusBar` — toggle the bottom hint row.
- `ThemeIndex` — index into `ThemePresets` for the active palette.

Exposed on `App.Settings`. The `SettingsProvider` content provider
(reference UI) renders these as an Apple-style page — categories on the
left, controls on the right — and is launched via `NewSettingsWindow`.

## Goroutine-Safe Updates **[done]**

Two methods on `App` let goroutines drive the UI without races:

- `App.Post(fn func())` — schedules `fn` to run on the main UI
  goroutine on the next event-loop iteration. Pass `nil` to wake the
  loop for a redraw without other work (useful when state read by a
  Compute callback has changed).
- `App.Tick(d, fn) (stop func())` — fires `fn` on the main goroutine
  every `d`. Returns a stop function. `fn` may be `nil` for a steady
  redraw cadence (animated spinners, clocks, refresh status).

Both are implemented as `Screen.PostEvent(tcell.NewEventInterrupt)`
from a goroutine; `Run` handles `*tcell.EventInterrupt` and runs the
attached callback before its implicit redraw.

## Boxed Container **[done]**

`BoxedProvider` wraps any `ContentProvider` with a single-line border
(`┌─┐│└┘`) and an internal vertical scrollbar drawn on the right
column when the inner is `Scrollable` and overflows.

- Optional `Title` is centred in the top border in the accent colour.
- The wrapper does **not** expose `Scrollable` to the outside — it
  owns its scrollbar — so it composes safely inside containers like
  `PaneProvider` without competing chrome bars.
- Forwards `MouseHandler`, `WheelHandler`, and `StatusHinter` to the
  wrapped provider; click on the box's scrollbar runs its own scroll
  logic.

## Pane Container **[done]**

`PaneProvider` splits a window into a left + right pane, each running
its own `ContentProvider`:

- `Left`, `Right` — child providers; `LeftWidth` is the cells reserved
  for the left pane.
- `Tab` / `Shift+Tab` cycle focus between the two panes; mouse clicks
  also focus and forward to the clicked pane's `MouseHandler`.
- `StatusHinter` is forwarded to the *focused* pane.

### Per-pane scrollbars

Each pane gets its own visible scrollbar — no more shared chrome bar
that confuses users about which pane it controls:

- **Right pane** uses the window's right-edge chrome scrollbar
  (`PaneProvider` forwards `Scrollable` to `Right` so the framework's
  default routing handles arrows, track click, and thumb drag).
- **Left pane** scrollbar is overlaid on the divider column itself
  (PaneProvider draws it, handles arrow + track clicks). Thumb drag
  for the left pane is not yet implemented.

### Wheel routing

`PaneProvider` implements the optional `WheelHandler` interface so
mouse-wheel events route to whichever pane the cursor is over,
independent of which pane has keyboard focus.

## Tree View **[done]**

`TreeView` is a built-in `ContentProvider` for hierarchical data:

- `TreeNode{Label, Payload, Children, Expanded, Loader}` — payloads are
  `interface{}`; `Loader` enables lazy children.
- Implements `Scrollable`, `MouseHandler`, `StatusHinter` —
  scrollbars, click-to-select, click-on-marker-to-toggle, and contextual
  hint all wired automatically.
- Keyboard: `↑/↓` move, `←/→` collapse / expand, `Enter` toggle + activate.
- `OnSelect(*TreeNode)` fires whenever the selection moves;
  `OnActivate(*TreeNode)` fires on Enter (after toggling expansion).
- `HideRoot` flag draws root-children at indent 0 if you don't want the
  root visible.

## Widget Helpers **[done]**

Stateless drawing primitives in the `widgets/` subpackage for use
inside a `ContentProvider`. The package has no dependency on the core
`foxpro` package — functions take `tcell.Style` values directly so
callers decide which theme slots to feed in.

- `DrawCheckbox` — `[X] Label` row
- `DrawRadio` — `(•) Label` row
- `DrawListRow` — selectable row with full-width highlight
- `DrawButton` — `[ Label ]` in a single-line bordered box
- `CheckboxGlyph` / `RadioGlyph` — glyph string helpers

The `SettingsProvider` is a worked example of composing these into a
real screen.

## Keyboard Controls

### Global

- `F10`: open the menu bar (focuses first menu, opens its drop-down).
  **[done]**
- `Alt+<menu access key>`: open the matching top-level menu directly.
  **[done]**
- `F6` / `Shift+F6`: cycle window focus forward / backward, raising the
  newly focused window to the front. `Tab` is intentionally **not**
  bound at the app level so content providers (settings panes, dialogs,
  etc.) can use it to move focus between their own controls. **[done]**
- `F1`: toggle the bottom status bar (a shortcut for
  `Settings.ShowStatusBar`). **[done]**
- `F2`: toggle the command window. **[done]**
- `Esc` / `Ctrl+Q`: quit the app. **[done]** (callers can intercept via
  `App.OnKey` if they want different semantics)

### Status Bar **[done]**

When `Settings.ShowStatusBar` is true the bottom row is reserved for a
hint strip. The left side shows built-in app shortcuts (F1 / F10 / F6 /
Esc). The right side shows a contextual hint from the active window's
content provider — any `ContentProvider` that also implements
`StatusHinter` (single method `StatusHint() string`) contributes the
text. The hint is redrawn each frame, so providers can return different
text depending on internal focus state (e.g. left vs right panel).

### Menu Navigation

- `Left` / `Right`: move between top-level menus.
- `Up` / `Down`: move within drop-down items, skipping separators.
- `Enter`: invoke the highlighted item.
- `Esc`: close the menu, return focus to the previously active window.
- Access-key letters invoke the matching enabled item in the open
  drop-down. **[done]**

### Dialogs **[planned]**

- `Tab`: cycle focus through dialog controls.
- `Esc`: dismiss / cancel.
- `Enter`:
  - On a button → trigger that button.
  - Elsewhere → trigger the dialog's default button (if any).

### Window Content

- The active window's `ContentProvider.HandleKey` sees every key event
  the menu bar and built-ins haven't consumed. Returning `false` lets the
  framework continue the chain (currently no further chain). **[done]**

## Mouse Controls

### General

- **Click on a window** raises and focuses it. **[done]**
- **Drag the title bar** to move the window. Position is clamped so the
  title bar stays reachable on screen. **[done]**
- **Drag the bottom-right corner** to resize. Minimum size 8×3, clamped
  to the screen. **[done]**
- **Click on the menu bar** opens that menu's drop-down. Click outside
  closes the menu. **[done]**
- **Click the close box** (`■`, top-left of title bar) → window's
  `OnClose` runs, defaulting to `Manager.Remove(w)`. **[done]**
- **Click the maximize box** (`≡`, top-right of title bar) → window's
  `OnZoom` runs, defaulting to a maximize/restore toggle. Maximized
  bounds extend to the screen's right and bottom edges (the drop shadow
  is allowed to clip off-screen). **[done]**
- **Double-click title bar** → toggle window-shade: collapse the window
  to a 1-row title bar of fixed width (`ShadedWidth = 32`) so shaded
  titles stack neatly, or expand back to the prior size and width.
  Shaded windows render without a drop shadow so they sit flat on the
  desktop. Threshold is 500 ms / same-cell. **[done]**

### Mouse Cursor

- Drawn as a single inverted cell at the pointer position.
- Inversion uses the **CGA color complement** (XOR low-3-bits of the
  4-bit color slot, intensity preserved): blue↔brown, cyan↔red,
  green↔magenta, black↔light-gray, etc. The original rune is preserved
  so text remains legible underneath. Exposed as `CGAInvert(c)`.
  **[done]**
- **Auto-hide**: the cursor is shown on every mouse event and hidden on
  the next real key event. Terminals do not deliver modifier-only key
  events, so any received `EventKey` qualifies as input. **[done]**

### Scroll **[planned]**

- Wheel events route to the scrollable area under the pointer.
- `Shift + wheel` is the canonical horizontal scroll modifier.
- Optional axis-lock / hysteresis to reduce jitter from mixed-axis events.

## Menus **[done]**

- Top-level menus are arranged left-to-right on row 0.
- Each menu's `Label` uses `&` to mark the access-key letter (e.g.
  `&File`); the marker is stripped before display and the letter is
  drawn in the accent color (`MenuAccel`).
- Drop-down items support `Label`, optional `Hotkey` (right-aligned hint
  text such as `Ctrl+O`), `OnSelect` callback, and `Separator` flag.
- Drop-downs are drawn last in the render pass with their own drop
  shadow so they paint above all windows.
- Dynamic composition (menus that change based on app state) is the
  caller's responsibility — rebuild and reassign `App.MenuBar` whenever
  the menu structure should change.

## System Tray **[done]**

- Right-aligned items on the menu bar row, separated by ` │ `.
- Each `TrayItem` carries `Text` (static) **or** `Compute func() string`
  (refreshed every frame) — Compute wins when both are set. Use Compute
  for live status (clock, refresh state, connection indicator).
- Optional `OnClick func()` makes the item clickable; the bar caches
  hit-test rects from the last draw and routes presses back.
- Items drop right-to-left when they don't fit, so the rightmost
  (most important) survives a narrow terminal.

## System Display (Background) **[done]**

- The desktop is a single styled cell-fill (`Theme.Desktop` style with
  `Theme.DesktopRune` painted in every cell).
- Solid by default. Hatched patterns (e.g. `░` in a contrast color) can
  be enabled via theme override.

## Windows

### Lifecycle and Placement

- Windows are managed in z-order (`WindowManager.windows`). Index 0 is
  back-most; index `len-1` is the front and active. **[done]**
- `Add`, `Remove`, `Raise`, `FocusNext` mutate z-order/focus.
- Off-screen placement is constrained during drag (title bar must stay
  reachable). Initial placement is the caller's responsibility.

### Size and Bounds

- Minimum size during interactive resize: `8×3`. Programmatic sizes are
  not enforced.
- Resize is clamped to the visible screen.
- Bottom-right corner cell is the resize hit target.

### Window Chrome **[done]**

Floating (non-modal) windows have no visible border lines. The "frame"
is a 1-cell ring of plain spaces in the frame style; the perimeter
glyphs come entirely from the chrome controls and title text.

- **Frame**: top row, side columns, and bottom row are filled with
  spaces in `TitleActive` (when focused) or `TitleInactive` (otherwise).
- **Close box** (`■`): top-left **corner** at `(b.X, b.Y)`. Drawn only
  when the window is the active (topmost) one and `Window.Closable` is
  true.
- **Maximize box** (`≡`): top-right **corner** at `(b.X+b.W-1, b.Y)`.
  Drawn only on the active window when `Window.Zoomable` is true. Click
  toggles maximize against the screen edges; if currently shaded, it
  unshades first.
- **Resize handle** (`.`): bottom-right corner at
  `(b.X+b.W-1, b.Y+b.H-1)`. Drawn only on the active window.
- **Accent colour**: title text and all chrome glyphs use
  `Theme.TitleAccent` (yellow by default), combined with the current
  frame background — so they read correctly regardless of focus state.
- **Inactive windows**: show no chrome controls. Clicking on the
  position where a control would be just raises the window to active;
  the action does not fire on the same click. (FoxPro behavior.)
- Title text is centred in the top row and truncated to fit between the
  reserved chrome positions.

Modal dialogs (planned) will use double-line glyphs (`╔═╗║╚╝`) instead
of the chrome-only frame, to mark them visually as modal.

### Focus Rules

- Exactly one window is active at any time when at least one exists.
- Closing the active window reassigns focus to the new top-most window.
- Drawing reflects focus: active window gets `TitleActive`; others get
  `TitleInactive`.

### Z-Order

- Active window is always at the top of z-order.
- Z-order persistence is the caller's responsibility — the framework
  exposes `AllWindows()` for snapshotting. **[planned]** built-in helper.

## Drop Shadows **[done]**

- Geometry: a window-sized rect at `(+2, +1)` offset. The window itself
  paints over the overlap region, so the visible shadow is a 2-column
  right strip + 1-row bottom strip — offset such that the top-right and
  bottom-left corners stay clear.
- Color: `Theme.Shadow` (default `bg=CGABlack, fg=CGADarkGray`).
- The underlying rune is preserved; only fg+bg are replaced. Characters
  poking through the shadow render dim-but-legible.

## Dialog Boxes **[planned]**

- Modal while open — block input to other windows.
- Not resizable, but movable via title bar drag.
- Tab-focusable controls (buttons, input fields, lists).
- Single default action invoked by `Enter` from any non-button control.
- `Esc` maps to cancel/dismiss.

## Command Window **[done]**

- DOS-style command line as a non-modal floating window.
- Toggled by `F2`, or imperatively via `App.ToggleCommandWindow()`.
- Apps register commands on `App.Commands` (a `*CommandRegistry`).
  Commands are case-insensitive, keyed on the first whitespace-delimited
  token; remaining text is passed as `args`.
- Built-in commands: `HELP`, `CLEAR` / `CLS`, `QUIT`, `VER`.
- Handler signature: `func(cp *CommandProvider, args string)` — the
  handler can call `cp.Print(text)` to add output and `cp.Clear()` to
  wipe history. `cp.Registry()` exposes the full registry for HELP-style
  introspection.
- Up-arrow input history recall is **[planned]** — not yet implemented.

## Persistence (Foundation) **[planned]**

The framework will expose serialization helpers; storage location is the
caller's responsibility.

Typical persisted state:

- open window layout (position, size, title)
- z-order
- active window identity
- menu/command-window visibility toggles

## Non-Goals

- Not a widget toolkit (no buttons, lists, inputs as first-class
  primitives). Build those inside `ContentProvider` implementations or
  as part of the dialog system.
- Not an MVU framework. State lives where the caller puts it.
- No theming DSL. Themes are plain Go structs the caller can mutate.
