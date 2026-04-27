# foxpro-go

A FoxPro-for-DOS-style terminal UI framework for Go. Floating
windows, draggable / resizable / focusable, with menus, scrollbars,
and a CGA-style color palette. Built on top of
[`tcell`](https://github.com/gdamore/tcell).

The aesthetic target is the late-90s Borland / FoxPro DOS look:
single-line borders, blue/cyan/yellow chrome, drop shadows, mouse
support. Each window owns a `ContentProvider`; the framework handles
the chrome, hit-testing, focus, scrollbars, and event routing.

## Usage

This module is consumed by
[`6502-sim-tui`](https://github.com/carledwards/6502-sim-tui), which
is the primary working example. See `examples/` for smaller demos.

```go
import foxpro "github.com/carledwards/foxpro-go"

app := foxpro.NewApp()
app.Manager.AddWindow(&foxpro.Window{
    Title:   "Hello",
    Bounds:  foxpro.Rect{X: 5, Y: 3, W: 30, H: 10},
    Content: myProvider,
})
app.Run()
```

## Features

- Floating, movable, resizable, zoomable windows with focus tracking
- Title bars with close (■) and zoom (▲) glyphs
- Menu bar with hotkeys, separators, and a live "tray" area
- Scrollbars (vertical + horizontal) drawn in the FoxPro half-box
  style; `BoxedProvider` wraps any `ContentProvider` with chrome
- Mouse support: click, drag, wheel, drag-tracking via
  `MouseDragHandler`, wheel routing via `WheelHandler`
- 16-slot palette system — swap `ClassicPalette()` for
  `DraculaPalette()`, `RetroGreenPalette()`, etc., and the whole UI
  re-skins
- Pane splits (`PaneProvider`) for nested layouts
- `Canvas` helper for content with auto-clipping + scroll translation

## License

MIT — see [LICENSE](LICENSE).
