package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Settings tabs
var settingsTabs = []string{"Release", "Theme"}

// Number of focusable elements per tab
// 0=base branch, 1=env1 name, 2=env1 branch, 3=env2 name, 4=env2 branch,
// 5=env3 name, 6=env3 branch, 7=env4 name, 8=env4 branch, 9=textarea, 10=save button
const settingsReleaseFieldCount = 11
const settingsThemeFieldCount = 2 // theme list, save button

// Invalid characters for file patterns (excluding glob special chars)
var (
	invalidPatternChars = regexp.MustCompile(`[<>:"|?\\]`)
	tooWidePattern      = regexp.MustCompile(`^((/?\*\*?/?)|(/))$`)
	invalidPatternOrder = regexp.MustCompile(`(^/?\*/\*\*)|(^/?\*\*/\*)|(\*/\*\*/?$)|(\*\*/\*/?$)|(/\*\*([^/]|$))|(([^/]|^)\*\*/)|(([^/]|^)\*\*([^/]|$))`)
)

// updateSettings handles key events when settings modal is open
func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+q":
		// Close without saving; revert any unsaved theme preview
		loadThemeFromConfig()
		(&m).updateTextareaTheme()
		m.showSettings = false
		m.settingsBaseBranch.Blur()
		for i := 0; i < 4; i++ {
			m.settingsEnvNames[i].Blur()
			m.settingsEnvBranches[i].Blur()
		}
		m.settingsExcludePatterns.Blur()
		m.settingsError = ""
		m.settingsFocusIndex = 0
		return m, nil

	case "H":
		// Switch to previous tab (keep unsaved changes including theme preview)
		if m.settingsTab > 0 {
			m.settingsTab--
			m.settingsFocusIndex = 0
			return m.updateSettingsFocus()
		}
		return m, nil

	case "L":
		// Switch to next tab
		if m.settingsTab < len(settingsTabs)-1 {
			m.settingsBaseBranch.Blur()
			for i := 0; i < 4; i++ {
				m.settingsEnvNames[i].Blur()
				m.settingsEnvBranches[i].Blur()
			}
			m.settingsExcludePatterns.Blur()
			m.settingsTab++
			m.settingsFocusIndex = 0
			if m.settingsTab == 1 {
				m.loadSettingsThemes()
			}
			return m, nil
		}
		return m, nil
	}

	// Delegate to tab-specific handler
	switch m.settingsTab {
	case 0: // Release tab
		return m.updateSettingsRelease(msg)
	case 1: // Theme tab
		return m.updateSettingsTheme(msg)
	}

	return m, nil
}

