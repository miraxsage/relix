package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxOutputLines = 10000
)

// Styles for release screen
var (
	releasePercentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))

	releaseSuspendedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("236")).
				Background(lipgloss.Color("220")).
				PaddingLeft(1).
				PaddingRight(1)

	releaseSuccessGreenStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("29")).
					Foreground(lipgloss.Color("231"))

	releaseConflictStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("196")).
				PaddingLeft(1).
				PaddingRight(1).
				Bold(true)

	releaseErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("196")).
				PaddingLeft(1).
				PaddingRight(1).
				Bold(true)

	releaseOrangeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220"))

	releaseActiveTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("105"))

	getReleaseEnvStyle = func(env string) lipgloss.Style {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(getEnvBranchColor(env)))
	}

	releaseTerminalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	releaseTextActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("105"))

	releaseHorizontalLineStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("240"))
)

// initReleaseScreen initializes the release screen
func (m *model) initReleaseScreen() {
	if m.width == 0 || m.height == 0 {
		return
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4
	contentHeight := m.height - 4

	// Status 3 lines + top padding 1 + empty after status 1 + horizontal line 1 + border 2 + empty before buttons 1 + buttons 1 = 10
	// Use -11 to leave 1 empty line at the bottom
	viewportHeight := contentHeight - 11
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.releaseViewport = viewport.New(contentWidth-4, viewportHeight)
	// Note: Border style is applied in renderReleaseContent based on focus state
	m.updateReleaseViewport()
	m.updateReleaseButtons()
}

// updateReleaseButtons updates available buttons based on current state
func (m *model) updateReleaseButtons() {
	m.releaseButtons = nil

	if m.releaseState == nil {
		return
	}

	state := m.releaseState

	// Abort is available until complete (including during push step for error recovery)
	if state.CurrentStep != ReleaseStepComplete {
		m.releaseButtons = append(m.releaseButtons, ReleaseButtonAbort)
	}

	// Retry is available only on error
	if state.LastError != nil {
		m.releaseButtons = append(m.releaseButtons, ReleaseButtonRetry)
	}

	// Create MR is available at step 5 (waiting) and no error
	if state.CurrentStep == ReleaseStepWaitForMR && state.LastError == nil {
		m.releaseButtons = append(m.releaseButtons, ReleaseButtonCreateMR)
	}

	// Complete and Open are available after MR creation
	if state.CurrentStep == ReleaseStepComplete {
		m.releaseButtons = append(m.releaseButtons, ReleaseButtonComplete)
		if state.CreatedMRURL != "" {
			m.releaseButtons = append(m.releaseButtons, ReleaseButtonOpen)
		}
	}

	// Reset button index if out of bounds
	if m.releaseButtonIndex >= len(m.releaseButtons) {
		m.releaseButtonIndex = 0
	}
}

// updateReleaseViewport updates the viewport content with output buffer and virtual terminal screen
func (m *model) updateReleaseViewport() {
	// Combine command headers (output buffer) with current virtual terminal screen
	var content string
	if len(m.releaseOutputBuffer) > 0 {
		content = strings.Join(m.releaseOutputBuffer, "\n")
	}
	if m.releaseCurrentScreen != "" {
		if content != "" {
			content += "\n"
		}
		content += m.releaseCurrentScreen
	}
	m.releaseViewport.SetContent(content)
	m.releaseViewport.GotoBottom()
}

// appendReleaseOutput adds a line to the output buffer
func (m *model) appendReleaseOutput(line string) {
	m.releaseOutputBuffer = append(m.releaseOutputBuffer, line)

	// Enforce buffer limit
	if len(m.releaseOutputBuffer) > maxOutputLines {
		m.releaseOutputBuffer = m.releaseOutputBuffer[len(m.releaseOutputBuffer)-maxOutputLines:]
	}

	m.updateReleaseViewport()
}

// updateRelease handles key events on the release screen
func (m model) updateRelease(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle delete remote branch confirmation modal (second step after abort confirm)
	if m.showDeleteRemoteConfirm {
		switch msg.String() {
		case "y", "Y":
			m.showDeleteRemoteConfirm = false
			m.deleteRemoteConfirmIndex = 0
			return m.abortReleaseWithRemoteDeletion(true)
		case "enter":
			m.showDeleteRemoteConfirm = false
			deleteRemote := m.deleteRemoteConfirmIndex == 0
			m.deleteRemoteConfirmIndex = 0
			return m.abortReleaseWithRemoteDeletion(deleteRemote)
		case "n", "N":
			m.showDeleteRemoteConfirm = false
			m.deleteRemoteConfirmIndex = 0
			return m.abortReleaseWithRemoteDeletion(false)
		case "esc":
			m.showDeleteRemoteConfirm = false
			m.deleteRemoteConfirmIndex = 0
			return m, nil
		case "tab", "right", "l":
			if m.deleteRemoteConfirmIndex < 1 {
				m.deleteRemoteConfirmIndex++
			} else {
				m.deleteRemoteConfirmIndex = 0
			}
			return m, nil
		case "shift+tab", "left", "h":
			if m.deleteRemoteConfirmIndex > 0 {
				m.deleteRemoteConfirmIndex--
			} else {
				m.deleteRemoteConfirmIndex = 1
			}
			return m, nil
		}
		return m, nil
	}

	// Handle abort confirmation modal
	if m.showAbortConfirm {
		switch msg.String() {
		case "y", "Y":
			m.showAbortConfirm = false
			m.abortConfirmIndex = 0
			// If we're at or past push step, ask about remote branch deletion
			if m.releaseState != nil && m.releaseState.CurrentStep >= ReleaseStepPushBranches {
				m.showDeleteRemoteConfirm = true
				m.deleteRemoteConfirmIndex = 0
				return m, nil
			}
			return m.abortRelease()
		case "enter":
			m.showAbortConfirm = false
			if m.abortConfirmIndex == 0 {
				m.abortConfirmIndex = 0
				// If we're at or past push step, ask about remote branch deletion
				if m.releaseState != nil && m.releaseState.CurrentStep >= ReleaseStepPushBranches {
					m.showDeleteRemoteConfirm = true
					m.deleteRemoteConfirmIndex = 0
					return m, nil
				}
				return m.abortRelease()
			}
			m.abortConfirmIndex = 0
			return m, nil
		case "n", "N", "esc":
			m.showAbortConfirm = false
			m.abortConfirmIndex = 0
			return m, nil
		case "tab", "right", "l":
			if m.abortConfirmIndex < 1 {
				m.abortConfirmIndex++
			} else {
				m.abortConfirmIndex = 0
			}
			return m, nil
		case "shift+tab", "left", "h":
			if m.abortConfirmIndex > 0 {
				m.abortConfirmIndex--
			} else {
				m.abortConfirmIndex = 1
			}
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "left", "h":
		if m.releaseButtonIndex > 0 {
			m.releaseButtonIndex--
		}
		return m, nil

	case "right", "l":
		if m.releaseButtonIndex < len(m.releaseButtons)-1 {
			m.releaseButtonIndex++
		}
		return m, nil

	case "tab":
		// Cycle through buttons
		if m.releaseButtonIndex < len(m.releaseButtons)-1 {
			m.releaseButtonIndex++
		} else {
			m.releaseButtonIndex = 0
		}
		return m, nil

	case "shift+tab":
		// Cycle through buttons in reverse
		if m.releaseButtonIndex > 0 {
			m.releaseButtonIndex--
		} else if len(m.releaseButtons) > 0 {
			m.releaseButtonIndex = len(m.releaseButtons) - 1
		}
		return m, nil

	case "enter":
		return m.executeReleaseButton()

	case "up", "k":
		m.releaseViewport.LineUp(1)
		return m, nil

	case "down", "j":
		m.releaseViewport.LineDown(1)
		return m, nil

	case "d", "pgdown":
		m.releaseViewport.HalfViewDown()
		return m, nil

	case "u", "pgup":
		m.releaseViewport.HalfViewUp()
		return m, nil

	case "g":
		m.releaseViewport.GotoTop()
		return m, nil

	case "G":
		m.releaseViewport.GotoBottom()
		return m, nil

	case "o":
		// Open MR URL if available
		if m.releaseState != nil && m.releaseState.CreatedMRURL != "" {
			return m, openInBrowser(m.releaseState.CreatedMRURL)
		}
		return m, nil
	}

	// Viewport scrolling
	var cmd tea.Cmd
	m.releaseViewport, cmd = m.releaseViewport.Update(msg)
	return m, cmd
}

// executeReleaseButton handles button press
func (m model) executeReleaseButton() (tea.Model, tea.Cmd) {
	if len(m.releaseButtons) == 0 || m.releaseButtonIndex >= len(m.releaseButtons) {
		return m, nil
	}

	button := m.releaseButtons[m.releaseButtonIndex]

	switch button {
	case ReleaseButtonAbort:
		m.showAbortConfirm = true
		return m, nil

	case ReleaseButtonRetry:
		return m.retryRelease()

	case ReleaseButtonCreateMR:
		return m.startCreateMR()

	case ReleaseButtonComplete:
		return m.completeRelease()

	case ReleaseButtonOpen:
		if m.releaseState != nil && m.releaseState.CreatedMRURL != "" {
			return m, openInBrowser(m.releaseState.CreatedMRURL)
		}
	}

	return m, nil
}

// viewRelease renders the release screen
func (m model) viewRelease() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4
	contentHeight := m.height - 4
	totalHeight := contentHeight + 2

	// Build five sidebar (same as confirm screen)
	sidebar := m.renderFiveSidebar(sidebarW, totalHeight)

	// Build content
	contentContent := m.renderReleaseContent(contentWidth-2, contentHeight)

	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "tab: focus • j/k/d/u/g/G: scroll • enter: action"
	// Add "o: open" hint when MR URL is available
	if m.releaseState != nil && m.releaseState.CreatedMRURL != "" {
		helpText += " • o: open"
	}
	helpText += " • /: commands"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	view := lipgloss.JoinVertical(lipgloss.Left, main, help)

	// Overlay abort confirmation if shown
	if m.showAbortConfirm {
		view = m.overlayAbortConfirm(view)
	}

	// Overlay delete remote branch confirmation if shown
	if m.showDeleteRemoteConfirm {
		view = m.overlayDeleteRemoteConfirm(view)
	}

	return view
}

// renderReleaseContent renders the main content area
func (m model) renderReleaseContent(width, height int) string {
	var sb strings.Builder

	// Status section (minimum 3 lines, but can expand)
	sb.WriteString("\n")
	status := m.renderReleaseStatus(width)
	statusLines := strings.Count(status, "\n") + 1
	// Pad status to minimum 3 lines
	for statusLines < 3 {
		status += "\n"
		statusLines++
	}
	sb.WriteString(status)
	sb.WriteString("\n\n")

	// Horizontal line
	line := releaseHorizontalLineStyle.Render(strings.Repeat("─", width))
	sb.WriteString(line)
	sb.WriteString("\n")

	// Calculate dynamic viewport height based on status height
	// Base calculation: height - 11 (status 3 + top padding 1 + empty after status 1 + line 1 + border 2 + empty before buttons 1 + buttons 1 = 10, plus 1 empty at bottom)
	// If status takes more than 3 lines, shrink viewport accordingly
	extraStatusLines := statusLines - 3
	viewportHeight := height - 11 - extraStatusLines
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Create a viewport with dynamic height for this render
	vp := m.releaseViewport
	vp.Height = viewportHeight
	viewportContent := vp.View()
	sb.WriteString(releaseTerminalStyle.Render(viewportContent))

	// One empty line before buttons
	sb.WriteString("\n\n")

	// Buttons
	sb.WriteString(m.renderReleaseButtons(width))

	// One empty line after buttons
	sb.WriteString("\n")

	return sb.String()
}

// renderReleaseStatus renders the status section
func (m model) renderReleaseStatus(width int) string {
	if m.releaseState == nil {
		return "Initializing release..."
	}

	state := m.releaseState

	// Calculate progress percentage based on merged MRs
	totalMRs := len(state.MRBranches)
	mergedMRs := len(state.MergedBranches)
	var percentage int
	if totalMRs > 0 {
		percentage = (mergedMRs * 100) / totalMRs
	}
	// After all merges, show 95% until complete
	if state.CurrentStep >= ReleaseStepCheckoutEnv && state.CurrentStep < ReleaseStepComplete {
		percentage = 95
	}
	if state.CurrentStep == ReleaseStepComplete {
		percentage = 100
	}

	var status string

	// Check for error first
	if state.LastError != nil {
		// Suspended state
		var errorType string
		if DetectMergeConflict(state.WorkDir) {
			// Merge conflict
			branchName := ""
			if state.CurrentMRIndex < len(state.MRBranches) {
				branchName = state.MRBranches[state.CurrentMRIndex]
			}
			errorType = fmt.Sprintf("%s merge %s.",
				releaseOrangeStyle.Render(branchName),
				releaseConflictStyle.Render("CONFLICT"))
			status = fmt.Sprintf("Release is %s on %s because of\n%s\nResolve merge issues and press %s",
				releaseSuspendedStyle.Render("SUSPENDED"),
				releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
				errorType,
				releaseActiveTextStyle.Render("Retry"),
			)
		} else {
			// General error
			lastStatusLine := fmt.Sprintf("Resolve issue in terminal below and press %s",
				releaseActiveTextStyle.Render("Retry"),
			)
			if state.LastError.Code == "RELEASE_INTERRUPTED" {
				lastStatusLine = ""
			}
			status = fmt.Sprintf("Release is %s on %s because of\n%s %s\n%s",
				releaseSuspendedStyle.Render("SUSPENDED"),
				releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
				releaseErrorStyle.Render("ERROR"),
				state.LastError.Message,
				lastStatusLine,
			)
		}
		return status
	}

	// Normal progress states
	switch state.CurrentStep {
	case ReleaseStepCheckoutRoot:
		status = fmt.Sprintf("%s %s %s\nCreating root release branch...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)))

	case ReleaseStepMergeBranches:
		branchName := ""
		if state.CurrentMRIndex < len(state.MRBranches) {
			branchName = state.MRBranches[state.CurrentMRIndex]
		}
		status = fmt.Sprintf("%s %s %s\nMerging %s MR of %d: %s",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
			ordinal(state.CurrentMRIndex+1),
			totalMRs,
			branchName)

	case ReleaseStepCheckoutEnv:
		status = fmt.Sprintf("%s %s %s\nCreating environment branch for %s...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name))

	case ReleaseStepCopyContent:
		status = fmt.Sprintf("%s %s %s\nCopying content from root branch...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)))

	case ReleaseStepCommit:
		status = fmt.Sprintf("%s %s %s\nCreating release commit...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)))

	case ReleaseStepPushBranches:
		status = fmt.Sprintf("%s %s %s\nPushing %s and release branch to remote...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
			releaseOrangeStyle.Render(state.SourceBranch))

	case ReleaseStepWaitForMR:
		status = fmt.Sprintf("Release is %s\nNow do final step - create its merge request to %s\nPress %s",
			releaseSuccessGreenStyle.Render(" SUCCESSFULLY COMPOSED "),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name),
			releaseTextActiveStyle.Render("Create MR to "+state.Environment.Name),
		)

	case ReleaseStepPushAndCreateMR:
		status = fmt.Sprintf("%s %s %s\nCreating merge request to %s...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render("99%"),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name))

	case ReleaseStepMergeToRoot:
		status = fmt.Sprintf("%s %s %s\nMerging %s to root...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render("99%"),
			releaseOrangeStyle.Render(state.SourceBranch))

	case ReleaseStepMergeToDevelop:
		status = fmt.Sprintf("%s %s %s\nMerging root to develop...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render("99%"))

	case ReleaseStepComplete:
		status = fmt.Sprintf("Release is %s\nPress %s to open MR link, or press\n%s to exit this release screen",
			releaseSuccessGreenStyle.Render(" SUCCESSFULLY COMPLETED "),
			releaseActiveTextStyle.Render("Open"),
			releaseActiveTextStyle.Render("Complete"),
		)

	default:
		status = "Preparing release..."
	}

	return status
}

// ordinal returns the ordinal form of a number (1st, 2nd, 3rd, etc.)
func ordinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// renderReleaseButtons renders the button row
func (m model) renderReleaseButtons(width int) string {
	if len(m.releaseButtons) == 0 {
		return ""
	}

	var buttons []string
	for i, btn := range m.releaseButtons {
		var style lipgloss.Style
		var label string

		isFocused := i == m.releaseButtonIndex

		switch btn {
		case ReleaseButtonAbort:
			label = "Abort"
			if isFocused {
				style = buttonDangerStyle
			} else {
				style = buttonStyle
			}
		case ReleaseButtonRetry:
			label = "Retry"
			if isFocused {
				style = buttonActiveStyle
			} else {
				style = buttonStyle
			}
		case ReleaseButtonCreateMR:
			envName := ""
			if m.releaseState != nil {
				envName = m.releaseState.Environment.Name
			}
			label = fmt.Sprintf("Create MR to %s", envName)
			if isFocused {
				style = buttonActiveStyle
			} else {
				style = buttonStyle
			}
		case ReleaseButtonComplete:
			label = "Complete"
			if isFocused {
				style = buttonActiveStyle
			} else {
				style = buttonStyle
			}
		case ReleaseButtonOpen:
			label = "Open"
			if isFocused {
				style = buttonActiveStyle
			} else {
				style = buttonStyle
			}
		}

		buttons = append(buttons, style.Render(label))
	}

	buttonsRow := strings.Join(buttons, "  ")
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(buttonsRow)
}

// overlayAbortConfirm renders the abort confirmation modal
func (m model) overlayAbortConfirm(background string) string {
	var sb strings.Builder

	title := errorTitleStyle.Render("Abort Release?")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString("Are you sure to abort current release,\n")
	sb.WriteString("this will remove all current progress?\n\n")

	var yesBtn, cancelBtn string
	if m.abortConfirmIndex == 0 {
		yesBtn = buttonDangerStyle.Render("Yes")
		cancelBtn = buttonStyle.Render("Cancel")
	} else {
		yesBtn = buttonStyle.Render("Yes")
		cancelBtn = buttonActiveStyle.Render("Cancel")
	}
	sb.WriteString(fmt.Sprintf("        %s        %s", yesBtn, cancelBtn))

	config := ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: false},
		MinWidth: 40,
		MaxWidth: 60,
		Style:    errorBoxStyle,
	}

	modal := renderModal(sb.String(), config, m.width)
	return placeOverlayCenter(modal, background, m.width, m.height)
}

// overlayDeleteRemoteConfirm renders the delete remote branch confirmation modal
func (m model) overlayDeleteRemoteConfirm(background string) string {
	var sb strings.Builder

	title := errorTitleStyle.Render("Delete Remote Branch?")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString("The release branch was already pushed to remote.\n")
	sb.WriteString("Do you want to delete it from origin?\n\n")

	var yesBtn, noBtn string
	if m.deleteRemoteConfirmIndex == 0 {
		yesBtn = buttonDangerStyle.Render("Yes, delete")
		noBtn = buttonStyle.Render("No, keep it")
	} else {
		yesBtn = buttonStyle.Render("Yes, delete")
		noBtn = buttonActiveStyle.Render("No, keep it")
	}
	sb.WriteString(fmt.Sprintf("     %s     %s", yesBtn, noBtn))

	config := ModalConfig{
		Width:    ModalWidth{Value: 50, Percent: false},
		MinWidth: 40,
		MaxWidth: 60,
		Style:    errorBoxStyle,
	}

	modal := renderModal(sb.String(), config, m.width)
	return placeOverlayCenter(modal, background, m.width, m.height)
}

// startRelease initiates the release process
func (m *model) startRelease() (tea.Model, tea.Cmd) {
	// Find project root
	workDir, err := FindProjectRoot()
	if err != nil {
		m.showErrorModal = true
		m.errorModalMsg = fmt.Sprintf("Failed to find project root: %v", err)
		return m, nil
	}

	// Check for uncommitted changes
	hasChanges, err := HasUncommittedChanges(workDir)
	if err != nil {
		m.showErrorModal = true
		m.errorModalMsg = fmt.Sprintf("Failed to check git status: %v", err)
		return m, nil
	}
	if hasChanges {
		m.showErrorModal = true
		m.errorModalMsg = "Cannot start release: there are uncommitted changes in the working directory"
		return m, nil
	}

	// Collect selected MR branches
	items := m.list.Items()
	var mrIIDs []int
	var branches []string
	for _, item := range items {
		if mr, ok := item.(mrListItem); ok {
			if m.selectedMRs[mr.MR().IID] {
				mrIIDs = append(mrIIDs, mr.MR().IID)
				branches = append(branches, mr.MR().SourceBranch)
			}
		}
	}

	// Determine if source branch exists remotely based on the check status
	sourceBranchIsRemote := m.sourceBranchRemoteStatus == "exists-same" || m.sourceBranchRemoteStatus == "exists-diff" || m.sourceBranchRemoteStatus == "exists"

	// Create release state
	state := &ReleaseState{
		SelectedMRIIDs:       mrIIDs,
		MRBranches:           branches,
		Environment:          *m.selectedEnv,
		Version:              m.versionInput.Value(),
		SourceBranch:         m.sourceBranchInput.Value(),
		SourceBranchIsRemote: sourceBranchIsRemote,
		RootMerge:            m.rootMergeSelection,
		ProjectID:            m.selectedProject.ID,
		CurrentStep:          ReleaseStepCheckoutRoot,
		LastSuccessStep:      ReleaseStepIdle,
		MergedBranches:       []string{},
		WorkDir:              workDir,
	}

	m.releaseState = state
	m.screen = screenRelease
	m.releaseOutputBuffer = []string{}
	m.releaseCurrentScreen = ""
	m.initReleaseScreen()

	// Save initial state
	SaveReleaseState(state)

	// Start execution with spinner
	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepCheckoutRoot))
}

// executeReleaseStep runs the appropriate command for a step
func (m *model) executeReleaseStep(step ReleaseStep) tea.Cmd {
	return func() tea.Msg {
		if m.releaseState == nil {
			return releaseStepCompleteMsg{step: step, err: fmt.Errorf("no release state")}
		}

		state := m.releaseState
		workDir := state.WorkDir

		// Load config for exclude patterns
		config, _ := LoadConfig()
		patterns := strings.Split(config.ExcludePatterns, "\n")

		cmds := NewReleaseCommandsWithSourceBranch(workDir, state.Version, &state.Environment, patterns, state.MRBranches, state.SourceBranch, state.SourceBranchIsRemote)

		var command string
		var output string
		var err error

		executor := NewGitExecutor(workDir, m.program) // Pass program for real-time output

		// Set executor size based on viewport dimensions
		// Calculate width: total width - sidebar - content padding - viewport padding
		if m.width > 0 {
			sidebarW := sidebarWidth(m.width)
			terminalWidth := m.width - sidebarW - 4 - 4 // content padding, viewport padding
			if terminalWidth < 40 {
				terminalWidth = 40
			}
			terminalHeight := m.releaseViewport.Height
			if terminalHeight < 10 {
				terminalHeight = 10
			}
			executor.SetSize(uint16(terminalWidth), uint16(terminalHeight))
		}

		switch step {
		case ReleaseStepCheckoutRoot:
			command = cmds.Step1CheckoutRoot()
			output, err = executor.RunCommand(command)

		case ReleaseStepMergeBranches:
			// Check if we need to continue a merge
			if DetectMergeConflict(workDir) {
				command = "GIT_EDITOR=true git merge --continue"
			} else if state.CurrentMRIndex < len(state.MRBranches) {
				// Check if branch already merged
				branch := state.MRBranches[state.CurrentMRIndex]
				merged, _ := IsBranchMerged(workDir, "origin/"+branch)
				if merged {
					// Already merged, move to next
					return releaseStepCompleteMsg{step: step, err: nil, output: fmt.Sprintf("Branch %s already merged\n", branch)}
				}
				command = cmds.Step2MergeBranch(state.CurrentMRIndex)
			}
			if command != "" {
				output, err = executor.RunCommand(command)
			}

		case ReleaseStepCheckoutEnv:
			// Check if remote env branch exists
			if !RemoteBranchExists(workDir, state.Environment.BranchName) {
				return releaseStepCompleteMsg{
					step:   step,
					err:    fmt.Errorf("remote branch origin/%s does not exist", state.Environment.BranchName),
					output: "",
				}
			}
			command = cmds.Step3CheckoutEnv()
			output, err = executor.RunCommand(command)

		case ReleaseStepCopyContent:
			// Step 4.1: Remove all files
			output1, err1 := executor.RunCommand(cmds.Step4RemoveAll())
			if err1 != nil {
				return releaseStepCompleteMsg{step: step, err: err1, output: output1}
			}

			// Step 4.2: Checkout from root
			output2, err2 := executor.RunCommand(cmds.Step4CheckoutFromRoot())
			if err2 != nil {
				return releaseStepCompleteMsg{step: step, err: err2, output: output1 + output2}
			}

			// Step 4.3: Exclude files
			excluded, _ := GetExcludedFiles(workDir, patterns)
			var output3 string
			if len(excluded) > 0 {
				// Remove excluded files in batches
				for _, file := range excluded {
					output3 += fmt.Sprintf("Excluding: %s\n", releaseOrangeStyle.Render(file))
					rmCmd := fmt.Sprintf("git rm -rf --cached %q", file)
					executor.RunCommand(rmCmd)
				}
			}

			output = output1 + output2 + output3

		case ReleaseStepCommit:
			// Get next v-number and create commit
			vNumber, verr := GetNextVersionNumber(workDir, state.Environment.BranchName, state.Version)
			if verr != nil {
				return releaseStepCompleteMsg{step: step, err: verr, output: ""}
			}

			title, body := BuildCommitMessage(state.Version, state.Environment.BranchName, vNumber, state.MRBranches)
			// Use $'...' bash syntax to properly interpret \n as newlines in commit body
			// Escape single quotes and convert actual newlines to \n escape sequences
			escapedBody := strings.ReplaceAll(body, "'", "'\\''")
			escapedBody = strings.ReplaceAll(escapedBody, "\n", "\\n")
			commitCmd := fmt.Sprintf("git add -A && git commit -m %q -m $'%s'", title, escapedBody)
			output, err = executor.RunCommand(commitCmd)

		case ReleaseStepPushBranches:
			// Push source branch to remote (so it's available for next release)
			// TODO: output1, err1 := executor.RunCommand(cmds.Step6PushSourceBranch())
			output1, err1 := executor.RunCommand(fmt.Sprintf("git log %s -1 --oneline", cmds.RootBranch()))
			if err1 != nil {
				return releaseStepCompleteMsg{step: step, err: err1, output: output1}
			}

			// Push env release branch to remote
			output2, err2 := executor.RunCommand(cmds.Step6Push())
			output = output1 + output2
			err = err2

		case ReleaseStepPushAndCreateMR:
			// Branches are already pushed in ReleaseStepPushBranches
			// This step only creates the GitLab MR (handled via API after this step completes)
			output = "Branches already pushed. Creating merge request...\n"
			err = nil

		case ReleaseStepMergeToRoot:
			// Step 6b: Merge source branch to root and push
			// TODO: output, err = executor.RunCommand(cmds.StepMergeToRoot())
			output, err = executor.RunCommand("git log root -1 --oneline")

		case ReleaseStepMergeToDevelop:
			// Step 6c: Merge root to develop and push
			// TODO: output, err = executor.RunCommand(cmds.StepMergeToDevelop())
			output, err = executor.RunCommand("git log develop -1 --oneline")

		default:
			return releaseStepCompleteMsg{step: step, err: nil}
		}

		executor.Close()
		return releaseStepCompleteMsg{step: step, err: err, output: output}
	}
}

// handleReleaseStepComplete processes step completion
func (m *model) handleReleaseStepComplete(msg releaseStepCompleteMsg) (tea.Model, tea.Cmd) {
	// Reset empty line flag for next command
	m.releaseNeedEmptyLineAfterCommand = false

	// Save current terminal screen to buffer (for real-time streaming mode)
	// This preserves the command output before the next command starts
	if m.releaseCurrentScreen != "" {
		lines := strings.Split(m.releaseCurrentScreen, "\n")
		for _, line := range lines {
			m.releaseOutputBuffer = append(m.releaseOutputBuffer, line)
		}
		// Enforce buffer limit
		if len(m.releaseOutputBuffer) > maxOutputLines {
			m.releaseOutputBuffer = m.releaseOutputBuffer[len(m.releaseOutputBuffer)-maxOutputLines:]
		}
		m.releaseCurrentScreen = ""
		m.updateReleaseViewport()
	}

	// Append output to buffer only if not streamed in real-time
	// (when program is set, output is already streamed via releaseOutputMsg)
	if msg.output != "" && m.program == nil {
		lines := strings.Split(msg.output, "\n")
		for _, line := range lines {
			m.appendReleaseOutput(line)
		}
	}

	m.releaseRunning = false

	if m.releaseState == nil {
		return m, nil
	}

	state := m.releaseState

	if msg.err != nil {
		// Handle error
		state.LastError = &ReleaseError{
			Step:    msg.step,
			Message: msg.err.Error(),
		}
		// Save last 5000 lines of terminal output (buffer + current screen)
		fullOutput := strings.Join(m.releaseOutputBuffer, "\n")
		if m.releaseCurrentScreen != "" {
			fullOutput += "\n" + m.releaseCurrentScreen
		}
		state.ErrorOutput = GetLastNLines(fullOutput, 5000)
		// Save terminal output buffer for resume
		state.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
		copy(state.TerminalOutput, m.releaseOutputBuffer)
		SaveReleaseState(state)
		m.updateReleaseButtons()
		return m, nil
	}

	// Step succeeded
	state.LastSuccessStep = msg.step
	state.LastError = nil
	state.ErrorOutput = ""

	// Determine next step
	var nextStep ReleaseStep
	var nextCmd tea.Cmd

	switch msg.step {
	case ReleaseStepCheckoutRoot:
		nextStep = ReleaseStepMergeBranches
		state.CurrentStep = nextStep
		state.CurrentMRIndex = 0

	case ReleaseStepMergeBranches:
		// Mark current branch as merged
		if state.CurrentMRIndex < len(state.MRBranches) {
			state.MergedBranches = append(state.MergedBranches, state.MRBranches[state.CurrentMRIndex])
			state.CurrentMRIndex++
		}

		// Check if more branches to merge
		if state.CurrentMRIndex < len(state.MRBranches) {
			nextStep = ReleaseStepMergeBranches
			state.CurrentStep = nextStep
		} else {
			nextStep = ReleaseStepCheckoutEnv
			state.CurrentStep = nextStep
		}

	case ReleaseStepCheckoutEnv:
		nextStep = ReleaseStepCopyContent
		state.CurrentStep = nextStep

	case ReleaseStepCopyContent:
		nextStep = ReleaseStepCommit
		state.CurrentStep = nextStep

	case ReleaseStepCommit:
		nextStep = ReleaseStepPushBranches
		state.CurrentStep = nextStep

	case ReleaseStepPushBranches:
		nextStep = ReleaseStepWaitForMR
		state.CurrentStep = nextStep
		// Don't continue automatically - wait for user button press

	case ReleaseStepPushAndCreateMR:
		// Now create the MR via GitLab API
		// After MR is created, handleMRCreated will trigger root merge if enabled
		state.CurrentStep = ReleaseStepPushAndCreateMR // Keep same step until MR is created
		// Save terminal output buffer for resume
		state.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
		copy(state.TerminalOutput, m.releaseOutputBuffer)
		SaveReleaseState(state)
		m.updateReleaseButtons()

		// Create MR asynchronously
		return m, m.createGitLabMR()

	case ReleaseStepMergeToRoot:
		// Continue to merge root to develop
		nextStep = ReleaseStepMergeToDevelop
		state.CurrentStep = nextStep

	case ReleaseStepMergeToDevelop:
		// Root merge completed, go to complete
		nextStep = ReleaseStepComplete
		state.CurrentStep = nextStep

	default:
		// Save terminal output buffer for resume
		state.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
		copy(state.TerminalOutput, m.releaseOutputBuffer)
		SaveReleaseState(state)
		m.updateReleaseButtons()
		return m, nil
	}

	// Save terminal output buffer for resume
	state.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
	copy(state.TerminalOutput, m.releaseOutputBuffer)
	SaveReleaseState(state)
	m.updateReleaseButtons()

	// Continue to next step if not waiting
	if nextStep != ReleaseStepWaitForMR && nextStep != ReleaseStepComplete {
		m.releaseRunning = true
		nextCmd = tea.Batch(m.spinner.Tick, m.executeReleaseStep(nextStep))
	} else if nextStep == ReleaseStepWaitForMR {
		// Focus on "Create MR" button (index 1: Abort=0, CreateMR=1)
		m.releaseButtonIndex = 1
	} else if nextStep == ReleaseStepComplete {
		// Release complete (including root merge if enabled)
		// Clear release state so Ctrl+C goes to MRs list
		ClearReleaseState()

		// Reset selected MRs for next release
		m.initListScreen()
		m.updateListSize()

		// Reset version input
		m.versionInput.SetValue("")
		m.versionError = ""

		// Reset environment selection
		m.selectedEnv = nil
		m.envSelectIndex = 0
	}

	return m, nextCmd
}

// createGitLabMR creates the merge request via GitLab API
func (m *model) createGitLabMR() tea.Cmd {
	return func() tea.Msg {
		if m.releaseState == nil || m.creds == nil {
			return releaseMRCreatedMsg{err: fmt.Errorf("invalid state")}
		}

		state := m.releaseState
		client := NewGitLabClient(m.creds.GitLabURL, m.creds.Token)

		// Get commit info for title/description
		vNumber, _ := GetNextVersionNumber(state.WorkDir, state.Environment.BranchName, state.Version)
		title, body := BuildCommitMessage(state.Version, state.Environment.BranchName, vNumber, state.MRBranches)

		cmds := NewReleaseCommands(state.WorkDir, state.Version, &state.Environment, nil, nil)
		sourceBranch := cmds.EnvReleaseBranch()
		targetBranch := state.Environment.BranchName

		mr, err := client.CreateMergeRequest(state.ProjectID, sourceBranch, targetBranch, title, body)
		if err != nil {
			return releaseMRCreatedMsg{err: err}
		}

		return releaseMRCreatedMsg{url: mr.WebURL, iid: mr.IID, err: nil}
	}
}

// handleMRCreated processes MR creation result
func (m *model) handleMRCreated(msg releaseMRCreatedMsg) (tea.Model, tea.Cmd) {
	if m.releaseState == nil {
		return m, nil
	}

	if msg.err != nil {
		m.releaseState.LastError = &ReleaseError{
			Step:    ReleaseStepPushAndCreateMR,
			Message: msg.err.Error(),
		}
		m.releaseState.CurrentStep = ReleaseStepPushAndCreateMR
		m.appendReleaseOutput(fmt.Sprintf("ERROR: Failed to create MR: %v", msg.err))
		// Save state for retry
		m.releaseState.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
		copy(m.releaseState.TerminalOutput, m.releaseOutputBuffer)
		SaveReleaseState(m.releaseState)
	} else {
		m.releaseState.CreatedMRURL = msg.url
		m.releaseState.CreatedMRIID = msg.iid
		m.appendReleaseOutput("")
		m.appendReleaseOutput(fmt.Sprintf("Merge request created: %s", msg.url))

		// Check if root merge is enabled - if so, continue to merge steps
		if m.releaseState.RootMerge {
			m.releaseState.CurrentStep = ReleaseStepMergeToRoot
			m.appendReleaseOutput("")
			m.appendReleaseOutput("Starting root merge flow...")
			// Save state and continue to merge steps
			m.releaseState.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
			copy(m.releaseState.TerminalOutput, m.releaseOutputBuffer)
			SaveReleaseState(m.releaseState)
			m.updateReleaseButtons()
			// Execute root merge step
			m.releaseRunning = true
			return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepMergeToRoot))
		}

		// No root merge - mark as complete
		m.releaseState.CurrentStep = ReleaseStepComplete

		// MR created successfully - clear release.json so Ctrl+C goes to MRs list
		ClearReleaseState()

		// Reset selected MRs for next release
		m.initListScreen()
		m.updateListSize()

		// Reset version input
		m.versionInput.SetValue("")
		m.versionError = ""

		// Reset environment selection
		m.selectedEnv = nil
		m.envSelectIndex = 0
	}

	m.updateReleaseButtons()
	return m, nil
}

