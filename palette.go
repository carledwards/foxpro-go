package foxpro

import "github.com/gdamore/tcell/v2"

// Palette is a 16-slot color set, one entry per IBM CGA color slot. Themes
// are built from a Palette so the same UI structure can be re-skinned just
// by swapping the palette in.
//
// Slot semantics follow CGA: low 8 are the "dark" half (Black..LightGray)
// and high 8 are the "intensity" half (DarkGray..White). The mouse-cursor
// inversion rule (XOR low-3-bits) pairs slots within each half:
// black↔lightgray, blue↔brown, green↔magenta, cyan↔red — and the
// equivalent pairs in the intense half.
type Palette struct {
	Black, Blue, Green, Cyan         tcell.Color
	Red, Magenta, Brown, LightGray   tcell.Color
	DarkGray, LightBlue, LightGreen  tcell.Color
	LightCyan, LightRed, LightMagenta tcell.Color
	Yellow, White                    tcell.Color
}

// ClassicPalette is the IBM CGA / FoxPro for DOS palette in true RGB.
// Independent of the host terminal's own color theme.
func ClassicPalette() Palette {
	return Palette{
		Black:        tcell.NewRGBColor(0, 0, 0),
		Blue:         tcell.NewRGBColor(0, 0, 170),
		Green:        tcell.NewRGBColor(0, 170, 0),
		Cyan:         tcell.NewRGBColor(0, 170, 170),
		Red:          tcell.NewRGBColor(170, 0, 0),
		Magenta:      tcell.NewRGBColor(170, 0, 170),
		Brown:        tcell.NewRGBColor(170, 85, 0),
		LightGray:    tcell.NewRGBColor(170, 170, 170),
		DarkGray:     tcell.NewRGBColor(85, 85, 85),
		LightBlue:    tcell.NewRGBColor(85, 85, 255),
		LightGreen:   tcell.NewRGBColor(85, 255, 85),
		LightCyan:    tcell.NewRGBColor(85, 255, 255),
		LightRed:     tcell.NewRGBColor(255, 85, 85),
		LightMagenta: tcell.NewRGBColor(255, 85, 255),
		Yellow:       tcell.NewRGBColor(255, 255, 85),
		White:        tcell.NewRGBColor(255, 255, 255),
	}
}

// DraculaPalette maps the Dracula color scheme onto the 16 CGA slots.
// Approximate fit — Dracula isn't a 16-slot palette to begin with — but
// preserves the feel: dark background, vibrant accents.
func DraculaPalette() Palette {
	return Palette{
		Black:        tcell.NewRGBColor(40, 42, 54),    // background
		Blue:         tcell.NewRGBColor(98, 114, 164),  // comment
		Green:        tcell.NewRGBColor(80, 250, 123),  // green
		Cyan:         tcell.NewRGBColor(139, 233, 253), // cyan
		Red:          tcell.NewRGBColor(255, 85, 85),   // red
		Magenta:      tcell.NewRGBColor(255, 121, 198), // pink
		Brown:        tcell.NewRGBColor(255, 184, 108), // orange
		LightGray:    tcell.NewRGBColor(68, 71, 90),    // current line
		DarkGray:     tcell.NewRGBColor(98, 114, 164),  // comment (reused)
		LightBlue:    tcell.NewRGBColor(189, 147, 249), // purple
		LightGreen:   tcell.NewRGBColor(80, 250, 123),
		LightCyan:    tcell.NewRGBColor(139, 233, 253),
		LightRed:     tcell.NewRGBColor(255, 85, 85),
		LightMagenta: tcell.NewRGBColor(255, 121, 198),
		Yellow:       tcell.NewRGBColor(241, 250, 140),
		White:        tcell.NewRGBColor(248, 248, 242),
	}
}