// updateSettingsRelease handles key events on the Release settings tab
func (m model) updateSettingsRelease(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.settingsFocusIndex = (m.settingsFocusIndex + 1) % settingsReleaseFieldCount
		return m.updateSettingsFocus()

	case "shift+tab":
		m.settingsFocusIndex = (m.settingsFocusIndex - 1 + settingsReleaseFieldCount) % settingsReleaseFieldCount
		return m.updateSettingsFocus()

	case "down":
		// Jump to same column in next row (skip textarea internals)
		// 0=base → 1=env1 name; 1→3→5→7→9; 2→4→6→8→9; 9→10
		if m.settingsFocusIndex == 9 {
			// textarea: let down key pass through unless at save focus
			break
		}
		switch {
		case m.settingsFocusIndex == 0:
			m.settingsFocusIndex = 1
		case m.settingsFocusIndex == 7: // last env name → textarea
			m.settingsFocusIndex = 9
		case m.settingsFocusIndex == 8: // last env branch → textarea
			m.settingsFocusIndex = 9
		case m.settingsFocusIndex%2 == 1 && m.settingsFocusIndex < 8: // env name → next env name
			m.settingsFocusIndex += 2
		case m.settingsFocusIndex%2 == 0 && m.settingsFocusIndex < 9: // env branch → next env branch
			m.settingsFocusIndex += 2
		case m.settingsFocusIndex == 10:
			return m, nil
		}
		return m.updateSettingsFocus()

	case "up":
		// Jump to same column in previous row
		if m.settingsFocusIndex == 9 {
			// textarea: let up key pass through
			break
		}
		switch {
		case m.settingsFocusIndex == 0:
			return m, nil
		case m.settingsFocusIndex == 1: // first env name → base branch
			m.settingsFocusIndex = 0
		case m.settingsFocusIndex == 2: // first env branch → base branch
			m.settingsFocusIndex = 0
		case m.settingsFocusIndex == 10: // save → textarea
			m.settingsFocusIndex = 9
		case m.settingsFocusIndex%2 == 1: // env name → prev env name
			m.settingsFocusIndex -= 2
		case m.settingsFocusIndex%2 == 0: // env branch → prev env branch
			m.settingsFocusIndex -= 2
		}
		return m.updateSettingsFocus()

	case "enter":
		// If on save button (index 10), validate and save
		if m.settingsFocusIndex == 10 {
			m.settingsError = m.validateReleaseSettings()
			if m.settingsError == "" {
				m.saveAllSettings()
				m.showSettings = false
				m.settingsExcludePatterns.Blur()
				m.settingsBaseBranch.Blur()
				for i := 0; i < 4; i++ {
					m.settingsEnvNames[i].Blur()
					m.settingsEnvBranches[i].Blur()
				}
				m.settingsFocusIndex = 0
			}
			return m, nil
		}
		// If on textarea (index 9), pass through for newline
		if m.settingsFocusIndex == 9 {
			break
		}
		// On any text input, move to next field
		m.settingsFocusIndex = (m.settingsFocusIndex + 1) % settingsReleaseFieldCount
		return m.updateSettingsFocus()
	}

	// Route key events to the focused input
	return m.updateSettingsReleaseInput(msg)
}

// updateSettingsTheme handles key events on the Theme settings tab
func (m model) updateSettingsTheme(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.settingsFocusIndex = (m.settingsFocusIndex + 1) % settingsThemeFieldCount
		return m, nil

	case "shift+tab":
		m.settingsFocusIndex = (m.settingsFocusIndex - 1 + settingsThemeFieldCount) % settingsThemeFieldCount
		return m, nil

	case "up", "k":
		if m.settingsFocusIndex == 0 && m.settingsThemeIndex > 0 {
			m.settingsThemeIndex--
			if m.settingsThemeIndex < len(m.settingsThemes) {
				applyTheme(m.settingsThemes[m.settingsThemeIndex])
				(&m).updateTextareaTheme()
			}
		}
		return m, nil

	case "down", "j":
		if m.settingsFocusIndex == 0 && m.settingsThemeIndex < len(m.settingsThemes)-1 {
			m.settingsThemeIndex++
			applyTheme(m.settingsThemes[m.settingsThemeIndex])
			(&m).updateTextareaTheme()
		}
		return m, nil

	case "enter":
		// Save button: save all settings and close
		if m.settingsFocusIndex == 1 {
			m.settingsError = m.validatePatterns()
			if m.settingsError == "" {
				m.saveAllSettings()
				m.showSettings = false
				m.settingsFocusIndex = 0
			}
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// loadSettingsThemes reloads themes from config file
func (m *model) loadSettingsThemes() {
	config, err := LoadConfig()
	if err != nil {
		m.settingsThemes = nil
		m.settingsThemeIndex = 0
		m.settingsThemeError = fmt.Sprintf("Failed to load config: %v", err)
		return
	}
	m.settingsThemeError = ""
	if len(config.Themes) == 0 {
		m.settingsThemes = nil
		m.settingsThemeIndex = 0
		return
	}
	m.settingsThemes = config.Themes

	// Set cursor to currently selected theme
	m.settingsThemeIndex = 0
	for i, tc := range config.Themes {
		if tc.Name == config.SelectedTheme {
			m.settingsThemeIndex = i
			break
		}
	}
}

// updateTextareaTheme updates the settings textarea styles to match currentTheme.
// Must be called from Update path (pointer receiver) so the style pointer rebinds correctly.
func (m *model) updateTextareaTheme() {
	m.settingsExcludePatterns.FocusedStyle.CursorLine = lipgloss.NewStyle().Foreground(currentTheme.Foreground)
	m.settingsExcludePatterns.FocusedStyle.Text = lipgloss.NewStyle().Foreground(currentTheme.Foreground)
	m.settingsExcludePatterns.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	m.settingsExcludePatterns.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent)
	m.settingsExcludePatterns.BlurredStyle.Text = lipgloss.NewStyle().Foreground(currentTheme.Foreground)
	m.settingsExcludePatterns.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.BlurredStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	m.settingsExcludePatterns.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(currentTheme.Notion)
	// Re-focus/blur to rebind the internal style pointer to the updated styles
	if m.settingsExcludePatterns.Focused() {
		m.settingsExcludePatterns.Focus()
	} else {
		m.settingsExcludePatterns.Blur()
	}
}

// updateSettingsReleaseInput routes key events to the currently focused input
func (m model) updateSettingsReleaseInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.settingsFocusIndex {
	case 0: // Base branch
		m.settingsBaseBranch, cmd = m.settingsBaseBranch.Update(msg)
	case 1, 3, 5, 7: // Env names (indices 1,3,5,7)
		idx := (m.settingsFocusIndex - 1) / 2
		m.settingsEnvNames[idx], cmd = m.settingsEnvNames[idx].Update(msg)
		// Force uppercase
		upper := strings.ToUpper(m.settingsEnvNames[idx].Value())
		if upper != m.settingsEnvNames[idx].Value() {
			pos := m.settingsEnvNames[idx].Position()
			m.settingsEnvNames[idx].SetValue(upper)
			m.settingsEnvNames[idx].SetCursor(pos)
		}
	case 2, 4, 6, 8: // Env branches (indices 2,4,6,8)
		idx := (m.settingsFocusIndex - 2) / 2
		m.settingsEnvBranches[idx], cmd = m.settingsEnvBranches[idx].Update(msg)
	case 9: // Textarea
		m.settingsExcludePatterns, cmd = m.settingsExcludePatterns.Update(msg)
		m.settingsError = m.validatePatterns()
	}
	return m, cmd
}

