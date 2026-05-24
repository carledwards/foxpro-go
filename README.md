# foxpro-go

A FoxPro-for-DOS-style terminal UI framework for Go. Floating
windows, draggable / resizable / focusable, with menus, scrollbars,
and a CGA-style color palette. Built on top of
[`tcell`](https://github.com/gdamore/tcell).

The aesthetic target is the late-90s Borland / FoxPro DOS look:
single-line borders, blue/cyan/yellow chrome, drop shadows, mouse
support. Each window owns a `ContentProvider`; the framework handles
the chrome, hit-testing, focus, scrollbars, and event routing.

## Live demo

A WebAssembly build of the `examples/wasm-hello` demo runs in the
browser:

**[carledwards.github.io/foxpro-go](https://carledwards.github.io/foxpro-go/)**

Same widgets, menus, drag/resize, and scrollbars as the terminal
build — just rendered to a canvas instead of a TTY. See
[`WASM_PORT.md`](WASM_PORT.md) for how the port works and how to add
your own browser demo.

### Run it locally

**Terminal (TUI).** The same demo, rendered to your shell:

```
cd examples/hello
go run .
```

**Browser (WebAssembly).** Build the wasm bundle and serve it:

```
cd examples/wasm-hello
make            # build wasm + serve on http://localhost:8765
make build      # rebuild sim.wasm only
make serve      # serve existing web/ only
make clean      # remove build artifacts
```

Requires Go 1.21+. The wasm target additionally needs `python3` (for
the static file server); the Makefile auto-locates `wasm_exec.js`
under the active Go toolchain. To test on a phone, point the
device's browser at `http://<your-laptop-ip>:8765/` on the same
network.

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
