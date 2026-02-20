package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Settings tabs
var settingsTabs = []string{"Release", "Theme"}

// Number of focusable elements per tab
// 0=base branch, 1=env1 name, 2=env1 branch, 3=env2 name, 4=env2 branch,
// 5=env3 name, 6=env3 branch, 7=env4 name, 8=env4 branch, 9=textarea, 10=pipeline regex, 11=save button
const settingsReleaseFieldCount = 12
const settingsThemeFieldCount = 2 // theme list, save button

// Default regex matching Package/Deploy jobs for known apps and environments
const defaultPipelineJobsRegex = `(?i)(Package|Deploy) Application (Main|Admin|JudgePersonal|Touch) (for |to )?(dev|test|stage|prod)01`

// Invalid characters for file patterns (excluding glob special chars)
var (
	invalidPatternChars = regexp.MustCompile(`[<>:"|?\\]`)
	tooWidePattern      = regexp.MustCompile(`^((/?\*\*?/?)|(/))$`)
	invalidPatternOrder = regexp.MustCompile(`(^/?\*/\*\*)|(^/?\*\*/\*)|(\*/\*\*/?$)|(\*\*/\*/?$)|(/\*\*([^/]|$))|(([^/]|^)\*\*/)|(([^/]|^)\*\*([^/]|$))`)
)

