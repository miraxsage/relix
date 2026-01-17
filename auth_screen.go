package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewLoading renders the initial loading screen
func (m model) viewLoading() string {
	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("231"))
	content := loadingStyle.Render(m.spinner.View() + " Loading...")

	// Center vertically and horizontally
	centered := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	return centered
}

// initAuthInputs creates the text inputs for the auth form
func initAuthInputs() []textinput.Model {
	inputs := make([]textinput.Model, 3)

	// GitLab URL input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "https://gitlab.com"
	inputs[0].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	inputs[0].Focus()
	inputs[0].CharLimit = 256
	inputs[0].Width = 40
	inputs[0].Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))

	// Email input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "user@example.com"
	inputs[1].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	inputs[1].CharLimit = 256
	inputs[1].Width = 40
	inputs[1].Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))

	// Token input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "glpat-... (requires api scope)"
	inputs[2].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	inputs[2].CharLimit = 256
	inputs[2].Width = 40
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))

	return inputs
}

// checkStoredCredentials checks if credentials exist in keyring
func checkStoredCredentials() tea.Cmd {
	return func() tea.Msg {
		creds, err := LoadCredentials()
		if err != nil {
			return checkCredsMsg{creds: nil}
		}
		return checkCredsMsg{creds: creds}
	}
}

// validateCredentialsCmd validates credentials against GitLab API
func validateCredentialsCmd(creds Credentials) tea.Cmd {
	return func() tea.Msg {
		if err := ValidateCredentials(creds); err != nil {
			return authResultMsg{err: err}
		}

		// Save credentials on successful validation
		if err := SaveCredentials(creds); err != nil {
			return authResultMsg{err: err}
		}

		return authResultMsg{err: nil}
	}
}

// updateAuth handles key events on the auth screen
func (m model) updateAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.focusIndex++
		if m.focusIndex > len(m.inputs) {
			m.focusIndex = 0
		}
		return m.updateFocus(), nil

	case "shift+tab", "up":
		m.focusIndex--
		if m.focusIndex < 0 {
			m.focusIndex = len(m.inputs)
		}
		return m.updateFocus(), nil

	case "enter":
		if m.focusIndex == len(m.inputs) {
			// Submit button focused
			creds := Credentials{
				GitLabURL: strings.TrimSpace(m.inputs[0].Value()),
				Email:     strings.TrimSpace(m.inputs[1].Value()),
				Token:     strings.TrimSpace(m.inputs[2].Value()),
			}

			// Basic validation
			if creds.GitLabURL == "" || creds.Email == "" || creds.Token == "" {
				m.errorMsg = "All fields are required"
				m.screen = screenError
				return m, nil
			}

			m.loading = true
			return m, tea.Batch(m.spinner.Tick, validateCredentialsCmd(creds))
		}
		// Move to next field on enter
		m.focusIndex++
		if m.focusIndex > len(m.inputs) {
			m.focusIndex = 0
		}
		return m.updateFocus(), nil
	}

	// For all other keys (character input), update the focused text input
	return m.updateInputs(msg)
}

// updateFocus updates which input has focus
func (m model) updateFocus() model {
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return m
}

// updateInputs updates all text inputs
func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

// viewAuth renders the auth screen
func (m model) viewAuth() string {
	// Always build regular form content first to get exact dimensions
	var formBuilder strings.Builder

	// Form title
	formBuilder.WriteString(formTitleStyle.Render("GitLab Authentication"))
	formBuilder.WriteString("\n")

	// Input fields
	labels := []string{"GitLab URL", "Email", "Personal Access Token"}
	for i, input := range m.inputs {
		formBuilder.WriteString(inputLabelStyle.Render(labels[i]))
		formBuilder.WriteString("\n")
		formBuilder.WriteString(input.View())
		formBuilder.WriteString("\n\n")
	}

	// Submit button
	var submitStyle lipgloss.Style
	if m.focusIndex == len(m.inputs) {
		submitStyle = buttonActiveStyle
	} else {
		submitStyle = buttonStyle
	}

	submitButton := lipgloss.NewStyle().
		Width(40).
		Align(lipgloss.Center).
		Render(submitStyle.Render("Submit"))
	formBuilder.WriteString(submitButton)

	// Get the regular form box to measure exact dimensions
	regularFormContent := formStyle.Render(formBuilder.String())
	formBoxWidth := lipgloss.Width(regularFormContent)
	formBoxHeight := lipgloss.Height(regularFormContent)

	var formContent string

	// Show loading inside form box with exact same dimensions
	if m.loading {
		loadingText := m.spinner.View() + " Loading..."

		// Create loading content centered in exact same box size
		// Account for formStyle padding (1, 2) and border
		innerWidth := formBoxWidth - 2 - 4   // subtract border (2) and horizontal padding (2*2)
		innerHeight := formBoxHeight - 2 - 2 // subtract border (2) and vertical padding (1*2)

		loadingLine := lipgloss.NewStyle().
			Width(innerWidth).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("231")).
			Render(loadingText)

		// Build content with title and vertical centering for loading
		var b strings.Builder
		b.WriteString(formTitleStyle.Render("GitLab Authentication"))
		b.WriteString("\n")

		// Adjust for title height (title + newline = 2 lines)
		contentHeight := innerHeight - 2
		topPad := (contentHeight - 1) / 2
		bottomPad := contentHeight - 1 - topPad

		if topPad > 0 {
			b.WriteString(strings.Repeat("\n", topPad))
		}
		b.WriteString(loadingLine)
		if bottomPad > 0 {
			b.WriteString(strings.Repeat("\n", bottomPad))
		}

		formContent = formStyle.
			Width(formBoxWidth - 2).   // subtract border width
			Height(formBoxHeight - 2). // subtract border height
			Render(b.String())
	} else {
		formContent = regularFormContent
	}

	// Center the form horizontally
	formWidth := lipgloss.Width(formContent)
	horizontalPadding := max(0, (m.width-formWidth)/2)

	centeredForm := lipgloss.NewStyle().
		PaddingLeft(horizontalPadding).
		Render(formContent)

	// Help footer (centered) - hide during loading
	var help string
	if !m.loading {
		helpText := "tab/↓/↑: nav • enter: submit/next • C+c: quit"
		help = helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)
	}

	// Calculate heights
	formHeight := lipgloss.Height(centeredForm)
	helpHeight := lipgloss.Height(help)

	// Create spacer to push footer to bottom
	spacerHeight := max(0, m.height-formHeight-helpHeight)
	topPadding := spacerHeight / 2
	bottomPadding := spacerHeight - topPadding

	topSpacer := strings.Repeat("\n", topPadding)
	bottomSpacer := strings.Repeat("\n", bottomPadding)

	return topSpacer + centeredForm + bottomSpacer + help
}
