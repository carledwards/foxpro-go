// foxpro.js — shared canvas renderer + input bridge for foxpro-go
// wasm hosts.
//
// Single source of truth so apps don't each re-implement cell
// painting, pixel-layer compositing, tint masking, box-drawing
// fallback, or key/mouse plumbing. Consumers include this script
// alongside wasm_exec.js and call FoxproRender.attach(canvas).
//
// Typical usage:
//
//   <canvas id="screen" tabindex="0"></canvas>
//   <div id="status">loading…</div>
//   <script src="wasm_exec.js"></script>
//   <script src="foxpro.js"></script>
//   <script>
//     const canvas = document.getElementById('screen');
//     const statusEl = document.getElementById('status');
//     window.onFoxproReady = () => FoxproRender.attach(canvas, { statusEl });
//     FoxproRender.bootWasm('sim.wasm', statusEl);
//   </script>
//
// The renderer owns the canvas (sets its width/height/font), the
// snapshot buffer, the per-frame loop, and pixel-layer compositing.
// It does NOT own the wasm module — bootWasm() is a convenience for
// the standard "fetch + Go.run()" path; apps that load the runtime
// differently can just call attach() once foxproWasm is in place.
(function () {
  'use strict';

  const PIXEL_SENTINEL = 0xE000;
  const DEFAULT_COLOR_SENTINEL = 0xff000000;

  const DEFAULT_OPTS = {
    fontPx: 16,
    // cellH defaults to fontPx + 6 so glyphs with descenders (g/p/y/j)
    // don't get clipped at the bottom edge.
    cellH: undefined,
    fontFamily: 'ui-monospace, "SF Mono", Menlo, Consolas, monospace',
    defaultFG: '#cccccc',
    defaultBG: '#0000aa',
    // Color used to tint pixel-canvas regions when foxpro's drop
    // shadow lands on a sentinel cell. See the PixelContent /
    // pixelTintColors mechanism in wasm/bridge.go.
    tintOverlay: 'rgba(0, 0, 0, 0.7)',
    // Status element gets text updates on init / boot / errors.
    statusEl: null,
    // Hook for fatal errors (e.g. snapshot threw — the wasm exited).
    onError: null,
    // When true, forwards Cmd-modified keys to the host browser
    // (Cmd+R reloads the page, Cmd+L focuses URL bar, etc.) instead
    // of capturing them. Default true since most users expect this.
    passThroughMeta: true,
  };

  // ─── public API ───────────────────────────────────────────────

  // attach wires `canvas` to the foxpro wasm bridge: sizes the
  // canvas to the simulation grid, starts the per-frame render
  // loop, and forwards keyboard/mouse events to the bridge.
  // Returns a controller {stop, isStopped}.
  function attach(canvas, userOpts) {
    const opts = Object.assign({}, DEFAULT_OPTS, userOpts || {});
    if (opts.cellH == null) opts.cellH = opts.fontPx + 6;
    const ctx = canvas.getContext('2d', { alpha: false });

    const state = {
      canvas, ctx, opts,
      cellW: 9, cellH: opts.cellH, fontPx: opts.fontPx,
      buf: null, view: null,
      pixelLayerCanvases: new Map(),
      pixelBuf: null,
      tintColorSet: null,
      stopped: false,
      booted: false,
    };

    function ensureBooted() {
      if (state.booted) return true;
      const fw = window.foxproWasm;
      if (!fw) return false;
      ctx.font = `${state.fontPx}px ${opts.fontFamily}`;
      ctx.textBaseline = 'top';
      state.cellW = Math.max(7, Math.ceil(ctx.measureText('M').width));

      const [w, h] = fw.size();
      canvas.width = w * state.cellW;
      canvas.height = h * state.cellH;
      // Re-set after dim change — context state clears.
      ctx.font = `${state.fontPx}px ${opts.fontFamily}`;
      ctx.textBaseline = 'top';

      state.buf = new Uint8Array(w * h * 16);
      state.view = new DataView(state.buf.buffer);

      setupInput(state, fw);
      state.booted = true;
      if (opts.statusEl) {
        opts.statusEl.textContent = `${w}×${h} cells · click to focus`;
      }
      requestAnimationFrame(() => frame(state));
      return true;
    }

    // If the bridge is already in place, boot immediately. Otherwise
    // chain onto onFoxproReady (preserving any existing handler).
    if (!ensureBooted()) {
      const prev = window.onFoxproReady;
      window.onFoxproReady = function () {
        if (typeof prev === 'function') {
          try { prev(); } catch (e) { console.warn('[foxpro] prev onFoxproReady threw', e); }
        }
        ensureBooted();
      };
    }

    return {
      stop() { state.stopped = true; },
      isStopped() { return state.stopped; },
    };
  }

  // bootWasm fetches and instantiates the wasm module, running it
  // through the Go runtime (wasm_exec.js must already be loaded).
  // Returns a Promise that resolves when go.run is called; the
  // bridge becomes available shortly after, signaled by foxpro
  // calling window.onFoxproReady.
  async function bootWasm(wasmURL, statusEl) {
    if (typeof Go === 'undefined') {
      const msg = 'wasm_exec.js failed to load';
      if (statusEl) statusEl.textContent = msg;
      console.error('[foxpro] ' + msg);
      return;
    }
    const go = new Go();
    if (statusEl) statusEl.textContent = `fetching ${wasmURL}…`;
    try {
      const r = await WebAssembly.instantiateStreaming(fetch(wasmURL), go.importObject);
      if (statusEl) statusEl.textContent = 'starting Go runtime…';
      go.run(r.instance);
    } catch (e) {
      const msg = 'wasm load error: ' + e.message;
      if (statusEl) statusEl.textContent = msg;
      console.error('[foxpro]', e);
    }
  }

  // ─── render loop ──────────────────────────────────────────────

  function frame(state) {
    if (state.stopped) return;
    requestAnimationFrame(() => frame(state));
    const fw = window.foxproWasm;
    if (!fw || !state.buf) return;

    let sz;
    try {
      sz = fw.snapshot(state.buf);
    } catch (err) {
      state.stopped = true;
      if (state.opts.statusEl) {
        state.opts.statusEl.textContent = 'simulator exited — refresh to restart';
      }
      if (state.opts.onError) state.opts.onError(err);
      console.warn('[foxpro] frame error', err);
      return;
    }
    const w = sz[0], h = sz[1];

    // Cell pass: paint every cell. Sentinel cells get only their
    // bg fillRect — no glyph — so the pixel-layer pass has a known
    // canvas state to drawImage onto.
    for (let y = 0; y < h; y++) {
      let off = y * w * 16;
      for (let x = 0; x < w; x++) {
        const ch = state.view.getUint32(off, true);
        const fg = state.view.getUint32(off + 4, true);
        const bg = state.view.getUint32(off + 8, true);
        off += 16;
        paintCell(state, x, y, ch, fg, bg);
      }
    }

    // Refresh tint colors once per frame.
    if (typeof fw.pixelTintColors === 'function') {
      state.tintColorSet = new Set(fw.pixelTintColors() || []);
    } else {
      state.tintColorSet = null;
    }

    substitutePixelLayers(state, fw, w, h);
  }

  function substitutePixelLayers(state, fw, w, h) {
    if (typeof fw.pixelLayers !== 'function') return;
    const layers = fw.pixelLayers();
    if (!layers || layers.length === 0) {
      state.pixelLayerCanvases.clear();
      return;
    }
    const seen = new Set();
    for (const L of layers) {
      seen.add(L.id);
      let entry = state.pixelLayerCanvases.get(L.id);
      if (!entry) {
        const c = document.createElement('canvas');
        entry = { canvas: c, ctx: c.getContext('2d'), pxW: 0, pxH: 0 };
        state.pixelLayerCanvases.set(L.id, entry);
      }
      if (entry.pxW !== L.pxW || entry.pxH !== L.pxH) {
        entry.canvas.width = L.pxW;
        entry.canvas.height = L.pxH;
        entry.pxW = L.pxW;
        entry.pxH = L.pxH;
      }
      const need = L.pxW * L.pxH * 4;
      if (!state.pixelBuf || state.pixelBuf.length < need) {
        state.pixelBuf = new Uint8Array(Math.max(need, 256 * 1024));
      }
      if (!fw.pixelLayerData(L.id, state.pixelBuf)) continue;
      const clamped = new Uint8ClampedArray(state.pixelBuf.buffer, state.pixelBuf.byteOffset, need);
      entry.ctx.putImageData(new ImageData(clamped, L.pxW, L.pxH), 0, 0);

      const subW = L.pxW / L.cellW;
      const subH = L.pxH / L.cellH;
      for (let cy = 0; cy < L.cellH; cy++) {
        const screenY = L.cellY + cy;
        if (screenY < 0 || screenY >= h) continue;
        for (let cx = 0; cx < L.cellW; cx++) {
          const screenX = L.cellX + cx;
          if (screenX < 0 || screenX >= w) continue;
          const off = (screenY * w + screenX) * 16;
          const ch = state.view.getUint32(off, true);
          if (ch !== PIXEL_SENTINEL) continue;
          const bg = state.view.getUint32(off + 8, true);
          const px = screenX * state.cellW;
          const py = screenY * state.cellH;
          state.ctx.drawImage(
            entry.canvas,
            cx * subW, cy * subH, subW, subH,
            px, py, state.cellW, state.cellH,
          );
          if (state.tintColorSet && state.tintColorSet.has(bg)) {
            state.ctx.fillStyle = state.opts.tintOverlay;
            state.ctx.fillRect(px, py, state.cellW, state.cellH);
          }
        }
      }
    }
    for (const id of state.pixelLayerCanvases.keys()) {
      if (!seen.has(id)) state.pixelLayerCanvases.delete(id);
    }
  }

  function paintCell(state, x, y, ch, fg, bg) {
    const ctx = state.ctx;
    const px = x * state.cellW;
    const py = y * state.cellH;
    ctx.fillStyle = colorToCSS(bg, state.opts.defaultBG);
    ctx.fillRect(px, py, state.cellW, state.cellH);
    if (ch === 0 || ch === 32 || ch === PIXEL_SENTINEL) return;
    const fgCss = colorToCSS(fg, state.opts.defaultFG);
    if (ch >= 0x2500 && ch <= 0x258F) {
      if (drawBoxOrBlock(state, px, py, ch, fgCss)) return;
    }
    ctx.fillStyle = fgCss;
    // Vertically center the glyph in the cell. fontPx is the
    // declared font size; cellH-fontPx pixels of slack split
    // top/bottom keeps glyphs visually balanced for fonts whose
    // drawn height is roughly fontPx.
    const yOff = Math.max(0, Math.floor((state.cellH - state.fontPx) / 2));
    ctx.fillText(String.fromCodePoint(ch), px, py + yOff);
  }

  // Box-drawing (U+2500–U+257F) + block (U+2580–U+258F) glyphs
  // rendered as fillRect primitives so they connect cell-to-cell
  // (font glyphs leave visible gaps because their natural dims
  // are smaller than cellW × cellH).
  function drawBoxOrBlock(state, px, py, ch, fgCss) {
    const ctx = state.ctx;
    const cellW = state.cellW, cellH = state.cellH;
    ctx.fillStyle = fgCss;
    switch (ch) {
      case 0x2580: ctx.fillRect(px, py, cellW, Math.floor(cellH / 2)); return true;
      case 0x2584: ctx.fillRect(px, py + Math.floor(cellH / 2), cellW, cellH - Math.floor(cellH / 2)); return true;
      case 0x2588: ctx.fillRect(px, py, cellW, cellH); return true;
      case 0x258C: ctx.fillRect(px, py, Math.floor(cellW / 2), cellH); return true;
      case 0x2590: ctx.fillRect(px + Math.floor(cellW / 2), py, cellW - Math.floor(cellW / 2), cellH); return true;
    }
    // Single-line glyphs handled by the generic L/R/U/D logic — no
    // corner asymmetry to worry about.
    let L = 0, R = 0, U = 0, D = 0;
    switch (ch) {
      case 0x2500: L = R = 1; break; // ─
      case 0x2502: U = D = 1; break; // │
      case 0x250C: R = D = 1; break; // ┌
      case 0x2510: L = D = 1; break; // ┐
      case 0x2514: R = U = 1; break; // └
      case 0x2518: L = U = 1; break; // ┘
      case 0x251C: U = D = R = 1; break; // ├
      case 0x2524: U = D = L = 1; break; // ┤
      case 0x252C: L = R = D = 1; break; // ┬
      case 0x2534: L = R = U = 1; break; // ┴
      case 0x253C: L = R = U = D = 1; break; // ┼
    }
    const cx = px + Math.floor(cellW / 2);
    const cy = py + Math.floor(cellH / 2);
    const right = px + cellW;
    const bot = py + cellH;
    const lw = 1;
    if (L || R || U || D) {
      if (L) ctx.fillRect(px, cy, cx - px + lw, lw);
      if (R) ctx.fillRect(cx, cy, right - cx, lw);
      if (U) ctx.fillRect(cx, py, lw, cy - py + lw);
      if (D) ctx.fillRect(cx, cy, lw, bot - cy);
      return true;
    }
    // Double-line and mixed-line glyphs need explicit per-glyph
    // geometry — the corner asymmetry (outer L extends one pixel
    // past the inner L) doesn't fit a uniform direction abstraction.
    // Each case lists the rectangles that compose the glyph; the
    // box / mixed-line corners line up cleanly when adjacent cells
    // use the matching neighbour glyph.
    switch (ch) {
      case 0x2550: // ═
        ctx.fillRect(px, cy - 1, cellW, lw);
        ctx.fillRect(px, cy + 1, cellW, lw);
        return true;
      case 0x2551: // ║
        ctx.fillRect(cx - 1, py, lw, cellH);
        ctx.fillRect(cx + 1, py, lw, cellH);
        return true;
      case 0x2554: // ╔ outer corner (cx-1, cy-1), inner (cx+1, cy+1)
        ctx.fillRect(cx - 1, cy - 1, right - (cx - 1), lw);
        ctx.fillRect(cx - 1, cy - 1, lw, bot - (cy - 1));
        ctx.fillRect(cx + 1, cy + 1, right - (cx + 1), lw);
        ctx.fillRect(cx + 1, cy + 1, lw, bot - (cy + 1));
        return true;
      case 0x2557: // ╗ outer (cx+1, cy-1), inner (cx-1, cy+1)
        ctx.fillRect(px, cy - 1, (cx + 1) - px + lw, lw);
        ctx.fillRect(cx + 1, cy - 1, lw, bot - (cy - 1));
        ctx.fillRect(px, cy + 1, (cx - 1) - px + lw, lw);
        ctx.fillRect(cx - 1, cy + 1, lw, bot - (cy + 1));
        return true;
      case 0x255A: // ╚ outer (cx-1, cy+1), inner (cx+1, cy-1)
        ctx.fillRect(cx - 1, py, lw, (cy + 1) - py + lw);
        ctx.fillRect(cx - 1, cy + 1, right - (cx - 1), lw);
        ctx.fillRect(cx + 1, py, lw, (cy - 1) - py + lw);
        ctx.fillRect(cx + 1, cy - 1, right - (cx + 1), lw);
        return true;
      case 0x255D: // ╝ outer (cx+1, cy+1), inner (cx-1, cy-1)
        ctx.fillRect(cx + 1, py, lw, (cy + 1) - py + lw);
        ctx.fillRect(px, cy + 1, (cx + 1) - px + lw, lw);
        ctx.fillRect(cx - 1, py, lw, (cy - 1) - py + lw);
        ctx.fillRect(px, cy - 1, (cx - 1) - px + lw, lw);
        return true;
      case 0x2556: // ╖ single L, double D — single horizontal meets
                   // outer (right) line of the double-vertical.
        ctx.fillRect(px, cy, (cx + 1) - px + lw, lw);
        ctx.fillRect(cx + 1, cy, lw, bot - cy);
        ctx.fillRect(cx - 1, cy + 1, lw, bot - (cy + 1));
        return true;
      case 0x2558: // ╘ single U, double R — single vertical meets
                   // outer (bottom) line of the double-horizontal.
        ctx.fillRect(cx, py, lw, (cy + 1) - py + lw);
        ctx.fillRect(cx, cy + 1, right - cx, lw);
        ctx.fillRect(cx + 1, cy - 1, right - (cx + 1), lw);
        return true;
    }
    return false;
  }

  function colorToCSS(c, fallback, alpha) {
    if (c === DEFAULT_COLOR_SENTINEL) return fallback;
    const r = (c >> 16) & 0xff;
    const g = (c >> 8) & 0xff;
    const b = c & 0xff;
    if (alpha != null) return `rgba(${r},${g},${b},${alpha})`;
    return `rgb(${r},${g},${b})`;
  }

  // ─── input ────────────────────────────────────────────────────

  function setupInput(state, fw) {
    const canvas = state.canvas;
    const KEYS = fw.keys, MODS = fw.mods, BTN = fw.buttons;

    const specialKeys = {
      Enter: KEYS.Enter,
      Tab: KEYS.Tab,
      Escape: KEYS.Esc,
      Backspace: KEYS.Backspace2,
      ArrowUp: KEYS.Up,
      ArrowDown: KEYS.Down,
      ArrowLeft: KEYS.Left,
      ArrowRight: KEYS.Right,
      Home: KEYS.Home,
      End: KEYS.End,
      PageUp: KEYS.PgUp,
      PageDown: KEYS.PgDn,
      Insert: KEYS.Insert,
      Delete: KEYS.Delete,
    };
    for (let i = 1; i <= 12; i++) specialKeys['F' + i] = KEYS['F' + i];

    canvas.addEventListener('keydown', (e) => {
      if (state.opts.passThroughMeta && e.metaKey) return;
      const mods =
        (e.shiftKey ? MODS.Shift : 0) |
        (e.ctrlKey ? MODS.Ctrl : 0) |
        (e.altKey ? MODS.Alt : 0) |
        (e.metaKey ? MODS.Meta : 0);
      if (e.key === 'Tab' && e.shiftKey) {
        e.preventDefault();
        fw.injectKey(KEYS.Backtab, 0, mods);
        return;
      }
      const sk = specialKeys[e.key];
      if (sk !== undefined) {
        e.preventDefault();
        fw.injectKey(sk, 0, mods);
        return;
      }
      if (e.altKey && /^Key[A-Z]$/.test(e.code)) {
        e.preventDefault();
        fw.injectKey(KEYS.Rune, e.code.charCodeAt(3) + 32, mods);
        return;
      }
      if (e.key.length === 1) {
        e.preventDefault();
        fw.injectKey(KEYS.Rune, e.key.codePointAt(0), mods);
      }
    });

    function pixelToCell(e) {
      const r = canvas.getBoundingClientRect();
      const px = (e.clientX - r.left) * (canvas.width / r.width);
      const py = (e.clientY - r.top) * (canvas.height / r.height);
      return [Math.floor(px / state.cellW), Math.floor(py / state.cellH)];
    }
    function buttonsMaskFromEvent(e) {
      let m = 0;
      if (e.buttons & 1) m |= BTN.Primary;
      if (e.buttons & 2) m |= BTN.Secondary;
      if (e.buttons & 4) m |= BTN.Middle;
      return m;
    }
    function modMaskFromEvent(e) {
      let m = 0;
      if (e.shiftKey) m |= MODS.Shift;
      if (e.ctrlKey) m |= MODS.Ctrl;
      if (e.altKey) m |= MODS.Alt;
      if (e.metaKey) m |= MODS.Meta;
      return m;
    }

    canvas.addEventListener('mousedown', (e) => {
      e.preventDefault();
      canvas.focus();
      const [cx, cy] = pixelToCell(e);
      fw.injectMouse(cx, cy, buttonsMaskFromEvent(e), modMaskFromEvent(e));
    });
    canvas.addEventListener('mousemove', (e) => {
      const [cx, cy] = pixelToCell(e);
      fw.injectMouse(cx, cy, buttonsMaskFromEvent(e), modMaskFromEvent(e));
    });
    canvas.addEventListener('mouseup', (e) => {
      e.preventDefault();
      const [cx, cy] = pixelToCell(e);
      fw.injectMouse(cx, cy, buttonsMaskFromEvent(e), modMaskFromEvent(e));
    });
    canvas.addEventListener('contextmenu', (e) => e.preventDefault());
    canvas.addEventListener('wheel', (e) => {
      e.preventDefault();
      const [cx, cy] = pixelToCell(e);
      let btn = 0;
      if (e.deltaY < 0) btn = BTN.WheelUp;
      else if (e.deltaY > 0) btn = BTN.WheelDown;
      else if (e.deltaX < 0) btn = BTN.WheelLeft;
      else if (e.deltaX > 0) btn = BTN.WheelRight;
      if (btn) fw.injectMouse(cx, cy, btn, modMaskFromEvent(e));
    }, { passive: false });
  }

  // Export to window — namespaced under FoxproRender so the global
  // surface stays small and predictable.
  window.FoxproRender = {
    attach,
    bootWasm,
    PIXEL_SENTINEL,
    DEFAULT_COLOR_SENTINEL,
  };
})();