// updateSettings handles key events on the settings screen
func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+q":
		// Close without saving; revert any unsaved theme preview
		loadThemeFromConfig()
		(&m).updateTextareaTheme()
		m.screen = m.settingsPreviousScreen
		m.settingsBaseBranch.Blur()
		for i := 0; i < 4; i++ {
			m.settingsEnvNames[i].Blur()
			m.settingsEnvBranches[i].Blur()
		}
		m.settingsExcludePatterns.Blur()
		m.settingsPipelineRegex.Blur()
		m.settingsError = ""
		m.settingsFocusIndex = 0
		return m, nil

	case "H":
		// Switch to previous tab (keep unsaved changes including theme preview)
		if m.settingsTab > 0 {
			m.settingsTab--
			m.settingsFocusIndex = 0
			ret, cmd := m.updateSettingsFocus()
			mm := ret.(model)
			(&mm).initSettingsViewport()
			return mm, cmd
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
			m.settingsPipelineRegex.Blur()
			m.settingsTab++
			m.settingsFocusIndex = 0
			if m.settingsTab == 1 {
				m.loadSettingsThemes()
			}
			(&m).initSettingsViewport()
			return m, nil
		}
		return m, nil
	}

	// Delegate to tab-specific handler
	var ret tea.Model
	var cmd tea.Cmd
	switch m.settingsTab {
	case 0:
		ret, cmd = m.updateSettingsRelease(msg)
	case 1:
		ret, cmd = m.updateSettingsTheme(msg)
	default:
		return m, nil
	}

	// Refresh viewport after any tab-specific update
	mm := ret.(model)
	(&mm).refreshSettingsViewport()
	return mm, cmd
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
		// 0=base → 1=env1 name; 1→3→5→7→9; 2→4→6→8→9; 9→break; 10→11
		if m.settingsFocusIndex == 9 {
			// textarea: move focus to next field if cursor is on last line
			if m.settingsExcludePatterns.Line() >= m.settingsExcludePatterns.LineCount()-1 {
				m.settingsFocusIndex = 10
				return m.updateSettingsFocus()
			}
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
		case m.settingsFocusIndex == 10: // pipeline regex → save button
			m.settingsFocusIndex = 11
		case m.settingsFocusIndex == 11:
			return m, nil
		}
		return m.updateSettingsFocus()

	case "up":
		// Jump to same column in previous row
		if m.settingsFocusIndex == 9 {
			// textarea: move focus to previous field if cursor is on first line
			if m.settingsExcludePatterns.Line() == 0 {
				m.settingsFocusIndex = 8
				return m.updateSettingsFocus()
			}
			break
		}
		switch {
		case m.settingsFocusIndex == 0:
			return m, nil
		case m.settingsFocusIndex == 1: // first env name → base branch
			m.settingsFocusIndex = 0
		case m.settingsFocusIndex == 2: // first env branch → base branch
			m.settingsFocusIndex = 0
		case m.settingsFocusIndex == 10: // pipeline regex → textarea
			m.settingsFocusIndex = 9
		case m.settingsFocusIndex == 11: // save → pipeline regex
			m.settingsFocusIndex = 10
		case m.settingsFocusIndex%2 == 1: // env name → prev env name
			m.settingsFocusIndex -= 2
		case m.settingsFocusIndex%2 == 0: // env branch → prev env branch
			m.settingsFocusIndex -= 2
		}
		return m.updateSettingsFocus()

	case "enter":
		// If on save button (index 11), validate and save
		if m.settingsFocusIndex == 11 {
			m.settingsError = m.validateReleaseSettings()
			if m.settingsError == "" {
				m.saveAllSettings()
				m.screen = m.settingsPreviousScreen
				m.settingsExcludePatterns.Blur()
				m.settingsPipelineRegex.Blur()
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
		if m.settingsFocusIndex == 1 {
			// Save button → back to theme list
			m.settingsFocusIndex = 0
		} else if m.settingsFocusIndex == 0 && m.settingsThemeIndex > 0 {
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
		} else if m.settingsFocusIndex == 0 && m.settingsThemeIndex >= len(m.settingsThemes)-1 {
			// Past last theme → focus save button
			m.settingsFocusIndex = 1
		}
		return m, nil

	case "enter":
		// Save button: save all settings and close
		if m.settingsFocusIndex == 1 {
			m.settingsError = m.validatePatterns()
			if m.settingsError == "" {
				m.saveAllSettings()
				m.screen = m.settingsPreviousScreen
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
	m.settingsExcludePatterns.BlurredStyle.CursorLine = lipgloss.NewStyle().Foreground(currentTheme.Foreground)
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

	// Update textinput styles for all settings inputs
	updateInputTheme := func(ti *textinput.Model) {
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(currentTheme.Notion)
		ti.PromptStyle = lipgloss.NewStyle().Foreground(currentTheme.Accent)
		ti.TextStyle = lipgloss.NewStyle().Foreground(currentTheme.Foreground)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	}
	updateInputTheme(&m.settingsBaseBranch)
	updateInputTheme(&m.settingsPipelineRegex)
	for i := 0; i < 4; i++ {
		updateInputTheme(&m.settingsEnvNames[i])
		updateInputTheme(&m.settingsEnvBranches[i])
	}
}

// updateSettingsReleaseInput routes key events to the currently focused input
func (m model) updateSettingsReleaseInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Jump between env name ↔ env branch on the same row via left/right at boundaries
	if msg.String() == "right" {
		if m.settingsFocusIndex%2 == 1 && m.settingsFocusIndex >= 1 && m.settingsFocusIndex <= 7 {
			idx := (m.settingsFocusIndex - 1) / 2
			if m.settingsEnvNames[idx].Position() >= len(m.settingsEnvNames[idx].Value()) {
				m.settingsFocusIndex++
				m.settingsEnvBranches[idx].SetCursor(0)
				return m.updateSettingsFocus()
			}
		}
	}
	if msg.String() == "left" {
		if m.settingsFocusIndex%2 == 0 && m.settingsFocusIndex >= 2 && m.settingsFocusIndex <= 8 {
			idx := (m.settingsFocusIndex - 2) / 2
			if m.settingsEnvBranches[idx].Position() == 0 {
				m.settingsFocusIndex--
				m.settingsEnvNames[idx].SetCursor(len(m.settingsEnvNames[idx].Value()))
				return m.updateSettingsFocus()
			}
		}
	}

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
	case 10: // Pipeline regex
		m.settingsPipelineRegex, cmd = m.settingsPipelineRegex.Update(msg)
		// Live regex validation
		regexStr := strings.TrimSpace(m.settingsPipelineRegex.Value())
		if regexStr != "" {
			if _, err := regexp.Compile(regexStr); err != nil {
				m.settingsError = "Pipeline regex: " + err.Error()
			} else {
				m.settingsError = ""
			}
		} else {
			m.settingsError = ""
		}
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
	m.settingsPipelineRegex.Blur()

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
	case 10:
		return m, m.settingsPipelineRegex.Focus()
	}
	// Index 11 = save button, nothing to focus
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
	if err := m.validatePatterns(); err != "" {
		return err
	}

	// Validate pipeline jobs regex
	regexStr := strings.TrimSpace(m.settingsPipelineRegex.Value())
	if regexStr != "" {
		if _, err := regexp.Compile(regexStr); err != nil {
			return "Pipeline regex: " + err.Error()
		}
	}

	return ""
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
	// Release tab: pipeline jobs regex
	config.PipelineJobsRegex = strings.TrimSpace(m.settingsPipelineRegex.Value())
	// Theme tab: selected theme
	if m.settingsThemeIndex < len(m.settingsThemes) {
		config.SelectedTheme = m.settingsThemes[m.settingsThemeIndex].Name
	}
	SaveConfig(config)

	// Rebuild runtime environments from saved config
	m.environments = getEnvironments()
}

// settingsContentWidth returns the usable content width inside the settings screen
func (m model) settingsContentWidth() int {
	// contentStyle has border (2) + padding (1+1) = 4 horizontal
	w := m.width - 2 - 4
	if w < 30 {
		w = 30
	}
	return w
}

// initSettingsViewport creates/resets the viewport for the current tab content
func (m *model) initSettingsViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	contentHeight := m.height - 4 // same as history detail
	// Overhead: title(3) + blank(1) + tabs(1) + blank(1) = 6
	vpHeight := contentHeight - 6
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := m.settingsContentWidth()
	m.settingsViewport = viewport.New(vpWidth, vpHeight)
	m.refreshSettingsViewport()
}

