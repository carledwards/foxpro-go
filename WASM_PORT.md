# Running foxpro-go in the browser

foxpro-go targets a `tcell.Screen`, not a terminal. That gives us a
clean swap: in the browser we use `tcell.SimulationScreen` (a pure-Go
cell buffer with no syscalls) and render its contents to an HTML
canvas. Every widget, menu, drag handler, and key binding works
unchanged — only the rendering and event sources change.

## Architecture

```
                ┌─────────────────────────────────────┐
                │   foxpro App + Manager + Widgets    │   ← unchanged
                └────────────────┬────────────────────┘
                                 │  tcell.Screen interface
                ┌────────────────┴────────────────────┐
                │   tcell.SimulationScreen (cells)    │
                └─────┬──────────────────────────┬────┘
                      │                          │
                      │ snapshot/inject events   │
                      │ (foxpro-go/wasm bridge)  │
                      │                          │
                ┌─────┴─────────┐         ┌──────┴──────┐
                │ JS render loop│         │ JS input    │
                │ (canvas)      │         │ (keys/mouse)│
                └───────────────┘         └─────────────┘
```

The Go `app.Run()` loop runs in the wasm module and blocks on
`Screen.PollEvent()`. JS injects keyboard and mouse events through the
bridge; foxpro processes them and updates the cell buffer; JS reads
the buffer once per animation frame and paints it to a canvas.

## File layout

```
foxpro-go/
├── wasm/bridge.go              ← reusable JS bridge (build-tagged js && wasm)
├── examples/
│   ├── hello/                  ← terminal example
│   └── wasm-hello/
│       ├── main.go             ← demo-specific app code
│       ├── Makefile            ← build + serve
│       └── web/                ← static deploy target
│           ├── index.html
│           ├── sim.js
│           ├── wasm_exec.js    ← copied from $GOROOT/lib/wasm/
│           └── sim.wasm        ← built artifact
└── WASM_PORT.md                ← this document
```

## Building a new wasm demo

Mirror `examples/wasm-hello`. The Go side is short:

```go
//go:build js && wasm
package main

import (
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

    // Disable the built-in quit chord. In a browser, app.Quit
    // terminates the Go runtime and locks the page; users close the
    // tab instead. Without this, Esc and Ctrl+Q would brick the page.
    app.Settings.QuitKeys = nil

    setupWindows(app) // your demo content
    wasm.Run(app, s)  // installs the bridge, then app.Run
}
```

### Host-controlled built-ins

Foxpro's `Settings` struct holds the knobs that differ between hosts.
Today the only host-sensitive one is `QuitKeys`, but the pattern is
the same for any future addition:

| Setting | Terminal default | Browser recommendation |
| --- | --- | --- |
| `QuitKeys` | `[Esc, Ctrl+Q]` | `nil` (browser tab is the close button) |
| `BackgroundDragChords` | `[right-click, Shift+left-click]` | `[Shift+left-click]` (right-click = native context menu) |
| `ShowShadows` | `true` | `true` |
| `ShowStatusBar` | `true` | `true` |

Set these immediately after `NewAppWithScreen`, before building windows
or calling `wasm.Run`.

The JS frontend (`sim.js`) is renderer-agnostic — it only knows about
the bridge's API. You can copy it verbatim and adjust styling to taste.

## DOM ↔ foxpro interop

For browser-native UI (HTML buttons, inputs, links) that drives foxpro
state, register your own `js.Func` and wrap mutations in `app.Post` so
they run on foxpro's UI goroutine instead of the JS callback's
goroutine:

```go
js.Global().Set("simRun", js.FuncOf(func(this js.Value, args []js.Value) any {
    app.Post(func() {
        clockProv.SetRunning(true) // safe: runs on UI goroutine
    })
    return nil
}))
```

The reverse direction (Go → JS notifications) is just `js.Global().Call(...)`
or setting a property. No bridge support needed.

Rule of thumb: **canvas is for the foxpro UI, DOM is for everything
else**. Native DOM controls are better for clicks, focus,
accessibility, and styling.

