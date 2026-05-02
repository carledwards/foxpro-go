package foxpro

import "github.com/gdamore/tcell/v2"

// Settings holds runtime UI preferences that can be flipped on or off
// without restarting the app. In-memory only for now; a future pass can
// add load/save against the user's home directory or wherever.
type Settings struct {
	// ShowShadows controls whether windows render their drop shadow.
	ShowShadows bool
	// ShowStatusBar controls whether the bottom hint row is drawn.
	// When false the desktop reclaims that row.
	ShowStatusBar bool
	// StatusBarLeft, when non-empty, replaces the default left-side
	// hint text on the status bar (the "F1: status  F2: cmd  F10: menu
	// F6: next window  Esc: quit" string). Hosts that bind a different
	// set of keys — or none of them — can use this to surface a
	// shorter, app-appropriate hint without disabling the bar.
	// Empty means use the default.
	StatusBarLeft string
	// ThemeIndex is the currently selected entry of ThemePresets. The
	// settings window flips this and re-applies the corresponding palette
	// to App.Theme; client code should not edit it directly.
	ThemeIndex int
	// QuitKeys lists key chords that the built-in key handler treats as
	// "quit" — calling App.Quit and ending the event loop. Defaults to
	// [Esc, Ctrl+Q] for terminal apps. Set to nil to disable the built-in
	// binding entirely; useful for hosts where tearing the app down is
	// structurally broken (e.g. wasm: Quit terminates the Go runtime,
	// leaving the page locked). Apps can also override with their own
	// chord set without losing other built-ins (F1/F2/F6/F10).
	QuitKeys []tcell.Key
	// BackgroundDragChords lists press gestures on a window's title bar
	// that begin a "background drag" — moving the window without
	// changing its z-order. Defaults to right-click, Shift+left-click,
	// and Ctrl+Alt+left-click. Set to nil to disable, or override with
	// a host-specific set:
	//
	//   - Terminals: right-click is the conventional gesture, but some
	//     terminals (and multiplexers like tmux/cmux) intercept it for
	//     their own context menu. Shift+click can collide with text
	//     selection. Ctrl+Alt+click is a multiplexer-safe fallback —
	//     few terminals or multiplexers bind it.
	//   - Browsers: Shift+click is conventional; right-click typically
	//     opens a native context menu and is unreliable.
	//
	// Mods on a chord are "required modifiers": extras are OK, the
	// listed bits must all be present. A chord with Mods=0 accepts any
	// modifier state.
	BackgroundDragChords []BackgroundDragChord
}

// BackgroundDragChord names a press gesture (a button + required
// modifier mask) that initiates a background drag in the title bar.
type BackgroundDragChord struct {
	Button tcell.ButtonMask // typically Button1 or Button2
	Mods   tcell.ModMask    // modifiers that must be held; extras allowed
}

// DefaultSettings returns the all-on configuration with the Classic
// FoxPro theme selected.
func DefaultSettings() Settings {
	return Settings{
		ShowShadows:   true,
		ShowStatusBar: true,
		ThemeIndex:    0,
		QuitKeys:      []tcell.Key{tcell.KeyEscape, tcell.KeyCtrlQ},
		BackgroundDragChords: []BackgroundDragChord{
			{Button: tcell.Button2, Mods: 0},                              // right-click, any mods
			{Button: tcell.Button1, Mods: tcell.ModShift},                 // shift+left-click
			{Button: tcell.Button1, Mods: tcell.ModCtrl | tcell.ModAlt},   // ctrl+alt+left-click (multiplexer-safe)
		},
	}
}

// IsQuitKey reports whether k is configured as a quit chord in this
// Settings. nil/empty QuitKeys means the built-in quit binding is
// disabled.
func (s Settings) IsQuitKey(k tcell.Key) bool {
	for _, q := range s.QuitKeys {
		if q == k {
			return true
		}
	}
	return false
}

// IsBackgroundDragChord reports whether the given press matches any
// configured background-drag chord. Modifier bits required by the
// chord must all be present; extras are ignored.
func (s Settings) IsBackgroundDragChord(btn tcell.ButtonMask, mods tcell.ModMask) bool {
	for _, c := range s.BackgroundDragChords {
		if c.Button == btn && (mods&c.Mods) == c.Mods {
			return true
		}
	}
	return false
}

// ThemePreset is a named palette factory. ThemePresets is the catalogue
// the settings window iterates over for the Theme picker; Settings.ThemeIndex
// stores which entry is active.
type ThemePreset struct {
	Name    string
	Palette func() Palette
}

// ThemePresets is the list of built-in themes. Apps can prepend or
// append entries before calling NewSettingsWindow to expose custom looks.
var ThemePresets = []ThemePreset{
	{"Classic FoxPro", ClassicPalette},
	{"Dracula", DraculaPalette},
	{"Monochrome", MonochromePalette},
	{"Retro Green", RetroGreenPalette},
	{"Retro Amber", RetroAmberPalette},
}