// updateSettingsSize resizes the viewport on WindowSizeMsg
func (m *model) updateSettingsSize() {
	m.initSettingsViewport()
}

// refreshSettingsViewport re-renders the current tab content into the viewport
func (m *model) refreshSettingsViewport() {
	// Update dynamic widths on the actual model (pointer receiver)
	// so they persist and aren't lost in value-receiver render methods
	contentWidth := m.settingsContentWidth()
	m.settingsBaseBranch.Width = contentWidth - 2
	m.settingsPipelineRegex.Width = contentWidth - 2
	// Recompute visible text range for the new Width (textinput only
	// recalculates offset/offsetRight during SetValue or SetCursor)
	m.settingsBaseBranch.SetCursor(m.settingsBaseBranch.Position())
	m.settingsPipelineRegex.SetCursor(m.settingsPipelineRegex.Position())

	switch m.settingsTab {
	case 0:
		result := m.renderReleaseSettings()
		m.settingsViewport.SetContent(result.content)
		m.scrollSettingsToFocus(result.fieldLines)
	case 1:
		content := m.renderThemeSettings()
		m.settingsViewport.SetContent(content)
	}
}

// scrollSettingsToFocus scrolls the viewport the minimum distance needed
// to make the focused field fully visible, with a small margin.
func (m *model) scrollSettingsToFocus(fieldLines map[int][2]int) {
	pos, ok := fieldLines[m.settingsFocusIndex]
	if !ok {
		return
	}
	fieldTop := pos[0]
	fieldBottom := pos[1]
	vpHeight := m.settingsViewport.Height
	top := m.settingsViewport.YOffset
	bottom := top + vpHeight
	topMargin := 2
	bottomMargin := 1

	// Save button (last field): ensure at least 1 empty line visible below it
	if m.settingsFocusIndex == settingsReleaseFieldCount-1 {
		offset := fieldBottom + 2 - vpHeight
		if offset < 0 {
			offset = 0
		}
		if offset > top {
			m.settingsViewport.SetYOffset(offset)
		}
		return
	}

	// If the field fits in the viewport, ensure the whole range is visible
	fieldHeight := fieldBottom - fieldTop + 1
	if fieldHeight <= vpHeight-topMargin-bottomMargin {
		if fieldTop < top+topMargin {
			offset := fieldTop - topMargin
			if offset < 0 {
				offset = 0
			}
			m.settingsViewport.SetYOffset(offset)
		} else if fieldBottom >= bottom-bottomMargin {
			offset := fieldBottom - vpHeight + bottomMargin + 1
			if offset < 0 {
				offset = 0
			}
			m.settingsViewport.SetYOffset(offset)
		}
	} else {
		// Field taller than viewport: show the top of the field
		offset := fieldTop - topMargin
		if offset < 0 {
			offset = 0
		}
		m.settingsViewport.SetYOffset(offset)
	}
}

