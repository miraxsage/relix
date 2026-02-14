package main

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OpenOption represents a URL option in the open modal
type OpenOption struct {
	Label string
	URL   string
}

// jiraTaskRegex extracts task numbers from branch names
var jiraTaskRegex = regexp.MustCompile(`RUSSPASS-(\d+)`)

// extractJiraTaskNumber extracts task number from branch name (e.g. "RUSSPASS-1234")
func extractJiraTaskNumber(branchName string) string {
	if matches := jiraTaskRegex.FindStringSubmatch(branchName); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// buildJiraURL builds Jira URL from task number
func buildJiraURL(taskNumber string) string {
	return fmt.Sprintf("https://itpm.mos.ru/browse/RUSSPASS-%s", taskNumber)
}

// buildMROpenOptions builds options for MRs list context
func buildMROpenOptions(mr *MergeRequestDetails) []OpenOption {
	options := []OpenOption{
		{Label: "GitLab MR", URL: mr.WebURL},
	}

	if taskNum := extractJiraTaskNumber(mr.SourceBranch); taskNum != "" {
		options = append(options, OpenOption{
			Label: fmt.Sprintf("Jira Task (RUSSPASS-%s)", taskNum),
			URL:   buildJiraURL(taskNum),
		})
	}

	return options
}

// buildHistoryOpenOptions builds options for history detail context
// tab: 0=MRs, 1=Meta, 2=Logs
func buildHistoryOpenOptions(entry *ReleaseHistoryEntry, mrIndex int, tab int) []OpenOption {
	options := []OpenOption{}

	if tab == 0 {
		// MRs tab: show selected individual MR and Jira task
		if mrIndex >= 0 && mrIndex < len(entry.MRURLs) && entry.MRURLs[mrIndex] != "" {
			options = append(options, OpenOption{Label: "GitLab MR", URL: entry.MRURLs[mrIndex]})
		}

		// Add Jira task URL (if available)
		if mrIndex >= 0 && mrIndex < len(entry.MRBranches) {
			branchName := entry.MRBranches[mrIndex]
			if taskNum := extractJiraTaskNumber(branchName); taskNum != "" {
				options = append(options, OpenOption{
					Label: fmt.Sprintf("Jira Task (RUSSPASS-%s)", taskNum),
					URL:   buildJiraURL(taskNum),
				})
			}
		}
	} else {
		// Meta/Logs tabs: show environment release MR
		if entry.CreatedMRURL != "" {
			options = append(options, OpenOption{Label: "GitLab MR", URL: entry.CreatedMRURL})
		}
	}

	return options
}

// buildReleaseOpenOptions builds options for release screen context
func buildReleaseOpenOptions(state *ReleaseState, pipelineStatus *PipelineStatus) []OpenOption {
	options := []OpenOption{}

	if state.CreatedMRURL != "" {
		options = append(options, OpenOption{Label: "GitLab MR", URL: state.CreatedMRURL})
	}

	if pipelineStatus != nil && pipelineStatus.PipelineWebURL != "" {
		options = append(options, OpenOption{Label: "Pipeline", URL: pipelineStatus.PipelineWebURL})
	}

	return options
}

// handleOpenAction handles opening URLs, showing modal if multiple options or opening directly if one
func (m model) handleOpenAction(options []OpenOption) (tea.Model, tea.Cmd) {
	if len(options) == 0 {
		return m, nil
	}
	if len(options) == 1 {
		// Only one option - open directly
		return m, openInBrowser(options[0].URL)
	}
	// Multiple options - show modal
	m.openOptions = options
	m.showOpenOptionsModal = true
	m.openOptionsIndex = 0
	return m, nil
}

// overlayOpenOptionsModal renders the open options modal
func (m model) overlayOpenOptionsModal(background string) string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Accent).Render("Open")
	sb.WriteString(title)
	sb.WriteString("\n\n")

	for i, option := range m.openOptions {
		var line string
		if i == m.openOptionsIndex {
			line = commandItemSelectedStyle.Render("▸ " + option.Label)
		} else {
			line = commandItemStyle.Render("  " + option.Label)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("j/k: nav • enter: open • C+q: close"))

	config := ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: false},
		MinWidth: 40,
		MaxWidth: 60,
		Style:    commandMenuStyle,
	}

	modalContent := renderModal(sb.String(), config, m.width)
	return placeOverlayCenter(modalContent, background, m.width, m.height)
}

// executeOpenOption opens the selected option URL
func (m model) executeOpenOption() (tea.Model, tea.Cmd) {
	if m.openOptionsIndex >= 0 && m.openOptionsIndex < len(m.openOptions) {
		url := m.openOptions[m.openOptionsIndex].URL
		m.closeOpenOptionsModal()
		return m, openInBrowser(url)
	}
	m.closeOpenOptionsModal()
	return m, nil
}

// updateOpenOptionsModal handles key events for the open options modal
func (m model) updateOpenOptionsModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q", "q", "esc":
		m.closeOpenOptionsModal()
		return m, nil
	case "down", "j":
		if m.openOptionsIndex < len(m.openOptions)-1 {
			m.openOptionsIndex++
		}
		return m, nil
	case "up", "k":
		if m.openOptionsIndex > 0 {
			m.openOptionsIndex--
		}
		return m, nil
	case "enter":
		return m.executeOpenOption()
	}
	return m, nil
}
