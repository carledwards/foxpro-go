package foxpro

import "github.com/gdamore/tcell/v2"

// Scheme is one row of FoxPro's color configurator: a named visual
// role (Window, Dialog, Alert, DialogPop, ...) with the same 10
// slots in each. We currently only populate Theme.Dialog from this
// struct; the desktop-window styles still live as flat fields on
// Theme. See docs/wishlist.md → "FoxPro Scheme model" for the
// migration plan.
//
// Slot meanings (mirrors FoxPro's configurator labels):
//
//   - NormalText:   body text colour for the scheme
//   - TextBox:      input/text fields embedded in the scheme (cyan
//                   strips inside a magenta dialog, etc.)
//   - Border:       chrome lines (outer + inner frame characters)
//   - TitleActive:  title bar text when the surface is focused
//                   (yellow accent on FoxPro)
//   - TitleIdle:    title text when unfocused
//   - SelectedItem: focus stripe behind the highlighted list row /
//                   button (white-on-blue inside a dialog)
//   - HotKey:       accelerator letter inside a label
//   - Shadow:       drop-shadow style behind the surface
//   - EnabledCtrl:  buttons / clickable text in their resting state
//   - DisabledCtrl: greyed-out version of an enabled control
//   - CastsShadow:  whether surfaces using this scheme draw a shadow
type Scheme struct {
	NormalText, TextBox, Border        tcell.Style
	TitleActive, TitleIdle             tcell.Style
	SelectedItem, HotKey               tcell.Style
	Shadow                             tcell.Style
	EnabledCtrl, DisabledCtrl          tcell.Style
	CastsShadow                        bool
}

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

	// WaitWindow is the style used for transient WAIT WINDOW
	// notifications (toast-style overlays). Defaults to the FoxPro
	// dialog look — white text on the CGA "Magenta" slot, which is
	// the deep purple-maroon DOS apps used for modal-ish messages.
	WaitWindow tcell.Style

	// Dialog is the FoxPro modal-dialog scheme — magenta body, double-
	// line outer border, single-line inner border, blue selection
	// stripe. Used by windows constructed with Window.Dialog=true.
	// See Scheme for slot definitions.
	Dialog Scheme
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

		WaitWindow: tcell.StyleDefault.Background(p.Magenta).Foreground(p.White),

		Dialog: dialogScheme(p),
	}
}

// dialogScheme builds the Dialog scheme for the given palette. Layout
// (which slot maps to which palette slot) is fixed across themes —
// only the underlying colours change. Modeled on the FoxPro for DOS
// "Dialogs" colour scheme: white-on-magenta body, yellow-on-magenta
// title, white-on-blue focus stripe, cyan text-box strips.
func dialogScheme(p Palette) Scheme {
	bg := p.Magenta
	return Scheme{
		NormalText:   tcell.StyleDefault.Background(bg).Foreground(p.White),
		TextBox:      tcell.StyleDefault.Background(p.Cyan).Foreground(p.White),
		Border:       tcell.StyleDefault.Background(bg).Foreground(p.White),
		TitleActive:  tcell.StyleDefault.Background(bg).Foreground(p.Yellow),
		TitleIdle:    tcell.StyleDefault.Background(bg).Foreground(p.White),
		SelectedItem: tcell.StyleDefault.Background(p.Blue).Foreground(p.White),
		HotKey:       tcell.StyleDefault.Background(bg).Foreground(p.Yellow),
		Shadow:       tcell.StyleDefault.Background(p.Black).Foreground(p.DarkGray),
		EnabledCtrl:  tcell.StyleDefault.Background(bg).Foreground(p.White),
		DisabledCtrl: tcell.StyleDefault.Background(bg).Foreground(p.DarkGray),
		CastsShadow:  true,
	}
}

// InvertColor returns the cursor-inversion complement of c using this
// theme's palette. Convenience wrapper around Palette.Invert.
func (t Theme) InvertColor(c tcell.Color) tcell.Color { return t.Palette.Invert(c) }
