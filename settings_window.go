package foxpro

import (
	"github.com/gdamore/tcell/v2"

	"github.com/carledwards/foxpro-go/widgets"
)

// settingRow is one selectable line on the right side of the settings
// window. Implementations render themselves and respond to a single
// "activate" gesture (Space / Enter), which toggles a boolean or cycles
// a choice. Rows that also implement focusActivator are activated
// automatically as the highlight moves over them — useful for live
// previews like the theme picker.
type settingRow interface {
	Draw(screen tcell.Screen, x, y, width int, highlighted bool, theme Theme)
	Activate()
}

type focusActivator interface {
	OnFocus()
}

// radioSetting is one choice in a mutually-exclusive list. The marker
// (●) shows on the row whose isSelected returns true. select_ makes
// this row the chosen one. radioSetting auto-activates on focus, so
// just navigating the list with the arrow keys live-applies the choice.
type radioSetting struct {
	label      string
	isSelected func() bool
	choose     func()
}

func (r *radioSetting) Draw(s tcell.Screen, x, y, w int, hi bool, theme Theme) {
	widgets.DrawRadio(s, x, y, w, r.isSelected(), r.label, hi, theme.WindowBG, theme.Focus)
}

func (r *radioSetting) Activate() { r.choose() }
func (r *radioSetting) OnFocus()  { r.choose() }

// boolSetting is a checkbox row.
type boolSetting struct {
	label string
	get   func() bool
	set   func(bool)
}

func (b *boolSetting) Draw(s tcell.Screen, x, y, w int, hi bool, theme Theme) {
	widgets.DrawCheckbox(s, x, y, w, b.get(), b.label, hi, theme.WindowBG, theme.Focus)
}

func (b *boolSetting) Activate() { b.set(!b.get()) }

// choiceSetting is a single-row picker showing "Label: Value". Activate
// cycles forward through the choices, wrapping around at the end.
type choiceSetting struct {
	label   string
	choices []string
	get     func() int
	set     func(int)
}

func (c *choiceSetting) Draw(s tcell.Screen, x, y, w int, hi bool, theme Theme) {
	style := theme.WindowBG
	if hi {
		style = theme.MenuBarActive
		for i := 0; i < w; i++ {
			s.SetContent(x+i, y, ' ', nil, style)
		}
	}
	value := "?"
	if idx := c.get(); idx >= 0 && idx < len(c.choices) {
		value = c.choices[idx]
	}
	text := c.label + ": " + value
	if len(text) > w-1 {
		text = text[:w-1]
	}
	drawString(s, x+1, y, text, style)
}

func (c *choiceSetting) Activate() {
	if len(c.choices) == 0 {
		return
	}
	c.set((c.get() + 1) % len(c.choices))
}

// settingsCategory groups rows under a left-column entry.
type settingsCategory struct {
	name string
	rows []settingRow
}

// SettingsProvider renders an Apple-style settings page: a list of
// categories on the left, controls for the active category on the right.
type SettingsProvider struct {
	app        *App
	categories []settingsCategory

	catIndex int  // selected category in the left column
	rowIndex int  // selected row in the right column
	rightHas bool // true when keyboard focus is on the right column
}

// NewSettingsProvider constructs a settings provider bound to the given
// app's runtime Settings. Built-in categories: General (toggles) and
// Appearance (theme picker — live-previews as you navigate).
func NewSettingsProvider(app *App) *SettingsProvider {
	themeRows := make([]settingRow, len(ThemePresets))
	for i, preset := range ThemePresets {
		i, preset := i, preset // capture per row
		themeRows[i] = &radioSetting{
			label:      preset.Name,
			isSelected: func() bool { return app.Settings.ThemeIndex == i },
			choose: func() {
				app.Settings.ThemeIndex = i
				app.Theme = ThemeFromPalette(preset.Palette())
			},
		}
	}
	return &SettingsProvider{
		app: app,
		categories: []settingsCategory{
			{
				name: "General",
				rows: []settingRow{
					&boolSetting{
						label: "Show window shadows",
						get:   func() bool { return app.Settings.ShowShadows },
						set:   func(v bool) { app.Settings.ShowShadows = v },
					},
					&boolSetting{
						label: "Show status bar",
						get:   func() bool { return app.Settings.ShowStatusBar },
						set:   func(v bool) { app.Settings.ShowStatusBar = v },
					},
				},
			},
			{
				name: "Appearance",
				rows: themeRows,
			},
		},
		rightHas: false, // open with the left (categories) panel focused
	}
}

// NewSettingsWindow returns a non-modal floating settings window. Caller
// adds it to the WindowManager (e.g. from a menu item):
//
//   app.Manager.Add(foxpro.NewSettingsWindow(app))
func NewSettingsWindow(app *App) *Window {
	return NewWindow("Settings", Rect{X: 8, Y: 4, W: 56, H: 12}, NewSettingsProvider(app))
}

const settingsLeftWidth = 16

