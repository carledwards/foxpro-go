// sim.js — boots the wasm-hello demo and hands the canvas off to
// FoxproRender (shared renderer shipped by foxpro-go/wasm). All cell
// painting, pixel-layer compositing, drop-shadow tinting, and
// keyboard/mouse/touch plumbing lives in foxpro.js.
(() => {
  const canvas = document.getElementById('screen');
  const statusEl = document.getElementById('status');

  // cellH: 18 preserves the demo's historical canvas dimensions
  // (36 rows × 18 = 648 px tall). Drop the override to let foxpro.js
  // use its descender-safe default of fontPx + 6.
  //
  // autoFocus: true so the keyboard works immediately on load — the
  // canvas sits at the top of this page, so a focus jump won't scroll
  // anything important out of view (foxpro.js uses preventScroll
  // either way).
  FoxproRender.attach(canvas, { statusEl, cellH: 18, autoFocus: true });
  FoxproRender.bootWasm('sim.wasm', statusEl);
})();