// viewSettings renders the settings as a full screen (matching history detail pattern)
func (m model) viewSettings() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentHeight := m.height - 4

	// Title with border
	prefixStyle := lipgloss.NewStyle().
		Bold(true).
		Background(currentTheme.Accent).
		Foreground(currentTheme.AccentForeground).
		PaddingLeft(1).
		PaddingRight(1)
	titleText := prefixStyle.Render("Settings")
	titleWithBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		Padding(0, 1).
		Render(titleText)

	// Tabs
	var tabsBuilder strings.Builder
	for i, tab := range settingsTabs {
		if i == m.settingsTab {
			tabsBuilder.WriteString(settingsTabActiveStyle.Render(tab))
		} else {
			tabsBuilder.WriteString(settingsTabStyle.Render(tab))
		}
		if i < len(settingsTabs)-1 {
			tabsBuilder.WriteString(" ")
		}
	}
	tabs := tabsBuilder.String()

	// Compose main content area
	main := contentStyle.
		Width(m.width - 2).
		Height(contentHeight).
		Render(titleWithBorder + "\n\n" + tabs + "\n\n" + m.settingsViewport.View())

	// Help footer
	var helpText string
	if m.settingsTab == 1 {
		helpText = "j/k: nav • tab: focus • enter: save • H/L: switch tab • esc/C+q: back"
	} else {
		helpText = "tab: focus • enter: save • H/L: switch tab • esc/C+q: back"
	}
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help, "")
}

// releaseSettingsResult holds rendered content and field line positions
type releaseSettingsResult struct {
	content    string
	fieldLines map[int][2]int // focusIndex → [startLine, endLine] in content
}

