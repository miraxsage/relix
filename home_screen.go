package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// RELIX FIGlet ANSI Shadow font
const relixASCII = `
██████╗ ███████╗██╗     ██╗██╗  ██╗
██╔══██╗██╔════╝██║     ██║╚██╗██╔╝
██████╔╝█████╗  ██║     ██║ ╚███╔╝
██╔══██╗██╔══╝  ██║     ██║ ██╔██╗
██║  ██║███████╗███████╗██║██╔╝ ██╗
╚═╝  ╚═╝╚══════╝╚══════╝╚═╝╚═╝  ╚═╝`

// hasInProgressRelease checks if there is an uncompleted release
func hasInProgressRelease() bool {
	state, err := LoadReleaseState()
	return err == nil && state != nil
}

// updateHome handles key events on the home screen
func (m model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Go to release (MR list screen)
		// Initialize list if not ready
		if !m.ready {
			(&m).initListScreen()
			(&m).updateListSize()
		}
		m.screen = screenMain
		if m.selectedProject == nil {
			// No project selected - show project selector
			m.showProjectSelector = true
			m.loadingProjects = true
			return m, tea.Batch(m.spinner.Tick, m.fetchProjects())
		}
		if !m.mrsLoaded {
			m.loadingMRs = true
			return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
		}
		return m, nil
	case "h":
		// Go to releases history
		m.screen = screenHistoryList
		m.loadingHistory = true
		m.initHistoryListScreen()
		return m, tea.Batch(m.spinner.Tick, m.fetchHistory())
	case "s":
		// Open settings modal
		m.showSettings = true
		m.settingsTab = 0
		if config, err := LoadConfig(); err == nil {
			m.settingsExcludePatterns.SetValue(config.ExcludePatterns)
		}
		return m, m.settingsExcludePatterns.Focus()
	}

	return m, nil
}

// viewHome renders the home screen
func (m model) viewHome() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Render ASCII title
	title := homeTitleStyle.Render(relixASCII)
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// Menu items
	releaseLabel := "Release"
	if hasInProgressRelease() {
		releaseLabel = "Continue release"
	}

	menuItems := []struct {
		key   string
		label string
	}{
		{"r", releaseLabel},
		{"h", "Releases history"},
		{"s", "Settings"},
	}

	// Get the width of the ASCII title to use as reference
	titleWidth := 0
	for _, line := range strings.Split(relixASCII, "\n") {
		if w := ansi.StringWidth(line); w > titleWidth {
			titleWidth = w
		}
	}

	// Build menu lines (left-aligned text)
	var menuLines []string
	for _, item := range menuItems {
		menuLines = append(menuLines, homeMenuKeyStyle.Render("["+item.key+"]")+" "+homeMenuItemStyle.Render(item.label))
	}

	// Find the widest menu line
	menuWidth := 0
	for _, line := range menuLines {
		if w := ansi.StringWidth(line); w > menuWidth {
			menuWidth = w
		}
	}

	// Center the left-aligned menu block within the title width
	menuBlock := lipgloss.NewStyle().
		Width(titleWidth).
		Align(lipgloss.Center).
		Render(
			lipgloss.NewStyle().
				Width(menuWidth).
				Align(lipgloss.Left).
				Render(strings.Join(menuLines, "\n")),
		)
	sb.WriteString(menuBlock)

	// Version centered within title width
	sb.WriteString("\n\n")
	version := lipgloss.NewStyle().
		Width(titleWidth).
		Align(lipgloss.Center).
		Render(homeVersionStyle.Render("v" + AppVersion))
	sb.WriteString(version)

	// Center the whole block on screen
	content := sb.String()
	contentBlock := contentStyle.
		Width(m.width - 2).
		Height(m.height - 4).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	// Help footer
	helpText := "/: commands • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, contentBlock, help)
}
