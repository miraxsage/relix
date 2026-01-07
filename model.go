package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// model is the main application model
type model struct {
	screen screen
	width  int
	height int

	// Auth form
	inputs     []textinput.Model
	focusIndex int
	loading    bool
	spinner    spinner.Model

	// Error
	errorMsg string

	// Main screen
	list     list.Model
	viewport viewport.Model
	ready    bool
	creds    *Credentials

	// Command menu
	showCommandMenu  bool
	commandMenuIndex int

	// Error modal
	showErrorModal bool
	errorModalMsg  string

	// Project selector
	showProjectSelector  bool
	projects             []Project
	projectSelectorIndex int
	projectFilter        string
	selectedProject      *Project
}

// NewModel creates a new application model
func NewModel() model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot

	return model{
		screen:     screenAuth,
		inputs:     initAuthInputs(),
		focusIndex: 0,
		spinner:    s,
	}
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, checkStoredCredentials())
}

// Update handles all messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle error modal if open
		if m.showErrorModal {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter", "esc":
				m.showErrorModal = false
				m.errorModalMsg = ""
			}
			return m, nil
		}

		// Handle project selector if open
		if m.showProjectSelector {
			return m.updateProjectSelector(msg)
		}

		// Handle command menu if open
		if m.showCommandMenu {
			return m.updateCommandMenu(msg)
		}

		// Open command menu with "/" (except on auth screen)
		if msg.String() == "/" && m.screen != screenAuth {
			m.showCommandMenu = true
			m.commandMenuIndex = 0
			return m, nil
		}

		switch m.screen {
		case screenAuth:
			return m.updateAuth(msg)
		case screenError:
			return m.updateError(msg)
		case screenMain:
			return m.updateList(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if m.screen == screenMain {
			m.updateListSize()
		}

	case checkCredsMsg:
		if msg.creds != nil {
			m.creds = msg.creds
			m.screen = screenMain
			m.initListScreen()
			m.updateListSize()

			// Load saved project from config
			if config, err := LoadConfig(); err == nil && config.SelectedProjectID != 0 {
				m.selectedProject = &Project{
					ID:                config.SelectedProjectID,
					Name:              config.SelectedProjectShortName,
					PathWithNamespace: config.SelectedProjectPath,
					NameWithNamespace: config.SelectedProjectName,
				}
			}

			// Fetch projects first, MRs will be fetched after project is confirmed
			return m, m.fetchProjects()
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case authResultMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			m.screen = screenError
		} else {
			// Load credentials from keyring after successful auth
			creds, err := LoadCredentials()
			if err != nil {
				m.errorMsg = "Failed to load credentials: " + err.Error()
				m.screen = screenError
				return m, nil
			}
			m.creds = creds

			m.screen = screenMain
			m.initListScreen()
			m.updateListSize()

			// Load saved project from config
			if config, err := LoadConfig(); err == nil && config.SelectedProjectID != 0 {
				m.selectedProject = &Project{
					ID:                config.SelectedProjectID,
					Name:              config.SelectedProjectShortName,
					PathWithNamespace: config.SelectedProjectPath,
					NameWithNamespace: config.SelectedProjectName,
				}
			}

			// Fetch projects first, MRs will be fetched after project is confirmed
			return m, m.fetchProjects()
		}

	case fetchProjectsMsg:
		if msg.err != nil {
			m.showErrorModal = true
			m.errorModalMsg = "Failed to fetch projects: " + msg.err.Error()
		} else {
			m.projects = msg.projects

			// If no project selected, show project selector
			if m.selectedProject == nil {
				m.showProjectSelector = true
				m.projectSelectorIndex = 0
				m.projectFilter = ""
			} else {
				// Project already selected, fetch MRs
				m.list.Title = m.selectedProject.Name + " (loading...)"
				return m, m.fetchMRs()
			}
		}

	case fetchMRsMsg:
		if msg.err != nil {
			m.showErrorModal = true
			m.errorModalMsg = msg.err.Error()
			m.list.Title = "Open MRs"
		} else {
			items := make([]list.Item, len(msg.mrs))
			for i, mr := range msg.mrs {
				items[i] = mrListItem{mr: mr}
			}
			m.list.SetItems(items)

			// Build title with project name if selected
			if m.selectedProject != nil {
				m.list.Title = fmt.Sprintf("%s (%d)", m.selectedProject.Name, len(msg.mrs))
			} else {
				m.list.Title = fmt.Sprintf("All MRs (%d)", len(msg.mrs))
			}

			if m.ready {
				m.viewport.SetContent(m.renderMarkdown())
			}
		}
	}

	// Update inputs if on auth screen (for non-KeyMsg messages like Blink)
	if m.screen == screenAuth {
		var cmd tea.Cmd
		var updatedModel tea.Model
		updatedModel, cmd = m.updateInputs(msg)
		m = updatedModel.(model)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the current screen
func (m model) View() string {
	var view string
	switch m.screen {
	case screenAuth:
		view = m.viewAuth()
	case screenError:
		view = m.viewError()
	case screenMain:
		view = m.viewList()
	}

	// Overlay command menu if open
	if m.showCommandMenu {
		view = m.overlayCommandMenu(view)
	}

	// Overlay project selector if open
	if m.showProjectSelector {
		view = m.overlayProjectSelector(view)
	}

	// Overlay error modal if open
	if m.showErrorModal {
		view = m.overlayErrorModal(view)
	}

	return view
}
