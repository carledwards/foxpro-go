package foxpro

import "github.com/gdamore/tcell/v2"

// Theme holds the styles used for the desktop, menu bar, windows, and
// chrome. Themes are built from a Palette so a single layout works across
// many color schemes — see ThemeFromPalette and the *Palette presets.
type Theme struct {
	// Palette this theme was built from. Held so widgets that need to
	// derive related colours (mouse-cursor invert, palette swatches,
	// etc.) can reach back to the slot definitions.
	Palette Palette

	// Desktop background — DesktopRune is painted in every desktop cell.
	Desktop     tcell.Style
	DesktopRune rune

	// Menu bar (row 0).
	MenuBar         tcell.Style // unselected, enabled item
	MenuBarActive   tcell.Style // highlighted (hover/keyboard) item
	MenuAccel       tcell.Style // accelerator letter on a normal item
	MenuAccelActive tcell.Style // accelerator letter on highlighted item
	MenuDisabled    tcell.Style // greyed-out items
	MenuHotkey      tcell.Style // shortcut hint text (e.g. "Ctrl+O") —
	                            // not yet used; reserved for the future

	// Single-line text input fields (InputProvider). FoxPro draws
	// inputs as a flat gray strip distinct from the surrounding
	// content background.
	Input tcell.Style

	// Floating windows.
	WindowBG      tcell.Style
	Border        tcell.Style // top/left/bottom edges of bordered
	                          // panels (BoxedProvider). Convention:
	                          // bg = window content colour, fg = a
	                          // contrasting line colour
	Scrollbar     tcell.Style // right edge of bordered panels — the
	                          // column that hosts vertical scrollbar
	                          // arrows + thumb. Convention: inverted
	                          // bg vs Border so the scroll column reads
	                          // as a distinct strip
	TitleActive   tcell.Style // top-row frame for the focused window
	TitleInactive tcell.Style // top-row frame for unfocused windows
	TitleAccent   tcell.Color // foreground for title text and chrome
	                          // glyphs; combined with the title row bg
	Shadow        tcell.Style

	// Focused widget — the brown "selected button" look from FoxPro.
	// Use for the currently focused row/control inside a content
	// provider so keyboard focus is unmistakable.
	Focus tcell.Style
}

// CGA palette accessors — kept as package-level values for back-compat
// with existing code. They are exactly the Classic palette's slots.
var (
	CGABlack        = ClassicPalette().Black
	CGABlue         = ClassicPalette().Blue
	CGAGreen        = ClassicPalette().Green
	CGACyan         = ClassicPalette().Cyan
	CGARed          = ClassicPalette().Red
	CGAMagenta      = ClassicPalette().Magenta
	CGABrown        = ClassicPalette().Brown
	CGALightGray    = ClassicPalette().LightGray
	CGADarkGray     = ClassicPalette().DarkGray
	CGALightBlue    = ClassicPalette().LightBlue
	CGALightGreen   = ClassicPalette().LightGreen
	CGALightCyan    = ClassicPalette().LightCyan
	CGALightRed     = ClassicPalette().LightRed
	CGALightMagenta = ClassicPalette().LightMagenta
	CGAYellow       = ClassicPalette().Yellow
	CGAWhite        = ClassicPalette().White
)

// DefaultTheme returns the Classic FoxPro-for-DOS theme.
func DefaultTheme() Theme { return ThemeFromPalette(ClassicPalette()) }

// ThemeFromPalette builds the standard FoxPro-style Theme using the slots
// of the given palette. Layout and role assignments are identical across
// palettes — only the underlying colours change.
func ThemeFromPalette(p Palette) Theme {
	return Theme{
		Palette: p,

		Desktop:     tcell.StyleDefault.Background(p.Blue).Foreground(p.Blue),
		DesktopRune: ' ',

		MenuBar:         tcell.StyleDefault.Background(p.LightGray).Foreground(p.Black),
		MenuBarActive:   tcell.StyleDefault.Background(p.Cyan).Foreground(p.White),
		MenuAccel:       tcell.StyleDefault.Background(p.LightGray).Foreground(p.White),
		MenuAccelActive: tcell.StyleDefault.Background(p.Cyan).Foreground(p.White),
		MenuDisabled:    tcell.StyleDefault.Background(p.LightGray).Foreground(p.Cyan),
		MenuHotkey:      tcell.StyleDefault.Background(p.LightGray).Foreground(p.White),

		Input:         tcell.StyleDefault.Background(p.LightGray).Foreground(p.White),

		WindowBG:      tcell.StyleDefault.Background(p.Cyan).Foreground(p.White),
		Border:        tcell.StyleDefault.Background(p.Cyan).Foreground(p.Blue),
		Scrollbar:     tcell.StyleDefault.Background(p.Blue).Foreground(p.LightCyan),
		TitleActive:   tcell.StyleDefault.Background(p.LightGray).Foreground(p.DarkGray),
		TitleInactive: tcell.StyleDefault.Background(p.LightGray).Foreground(p.DarkGray),
		TitleAccent:   p.Yellow,
		Shadow:        tcell.StyleDefault.Background(p.Black).Foreground(p.DarkGray),

		Focus: tcell.StyleDefault.Background(p.Brown).Foreground(p.White),
	}
}

// InvertColor returns the cursor-inversion complement of c using this
// theme's palette. Convenience wrapper around Palette.Invert.
func (t Theme) InvertColor(c tcell.Color) tcell.Color { return t.Palette.Invert(c) }