## Bridge JS API

After `wasm.Install`, `window.foxproWasm` exposes:

| Function | Returns | Notes |
| --- | --- | --- |
| `size()` | `[w, h]` | Current grid dimensions |
| `snapshot(uint8Buf)` | `[w, h]` | Fills `uint8Buf` with packed cells |
| `injectKey(key, ch, mod)` | `bool` | Use `keys.*` constants for special keys; `keys.Rune` + codepoint for printables |
| `injectMouse(x, y, btn, mod?)` | `bool` | Cell coords; use `buttons.*` constants |
| `resize(w, h)` | `bool` | Resize the cell grid; foxpro re-flows on next draw |
| `quit()` | — | Asks the foxpro event loop to exit |
| `keys` | object | tcell key constants (`Enter`, `F1`–`F12`, `Up`, `Esc`, …) |
| `mods` | object | `Shift`, `Ctrl`, `Alt`, `Meta` modifier bits |
| `buttons` | object | `Primary`, `Secondary`, `WheelUp`, … |
| `defaultColor` | int | Sentinel returned by `snapshot` for unset colors |

### Cell packing

`snapshot` returns 16 bytes per cell, row-major. Use a `DataView` over
your `Uint8Array`:

| Offset | Bytes | Meaning |
| --- | --- | --- |
| 0 | 4 (uint32 LE) | Rune (BMP only) |
| 4 | 4 (uint32 LE) | Foreground color, `0xRRGGBB` (or `defaultColor`) |
| 8 | 4 (uint32 LE) | Background color, same encoding |
| 12 | 4 (uint32 LE) | `tcell.AttrMask` bits (bold/underline/reverse/dim/blink/italic) |

## Build & serve

From `examples/wasm-hello`:

```
make           # build wasm + serve on :8765
make build     # rebuild only
make serve     # serve only (existing artifacts)
make clean     # nuke artifacts
```

The Makefile auto-locates `wasm_exec.js` under
`$(go env GOROOT)/lib/wasm/` (Go 1.24+) or `…/misc/wasm/` (older), so
it survives toolchain bumps.

## Deployment

The `web/` directory is fully static: HTML, JS, wasm. Drop it on
anything that serves files.

**GitHub Pages.** Two patterns:
1. Commit the built `web/` to a `docs/` folder; Settings → Pages →
   Source: `main / docs`.
2. Use a GitHub Actions workflow that runs `make build` and pushes
   `web/` to a `gh-pages` branch.

GH Pages serves `.wasm` as `application/wasm` and applies gzip
automatically. A 3.3 MB sim.wasm compresses to ~1.0 MB on the wire.

**Other static hosts** (Netlify, Cloudflare Pages, S3+CloudFront,
plain nginx): same — no special configuration needed.

Cache-bust JS/wasm on releases by appending `?v=N` to the script tag
in `index.html` and bumping `N`. Otherwise users will see stale
modules.

## Known limitations

- **Single-threaded.** Foxpro and tcell are single-threaded, and wasm
  runs on one goroutine scheduler. CPU-heavy work blocks input until
  it yields. Use `app.Tick` to chunk.
- **No clipboard.** Browser clipboard requires `navigator.clipboard`
  and async permissions; not wired into the bridge yet.
- **Resize is manual.** The grid size is fixed at boot. Wiring a JS
  resize observer to `foxproWasm.resize(w, h)` is the path forward.
- **Bundle size.** ~3.3 MB raw / ~1.0 MB gzipped via stdlib Go.
  TinyGo can drop this to ~250 KB but loses `reflect`; not yet
  tested against tcell + foxpro.
- **Font fidelity.** The default JS renderer measures `M` to size
  cells, then draws each glyph with the system monospace font. For
  pixel-perfect FoxPro look-and-feel, swap in a CP437 font atlas
  (e.g. Px437 IBM VGA).

## Adding more demos

Drop a new directory under `examples/` and follow the wasm-hello
pattern. The bridge package handles every concern that isn't your
specific UI — windows, menus, content providers, and any DOM
interop you choose to wire alongside it.