// renderReleaseSettings renders the Release tab content and tracks
// the line offset for each focusable field (used for auto-scroll).
func (m model) renderReleaseSettings() releaseSettingsResult {
	var b strings.Builder
	fl := make(map[int][2]int)
	line := 0

	// write appends s to the builder and tracks newline count
	write := func(s string) {
		b.WriteString(s)
		line += strings.Count(s, "\n")
	}

	contentWidth := m.settingsContentWidth()

	// --- Base branch ---
	write(settingsLabelStyle.Render("Base branch"))
	write("\n")
	write(helpStyle.Render("Branch from which release branches are created and merged back to."))
	write("\n")

	fl[0] = [2]int{line, line} // base branch input
	write(m.settingsBaseBranch.View())
	write("\n\n")

	// --- Environment branches ---
	write(settingsLabelStyle.Render("Environment branches"))
	write("\n")
	write(helpStyle.Render("Customize environment names and their git branch mappings."))
	write("\n")

	envColors := []lipgloss.Color{currentTheme.EnvDevelop, currentTheme.EnvTest, currentTheme.EnvStage, currentTheme.EnvProd}
	arrowStyle := lipgloss.NewStyle().Foreground(currentTheme.Notion)

	for i := 0; i < 4; i++ {
		dot := lipgloss.NewStyle().Foreground(envColors[i]).Render("●")
		m.settingsEnvNames[i].Width = 10
		m.settingsEnvBranches[i].Width = 20
		fl[i*2+1] = [2]int{line, line} // env name
		fl[i*2+2] = [2]int{line, line} // env branch (same row)
		write(fmt.Sprintf("%s  %s  %s  %s",
			dot,
			m.settingsEnvNames[i].View(),
			arrowStyle.Render("→"),
			m.settingsEnvBranches[i].View(),
		))
		write("\n")
	}

	// --- Exclude patterns ---
	write("\n")
	write(settingsLabelStyle.Render("Files to exclude from release"))
	write("\n")
	desc := "Enumerate file paths patterns to exclude from release, one per line. " +
		"/**/ - any folders, * - any char sequence."
	write(helpStyle.Render(desc))
	write("\n")

	textareaStart := line
	m.settingsExcludePatterns.SetWidth(contentWidth)
	m.settingsExcludePatterns.SetHeight(6)
	write(m.settingsExcludePatterns.View())
	fl[9] = [2]int{textareaStart, line} // textarea (multi-line)

	// --- Pipeline jobs regex ---
	write("\n\n")
	write(settingsLabelStyle.Render("Observable pipeline jobs regex"))
	write("\n")
	desc2 := "Define regex for pipeline job names to observe for completion. Leave empty to track all jobs. " +
		"Uses Go regexp syntax: (?i) for case-insensitive, (?m) for multiline, etc."
	write(helpStyle.Width(contentWidth).Render(desc2))
	write("\n")

	fl[10] = [2]int{line, line} // pipeline regex input
	write(m.settingsPipelineRegex.View())

	// Error hint
	if m.settingsError != "" {
		write("\n")
		write(settingsErrorStyle.Render(m.settingsError))
	}

	// Save button (centered)
	write("\n\n")
	fl[11] = [2]int{line, line} // save button
	buttonText := "Save and close"
	var btnStyle lipgloss.Style
	if m.settingsFocusIndex == 11 && m.settingsError == "" {
		btnStyle = buttonActiveStyle
	} else {
		btnStyle = buttonStyle
	}
	button := btnStyle.Render(buttonText)
	buttonWidth := lipgloss.Width(button)
	pad := (contentWidth - buttonWidth) / 2
	if pad > 0 {
		write(strings.Repeat(" ", pad))
	}
	write(button)
	write("\n") // trailing line so viewport can scroll past the button

	return releaseSettingsResult{content: b.String(), fieldLines: fl}
}

