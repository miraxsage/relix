package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
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
		logsHeight := contentHeight - 7 // tabs + padding
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
		vpHeight := contentHeight - 7
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.historyMRViewport = viewport.New(contentWidth-4, vpHeight)
		// Set initial content (placeholder message)
		m.updateHistoryMRViewport()
	}
}

// initHistoryMRFetch triggers fetching all MRs when entering detail screen
func (m *model) initHistoryMRFetch() tea.Cmd {
	if m.historySelected == nil || len(m.historySelected.MRBranches) == 0 {
		return nil
	}
	if m.historyMRIndex < 0 {
		m.historyMRIndex = 0
	}
	m.loadingHistoryMRs = true
	m.historyMRsLoadError = false
	return tea.Batch(m.spinner.Tick, m.fetchAllHistoryMRs())
}

// updateHistoryDetail handles key events on the history detail screen
func (m model) updateHistoryDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle open options modal first
	if m.showOpenOptionsModal {
		return m.updateOpenOptionsModal(msg)
	}

	switch msg.String() {
	case "ctrl+q", "esc":
		// Go back to history list
		m.screen = screenHistoryList
		m.historySelected = nil
		m.historyMRDetailsMap = make(map[int]*MergeRequestDetails)
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
		// Show options modal based on current tab
		if m.historySelected != nil {
			return m.handleOpenAction(buildHistoryOpenOptions(m.historySelected, m.historyMRIndex, m.historyDetailTab))
		}
		return m, nil
	case "r":
		// Reload MRs (if on MRs tab)
		if m.historyDetailTab == 0 && m.historySelected != nil {
			return m, m.initHistoryMRFetch()
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

	prevIndex := m.historyMRIndex

	switch msg.String() {
	case "up", "k":
		// Navigate MR list
		if m.historyMRIndex > 0 {
			m.historyMRIndex--
		}
	case "down", "j":
		// Navigate MR list
		if m.historyMRIndex < len(m.historySelected.MRBranches)-1 {
			m.historyMRIndex++
		}
	case "d":
		// Scroll viewport down
		m.historyMRViewport.HalfViewDown()
		return m, nil
	case "pgup":
		// Scroll viewport up
		m.historyMRViewport.HalfViewUp()
		return m, nil
	}

	// Update viewport if selection changed
	if m.historyMRIndex != prevIndex {
		m.updateHistoryMRViewport()
	}

	return m, nil
}

// fetchAllHistoryMRs fetches all MR details for the history entry
func (m *model) fetchAllHistoryMRs() tea.Cmd {
	if m.historySelected == nil || len(m.historySelected.MRBranches) == 0 {
		return nil
	}

	return func() tea.Msg {
		if m.creds == nil || m.selectedProject == nil {
			return fetchAllHistoryMRsMsg{err: fmt.Errorf("no credentials or project")}
		}

		client := NewGitLabClient(m.creds.GitLabURL, m.creds.Token)
		mrDetailsMap := make(map[int]*MergeRequestDetails)

		// Fetch all MRs - prefer using saved IIDs if available, fallback to branch search
		successCount := 0
		var lastError error

		// Check if we have saved IIDs (new format)
		hasIIDs := len(m.historySelected.MRIIDs) == len(m.historySelected.MRBranches)

		for i := range m.historySelected.MRBranches {
			var details *MergeRequestDetails
			var err error

			if hasIIDs {
				// Use saved IID for direct fetch (faster and more reliable)
				details, err = client.GetMergeRequestByIID(m.selectedProject.ID, m.historySelected.MRIIDs[i])
			} else {
				// Fallback to branch search for old history entries
				details, err = client.GetMergeRequestBySourceBranch(m.selectedProject.ID, m.historySelected.MRBranches[i])
			}

			if err == nil && details != nil {
				mrDetailsMap[i] = details
				successCount++
			} else if err != nil {
				lastError = err
			}
			// Continue even if some MRs fail to load
		}

		// Return error if all MRs failed to load (return the actual error, not a generic message)
		if successCount == 0 && len(m.historySelected.MRBranches) > 0 && lastError != nil {
			return fetchAllHistoryMRsMsg{err: lastError}
		}

		return fetchAllHistoryMRsMsg{mrDetailsMap: mrDetailsMap, err: nil}
	}
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
	// Account for: title border (2) + tabs line (1) + spacing (2) + main border (2) = 7 total
	var content string
	switch m.historyDetailTab {
	case 0:
		content = m.viewHistoryMRsTab(contentHeight - 8)
	case 1:
		content = m.viewHistoryMetaTab(contentHeight - 6)
	case 2:
		content = m.viewHistoryLogsTab(contentHeight - 6)
	}

	// Build title
	entry := m.historySelected
	envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(getEnvBranchColor(entry.Environment)))
	statusStyle := historyStatusCompletedStyle
	if entry.Status == "aborted" {
		statusStyle = historyStatusAbortedStyle
	}

	// Extract version number from tag (format: "{env}-{version}-v{number}")
	vNumber := ""
	if entry.Tag != "" {
		parts := strings.Split(entry.Tag, "-v")
		if len(parts) > 1 {
			vNumber = " v" + parts[len(parts)-1]
		}
	}

	// Prefix style (matching history list title)
	prefixStyle := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("231")).
		PaddingLeft(1).
		PaddingRight(1)

	titleText := prefixStyle.Render("Release") + " " +
		entry.Version + vNumber + " to " +
		envStyle.Render(entry.Environment) + " was " +
		statusStyle.Render(entry.Status) + " at " +
		entry.DateTime.Format("02.01.2006 15:04")

	// Wrap title with border (matching contentStyle border color)
	titleWithBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Render(titleText)

	// Compose (increase height by 2 to fill available space)
	main := contentStyle.
		Width(m.width - 2).
		Height(contentHeight).
		Render(titleWithBorder + "\n\n" + tabs + "\n\n" + content)

	// Help footer with empty line after
	helpText := "tab: next tab • j/k: nav • d/pgup: scroll • o: open • r: reload • C+q: back"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help, "")
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
	sidebarBuilder.WriteString(envTitleStyle.Render(mrCountTitle))
	sidebarBuilder.WriteString("\n\n")

	// Style for MR branches (matching release screen)
	mrBranchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	selectedMRBranchStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("105")).
		PaddingLeft(1)

	for i, branch := range branches {
		branchDisplay := truncateWithEllipsis(branch, sidebarW-6)
		if i == m.historyMRIndex {
			sidebarBuilder.WriteString(selectedMRBranchStyle.Render(branchDisplay))
		} else {
			sidebarBuilder.WriteString(mrBranchStyle.Render("  " + branchDisplay))
		}
		sidebarBuilder.WriteString("\n")
	}

	sidebar := sidebarStyle.
		Width(sidebarW).
		Height(height).
		Render(sidebarBuilder.String())

	// Build content area - show MR details viewport
	contentArea := lipgloss.NewStyle().
		Width(contentWidth).
		Height(height).
		PaddingLeft(2).
		PaddingTop(1).
		Render(m.historyMRViewport.View())

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
		{"Date", entry.DateTime.Format("02.01.2006 15:04")},
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

