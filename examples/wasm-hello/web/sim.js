// sim.js — JS bridge for foxpro-go in the browser.
//
// Boots the wasm module, sizes a canvas to match the SimulationScreen
// grid, paints cells from snapshots taken each animation frame, and
// forwards keyboard/mouse to foxpro via the exported wasm functions.

(() => {
  // Cell metrics. Matched to a sensible monospace cap-height; the
  // canvas dimensions and font are derived from these.
  const FONT_PX = 16;
  const CELL_H = 18;
  const CELL_W = 9; // overwritten by measureText after font loads
  const DEFAULT_FG = '#cccccc';
  const DEFAULT_BG = '#0000aa';
  const DEFAULT_COLOR_SENTINEL = 0xff000000;

  const canvas = document.getElementById('screen');
  const ctx = canvas.getContext('2d', { alpha: false });
  const statusEl = document.getElementById('status');

  let cellW = CELL_W;
  let buf = null;     // Uint8Array snapshot scratch
  let view = null;    // DataView over buf
  let booted = false;

  // Tcell wires Backspace as KeyBackspace2 (0x7f) on real terminals.
  // Browsers emit 'Backspace' for the same key — we route it accordingly.

  window.onFoxproReady = () => {
    if (booted) return;
    booted = true;
    boot();
  };

  async function start() {
    if (typeof Go === 'undefined') {
      statusEl.textContent = 'wasm_exec.js failed to load';
      return;
    }
    const go = new Go();
    statusEl.textContent = 'fetching sim.wasm…';
    try {
      const r = await WebAssembly.instantiateStreaming(fetch('sim.wasm'), go.importObject);
      statusEl.textContent = 'starting Go runtime…';
      go.run(r.instance); // does not resolve while app.Run() is alive
    } catch (e) {
      statusEl.textContent = 'wasm load error: ' + e.message;
      console.error(e);
    }
  }

  function boot() {
    const fw = window.foxproWasm;
    if (!fw) {
      statusEl.textContent = 'foxproWasm bridge missing';
      return;
    }

    // Set the font, measure a cell width, then size the canvas.
    ctx.font = `${FONT_PX}px ui-monospace, "SF Mono", Menlo, Consolas, monospace`;
    ctx.textBaseline = 'top';
    cellW = Math.ceil(ctx.measureText('M').width);
    if (cellW < 7) cellW = 7;

    const [w, h] = fw.size();
    canvas.width = w * cellW;
    canvas.height = h * CELL_H;
    // Don't pin canvas.style.width/height — CSS max-width/max-height
    // in index.html lets the browser scale the canvas down to fit the
    // viewport while preserving aspect ratio.

    // Re-set font after canvas resize (resizing clears state).
    ctx.font = `${FONT_PX}px ui-monospace, "SF Mono", Menlo, Consolas, monospace`;
    ctx.textBaseline = 'top';

    buf = new Uint8Array(w * h * 16);
    view = new DataView(buf.buffer);

    canvas.focus();
    setupInput();
    requestAnimationFrame(frame);
    statusEl.textContent = `${w}×${h} cells · click to focus · F10 = menu · F2 = command window`;
  }

  let stopped = false;

  function frame() {
    if (stopped) return;
    requestAnimationFrame(frame);
    const fw = window.foxproWasm;
    if (!fw || !buf) return;

    let sz;
    try {
      sz = fw.snapshot(buf);
    } catch (err) {
      // Most common cause: foxpro called app.Quit, the Go runtime
      // exited, and every subsequent JS→wasm call now throws
      // "Go program has already exited". Stop the render loop and
      // tell the user, instead of spamming the console at 60 Hz.
      stopped = true;
      statusEl.textContent = 'foxpro exited — refresh to restart (' + err.message + ')';
      console.warn('foxpro snapshot failed:', err);
      return;
    }
    const w = sz[0], h = sz[1];

    // Naive full repaint each frame. Good enough at 80×30 / 60 fps.
    // Optimization later: track dirty rectangles in Go and skip clean cells.
    for (let y = 0; y < h; y++) {
      let rowOff = y * w * 16;
      for (let x = 0; x < w; x++) {
        const ch = view.getUint32(rowOff, true);
        const fg = view.getUint32(rowOff + 4, true);
        const bg = view.getUint32(rowOff + 8, true);
        // const attr = view.getUint32(rowOff + 12, true);  // unused for now
        rowOff += 16;
        paintCell(x, y, ch, fg, bg);
      }
    }
  }

  function paintCell(x, y, ch, fg, bg) {
    const px = x * cellW;
    const py = y * CELL_H;
    ctx.fillStyle = colorToCSS(bg, DEFAULT_BG);
    ctx.fillRect(px, py, cellW, CELL_H);
    if (ch !== 0 && ch !== 32) {
      ctx.fillStyle = colorToCSS(fg, DEFAULT_FG);
      ctx.fillText(String.fromCodePoint(ch), px, py + 1);
    }
  }

  function colorToCSS(c, fallback) {
    // Only the explicit sentinel means "unset" — RGB 0,0,0 is real black
    // (used by the drop shadow's Black background).
    if (c === DEFAULT_COLOR_SENTINEL) return fallback;
    const r = (c >> 16) & 0xff;
    const g = (c >> 8) & 0xff;
    const b = c & 0xff;
    return `rgb(${r},${g},${b})`;
  }

  function setupInput() {
    const fw = window.foxproWasm;
    const KEYS = fw.keys;
    const MODS = fw.mods;
    const BTN = fw.buttons;

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
      // Let real browser shortcuts (Cmd+R reload, Cmd+T new tab,
      // Cmd+L address bar, …) pass through. Anything with the
      // platform meta key, or Ctrl chords with letters that match
      // common browser bindings, is left alone.
      if (e.metaKey) return;

      const mods =
        (e.shiftKey ? MODS.Shift : 0) |
        (e.ctrlKey ? MODS.Ctrl : 0) |
        (e.altKey ? MODS.Alt : 0) |
        (e.metaKey ? MODS.Meta : 0);

      // Shift+Tab → Backtab (foxpro maps it to FocusPrev when bound).
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

      // Printable char. preventDefault always — otherwise the browser
      // treats the canvas as an unhandled keystroke target and rings
      // the system bell on every press.
      //
      // Alt-chord layout fix (macOS especially): Option+F produces "ƒ",
      // not "f", so e.key is the wrong codepoint for foxpro accelerators.
      // When Alt is held, derive the letter from e.code (physical key).
      if (e.altKey && /^Key[A-Z]$/.test(e.code)) {
        e.preventDefault();
        const letter = e.code.charCodeAt(3) + 32; // 'A'..'Z' → 'a'..'z'
        fw.injectKey(KEYS.Rune, letter, mods);
        return;
      }
      if (e.key.length === 1) {
        e.preventDefault();
        fw.injectKey(KEYS.Rune, e.key.codePointAt(0), mods);
        return;
      }
      // Modifier-only / dead keys: drop.
    });

    function pixelToCell(e) {
      const r = canvas.getBoundingClientRect();
      const px = (e.clientX - r.left) * (canvas.width / r.width);
      const py = (e.clientY - r.top) * (canvas.height / r.height);
      return [Math.floor(px / cellW), Math.floor(py / CELL_H)];
    }

    function buttonsMaskFromEvent(e) {
      // e.buttons: bit 0 = primary, bit 1 = secondary, bit 2 = middle.
      let m = 0;
      if (e.buttons & 1) m |= BTN.Primary;
      if (e.buttons & 2) m |= BTN.Secondary;
      if (e.buttons & 4) m |= BTN.Middle;
      return m;
    }

    function modMaskFromEvent(e) {
      // Shift+click is meaningful to foxpro (e.g. background-drag a
      // window without raising z-order), so the mod mask must reach
      // the Go side via injectMouse — earlier versions hard-coded 0.
      let m = 0;
      if (e.shiftKey) m |= MODS.Shift;
      if (e.ctrlKey)  m |= MODS.Ctrl;
      if (e.altKey)   m |= MODS.Alt;
      if (e.metaKey)  m |= MODS.Meta;
      return m;
    }

    canvas.addEventListener('mousedown', (e) => {
      e.preventDefault();
      canvas.focus();
      const [cx, cy] = pixelToCell(e);
      // After mousedown the button is held; e.buttons reflects that.
      fw.injectMouse(cx, cy, buttonsMaskFromEvent(e), modMaskFromEvent(e));
    });
    canvas.addEventListener('mousemove', (e) => {
      const [cx, cy] = pixelToCell(e);
      fw.injectMouse(cx, cy, buttonsMaskFromEvent(e), modMaskFromEvent(e));
    });
    canvas.addEventListener('mouseup', (e) => {
      e.preventDefault();
      const [cx, cy] = pixelToCell(e);
      // After mouseup, e.buttons reflects the *remaining* held buttons.
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

  start();
})();
