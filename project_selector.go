package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	projectItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	projectItemSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("170"))

	projectItemActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	projectItemActiveSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("42"))

	projectFilterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))

	projectSelectorStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)
)

// updateProjectSelector handles key events for the project selector
func (m model) updateProjectSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Only allow closing if a project is already selected
		if m.selectedProject != nil {
			m.showProjectSelector = false
			m.projectFilter = ""
		}
		return m, nil

	case "up", "k":
		if m.projectSelectorIndex > 0 {
			m.projectSelectorIndex--
		}
		return m, nil

	case "down", "j":
		filtered := m.getFilteredProjects()
		if m.projectSelectorIndex < len(filtered)-1 {
			m.projectSelectorIndex++
		}
		return m, nil

	case "enter":
		filtered := m.getFilteredProjects()
		if len(filtered) > 0 && m.projectSelectorIndex < len(filtered) {
			selected := filtered[m.projectSelectorIndex]
			m.selectedProject = &selected
			m.showProjectSelector = false
			m.projectFilter = ""

			// Save to config
			SaveSelectedProject(&selected)

			// Refresh MRs for the new project
			m.list.Title = "Open MRs (loading...)"
			return m, m.fetchMRs()
		}
		return m, nil

	case "backspace":
		if len(m.projectFilter) > 0 {
			m.projectFilter = m.projectFilter[:len(m.projectFilter)-1]
			m.projectSelectorIndex = 0
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	default:
		// Add character to filter if it's printable
		if len(msg.String()) == 1 {
			char := msg.String()[0]
			if char >= 32 && char < 127 {
				m.projectFilter += msg.String()
				m.projectSelectorIndex = 0
			}
		}
		return m, nil
	}
}

// getFilteredProjects returns projects matching the current filter
func (m model) getFilteredProjects() []Project {
	if m.projectFilter == "" {
		return m.projects
	}

	filter := strings.ToLower(m.projectFilter)
	var filtered []Project
	for _, p := range m.projects {
		name := strings.ToLower(p.NameWithNamespace)
		path := strings.ToLower(p.PathWithNamespace)
		if strings.Contains(name, filter) || strings.Contains(path, filter) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// overlayProjectSelector renders the project selector modal
func (m model) overlayProjectSelector(background string) string {
	var b strings.Builder

	if m.selectedProject == nil {
		b.WriteString(commandMenuTitleStyle.Render("Select Project (required)"))
	} else {
		b.WriteString(commandMenuTitleStyle.Render("Select Project"))
	}
	b.WriteString("\n")

	// Show filter input
	filterDisplay := m.projectFilter
	if filterDisplay == "" {
		filterDisplay = "type to filter..."
	}
	b.WriteString(projectFilterStyle.Render("> " + filterDisplay))
	b.WriteString("\n\n")

	// Show filtered projects
	filtered := m.getFilteredProjects()
	maxVisible := 10
	startIdx := 0

	// Adjust scroll position
	if m.projectSelectorIndex >= maxVisible {
		startIdx = m.projectSelectorIndex - maxVisible + 1
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}

	if len(filtered) == 0 {
		b.WriteString(helpStyle.Render("No projects match filter"))
		b.WriteString("\n")
	} else {
		for i := startIdx; i < endIdx; i++ {
			p := filtered[i]
			isSelected := i == m.projectSelectorIndex
			isActive := m.selectedProject != nil && m.selectedProject.ID == p.ID

			var style lipgloss.Style
			prefix := "  "

			if isSelected && isActive {
				style = projectItemActiveSelectedStyle
				prefix = "> "
			} else if isSelected {
				style = projectItemSelectedStyle
				prefix = "> "
			} else if isActive {
				style = projectItemActiveStyle
			} else {
				style = projectItemStyle
			}

			line := prefix + p.NameWithNamespace
			if isActive {
				line += " (current)"
			}
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}

		// Show scroll indicator
		if len(filtered) > maxVisible {
			b.WriteString(helpStyle.Render(
				fmt.Sprintf("  (%d/%d)", m.projectSelectorIndex+1, len(filtered))))
			b.WriteString("\n")
		}
	}

	// Help footer
	b.WriteString("\n")
	if m.selectedProject == nil {
		b.WriteString(helpStyle.Render("↑/↓: navigate • enter: select (required)"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓: navigate • enter: select • esc: close"))
	}

	config := ModalConfig{
		Width:    ModalWidth{Value: 70, Percent: true},
		MinWidth: 50,
		MaxWidth: 90,
		Style:    projectSelectorStyle,
	}

	modalContent := renderModal(b.String(), config, m.width)
	return placeOverlayCenter(modalContent, background, m.width, m.height)
}

// fetchProjects creates a command to fetch projects from GitLab
func (m *model) fetchProjects() tea.Cmd {
	return func() tea.Msg {
		if m.creds == nil {
			return fetchProjectsMsg{err: nil}
		}

		client := NewGitLabClient(m.creds.GitLabURL, m.creds.Token)
		projects, err := client.GetProjects()
		return fetchProjectsMsg{projects: projects, err: err}
	}
}
