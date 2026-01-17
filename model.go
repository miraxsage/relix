package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// model is the main application model
type model struct {
	screen  screen
	width   int
	height  int
	program *tea.Program // Reference for sending async messages

	// Auth form
	inputs     []textinput.Model
	focusIndex int
	loading    bool
	spinner    spinner.Model

	// Error
	errorMsg string

	// Main screen
	list        list.Model
	viewport    viewport.Model
	ready       bool
	creds       *Credentials
	selectedMRs map[int]bool // Track selected MRs by IID
	loadingMRs  bool         // Loading modal for MRs
	mrsLoaded   bool         // True after first MR load completes

	// Environment selection screen
	environments   []Environment
	envSelectIndex int

	// Version input screen
	versionInput textinput.Model
	selectedEnv  *Environment
	versionError string

	// Source branch input screen
	sourceBranchInput         textinput.Model
	sourceBranchError         string
	sourceBranchVersion       string    // Version used when source branch was last modified
	sourceBranchRemoteStatus  string    // "exists-same", "exists-diff", "new", "checking", ""
	sourceBranchLastCheckTime time.Time // For throttling checks
	sourceBranchCheckedName   string    // Branch name that was last checked

	// Root merge screen
	rootMergeButtonIndex int  // 0 = Yes, 1 = No
	rootMergeSelection   bool // true = merge, false = skip

	// Confirmation screen
	confirmViewport viewport.Model

	// Command menu
	showCommandMenu  bool
	commandMenuIndex int

	// Error modal
	showErrorModal bool
	errorModalMsg  string

	// Project selector
	showProjectSelector  bool
	projects             []Project
	projectsLoaded       bool // True after projects are fetched
	loadingProjects      bool // Loading state for project selector
	projectSelectorIndex int
	projectFilter        string
	selectedProject      *Project

	// Settings modal
	showSettings            bool
	settingsTab             int // Current tab index (0 = Release)
	settingsExcludePatterns textarea.Model
	settingsError           string // Validation error message
	settingsFocusIndex      int    // 0 = textarea, 1 = save button

	// Release execution screen
	releaseState                     *ReleaseState
	releaseViewport                  viewport.Model
	releaseOutputBuffer              []string
	releaseCurrentScreen             string // Virtual terminal screen content
	releaseButtonIndex               int
	releaseButtons                   []ReleaseButton
	releaseRunning                   bool
	releaseExecutor                  *GitExecutor
	showAbortConfirm                 bool
	abortConfirmIndex                int  // 0 = Yes, 1 = Cancel
	showDeleteRemoteConfirm          bool // Second confirmation for deleting remote branch
	deleteRemoteConfirmIndex         int  // 0 = Yes, 1 = No
	releaseNeedEmptyLineAfterCommand bool // Flag to add empty line after command output if needed
}

// NewModel creates a new application model
func NewModel() model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("231"))

	// Initialize settings textarea
	ta := textarea.New()
	ta.Placeholder = "Enter file paths patterns to exclude..."
	ta.ShowLineNumbers = true
	ta.SetHeight(6)
	ta.SetWidth(50)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	return model{
		screen:                  screenLoading,
		inputs:                  initAuthInputs(),
		focusIndex:              0,
		spinner:                 s,
		loading:                 true, // Initial loading state
		settingsExcludePatterns: ta,
		environments: []Environment{
			{Name: "DEVELOP", BranchName: "develop"},
			{Name: "TEST", BranchName: "testing"},
			{Name: "STAGE", BranchName: "stable"},
			{Name: "PROD", BranchName: "master"},
		},
		selectedMRs:          make(map[int]bool),
		rootMergeSelection:   true, // Default to "Yes, merge it"
		rootMergeButtonIndex: 0,    // Default button is "Yes, merge it"
	}
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		checkStoredCredentials(),
	)
}

// closeAllModals closes all open modals
func (m *model) closeAllModals() {
	m.showCommandMenu = false
	m.showProjectSelector = false
	m.showErrorModal = false
	m.errorModalMsg = ""
	m.showSettings = false
	m.settingsError = ""
	m.settingsExcludePatterns.Blur()
}