// retryRelease retries from the last failed step
func (m model) retryRelease() (tea.Model, tea.Cmd) {
	if m.releaseState == nil || m.releaseState.LastError == nil {
		return m, nil
	}

	// Clear error
	step := m.releaseState.LastError.Step
	m.releaseState.LastError = nil
	m.releaseState.ErrorOutput = ""
	m.releaseState.CurrentStep = step

	SaveReleaseState(m.releaseState)
	m.updateReleaseButtons()

	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(step))
}

// abortRelease cleans up and aborts the release
func (m model) abortRelease() (tea.Model, tea.Cmd) {
	if m.releaseState != nil {
		workDir := m.releaseState.WorkDir
		version := m.releaseState.Version
		envBranch := m.releaseState.Environment.BranchName

		// Kill any running process
		if m.releaseExecutor != nil {
			m.releaseExecutor.Kill()
			m.releaseExecutor.Close()
			m.releaseExecutor = nil
		}

		// Reset to clean state
		cmd := fmt.Sprintf("cd %q && git reset --hard && git checkout root", workDir)
		exec := NewGitExecutor(workDir, nil)
		exec.RunCommand(cmd)
		exec.Close()

		// Delete created branches
		DeleteLocalBranches(workDir, version, envBranch)
	}

	// Clear state
	ClearReleaseState()
	m.releaseState = nil
	m.releaseOutputBuffer = nil
	m.releaseCurrentScreen = ""
	m.releaseRunning = false

	// Reset selections for next release
	(&m).initListScreen()
	(&m).updateListSize()
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""

	// Go back to main screen and reload MRs
	m.screen = screenMain
	m.loadingMRs = true

	return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
}

