package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// initSourceBranchInput initializes the source branch text input with default value
func (m *model) initSourceBranchInput() {
	ti := textinput.New()
	ti.Placeholder = "e.g. release/rpb-1.0.0-root"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))

	// Set default value based on version
	version := m.versionInput.Value()
	if version != "" {
		ti.SetValue("release/rpb-" + version + "-root")
	}

	m.sourceBranchInput = ti
	m.sourceBranchError = ""
}

// validateSourceBranch checks if the version is present in the branch name
func (m model) validateSourceBranch(branchName string) bool {
	if branchName == "" {
		return true // Empty is valid (no error shown yet)
	}
	version := m.versionInput.Value()
	if version == "" {
		return true // No version to validate against
	}
	return strings.Contains(branchName, version)
}

// updateSourceBranch handles key events on the source branch input screen
func (m model) updateSourceBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u":
		// Go back to version input
		m.screen = screenVersion
		m.sourceBranchError = ""
		return m, nil
	case "enter":
		// Validate and proceed
		branchName := m.sourceBranchInput.Value()
		if branchName == "" {
			m.sourceBranchError = "Source branch name is required"
			return m, nil
		}
		if !m.validateSourceBranch(branchName) {
			m.sourceBranchError = "Branch name must contain version: " + m.versionInput.Value()
			return m, nil
		}
		// Branch name is valid - proceed to root merge screen
		m.sourceBranchError = ""
		m.screen = screenRootMerge
		// Preserve previous selection: 0 = Yes (rootMergeSelection=true), 1 = No (rootMergeSelection=false)
		if m.rootMergeSelection {
			m.rootMergeButtonIndex = 0
		} else {
			m.rootMergeButtonIndex = 1
		}
		return m, nil
	}

	// Handle text input updates
	var cmd tea.Cmd
	m.sourceBranchInput, cmd = m.sourceBranchInput.Update(msg)

	// Clear error when user types valid input
	if m.sourceBranchError != "" && m.validateSourceBranch(m.sourceBranchInput.Value()) {
		m.sourceBranchError = ""
	} else if !m.validateSourceBranch(m.sourceBranchInput.Value()) && m.sourceBranchInput.Value() != "" {
		m.sourceBranchError = "Branch name must contain version: " + m.versionInput.Value()
	}

	return m, cmd
}

// viewSourceBranch renders the source branch input screen
func (m model) viewSourceBranch() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Content height (same as other screens)
	contentHeight := m.height - 4

	// Total rendered height for sidebar/content (content height + 2 for border)
	totalHeight := contentHeight + 2

	// Build triple sidebar (same as confirmation step)
	sidebar := m.renderTripleSidebar(sidebarW, totalHeight)

	// Build content - source branch input
	contentContent := m.renderSourceBranchInput(contentWidth - 4)

	// Render content with border
	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "enter: confirm • u: go back • /: commands • Ctrl+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderSourceBranchInput renders the source branch input content area
func (m model) renderSourceBranchInput(width int) string {
	var sb strings.Builder

	// Step title
	sb.WriteString(envTitleStepStyle.Render("[4]") + envTitleStyle.Render(" Source branch "))
	sb.WriteString("\n\n")

	// Prompt
	prompt := "Specify source branch where we will accumulate releasing commits."
	sb.WriteString(envPromptStyle.Render(prompt))
	sb.WriteString("\n")

	// Sub-prompt
	subPrompt := "If the branch does not exist locally and remotely it will be created from root branch."
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render(subPrompt))
	sb.WriteString("\n\n")

	// Source branch input field
	sb.WriteString(versionInputStyle.Render("Source branch: "))
	sb.WriteString(m.sourceBranchInput.View())

	// Error message if any (uses same style as error modal)
	if m.sourceBranchError != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorTitleStyle.Render(m.sourceBranchError))
	}

	return sb.String()
}