// Update handles all messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Block all input during loading states
		if m.loading || m.loadingProjects || m.loadingMRs {
			return m, nil
		}

		// Handle error modal if open
		if m.showErrorModal {
			switch msg.String() {
			case "enter", "esc":
				m.showErrorModal = false
				m.errorModalMsg = ""
			}
			return m, nil
		}

		// Handle settings modal if open
		if m.showSettings {
			return m.updateSettings(msg)
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
			m.closeAllModals()
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
		case screenEnvSelect:
			return m.updateEnvSelect(msg)
		case screenVersion:
			return m.updateVersion(msg)
		case screenSourceBranch:
			return m.updateSourceBranch(msg)
		case screenRootMerge:
			return m.updateRootMerge(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenRelease:
			return m.updateRelease(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if m.screen == screenMain {
			m.updateListSize()
		}
		if m.screen == screenConfirm {
			m.initConfirmViewport()
		}
		if m.screen == screenRelease {
			m.initReleaseScreen()
			if m.releaseExecutor != nil {
				m.releaseExecutor.Resize(uint16(m.height-10), uint16(m.width-sidebarWidth(m.width)-10))
			}
		}

	case checkCredsMsg:
		m.loading = false
		if msg.creds != nil {
			m.creds = msg.creds

			// Check for existing release state first
			if releaseState, err := LoadReleaseState(); err == nil && releaseState != nil {
				// Resume existing release
				m.initListScreen()
				m.updateListSize()
				// Load project info for release
				if config, err := LoadConfig(); err == nil && config.SelectedProjectID != 0 {
					m.selectedProject = &Project{
						ID:                config.SelectedProjectID,
						Name:              config.SelectedProjectShortName,
						PathWithNamespace: config.SelectedProjectPath,
						NameWithNamespace: config.SelectedProjectName,
					}
				}
				return m, m.resumeRelease(releaseState)
			}

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
				// Project saved - fetch MRs directly, skip project loading
				m.loadingMRs = true
				return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
			}

			// No project saved - show project selector and load projects
			m.showProjectSelector = true
			m.loadingProjects = true
			return m, tea.Batch(m.spinner.Tick, m.fetchProjects())
		}
		// No credentials - show auth screen
		m.screen = screenAuth

	case spinner.TickMsg:
		if m.loading || m.loadingProjects || m.loadingMRs || m.releaseRunning || m.sourceBranchRemoteStatus == "checking" {
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
				// Project saved - fetch MRs directly, skip project loading
				m.loadingMRs = true
				return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
			}

			// No project saved - show project selector and load projects
			m.showProjectSelector = true
			m.loadingProjects = true
			return m, tea.Batch(m.spinner.Tick, m.fetchProjects())
		}

	case fetchProjectsMsg:
		m.loadingProjects = false
		m.projectsLoaded = true
		if msg.err != nil {
			m.closeAllModals()
			m.showErrorModal = true
			m.errorModalMsg = "Failed to fetch projects: " + msg.err.Error()
		} else {
			m.projects = msg.projects
			m.projectSelectorIndex = 0
			m.projectFilter = ""
		}

	case fetchMRsMsg:
		m.loadingMRs = false
		m.mrsLoaded = true
		if msg.err != nil {
			m.closeAllModals()
			m.showErrorModal = true
			m.errorModalMsg = msg.err.Error()
			// Update viewport to show hint even on error
			if m.ready {
				m.viewport.SetContent(m.renderMarkdown())
			}
		} else {
			// Sort MRs: non-drafts first (by date newest first), then drafts (by date newest first)
			sort.Slice(msg.mrs, func(i, j int) bool {
				// Both drafts or both non-drafts: sort by date (newest first)
				if msg.mrs[i].Draft == msg.mrs[j].Draft {
					return msg.mrs[i].CreatedAt.After(msg.mrs[j].CreatedAt)
				}
				// Drafts go last
				return !msg.mrs[i].Draft && msg.mrs[j].Draft
			})

			items := make([]list.Item, len(msg.mrs))
			for i, mr := range msg.mrs {
				items[i] = mrListItem{mr: mr}
			}
			m.list.SetItems(items)

			// Build title: "Open MRs (count)"
			m.list.Title = fmt.Sprintf("Open MRs (%d)", len(msg.mrs))

			if m.ready {
				m.viewport.SetContent(m.renderMarkdown())
			}
		}

	case existingReleaseMsg:
		if msg.state != nil {
			// Found existing release - resume it
			return m, m.resumeRelease(msg.state)
		}

	case releaseOutputMsg:
		m.appendReleaseOutput(msg.line)
		return m, nil

	case releaseCommandStartMsg:
		// Smart empty line before command: only add if last line isn't empty
		if len(m.releaseOutputBuffer) > 0 {
			lastLine := m.releaseOutputBuffer[len(m.releaseOutputBuffer)-1]
			if strings.TrimSpace(lastLine) != "" {
				m.appendReleaseOutput("")
			}
		}
		// Wrap long commands to fit terminal width
		terminalWidth := m.getTerminalWidth()
		wrappedLines := wrapText(msg.command, terminalWidth)
		for _, line := range wrappedLines {
			m.appendReleaseOutput(commandLogStyle.Render(line))
		}
		// Mark that we need to check for empty line after command output
		m.releaseNeedEmptyLineAfterCommand = true
		return m, nil

	case releaseScreenMsg:
		// Handle empty line after command if needed
		if m.releaseNeedEmptyLineAfterCommand && msg.content != "" {
			m.releaseNeedEmptyLineAfterCommand = false
			// Check if first line of output is empty
			lines := strings.Split(msg.content, "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				// First line has content, add empty line separator
				m.appendReleaseOutput("")
			}
		}
		m.releaseCurrentScreen = msg.content
		m.updateReleaseViewport()
		return m, nil

	case releaseStepCompleteMsg:
		return m.handleReleaseStepComplete(msg)

	case releaseMRCreatedMsg:
		return m.handleMRCreated(msg)

	case setProgramMsg:
		m.program = msg.program
		return m, nil

	case sourceBranchCheckMsg:
		// Only update if this check is for the current branch name
		if msg.branchName == m.sourceBranchCheckedName {
			if msg.err != nil {
				// On error, show as new branch (can't confirm remote status)
				m.sourceBranchRemoteStatus = "new"
			} else if msg.exists {
				if msg.sameAsRoot {
					m.sourceBranchRemoteStatus = "exists-same"
				} else {
					m.sourceBranchRemoteStatus = "exists-diff"
				}
			} else {
				m.sourceBranchRemoteStatus = "new"
			}
		}
		return m, nil
	}

	// Update inputs if on auth screen (for non-KeyMsg messages like Blink)
	if m.screen == screenAuth {
		var cmd tea.Cmd
		var updatedModel tea.Model
		updatedModel, cmd = m.updateInputs(msg)
		m = updatedModel.(model)
		cmds = append(cmds, cmd)
	}

	// Update settings textarea for non-KeyMsg messages (like cursor blink)
	if m.showSettings {
		var cmd tea.Cmd
		m.settingsExcludePatterns, cmd = m.settingsExcludePatterns.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update version input for non-KeyMsg messages (like cursor blink)
	if m.screen == screenVersion {
		var cmd tea.Cmd
		m.versionInput, cmd = m.versionInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update source branch input for non-KeyMsg messages (like cursor blink)
	if m.screen == screenSourceBranch {
		var cmd tea.Cmd
		m.sourceBranchInput, cmd = m.sourceBranchInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the current screen
func (m model) View() string {
	var view string
	switch m.screen {
	case screenLoading:
		view = m.viewLoading()
	case screenAuth:
		view = m.viewAuth()
	case screenError:
		view = m.viewError()
	case screenMain:
		view = m.viewList()
	case screenEnvSelect:
		view = m.viewEnvSelect()
	case screenVersion:
		view = m.viewVersion()
	case screenSourceBranch:
		view = m.viewSourceBranch()
	case screenRootMerge:
		view = m.viewRootMerge()
	case screenConfirm:
		view = m.viewConfirm()
	case screenRelease:
		view = m.viewRelease()
	}

	// Overlay loading modal if loading MRs
	if m.loadingMRs {
		view = overlayLoadingModal(m.spinner.View(), view, m.width, m.height)
	}

	// Overlay command menu if open
	if m.showCommandMenu {
		view = m.overlayCommandMenu(view)
	}

	// Overlay project selector if open
	if m.showProjectSelector {
		view = m.overlayProjectSelector(view)
	}

	// Overlay settings modal if open
	if m.showSettings {
		view = m.overlaySettings(view)
	}

	// Overlay error modal if open
	if m.showErrorModal {
		view = m.overlayErrorModal(view)
	}

	return view
}

// getTerminalWidth returns the width available for terminal content
func (m *model) getTerminalWidth() int {
	if m.width == 0 {
		return 80 // Default fallback
	}
	sidebarW := sidebarWidth(m.width)
	terminalWidth := m.width - sidebarW - 4 - 4 // content padding, viewport padding
	if terminalWidth < 40 {
		terminalWidth = 40
	}
	return terminalWidth
}