// abortReleaseWithRemoteDeletion cleans up and aborts the release, optionally deleting remote branch
func (m model) abortReleaseWithRemoteDeletion(deleteRemote bool) (tea.Model, tea.Cmd) {
	if m.releaseState != nil {
		workDir := m.releaseState.WorkDir
		version := m.releaseState.Version
		envBranch := m.releaseState.Environment.BranchName

		// Kill any running process
		if m.releaseExecutor != nil {
			m.releaseExecutor.Kill()
			m.releaseExecutor.Close()
			m.releaseExecutor = nil
		}

		// Delete remote branch if requested
		if deleteRemote {
			cmds := NewReleaseCommands(workDir, version, &m.releaseState.Environment, nil, nil)
			remoteBranch := cmds.EnvReleaseBranch()
			deleteCmd := fmt.Sprintf("cd %q && git push origin --delete %s", workDir, remoteBranch)
			exec := NewGitExecutor(workDir, nil)
			exec.RunCommand(deleteCmd)
			exec.Close()
		}

		// Reset to clean state
		cmd := fmt.Sprintf("cd %q && git reset --hard && git checkout root", workDir)
		exec := NewGitExecutor(workDir, nil)
		exec.RunCommand(cmd)
		exec.Close()

		// Delete created branches
		DeleteLocalBranches(workDir, version, envBranch)
	}

	// Clear state
	ClearReleaseState()
	m.releaseState = nil
	m.releaseOutputBuffer = nil
	m.releaseCurrentScreen = ""
	m.releaseRunning = false

	// Reset selections for next release
	(&m).initListScreen()
	(&m).updateListSize()
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""

	// Go back to main screen and reload MRs
	m.screen = screenMain
	m.loadingMRs = true

	return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
}