func (s *SettingsProvider) currentRows() []settingRow {
	if s.catIndex < 0 || s.catIndex >= len(s.categories) {
		return nil
	}
	return s.categories[s.catIndex].rows
}

func (s *SettingsProvider) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	if inner.W < 4 || inner.H < 1 {
		return
	}

	// Pick column widths that fit inside `inner`. When the window is
	// shrunk too narrow for the standard split we fall back to a
	// left-only or right-only view rather than letting cells leak past
	// the window border.
	leftW := settingsLeftWidth
	if leftW > inner.W {
		leftW = inner.W
	}
	dividerX := inner.X + leftW
	rightX := dividerX + 1
	rightW := inner.X + inner.W - rightX
	hasRight := rightW > 0

	// Left column: category list.
	for i, cat := range s.categories {
		y := inner.Y + i
		if y >= inner.Y+inner.H {
			break
		}
		highlighted := focused && !s.rightHas && i == s.catIndex
		widgets.DrawListRow(screen, inner.X, y, leftW, cat.name, highlighted, theme.WindowBG, theme.Focus)
		// Selected category gets an arrow marker in the accent colour
		// even when keyboard focus is on the right side.
		if i == s.catIndex {
			markerStyle := theme.WindowBG.Foreground(theme.TitleAccent)
			if highlighted {
				markerStyle = theme.Focus.Foreground(theme.TitleAccent)
			}
			screen.SetContent(inner.X, y, '▸', nil, markerStyle)
		}
	}

	if !hasRight {
		return
	}

	// Vertical divider (only when we have room for both columns).
	if dividerX < inner.X+inner.W {
		for y := inner.Y; y < inner.Y+inner.H; y++ {
			screen.SetContent(dividerX, y, '│', nil, theme.WindowBG)
		}
	}

	// Right column: rows for the selected category.
	for i, row := range s.currentRows() {
		y := inner.Y + i
		if y >= inner.Y+inner.H {
			break
		}
		highlighted := focused && s.rightHas && i == s.rowIndex
		row.Draw(screen, rightX, y, rightW, highlighted, theme)
	}
}

// StatusHint contributes a contextual key hint to the bottom status bar
// while the settings window is active.
func (s *SettingsProvider) StatusHint() string {
	if s.rightHas {
		return "Space: toggle  ↑↓: move  Shift+Tab: back "
	}
	return "↑↓: change category  Tab: controls "
}

// HandleMouse routes presses inside the settings window body. Click on
// the left column selects a category; click on the right column selects
// (and activates) a control.
func (s *SettingsProvider) HandleMouse(ev *tcell.EventMouse, inner Rect) bool {
	mx, my := ev.Position()
	if !inner.Contains(mx, my) {
		return false
	}
	row := my - inner.Y
	if mx < inner.X+settingsLeftWidth {
		// Left column.
		if row >= 0 && row < len(s.categories) {
			s.catIndex = row
			s.rowIndex = 0
			s.rightHas = false
		}
		return true
	}
	if mx >= inner.X+settingsLeftWidth+1 {
		// Right column.
		rows := s.currentRows()
		if row >= 0 && row < len(rows) {
			s.rowIndex = row
			s.rightHas = true
			rows[row].Activate()
		}
		return true
	}
	return false
}

func (s *SettingsProvider) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyTab:
		s.rightHas = true
		return true
	case tcell.KeyBacktab:
		s.rightHas = false
		return true
	case tcell.KeyLeft:
		s.rightHas = false
		return true
	case tcell.KeyRight:
		s.rightHas = true
		return true
	case tcell.KeyUp:
		if s.rightHas {
			if s.rowIndex > 0 {
				s.rowIndex--
				s.maybeFocusActivate()
			}
		} else if s.catIndex > 0 {
			s.catIndex--
			s.rowIndex = 0
		}
		return true
	case tcell.KeyDown:
		if s.rightHas {
			if rows := s.currentRows(); s.rowIndex < len(rows)-1 {
				s.rowIndex++
				s.maybeFocusActivate()
			}
		} else if s.catIndex < len(s.categories)-1 {
			s.catIndex++
			s.rowIndex = 0
		}
		return true
	case tcell.KeyEnter:
		s.activateRow()
		return true
	case tcell.KeyRune:
		if ev.Rune() == ' ' {
			s.activateRow()
			return true
		}
	}
	return false
}

func (s *SettingsProvider) activateRow() {
	if !s.rightHas {
		return
	}
	rows := s.currentRows()
	if s.rowIndex < 0 || s.rowIndex >= len(rows) {
		return
	}
	rows[s.rowIndex].Activate()
}

// maybeFocusActivate fires the OnFocus hook of the newly highlighted row,
// if it has one. Used by radio rows to live-apply on navigation.
func (s *SettingsProvider) maybeFocusActivate() {
	rows := s.currentRows()
	if s.rowIndex < 0 || s.rowIndex >= len(rows) {
		return
	}
	if fa, ok := rows[s.rowIndex].(focusActivator); ok {
		fa.OnFocus()
	}
}
