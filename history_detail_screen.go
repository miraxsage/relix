package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var historyDetailTabs = []string{"MRs", "Meta", "Logs"}

// initHistoryDetailScreen initializes the history detail screen viewports
func (m *model) initHistoryDetailScreen() {
	if m.width == 0 || m.height == 0 || m.historySelected == nil {
		return
	}

	contentHeight := m.height - 4

	// Initialize logs viewport
	if m.historyDetailTab == 2 { // Logs tab
		logsWidth := m.width - 8
		logsHeight := contentHeight - 5 // tabs + padding
		if logsHeight < 1 {
			logsHeight = 1
		}
		m.historyLogsViewport = viewport.New(logsWidth, logsHeight)
		m.historyLogsViewport.SetContent(strings.Join(m.historySelected.TerminalOutput, "\n"))
	}

	// Initialize MRs viewport for MRs tab
	if m.historyDetailTab == 0 {
		sidebarW := sidebarWidth(m.width)
		contentWidth := m.width - sidebarW - 4
		vpHeight := contentHeight - 5
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.historyMRViewport = viewport.New(contentWidth-4, vpHeight)
		// Content set when selecting MR from list
		if m.historyMRIndex >= 0 && m.historyMRIndex < len(m.historySelected.MRBranches) {
			m.historyMRViewport.SetContent(
				lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Render(
					"Branch: "+m.historySelected.MRBranches[m.historyMRIndex],
				),
			)
		}
	}
}

// updateHistoryDetail handles key events on the history detail screen
func (m model) updateHistoryDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u", "q", "esc":
		// Go back to history list
		m.screen = screenHistoryList
		m.historySelected = nil
		return m, nil
	case "1":
		m.historyDetailTab = 0
		(&m).initHistoryDetailScreen()
		return m, nil
	case "2":
		m.historyDetailTab = 1
		(&m).initHistoryDetailScreen()
		return m, nil
	case "3":
		m.historyDetailTab = 2
		(&m).initHistoryDetailScreen()
		return m, nil
	case "tab":
		m.historyDetailTab = (m.historyDetailTab + 1) % len(historyDetailTabs)
		(&m).initHistoryDetailScreen()
		return m, nil
	case "shift+tab":
		m.historyDetailTab = (m.historyDetailTab + len(historyDetailTabs) - 1) % len(historyDetailTabs)
		(&m).initHistoryDetailScreen()
		return m, nil
	case "o":
		// Open MR URL in browser (if on MRs tab and URL exists)
		if m.historyDetailTab == 0 && m.historySelected != nil && m.historySelected.CreatedMRURL != "" {
			return m, openInBrowser(m.historySelected.CreatedMRURL)
		}
		return m, nil
	}

	// Handle tab-specific navigation
	switch m.historyDetailTab {
	case 0: // MRs tab - navigate MR list
		return m.updateHistoryMRsTab(msg)
	case 2: // Logs tab - scroll viewport
		var cmd tea.Cmd
		m.historyLogsViewport, cmd = m.historyLogsViewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateHistoryMRsTab handles navigation in the MRs tab
func (m model) updateHistoryMRsTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.historySelected == nil || len(m.historySelected.MRBranches) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.historyMRIndex > 0 {
			m.historyMRIndex--
		}
		return m, nil
	case "down", "j":
		if m.historyMRIndex < len(m.historySelected.MRBranches)-1 {
			m.historyMRIndex++
		}
		return m, nil
	}

	return m, nil
}

// viewHistoryDetail renders the history detail screen
func (m model) viewHistoryDetail() string {
	if m.width == 0 || m.height == 0 || m.historySelected == nil {
		return ""
	}

	contentHeight := m.height - 4

	// Render tabs
	var tabsBuilder strings.Builder
	for i, tab := range historyDetailTabs {
		if i == m.historyDetailTab {
			tabsBuilder.WriteString(historyTabActiveStyle.Render(tab))
		} else {
			tabsBuilder.WriteString(historyTabStyle.Render(tab))
		}
		if i < len(historyDetailTabs)-1 {
			tabsBuilder.WriteString(" ")
		}
	}
	tabs := tabsBuilder.String()

	// Render content based on tab
	var content string
	switch m.historyDetailTab {
	case 0:
		content = m.viewHistoryMRsTab(contentHeight - 4)
	case 1:
		content = m.viewHistoryMetaTab(contentHeight - 4)
	case 2:
		content = m.viewHistoryLogsTab(contentHeight - 4)
	}

	// Build title
	entry := m.historySelected
	envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(getEnvBranchColor(entry.Environment)))
	statusStyle := historyStatusCompletedStyle
	if entry.Status == "aborted" {
		statusStyle = historyStatusAbortedStyle
	}
	titleText := entry.Version + " " +
		envStyle.Render(entry.Environment) + " " +
		statusStyle.Render(entry.Status) + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render(entry.DateTime.Format("2006-01-02 15:04"))

	// Compose
	main := contentStyle.
		Width(m.width - 2).
		Height(contentHeight).
		Render(titleText + "\n\n" + tabs + "\n\n" + content)

	// Help footer
	helpText := "1/2/3: tabs • tab: next tab • j/k: nav • o: open • u: back"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// viewHistoryMRsTab renders the MRs tab with sidebar list