// startCreateMR initiates the push and MR creation
func (m model) startCreateMR() (tea.Model, tea.Cmd) {
	if m.releaseState == nil {
		return m, nil
	}

	m.releaseState.CurrentStep = ReleaseStepPushAndCreateMR
	m.updateReleaseButtons()
	SaveReleaseState(m.releaseState)

	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepPushAndCreateMR))
}

// completeRelease finishes the release and cleans up
func (m model) completeRelease() (tea.Model, tea.Cmd) {
	ClearReleaseState()
	m.releaseState = nil
	m.releaseOutputBuffer = nil
	m.releaseCurrentScreen = ""
	m.releaseRunning = false

	// Reset selections for next release
	(&m).initListScreen()
	(&m).updateListSize()
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""

	// Go back to main screen and reload MRs
	m.screen = screenMain
	m.loadingMRs = true

	return m, tea.Batch(m.spinner.Tick, m.fetchMRs())
}

// resumeRelease resumes from saved release state
func (m *model) resumeRelease(state *ReleaseState) tea.Cmd {
	m.releaseState = state
	m.screen = screenRelease
	m.releaseCurrentScreen = ""

	// Restore saved terminal output
	if len(state.TerminalOutput) > 0 {
		m.releaseOutputBuffer = make([]string, len(state.TerminalOutput))
		copy(m.releaseOutputBuffer, state.TerminalOutput)
	} else if state.ErrorOutput != "" {
		// Fallback: if no terminal output saved but there's error output, show it
		m.releaseOutputBuffer = []string{"Resuming previous release...", ""}
		lines := strings.Split(state.ErrorOutput, "\n")
		m.releaseOutputBuffer = append(m.releaseOutputBuffer, lines...)
	} else {
		m.releaseOutputBuffer = []string{}
	}

	m.initReleaseScreen()

	// If step is in progress (not waiting for user action or complete),
	// mark as interrupted so user must press Retry to continue
	if state.LastError == nil &&
		state.CurrentStep != ReleaseStepWaitForMR &&
		state.CurrentStep != ReleaseStepComplete {
		state.LastError = &ReleaseError{
			Step: state.CurrentStep,
			Code: "RELEASE_INTERRUPTED",
			Message: fmt.Sprintf("it was interrupted. Press %s to continue.",
				releaseActiveTextStyle.Render("Retry"),
			),
		}
	}

	m.updateReleaseButtons()

	// If there's an error (including synthetic interrupted error), wait for retry
	// Focus on Retry button (index 1: Abort=0, Retry=1)
	if state.LastError != nil {
		m.releaseButtonIndex = 1
		return nil
	}

	// Handle user action steps
	if state.CurrentStep == ReleaseStepWaitForMR {
		// Focus on "Create MR" button (index 1: Abort=0, CreateMR=1)
		m.releaseButtonIndex = 1
		return nil
	}
	if state.CurrentStep == ReleaseStepComplete {
		return nil
	}

	return nil
}

// checkExistingRelease checks if there's an in-progress release on startup
func checkExistingRelease() tea.Cmd {
	return func() tea.Msg {
		state, err := LoadReleaseState()
		if err != nil || state == nil {
			return nil
		}
		return existingReleaseMsg{state: state}
	}
}

// delayedStartRelease adds a small delay before starting release execution
func delayedStartRelease() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return nil
	})
}
