# Changelog

Notable changes to the foxpro-go framework, in reverse chronological order.
Each entry is a "checkpoint" — a coherent snapshot of what works.

## Unreleased

- **`BoxedProvider`** — wraps any provider with a single-line border
  + internal scrollbar. Composes inside `PaneProvider` for FoxPro-style
  panel-in-window layouts without competing scrollbar geometry.
- **`PaneProvider.HandleWheel`** now tries `WheelHandler` on each
  child before falling back to `Scrollable`, so nested split layouts
  and `BoxedProvider` panes route the wheel correctly.
- **`PaneProvider` per-pane scrollbars** — left pane scrollbar drawn
  on the divider column; right pane uses the window's chrome bar.
  No more shared chrome bar tracking focus.
- **`WheelHandler` optional interface** — providers receive wheel
  events with positional context. Used by `PaneProvider` to route
  the wheel to whichever pane the cursor is over (not just whichever
  has keyboard focus).
- **`PaneProvider`** — split-window container; two child
  `ContentProvider`s share a window with `Tab`/`Shift+Tab` focus
  cycling and one-cell divider. `MouseHandler` and `StatusHinter`
  forwarded to the focused pane.
- **Horizontal scroll** in `TreeView` and `TextProvider` — both report
  a real `ContentSize` width and apply `scrollX` when drawing. The
  framework's existing horizontal-scrollbar code (arrows, track click,
  thumb drag, mouse wheel) just works.
- **`TreeView.ReplaceRoot`**, **`SelectedPath`**, **`SelectByPath`**,
  **`MergeTreeState`** — preserve user expansion + selection across
  data refreshes.
- **`App.Post` / `App.Tick`** — goroutine-safe UI updates and a steady
  redraw cadence. Implemented via `tcell.EventInterrupt` posted from
  background goroutines; the main loop runs the callback then redraws.
- **Tree view (`TreeView` + `widgets.DrawTreeRow`)** — hierarchical
  `*TreeNode` browser with expand/collapse, lazy children via
  `TreeNode.Loader`, `OnSelect` / `OnActivate` callbacks, scrollbars,
  mouse, and keyboard.
- **System tray** (`MenuBar.Tray`) — right-aligned status items on
  the menu bar with optional `Compute` and `OnClick`. Items drop
  right-to-left when the terminal is narrow.
- **Mouse routing into content providers** via `MouseHandler`
  optional interface.
- **Scrollbars + Scrollable interface** — vertical and horizontal
  bars draw automatically when content exceeds the viewport. Mouse
  wheel, arrow clicks, track click (page jump), and thumb drag all
  routed to `SetScrollOffset`.
- **Window shade** — double-click title to collapse to a fixed-width
  title-only bar.
- **Command window** — F2 toggles, registry-based, `HELP`/`CLEAR`/
  `QUIT`/`VER` built in. State survives hide/show cycles.
- **Settings page + Theme picker** — Apple-style page with categories,
  live-preview radio rows; built-in palettes Classic FoxPro, Dracula,
  Monochrome, Retro Green, Retro Amber.
- **Theme.Focus** slot — brown FoxPro "selected" look.

## Checkpoint 1 — 2026-04-25

The first end-to-end pass: a windowed desktop, menu bar, settings page,
swappable palettes, and FoxPro-faithful chrome.

### Rendering & theming

- tcell-backed cell grid; all colors specified as 24-bit RGB so the host
  terminal's palette can't recolor the UI.
- `Palette` type with 16 named CGA slots; `Theme` is built from a
  `Palette` via `ThemeFromPalette`.
- Built-in palettes: `ClassicPalette`, `DraculaPalette`,
  `MonochromePalette`, `RetroGreenPalette`, `RetroAmberPalette`.
- `Theme.InvertColor` returns the palette-specific cursor complement
  (XOR-low-3-bits rule).

### Windows

- Floating windows with no visible borders — just a 1-cell ring of
  spaces in the title-bar color. Top-row chrome only.
- Active windows show close (`■`), maximize (`≡`), and resize (`.`)
  glyphs in the accent color (yellow by default). Inactive windows show
  none.
- Drag title to move, drag bottom-right corner to resize, click to
  raise. Click positions on inactive windows always raise first
  (FoxPro behavior).
- Maximize fills to the screen's right and bottom edges; restore
  reverts to prior bounds.
- Double-click title (500 ms / same cell) toggles **window shade**:
  collapse to a 1-row title bar of fixed `ShadedWidth = 32`, no drop
  shadow.
- Drop shadows: full window-rect at `(+2, +1)` offset, replaces fg+bg
  with `Theme.Shadow` (dark-gray on black) while preserving the rune,
  so chars underneath stay legible.
- `Window.OnClose` / `OnZoom` callbacks override default actions.

### Menu bar

- Top-row bar with `&accelerator` syntax, click or `Alt+letter` opens
  drop-downs.
- Items support label, optional `Hotkey` hint (right-aligned), and
  separators.
- Theme slots: `MenuBar`, `MenuBarActive`, `MenuAccel`,
  `MenuAccelActive`, `MenuDisabled` (cyan), `MenuHotkey` (white;
  reserved for future use).

### Mouse

- Inverted-cell cursor using the active palette's CGA complement.
- Auto-hides on any key press (terminals don't deliver modifier-only
  events, so every `EventKey` qualifies).

### Status bar

- Bottom row, toggled by `F1` (or `Settings.ShowStatusBar`).
- Left side: built-in shortcuts. Right side: contextual hint from the
  active window's content provider via the optional `StatusHinter`
  interface.

### Settings

- `Settings` struct (in-memory): `ShowShadows`, `ShowStatusBar`,
  `ThemeIndex`. Exposed on `App.Settings`.
- `SettingsProvider` is a reference content provider rendering an
  Apple-style page: categories on the left, controls on the right.
- Built-in categories: **General** (the toggles) and **Appearance**
  (a radio list of `ThemePresets` with live preview as you navigate).
- Open from menu: `app.Manager.Add(foxpro.NewSettingsWindow(app))`.

### Widget helpers

Stateless drawing primitives in the `widgets/` subpackage (functions
take `tcell.Style` values, no dependency on the core `foxpro` package):

- `DrawCheckbox(...)` — `[X]`/`[ ]` rows
- `DrawRadio(...)` — `(•)`/`( )` rows
- `DrawListRow(...)` — selectable list line
- `DrawButton(...)` — `[ Label ]` with single-line border
- `CheckboxGlyph` / `RadioGlyph` — glyph string helpers

### Keyboard

| Key | Action |
| --- | --- |
| `F1` | Toggle status bar |
| `F6` / `Shift+F6` | Cycle window focus forward / backward |
| `F10` | Open menu bar |
| `Alt+<letter>` | Open menu by accelerator |
| `Esc` / `Ctrl+Q` | Quit |
| `Tab` | Reserved for content providers (NOT bound globally) |

### Still on the roadmap

- Mouse handling on content (clicks inside a window's body)
- Modal dialogs (with double-line borders)
- Scrollbars + extended `ContentProvider` interface
- Command window (DOS-style commands)
- Persistence (save/load `Settings` and window layout)

See `foundation-ui-spec.md` for the full spec with `[done]`/`[planned]`
markers.
