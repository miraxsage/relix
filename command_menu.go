package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commandItem represents a command in the menu
type commandItem struct {
	name string
	desc string
}

// commands is the list of available commands
var commands = []commandItem{
	{name: "project", desc: "Select GitLab project to filter MRs"},
	{name: "settings", desc: "Configure application settings"},
	{name: "logout", desc: "Clear your current gitlab credentials to auth again"},
}

// updateCommandMenu handles key events when command menu is open
func (m model) updateCommandMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q", "q", "esc":
		m.showCommandMenu = false
		return m, nil

	case "up", "k":
		if m.commandMenuIndex > 0 {
			m.commandMenuIndex--
		}
		return m, nil

	case "down", "j":
		if m.commandMenuIndex < len(commands)-1 {
			m.commandMenuIndex++
		}
		return m, nil

	case "enter":
		return m.executeCommand(commands[m.commandMenuIndex].name)
	}

	return m, nil
}

// executeCommand executes the selected command
func (m model) executeCommand(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "project":
		m.closeAllModals()
		m.showProjectSelector = true
		m.projectSelectorIndex = 0
		m.projectFilter = ""

		// Load projects if not already loaded
		if !m.projectsLoaded {
			m.loadingProjects = true
			return m, tea.Batch(m.spinner.Tick, m.fetchProjects())
		}
		return m, nil

	case "settings":
		m.settingsPreviousScreen = m.screen
		m.closeAllModals()
		m.settingsTab = 0
		m.settingsFocusIndex = 0
		// Load current settings
		if config, err := LoadConfig(); err == nil {
			m.settingsExcludePatterns.SetValue(config.ExcludePatterns)
			pipelineRegex := config.PipelineJobsRegex
			if pipelineRegex == "" {
				pipelineRegex = defaultPipelineJobsRegex
			}
			m.settingsPipelineRegex.SetValue(pipelineRegex)
			// Load base branch
			baseBranch := config.BaseBranch
			if baseBranch == "" {
				baseBranch = "root"
			}
			m.settingsBaseBranch.SetValue(baseBranch)
			// Load environment settings
			envs := config.Environments
			if len(envs) == 0 {
				envs = defaultEnvironments()
			}
			for i := 0; i < 4 && i < len(envs); i++ {
				m.settingsEnvNames[i].SetValue(strings.ToUpper(envs[i].Name))
				m.settingsEnvBranches[i].SetValue(envs[i].BranchName)
			}
		}
		(&m).updateTextareaTheme()
		m.screen = screenSettings
		(&m).initSettingsViewport()
		return m, m.settingsBaseBranch.Focus()

	case "logout":
		m.closeAllModals()
		// Delete credentials from keyring
		DeleteCredentials()

		// Clear project from config
		SaveSelectedProject(nil)

		// Reset to auth screen
		m.screen = screenAuth
		m.inputs = initAuthInputs()
		m.focusIndex = 0
		m.creds = nil
		m.ready = false
		m.selectedProject = nil
		m.projects = nil
		m.projectsLoaded = false
		m.mrsLoaded = false

		return m, nil
	}

	return m, nil
}

// overlayCommandMenu renders the command menu as an overlay on top of the current view
func (m model) overlayCommandMenu(background string) string {
	// Build menu content
	var b strings.Builder

	b.WriteString(commandMenuTitleStyle.Render("Commands"))
	b.WriteString("\n")

	for i, cmd := range commands {
		var nameStyle lipgloss.Style
		prefix := "  "
		if i == m.commandMenuIndex {
			nameStyle = commandItemSelectedStyle
			prefix = "> "
		} else {
			nameStyle = commandItemStyle
		}

		b.WriteString(nameStyle.Render(fmt.Sprintf("%s%s", prefix, cmd.name)))
		b.WriteString("\n")
		b.WriteString(commandDescStyle.Render("    " + cmd.desc))
		b.WriteString("\n")
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: nav • enter: select • C+q: close"))

	config := ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: true},
		MinWidth: 30,
		MaxWidth: 70,
		Style:    commandMenuStyle,
	}
	menuContent := renderModal(b.String(), config, m.width)

	// Overlay menu on top of background (centered)
	return placeOverlayCenter(menuContent, background, m.width, m.height)
}

// overlayErrorModal renders the error modal as an overlay
func (m model) overlayErrorModal(background string) string {
	var b strings.Builder

	b.WriteString(errorTitleStyle.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(m.errorModalMsg)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("C+q: close"))

	modalContent := renderModal(b.String(), ErrorModalConfig(), m.width)

	return placeOverlayCenter(modalContent, background, m.width, m.height)
}
