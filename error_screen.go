package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// updateError handles key events on the error screen
func (m model) updateError(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.screen = screenAuth
		m.errorMsg = ""
		return m, nil
	}
	return m, nil
}

// viewError renders the error screen
func (m model) viewError() string {
	var b strings.Builder

	b.WriteString(errorTitleStyle.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(m.errorMsg)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter: back to form • C+c: quit"))

	// Use width-restricted modal
	errorContent := renderModal(b.String(), ErrorModalConfig(), m.width)

	// Center the error box horizontally
	errorWidth := lipgloss.Width(errorContent)
	horizontalPadding := max(0, (m.width-errorWidth)/2)

	centeredError := lipgloss.NewStyle().
		PaddingLeft(horizontalPadding).
		Render(errorContent)

	// Center vertically
	errorHeight := lipgloss.Height(centeredError)
	topPadding := max(0, (m.height-errorHeight)/2)
	bottomPadding := max(0, m.height-errorHeight-topPadding)

	topSpacer := strings.Repeat("\n", topPadding)
	bottomSpacer := strings.Repeat("\n", bottomPadding)

	return topSpacer + centeredError + bottomSpacer
}
