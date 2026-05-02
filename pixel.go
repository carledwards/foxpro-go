package foxpro

// PixelContent is a ContentProvider whose body area is rendered by
// the host as a pixel buffer instead of by foxpro's cell grid. The
// window's chrome — frame, title bar, scrollbars — still draws
// normally; the inner area is left at theme.WindowBG so the host's
// overlay (e.g. an HTML canvas in the wasm bridge) renders on a
// known background.
//
// The provider's Draw method should be a no-op (or paint a fallback
// pattern useful when the host can't render — e.g. terminal builds
// without graphics protocol support). The host queries pixel data
// each frame via PixelSize + DrawPixels.
//
// Hosts discover PixelContent providers by walking the window
// manager and type-asserting against this interface. The wasm bridge
// exposes the list to JS via foxproWasm.pixelLayers().
//
// Optional refinements via additional interfaces — see
// PixelRectContent below for sub-region overlays.
type PixelContent interface {
	ContentProvider

	// PixelLayerID is a stable identifier the host uses to match
	// Go-side providers with host-side surfaces (e.g. JS canvas
	// elements). Must be unique across simultaneously-visible
	// PixelContent windows.
	PixelLayerID() string

	// PixelSize returns the pixel buffer dimensions (width, height).
	// Called each frame so providers can resize on demand without a
	// separate signal. Returning (0, 0) is allowed and tells the host
	// to skip rendering this layer.
	PixelSize() (w, h int)

	// DrawPixels fills buf with RGBA bytes — 4 bytes per pixel,
	// row-major: [R,G,B,A,R,G,B,A,...]. buf has length 4*w*h, where
	// (w, h) is the most recent PixelSize result. Implementations
	// should not assume the buffer is zeroed; treat it as scratch.
	DrawPixels(buf []byte)
}

// PixelRectContent extends PixelContent with an explicit sub-rectangle
// within the window's body. Useful when only part of a content provider
// is pixel-based — for instance a VIC display whose framebuffer area
// is graphics but whose right-column buttons and bottom hex strip
// remain cell-rendered.
//
// Coords returned are relative to the window's inner rect, in cells.
// (x=0, y=0, w=inner.W, h=inner.H) means "fill the whole body" and is
// equivalent to plain PixelContent without this extension.
//
// Hosts query this via type assertion; absence falls back to the
// full-body overlay PixelContent describes.
type PixelRectContent interface {
	PixelContent

	// PixelRect returns the cell-coord rectangle (x, y, w, h)
	// relative to the window's inner area where the host should
	// render the pixel buffer.
	PixelRect() (x, y, w, h int)
}
