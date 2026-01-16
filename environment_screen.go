package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for environment selection
var (
	envTitleStepStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("236")).
				Background(lipgloss.Color("220"))

	envTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("62"))

	envItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("189")).
			PaddingLeft(2)

	envPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("189"))

	envHintBaseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("189"))

	mrBranchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("189"))
)

// getEnvBranchColor returns the foreground color for branch name based on environment
func getEnvBranchColor(envName string) string {
	switch envName {
	case "DEVELOP":
		return "105"
	case "TEST":
		return "220"
	case "STAGE":
		return "36"
	case "PROD":
		return "210"
	default:
		return "231"
	}
}

// getEnvHintStyle returns the style for environment name in hint based on env name
func getEnvHintStyle(envName string) lipgloss.Style {
	switch envName {
	case "DEVELOP":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("62"))
	case "TEST":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Background(lipgloss.Color("220"))
	case "STAGE":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("29"))
	case "PROD":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("196"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("231"))
	}
}

// getEnvBranchStyle returns the style for branch name in hint
func getEnvBranchStyle(envName string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(getEnvBranchColor(envName)))
}

// getEnvSelectedStyle returns the style for selected environment item
func getEnvSelectedStyle(envName string) lipgloss.Style {
	color := getEnvBranchColor(envName)
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(color)).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(color)).
		PaddingLeft(1)
}

// updateEnvSelect handles key events on the environment selection screen
func (m model) updateEnvSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u":
		// Save current selection and go back to MR list
		m.selectedEnv = &m.environments[m.envSelectIndex]
		m.screen = screenMain
		return m, nil
	case "up", "k":
		if m.envSelectIndex > 0 {
			m.envSelectIndex--
		}
		return m, nil
	case "down", "j":
		if m.envSelectIndex < len(m.environments)-1 {
			m.envSelectIndex++
		}
		return m, nil
	case "enter":
		// Save selected environment and proceed to version input
		m.selectedEnv = &m.environments[m.envSelectIndex]
		// Only initialize version input if not already done
		if m.versionInput.CharLimit == 0 {
			m.versionInput = initVersionInput()
		}
		m.versionError = ""
		m.screen = screenVersion
		return m, nil
	}

	return m, nil
}

// viewEnvSelect renders the environment selection screen
func (m model) viewEnvSelect() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Build sidebar content - list of selected MRs
	sidebarContent := m.renderSelectedMRsSidebar(sidebarW - 4)

	// Build content - environment selection
	contentContent := m.renderEnvSelection(contentWidth - 4)

	// Render sidebar
	sidebar := sidebarStyle.
		Width(sidebarW).
		Height(m.height - 4).
		Render(sidebarContent)

	// Render content
	content := contentStyle.
		Width(contentWidth).
		Height(m.height - 4).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "j/k: nav • enter: select • u: go back • /: commands • Ctrl+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderSelectedMRsSidebar renders the sidebar with selected MRs list
func (m model) renderSelectedMRsSidebar(width int) string {
	var sb strings.Builder

	// Count selected MRs
	selectedCount := len(m.selectedMRs)

	title := fmt.Sprintf(" MRs to release (%d) ", selectedCount)
	sb.WriteString(envTitleStepStyle.Render("[1]") +
		envTitleStyle.Render(title))
	sb.WriteString("\n\n")

	// List selected MR branch names
	items := m.list.Items()
	for _, item := range items {
		if mr, ok := item.(mrListItem); ok {
			if m.selectedMRs[mr.MR().IID] {
				branchName := truncateWithEllipsis(mr.MR().SourceBranch, width-2)
				sb.WriteString(mrBranchStyle.Render(branchName))
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// renderEnvSelection renders the environment selection content
func (m model) renderEnvSelection(width int) string {
	var sb strings.Builder

	// Prompt with step number
	prompt := "Select environment to release selected MRs to:"
	sb.WriteString(envTitleStepStyle.Render("[2]") + " " + envPromptStyle.Render(prompt))
	sb.WriteString("\n\n")

	// Environment list
	for i, env := range m.environments {
		if i == m.envSelectIndex {
			sb.WriteString(getEnvSelectedStyle(env.Name).Render(env.Name))
		} else {
			sb.WriteString(envItemStyle.Render(env.Name))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Hint with styled parts
	selectedEnv := m.environments[m.envSelectIndex]
	branchPart := fmt.Sprintf("release/rpb-<version>-%s", selectedEnv.BranchName)
	hint := envHintBaseStyle.Render("Release branch ") +
		getEnvBranchStyle(selectedEnv.Name).Render(branchPart) +
		envHintBaseStyle.Render(" -> ") +
		getEnvHintStyle(selectedEnv.Name).Render(" "+selectedEnv.Name+" ")
	sb.WriteString(hint)

	return sb.String()
}