// MonochromePalette is a grey-on-black scheme — useful for screenshots,
// terminals without colour, and as a sanity check on theme contrast.
func MonochromePalette() Palette {
	g := func(v int32) tcell.Color { return tcell.NewRGBColor(v, v, v) }
	return Palette{
		Black: g(0), Blue: g(60), Green: g(100), Cyan: g(140),
		Red: g(60), Magenta: g(100), Brown: g(120), LightGray: g(180),
		DarkGray: g(80), LightBlue: g(140), LightGreen: g(180), LightCyan: g(220),
		LightRed: g(140), LightMagenta: g(180), Yellow: g(220), White: g(255),
	}
}

// RetroGreenPalette is a phosphor-CRT scheme — every slot is a shade of
// green on black, evoking a vintage monochrome terminal.
func RetroGreenPalette() Palette {
	gn := func(v int32) tcell.Color { return tcell.NewRGBColor(0, v, 0) }
	return Palette{
		Black:        tcell.NewRGBColor(0, 0, 0),
		Blue:         tcell.NewRGBColor(0, 0, 0), // desktop bg = pure black
		Green:        gn(150),
		Cyan:         gn(70), // window body bg
		Red:          gn(200),
		Magenta:      gn(180),
		Brown:        gn(100), // desktop cursor invert -> medium green
		LightGray:    gn(130), // title bar / menu bg
		DarkGray:     gn(60),  // shadow fg
		LightBlue:    gn(100),
		LightGreen:   tcell.NewRGBColor(100, 255, 100),
		LightCyan:    tcell.NewRGBColor(50, 255, 50),
		LightRed:     tcell.NewRGBColor(180, 255, 100),
		LightMagenta: tcell.NewRGBColor(100, 255, 100),
		Yellow:       tcell.NewRGBColor(180, 255, 100), // title accent
		White:        tcell.NewRGBColor(0, 255, 0),     // bright text
	}
}

// RetroAmberPalette is the amber-CRT counterpart to RetroGreenPalette.
func RetroAmberPalette() Palette {
	am := func(r, g int32) tcell.Color { return tcell.NewRGBColor(r, g, 0) }
	return Palette{
		Black:        tcell.NewRGBColor(0, 0, 0),
		Blue:         tcell.NewRGBColor(0, 0, 0), // desktop bg = pure black
		Green:        am(200, 130),
		Cyan:         am(100, 60), // window body bg
		Red:          am(255, 180),
		Magenta:      am(220, 150),
		Brown:        am(180, 100), // desktop cursor invert -> medium amber
		LightGray:    am(180, 110), // title bar / menu bg
		DarkGray:     am(90, 50),   // shadow fg
		LightBlue:    am(180, 100),
		LightGreen:   am(255, 200),
		LightCyan:    am(255, 220),
		LightRed:     am(255, 220),
		LightMagenta: am(255, 220),
		Yellow:       am(255, 230), // title accent
		White:        am(255, 200), // bright text
	}
}

// Invert returns the CGA-complement of c within this palette: pairs
// black↔lightgray, blue↔brown, green↔magenta, cyan↔red and the same
// pairs in the intense half. Returns c unchanged if not a recognised slot.
func (p Palette) Invert(c tcell.Color) tcell.Color {
	switch c {
	case p.Black:
		return p.LightGray
	case p.LightGray:
		return p.Black
	case p.Blue:
		return p.Brown
	case p.Brown:
		return p.Blue
	case p.Green:
		return p.Magenta
	case p.Magenta:
		return p.Green
	case p.Cyan:
		return p.Red
	case p.Red:
		return p.Cyan
	case p.DarkGray:
		return p.White
	case p.White:
		return p.DarkGray
	case p.LightBlue:
		return p.Yellow
	case p.Yellow:
		return p.LightBlue
	case p.LightGreen:
		return p.LightMagenta
	case p.LightMagenta:
		return p.LightGreen
	case p.LightCyan:
		return p.LightRed
	case p.LightRed:
		return p.LightCyan
	}
	return c
}