// renderThemeSettings renders the Theme tab content
func (m model) renderThemeSettings() string {
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

	// Build theme entries as styled strings
	var themeEntries []string
	for i, tc := range m.settingsThemes {
		isSelected := i == m.settingsThemeIndex
		isActive := tc.Name == activeThemeName

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
			themeEntries = append(themeEntries, prefix+dots+" "+nameStyled)
		} else {
			nameStyled := lipgloss.NewStyle().Foreground(currentTheme.Foreground).Render(name)
			themeEntries = append(themeEntries, "  "+dots+" "+nameStyled)
		}
	}

	// Build color entries for selected theme
	var colorEntries []string
	if m.settingsThemeIndex < len(m.settingsThemes) {
		tc := m.settingsThemes[m.settingsThemeIndex]
		resolved := themeFromConfig(tc)

		detailStyle := lipgloss.NewStyle().Foreground(currentTheme.Notion)
		renderColor := func(label string, value lipgloss.Color) string {
			hex := string(value)
			colorStyle := lipgloss.NewStyle().Foreground(value)
			if value != currentTheme.Muted {
				colorStyle = colorStyle.Background(currentTheme.Muted)
			}
			return detailStyle.Render(label+": ") + colorStyle.Render(" "+hex+" ")
		}

		var bgEntry string
		if resolved.HasBackground {
			bgEntry = renderColor("background", resolved.Background)
		} else {
			bgEntry = detailStyle.Render("background: ") + helpStyle.Render("transparent")
		}

		colorEntries = []string{
			bgEntry,
			renderColor("accent", resolved.Accent),
			renderColor("accent_fg", resolved.AccentForeground),
			renderColor("foreground", resolved.Foreground),
			renderColor("notion", resolved.Notion),
			renderColor("notion_fg", resolved.NotionForeground),
			renderColor("muted", resolved.Muted),
			renderColor("muted_fg", resolved.MutedForeground),
			renderColor("success", resolved.Success),
			renderColor("success_fg", resolved.SuccessForeground),
			renderColor("warning", resolved.Warning),
			renderColor("warning_fg", resolved.WarningForeground),
			renderColor("error", resolved.Error),
			renderColor("error_fg", resolved.ErrorForeground),
			renderColor("env_develop", resolved.EnvDevelop),
			renderColor("env_test", resolved.EnvTest),
			renderColor("env_stage", resolved.EnvStage),
			renderColor("env_prod", resolved.EnvProd),
		}
	}

	// Compute shared target row count so both lists have equal height.
	// Fixed overhead: header(3) + blank between lists(1) + blank(1) + count(1)
	//               + blank(2) + button(1) + trailing(1) = 10
	// Available for both lists = vpHeight - 10, split equally.
	fixedOverhead := 10
	availablePerList := (m.settingsViewport.Height - fixedOverhead) / 2
	if availablePerList < 1 {
		availablePerList = 1
	}

	// Only wrap if at least one list exceeds available rows
	larger := len(themeEntries)
	if len(colorEntries) > larger {
		larger = len(colorEntries)
	}
	targetRows := larger // default: single column
	if larger > availablePerList {
		targetRows = availablePerList
	}
	if targetRows < 1 {
		targetRows = 1
	}

	// Render theme list
	b.WriteString(renderColumnList(themeEntries, targetRows))

	// Render color list
	if len(colorEntries) > 0 {
		b.WriteString("\n")
		b.WriteString(renderColumnList(colorEntries, targetRows))
	}

	// Theme count
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d theme(s) available", len(m.settingsThemes))))

	// Save button (centered)
	b.WriteString("\n\n")
	contentWidth := m.settingsContentWidth()
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

// renderColumnList renders entries in an adaptive multi-column layout.
// If entries fit in targetRows as a single column, they are listed one per line.
// Otherwise they wrap into multiple columns (filled top-to-bottom) with a vertical divider.
func renderColumnList(entries []string, targetRows int) string {
	if len(entries) == 0 {
		return ""
	}
	if targetRows < 1 {
		targetRows = 1
	}

	cols := (len(entries) + targetRows - 1) / targetRows
	if cols < 1 {
		cols = 1
	}
	rows := (len(entries) + cols - 1) / cols

	var b strings.Builder

	if cols == 1 {
		for _, entry := range entries {
			b.WriteString(entry + "\n")
		}
		return b.String()
	}

	// Compute max visual width per column for alignment
	colWidths := make([]int, cols)
	for col := 0; col < cols; col++ {
		for row := 0; row < rows; row++ {
			idx := col*rows + row
			if idx < len(entries) {
				if w := lipgloss.Width(entries[idx]); w > colWidths[col] {
					colWidths[col] = w
				}
			}
		}
	}

	divider := lipgloss.NewStyle().Foreground(currentTheme.Notion).Render(" │ ")

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			idx := col*rows + row
			if col > 0 {
				b.WriteString(divider)
			}
			if idx < len(entries) {
				b.WriteString(lipgloss.NewStyle().Width(colWidths[col]).Render(entries[idx]))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