// updateHistoryMRViewport updates the history MR viewport with the fetched MR details
func (m *model) updateHistoryMRViewport() {
	if m.historyMRViewport.Width == 0 {
		return
	}

	content := m.renderHistoryMRMarkdown()
	m.historyMRViewport.SetContent(content)
	// Reset viewport position to top when switching between MRs
	m.historyMRViewport.GotoTop()
}

// renderHistoryMRMarkdown renders the markdown content for the selected history MR
func (m model) renderHistoryMRMarkdown() string {
	// Show loading state
	if m.loadingHistoryMRs {
		return m.spinner.View() + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Render("Loading MR details...")
	}

	// Show error state
	if m.historyMRsLoadError {
		return lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("189")).Render("Failed to load MR details.\nCheck your GitLab credentials and project access.")
	}

	// Get details from the map
	details := m.historyMRDetailsMap[m.historyMRIndex]

	// Show placeholder if MRs haven't been loaded yet
	if details == nil && len(m.historyMRDetailsMap) == 0 {
		var message strings.Builder
		tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Underline(true)

		// Show individual MR link if available
		if m.historySelected != nil && m.historyMRIndex >= 0 && m.historyMRIndex < len(m.historySelected.MRURLs) {
			mrURL := m.historySelected.MRURLs[m.historyMRIndex]
			if mrURL != "" {
				message.WriteString(linkStyle.Render(mrURL))
				message.WriteString("\n\n")
			}
		}

		tipText := "You can go to gitlab merge request or jira task by key \"o\"\nor press \"r\" to load detail information about this release mrs"
		message.WriteString(tipStyle.Render(tipText))

		return message.String()
	}

	if details == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render("No MR details available for this branch")
	}

	style := styles.DarkStyleConfig
	style.Document.StylePrimitive.Color = stringPtr("189")
	style.Strong.Color = stringPtr("220")
	style.H1.Prefix = " "
	style.H1.BackgroundColor = stringPtr("62")
	style.H1.Color = stringPtr("231")
	style.H2.Prefix = ""
	style.H3.Prefix = ""
	style.H3.Color = stringPtr("105")

	// Clean up author name (replace multiple spaces with single space)
	authorName := strings.Join(strings.Fields(details.Author.Name), " ")

	// Build info table row
	discussionInfo := fmt.Sprintf(
		"%d/%d",
		details.DiscussionsResolved,
		details.DiscussionsTotal)
	if details.DiscussionsTotal == 0 {
		discussionInfo = "0/0"
	}

	changesCount := details.ChangesCount
	if changesCount == "" {
		changesCount = "0"
	}

	// Build markdown content
	markdown := fmt.Sprintf(`# %s

### %s (@%s)
**%s** -> %s (at %s)

 | Overview | Commits | Changes |
 |:--------:|:-------:|:-------:|
 | %s | %d | %s |

 %s
 `,
		details.Title,
		authorName,
		details.Author.Username,
		details.SourceBranch,
		details.TargetBranch,
		details.CreatedAt.Format("02.01.2006 15:04"),
		discussionInfo,
		details.CommitsCount,
		changesCount,
		details.Description,
	)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(m.historyMRViewport.Width),
		glamour.WithPreservedNewLines(),
	)

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	return strings.Trim(rendered, "\n")
}
