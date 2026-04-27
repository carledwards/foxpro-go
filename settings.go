package foxpro

// Settings holds runtime UI preferences that can be flipped on or off
// without restarting the app. In-memory only for now; a future pass can
// add load/save against the user's home directory or wherever.
type Settings struct {
	// ShowShadows controls whether windows render their drop shadow.
	ShowShadows bool
	// ShowStatusBar controls whether the bottom hint row is drawn.
	// When false the desktop reclaims that row.
	ShowStatusBar bool
	// ThemeIndex is the currently selected entry of ThemePresets. The
	// settings window flips this and re-applies the corresponding palette
	// to App.Theme; client code should not edit it directly.
	ThemeIndex int
}

// DefaultSettings returns the all-on configuration with the Classic
// FoxPro theme selected.
func DefaultSettings() Settings {
	return Settings{
		ShowShadows:   true,
		ShowStatusBar: true,
		ThemeIndex:    0,
	}
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