func (m model) viewHistoryMRsTab(height int) string {
	if m.historySelected == nil {
		return ""
	}

	branches := m.historySelected.MRBranches
	if len(branches) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render("No MRs in this release")
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 8

	// Build MR list sidebar
	var sidebarBuilder strings.Builder
	mrCountTitle := fmt.Sprintf(" MR Branches (%d) ", len(branches))
	sidebarBuilder.WriteString(envTitleStepStyle.Render("[1]") + envTitleStyle.Render(mrCountTitle))
	sidebarBuilder.WriteString("\n\n")

	for i, branch := range branches {
		branchDisplay := truncateWithEllipsis(branch, sidebarW-6)
		if i == m.historyMRIndex {
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("105")).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("105")).
				PaddingLeft(1)
			sidebarBuilder.WriteString(selectedStyle.Render(branchDisplay))
		} else {
			sidebarBuilder.WriteString(lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("189")).Render(branchDisplay))
		}
		sidebarBuilder.WriteString("\n")
	}

	sidebar := sidebarStyle.
		Width(sidebarW).
		Height(height).
		Render(sidebarBuilder.String())

	// Build content area - show selected branch info
	var contentBuilder strings.Builder
	if m.historyMRIndex >= 0 && m.historyMRIndex < len(branches) {
		branch := branches[m.historyMRIndex]
		contentBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("105")).Render("Branch"))
		contentBuilder.WriteString("\n")
		contentBuilder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(branch))

		if m.historySelected.CreatedMRURL != "" {
			contentBuilder.WriteString("\n\n")
			contentBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("105")).Render("Merge Request"))
			contentBuilder.WriteString("\n")
			contentBuilder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Render(m.historySelected.CreatedMRURL))
			contentBuilder.WriteString("\n")
			contentBuilder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render("Press 'o' to open in browser"))
		}
	}

	contentArea := lipgloss.NewStyle().
		Width(contentWidth).
		Height(height).
		PaddingLeft(2).
		Render(contentBuilder.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, contentArea)
}

// viewHistoryMetaTab renders the Meta tab
func (m model) viewHistoryMetaTab(height int) string {
	if m.historySelected == nil {
		return ""
	}

	entry := m.historySelected
	var sb strings.Builder

	rows := []struct {
		label string
		value string
	}{
		{"Date", entry.DateTime.Format("2006-01-02 15:04:05")},
		{"Version", entry.Version},
		{"Environment", entry.Environment},
		{"Tag", entry.Tag},
		{"Status", entry.Status},
		{"Root merge", fmt.Sprintf("%v", entry.RootMerge)},
		{"Release branch", entry.SourceBranch},
		{"Env branch", entry.EnvBranch},
		{"MR count", fmt.Sprintf("%d", entry.MRCount)},
	}

	if entry.CreatedMRURL != "" {
		rows = append(rows, struct {
			label string
			value string
		}{"MR URL", entry.CreatedMRURL})
	}

	for _, row := range rows {
		label := historyMetaLabelStyle.Width(20).Render(row.label)

		// Color the environment value
		valueStyle := historyMetaValueStyle
		if row.label == "Environment" {
			valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(getEnvBranchColor(row.value)))
		}
		if row.label == "Status" {
			if row.value == "completed" {
				valueStyle = historyStatusCompletedStyle
			} else {
				valueStyle = historyStatusAbortedStyle
			}
		}

		sb.WriteString(label + "  " + valueStyle.Render(row.value))
		sb.WriteString("\n")
	}

	return sb.String()
}

// viewHistoryLogsTab renders the Logs tab
func (m model) viewHistoryLogsTab(height int) string {
	if m.historySelected == nil {
		return ""
	}

	if len(m.historySelected.TerminalOutput) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render("No logs available")
	}

	return m.historyLogsViewport.View()
}
