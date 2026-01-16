package main

import (
	"regexp"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Settings tabs
var settingsTabs = []string{"Release"}

// Number of focusable elements in Release tab
const settingsReleaseFieldCount = 2 // textarea, save button

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
		// Close without saving
		m.showSettings = false
		m.settingsExcludePatterns.Blur()
		m.settingsError = ""
		m.settingsFocusIndex = 0
		return m, nil

	case "tab":
		// Move to next field
		m.settingsFocusIndex = (m.settingsFocusIndex + 1) % settingsReleaseFieldCount
		return m.updateSettingsFocus()

	case "shift+tab":
		// Move to previous field
		m.settingsFocusIndex--
		if m.settingsFocusIndex < 0 {
			m.settingsFocusIndex = settingsReleaseFieldCount - 1
		}
		return m.updateSettingsFocus()

	case "enter":
		// If on save button, save and close
		if m.settingsFocusIndex == 1 {
			m.settingsError = m.validatePatterns()
			if m.settingsError == "" {
				m.saveSettings()
				m.showSettings = false
				m.settingsExcludePatterns.Blur()
				m.settingsFocusIndex = 0
			}
			return m, nil
		}
		// Otherwise pass to textarea (for newline)
	}

	// Pass key events to textarea if it's focused
	if m.settingsFocusIndex == 0 {
		var cmd tea.Cmd
		m.settingsExcludePatterns, cmd = m.settingsExcludePatterns.Update(msg)
		// Validate on each change
		m.settingsError = m.validatePatterns()
		return m, cmd
	}

	return m, nil
}

// updateSettingsFocus updates focus state based on settingsFocusIndex
func (m model) updateSettingsFocus() (tea.Model, tea.Cmd) {
	if m.settingsFocusIndex == 0 {
		return m, m.settingsExcludePatterns.Focus()
	}
	m.settingsExcludePatterns.Blur()
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

// saveSettings saves the current settings to config file
func (m *model) saveSettings() {
	config, err := LoadConfig()
	if err != nil {
		config = &AppConfig{}
	}
	config.ExcludePatterns = m.settingsExcludePatterns.Value()
	SaveConfig(config)
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
	}

	// Help footer text
	helpText := helpStyle.Render("tab/shift+tab: focus • esc/ctrl+q: close without saving")

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
		BorderForeground(lipgloss.Color("62")).
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

	// Setting title
	b.WriteString(settingsLabelStyle.Render("Files to exclude from release"))
	b.WriteString("\n")

	// Description (same color as MR list item description - 244)
	desc := "Enumerate file paths patterns to exclude from release, one per line. " +
		"/**/ - any folders, * - any char sequence.\n" +
		"e.g. garbage/ - any whole \"garbage\" folder, /garbage/ - whole \"garbage\" folder at project root, " +
		"/node_modules/**/*.js, /file-*.ts, .gitlab-ci.yml"
	b.WriteString(helpStyle.Render(desc))
	b.WriteString("\n\n")

	// Update textarea width to fit modal (accounting for padding only, border is outside Width)
	contentWidth := modalWidth - 4 // 4 horizontal padding (border is added outside)
	if contentWidth < 30 {
		contentWidth = 30
	}
	m.settingsExcludePatterns.SetWidth(contentWidth)

	// Fixed textarea height
	m.settingsExcludePatterns.SetHeight(10)

	// Textarea
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
	if m.settingsFocusIndex == 1 && m.settingsError == "" {
		// Focused and no errors
		btnStyle = buttonActiveStyle
	} else {
		// Normal or disabled (same unfocused style)
		btnStyle = buttonStyle
	}
	button := btnStyle.Render(buttonText)
	// Center the button
	buttonWidth := lipgloss.Width(button)
	padding := (contentWidth - buttonWidth) / 2
	if padding > 0 {
		b.WriteString(strings.Repeat(" ", padding))
	}
	b.WriteString(button)

	return b.String()
}