// updateSettingsFocus updates focus state based on settingsFocusIndex
func (m model) updateSettingsFocus() (tea.Model, tea.Cmd) {
	// Blur everything first
	m.settingsBaseBranch.Blur()
	for i := 0; i < 4; i++ {
		m.settingsEnvNames[i].Blur()
		m.settingsEnvBranches[i].Blur()
	}
	m.settingsExcludePatterns.Blur()

	// Focus the right input
	switch m.settingsFocusIndex {
	case 0:
		return m, m.settingsBaseBranch.Focus()
	case 1, 3, 5, 7:
		idx := (m.settingsFocusIndex - 1) / 2
		return m, m.settingsEnvNames[idx].Focus()
	case 2, 4, 6, 8:
		idx := (m.settingsFocusIndex - 2) / 2
		return m, m.settingsEnvBranches[idx].Focus()
	case 9:
		return m, m.settingsExcludePatterns.Focus()
	}
	// Index 10 = save button, nothing to focus
	return m, nil
}

// validatePatterns validates the exclude patterns and returns an error message if invalid
func (m *model) validatePatterns() string {
	value := m.settingsExcludePatterns.Value()
	if value == "" {
		return ""
	}

	lines := strings.Split(value, "\n")
	for i, line := range lines {
		// Check for empty lines (not allowed)
		if strings.TrimSpace(line) == "" {
			return "Line " + itoa(i+1) + ": empty lines are not allowed"
		}

		// Check for max line length
		if len(line) > 80 {
			return "Line " + itoa(i+1) + ": exceeds maximum length of 80 characters"
		}

		if strings.Contains(line, "//") {
			return "Line " + itoa(i+1) + ": empty folders // are not allowed"
		}

		if tooWidePattern.MatchString(line) {
			return "Line " + itoa(i+1) + ": it's too wide pattern that will affect many unintended files/folders"
		}

		// Check for invalid characters
		if invalidPatternChars.MatchString(line) {
			return "Line " + itoa(i+1) + ": contains invalid characters (<>:\"|?\\)"
		}

		// Check for non-printable characters
		for _, r := range line {
			if !unicode.IsPrint(r) && r != '\t' {
				return "Line " + itoa(i+1) + ": contains non-printable characters"
			}
		}

		// Check for valid glob pattern structure
		if err := validateGlobPattern(line); err != "" {
			return "Line " + itoa(i+1) + ": " + err
		}
	}

	return ""
}

// validateGlobPattern validates a single glob pattern
func validateGlobPattern(pattern string) string {
	if invalidPatternOrder.MatchString(pattern) {
		return "wrong pattern wildcards order or its format"
	}
	return ""
}

// itoa converts int to string (simple implementation to avoid importing strconv)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// validateReleaseSettings validates all Release tab fields
func (m *model) validateReleaseSettings() string {
	// Validate base branch
	baseBranch := strings.TrimSpace(m.settingsBaseBranch.Value())
	if baseBranch == "" {
		return "Base branch cannot be empty"
	}
	if strings.Contains(baseBranch, " ") {
		return "Base branch cannot contain spaces"
	}

	// Validate environment names and branches
	for i := 0; i < 4; i++ {
		name := strings.TrimSpace(m.settingsEnvNames[i].Value())
		branch := strings.TrimSpace(m.settingsEnvBranches[i].Value())
		if name == "" {
			return fmt.Sprintf("Environment %d: name cannot be empty", i+1)
		}
		if branch == "" {
			return fmt.Sprintf("Environment %d: branch cannot be empty", i+1)
		}
		if strings.Contains(branch, " ") {
			return fmt.Sprintf("Environment %d: branch cannot contain spaces", i+1)
		}
	}

	// Validate exclude patterns
	return m.validatePatterns()
}

// saveAllSettings saves all settings across tabs to config file
func (m *model) saveAllSettings() {
	config, err := LoadConfig()
	if err != nil {
		config = &AppConfig{}
	}
	// Release tab: base branch
	config.BaseBranch = strings.TrimSpace(m.settingsBaseBranch.Value())
	// Release tab: environments
	config.Environments = make([]EnvConfig, 4)
	for i := 0; i < 4; i++ {
		config.Environments[i] = EnvConfig{
			Name:       strings.TrimSpace(m.settingsEnvNames[i].Value()),
			BranchName: strings.TrimSpace(m.settingsEnvBranches[i].Value()),
		}
	}
	// Release tab: exclude patterns
	config.ExcludePatterns = m.settingsExcludePatterns.Value()
	// Theme tab: selected theme
	if m.settingsThemeIndex < len(m.settingsThemes) {
		config.SelectedTheme = m.settingsThemes[m.settingsThemeIndex].Name
	}
	SaveConfig(config)

	// Rebuild runtime environments from saved config
	m.environments = getEnvironments()
}

// overlaySettings renders the settings modal as an overlay
func (m model) overlaySettings(background string) string {
	// Calculate 80% dimensions
	modalWidth := m.width * 80 / 100
	modalHeight := m.height * 80 / 100

	// Minimum sizes
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalHeight < 15 {
		modalHeight = 15
	}

	var b strings.Builder

	// Title
	b.WriteString(settingsTitleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Tabs
	for i, tab := range settingsTabs {
		if i == m.settingsTab {
			b.WriteString(settingsTabActiveStyle.Render(tab))
		} else {
			b.WriteString(settingsTabStyle.Render(tab))
		}
		if i < len(settingsTabs)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")

	// Tab content
	switch m.settingsTab {
	case 0: // Release tab
		b.WriteString(m.renderReleaseSettings(modalWidth, modalHeight))
	case 1: // Theme tab
		b.WriteString(m.renderThemeSettings(modalWidth, modalHeight))
	}

	// Help footer text
	var helpText string
	if m.settingsTab == 1 {
		helpText = helpStyle.Render("j/k: nav • tab: focus • enter: save • H/L: switch tab • esc/C+q: close")
	} else {
		helpText = helpStyle.Render("tab: focus • enter: save • H/L: switch tab • esc/C+q: close")
	}

	// Calculate inner dimensions (modal minus padding only, border is outside Width)
	innerWidth := modalWidth - 4   // 4 horizontal padding (border is added outside)
	innerHeight := modalHeight - 2 // top padding handled separately

	// Main content (everything above help)
	mainContent := b.String()
	mainHeight := lipgloss.Height(mainContent)

	// Calculate spacing needed (at least 1 line gap before help)
	spacingNeeded := innerHeight - mainHeight - 1 // -1 for help line
	if spacingNeeded < 1 {
		spacingNeeded = 1
	}

	// Build final content with help at bottom
	finalContent := mainContent + strings.Repeat("\n", spacingNeeded) + helpText

	// Create modal style with dynamic size (no bottom padding to keep help at very bottom)
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		PaddingTop(1).
		PaddingLeft(2).
		PaddingRight(2).
		Width(modalWidth)

	modalContent := modalStyle.Render(lipgloss.NewStyle().
		Width(innerWidth).
		Height(innerHeight).
		Render(finalContent))

	return placeOverlayCenter(modalContent, background, m.width, m.height)
}

// renderReleaseSettings renders the Release tab content
func (m model) renderReleaseSettings(modalWidth, modalHeight int) string {
	var b strings.Builder

	contentWidth := modalWidth - 4
	if contentWidth < 30 {
		contentWidth = 30
	}

	// --- Base branch ---
	b.WriteString(settingsLabelStyle.Render("Base branch"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Branch from which release branches are created and merged back to."))
	b.WriteString("\n\n")

	m.settingsBaseBranch.Width = contentWidth - 2
	b.WriteString(m.settingsBaseBranch.View())
	b.WriteString("\n\n")

	// --- Environment branches ---
	b.WriteString(settingsLabelStyle.Render("Environment branches"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Customize environment names and their git branch mappings."))
	b.WriteString("\n\n")

	// Environment color dots by position
	envColors := []lipgloss.Color{currentTheme.EnvDevelop, currentTheme.EnvTest, currentTheme.EnvStage, currentTheme.EnvProd}
	arrowStyle := lipgloss.NewStyle().Foreground(currentTheme.Notion)

	for i := 0; i < 4; i++ {
		dot := lipgloss.NewStyle().Foreground(envColors[i]).Render("●")
		m.settingsEnvNames[i].Width = 7
		m.settingsEnvBranches[i].Width = 20
		b.WriteString(fmt.Sprintf("%s  %s  %s  %s",
			dot,
			m.settingsEnvNames[i].View(),
			arrowStyle.Render("→"),
			m.settingsEnvBranches[i].View(),
		))
		b.WriteString("\n")
	}

	// --- Exclude patterns ---
	b.WriteString("\n")
	b.WriteString(settingsLabelStyle.Render("Files to exclude from release"))
	b.WriteString("\n")
	desc := "Enumerate file paths patterns to exclude from release, one per line. " +
		"/**/ - any folders, * - any char sequence."
	b.WriteString(helpStyle.Render(desc))
	b.WriteString("\n\n")

	m.settingsExcludePatterns.SetWidth(contentWidth)
	m.settingsExcludePatterns.SetHeight(6)
	b.WriteString(m.settingsExcludePatterns.View())

	// Error hint
	if m.settingsError != "" {
		b.WriteString("\n")
		b.WriteString(settingsErrorStyle.Render(m.settingsError))
	}

	// Save button (centered)
	b.WriteString("\n\n")
	buttonText := "Save and close"
	var btnStyle lipgloss.Style
	if m.settingsFocusIndex == 10 && m.settingsError == "" {
		btnStyle = buttonActiveStyle
	} else {
		btnStyle = buttonStyle
	}
	button := btnStyle.Render(buttonText)
	buttonWidth := lipgloss.Width(button)
	padding := (contentWidth - buttonWidth) / 2
	if padding > 0 {
		b.WriteString(strings.Repeat(" ", padding))
	}
	b.WriteString(button)

	return b.String()
}

// renderThemeSettings renders the Theme tab content
func (m model) renderThemeSettings(modalWidth, modalHeight int) string {
	var b strings.Builder

	b.WriteString(settingsLabelStyle.Render("Select theme"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Themes are configured in ~/.relix/config.json"))
	b.WriteString("\n\n")

	if m.settingsThemeError != "" {
		b.WriteString(settingsErrorStyle.Render(m.settingsThemeError))
		return b.String()
	}

	if len(m.settingsThemes) == 0 {
		b.WriteString(helpStyle.Render("No themes found in config file"))
		return b.String()
	}

	// Get current active theme name from config
	activeThemeName := ""
	if config, err := LoadConfig(); err == nil {
		activeThemeName = config.SelectedTheme
	}

	for i, tc := range m.settingsThemes {
		isSelected := i == m.settingsThemeIndex
		isActive := tc.Name == activeThemeName

		// Build theme line with color preview dots (resolved with fallbacks)
		r := themeFromConfig(tc)
		accentDot := lipgloss.NewStyle().Foreground(r.Accent).Render("●")
		fgDot := lipgloss.NewStyle().Foreground(r.Foreground).Render("●")
		successDot := lipgloss.NewStyle().Foreground(r.Success).Render("●")
		warningDot := lipgloss.NewStyle().Foreground(r.Warning).Render("●")
		errorDot := lipgloss.NewStyle().Foreground(r.Error).Render("●")
		dots := accentDot + fgDot + successDot + warningDot + errorDot

		name := tc.Name
		if isActive {
			name += " (active)"
		}

		if isSelected {
			prefix := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Accent).Render("> ")
			nameStyled := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Accent).Render(name)
			b.WriteString(prefix + dots + " " + nameStyled)
		} else {
			nameStyled := lipgloss.NewStyle().Foreground(currentTheme.Foreground).Render(name)
			b.WriteString("  " + dots + " " + nameStyled)
		}
		b.WriteString("\n")
	}

	// Show theme detail preview with resolved colors
	if m.settingsThemeIndex < len(m.settingsThemes) {
		tc := m.settingsThemes[m.settingsThemeIndex]
		resolved := themeFromConfig(tc)
		b.WriteString("\n")

		detailStyle := lipgloss.NewStyle().Foreground(currentTheme.Notion)
		renderColor := func(label string, value lipgloss.Color) string {
			hex := string(value)
			return detailStyle.Render(label+": ") + lipgloss.NewStyle().Foreground(value).Render(hex)
		}

		b.WriteString(renderColor("accent", resolved.Accent))
		b.WriteString("  " + renderColor("accent_fg", resolved.AccentForeground))
		b.WriteString("  " + renderColor("foreground", resolved.Foreground))
		b.WriteString("\n")
		b.WriteString(renderColor("notion", resolved.Notion))
		b.WriteString("  " + renderColor("notion_fg", resolved.NotionForeground))
		b.WriteString("  " + renderColor("success", resolved.Success))
		b.WriteString("  " + renderColor("success_fg", resolved.SuccessForeground))
		b.WriteString("\n")
		b.WriteString(renderColor("warning", resolved.Warning))
		b.WriteString("  " + renderColor("warning_fg", resolved.WarningForeground))
		b.WriteString("  " + renderColor("error", resolved.Error))
		b.WriteString("  " + renderColor("error_fg", resolved.ErrorForeground))
		b.WriteString("\n")
		b.WriteString(renderColor("env_develop", resolved.EnvDevelop))
		b.WriteString("  " + renderColor("env_test", resolved.EnvTest))
		b.WriteString("\n")
		b.WriteString(renderColor("env_stage", resolved.EnvStage))
		b.WriteString("  " + renderColor("env_prod", resolved.EnvProd))
	}

	// Theme count
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d theme(s) available", len(m.settingsThemes))))

	// Save button (centered)
	b.WriteString("\n\n")
	contentWidth := modalWidth - 4
	if contentWidth < 30 {
		contentWidth = 30
	}
	buttonText := "Save and close"
	var btnStyle lipgloss.Style
	if m.settingsFocusIndex == 1 {
		btnStyle = buttonActiveStyle
	} else {
		btnStyle = buttonStyle
	}
	button := btnStyle.Render(buttonText)
	buttonWidth := lipgloss.Width(button)
	padding := (contentWidth - buttonWidth) / 2
	if padding > 0 {
		b.WriteString(strings.Repeat(" ", padding))
	}
	b.WriteString(button)

	return b.String()
}
