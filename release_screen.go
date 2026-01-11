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
	releaseLoaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	releasePercentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	releaseSuspendedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	releaseSuccessBlueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("75"))

	releaseSuccessGreenStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("34"))

	releaseConflictStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Bold(true)

	releaseErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Bold(true)

	releaseOrangeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	releaseTerminalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	releaseTerminalFocusedStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("62"))

	releaseButtonInactiveStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("255")).
					Background(lipgloss.Color("240")).
					Padding(0, 2)

	releaseButtonActiveStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("255")).
					Background(lipgloss.Color("62")).
					Padding(0, 2)

	releaseButtonDangerStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("255")).
					Background(lipgloss.Color("196")).
					Padding(0, 2)

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

	// Status takes ~4 lines, buttons take ~2 lines, horizontal line ~1, padding ~3, terminal border ~1
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

	// Abort is available until push starts (step 6)
	if state.CurrentStep < ReleaseStepPushAndCreateMR && state.CurrentStep != ReleaseStepComplete {
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

// updateReleaseViewport updates the viewport content with output buffer
func (m *model) updateReleaseViewport() {
	content := strings.Join(m.releaseOutputBuffer, "\n")
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
	// Handle abort confirmation modal
	if m.showAbortConfirm {
		switch msg.String() {
		case "y", "Y":
			m.showAbortConfirm = false
			m.abortConfirmIndex = 0
			return m.abortRelease()
		case "enter":
			m.showAbortConfirm = false
			if m.abortConfirmIndex == 0 {
				m.abortConfirmIndex = 0
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
		// Cycle through: buttons -> viewport -> buttons
		if m.releaseViewportFocus {
			// From viewport, go to first button
			m.releaseViewportFocus = false
			m.releaseButtonIndex = 0
		} else if m.releaseButtonIndex < len(m.releaseButtons)-1 {
			// Move to next button
			m.releaseButtonIndex++
		} else {
			// From last button, go to viewport
			m.releaseViewportFocus = true
		}
		return m, nil

	case "shift+tab":
		// Cycle in reverse: viewport -> buttons -> viewport
		if m.releaseViewportFocus {
			// From viewport, go to last button
			m.releaseViewportFocus = false
			if len(m.releaseButtons) > 0 {
				m.releaseButtonIndex = len(m.releaseButtons) - 1
			}
		} else if m.releaseButtonIndex > 0 {
			// Move to previous button
			m.releaseButtonIndex--
		} else {
			// From first button, go to viewport
			m.releaseViewportFocus = true
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

	case "g":
		m.releaseViewport.GotoTop()
		return m, nil

	case "G":
		m.releaseViewport.GotoBottom()
		return m, nil

	case "pgup":
		m.releaseViewport.HalfViewUp()
		return m, nil

	case "pgdown":
		m.releaseViewport.HalfViewDown()
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
			openInBrowser(m.releaseState.CreatedMRURL)
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

	// Build triple sidebar (same as confirm screen)
	sidebar := m.renderTripleSidebar(sidebarW, totalHeight)

	// Build content
	contentContent := m.renderReleaseContent(contentWidth-2, contentHeight)

	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "←/→/tab: buttons • ↓/↑/j/k/g/G: scroll • enter: action • /: commands • Ctrl+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	view := lipgloss.JoinVertical(lipgloss.Left, main, help)

	// Overlay abort confirmation if shown
	if m.showAbortConfirm {
		view = m.overlayAbortConfirm(view)
	}

	return view
}

// renderReleaseContent renders the main content area
func (m model) renderReleaseContent(width, height int) string {
	var sb strings.Builder

	// Status section (always 3 lines for consistent layout)
	sb.WriteString("\n")
	status := m.renderReleaseStatus(width)
	// Pad status to always be 3 lines
	lines := strings.Count(status, "\n") + 1
	for lines < 3 {
		status += "\n"
		lines++
	}
	sb.WriteString(status)
	sb.WriteString("\n\n")

	// Horizontal line
	line := releaseHorizontalLineStyle.Render(strings.Repeat("─", width))
	sb.WriteString(line)
	sb.WriteString("\n")

	// Terminal output viewport
	viewportContent := m.releaseViewport.View()
	// Wrap with appropriate border style based on focus
	if m.releaseViewportFocus {
		sb.WriteString(releaseTerminalFocusedStyle.Render(viewportContent))
	} else {
		sb.WriteString(releaseTerminalStyle.Render(viewportContent))
	}

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
	// After all merges, show 99% until complete
	if state.CurrentStep >= ReleaseStepCheckoutEnv && state.CurrentStep < ReleaseStepComplete {
		percentage = 99
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
			status = fmt.Sprintf("Release is %s on %s because of\n%s\nResolve merge issues and press \"Retry\"",
				releaseSuspendedStyle.Render("SUSPENDED"),
				releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
				errorType)
		} else {
			// General error
			status = fmt.Sprintf("Release is %s on %s because of\n%s: %s\nResolve issue in terminal below and press \"Retry\"",
				releaseSuspendedStyle.Render("SUSPENDED"),
				releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
				releaseErrorStyle.Render("ERROR"),
				state.LastError.Message)
		}
		return status
	}

	// Normal progress states
	switch state.CurrentStep {
	case ReleaseStepCheckoutRoot:
		status = fmt.Sprintf("%s %s %s\nCreating root release branch...",
			m.spinner.View(),
			releaseLoaderStyle.Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)))

	case ReleaseStepMergeBranches:
		branchName := ""
		if state.CurrentMRIndex < len(state.MRBranches) {
			branchName = state.MRBranches[state.CurrentMRIndex]
		}
		status = fmt.Sprintf("%s %s %s\nMerging %s MR of %d: %s",
			m.spinner.View(),
			releaseLoaderStyle.Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
			ordinal(state.CurrentMRIndex+1),
			totalMRs,
			branchName)

	case ReleaseStepCheckoutEnv:
		status = fmt.Sprintf("%s %s %s\nCreating environment branch for %s...",
			m.spinner.View(),
			releaseLoaderStyle.Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)),
			state.Environment.Name)

	case ReleaseStepCopyContent:
		status = fmt.Sprintf("%s %s %s\nCopying content and creating release commit...",
			m.spinner.View(),
			releaseLoaderStyle.Render("RELEASING"),
			releasePercentStyle.Render(fmt.Sprintf("%d%%", percentage)))

	case ReleaseStepWaitForMR:
		status = fmt.Sprintf("Release is %s composed\nNow do final step - create its merge request to %s\nPress \"Create MR to %s\"",
			releaseSuccessBlueStyle.Render("SUCCESSFULLY"),
			state.Environment.Name,
			state.Environment.Name)

	case ReleaseStepPushAndCreateMR:
		status = fmt.Sprintf("%s %s %s\nPushing branch and creating merge request...",
			m.spinner.View(),
			releaseLoaderStyle.Render("RELEASING"),
			releasePercentStyle.Render("99%"))

	case ReleaseStepComplete:
		status = fmt.Sprintf("Release is %s completed\nPress \"Open\" to open MR link, or press\n\"Complete\" to exit this release screen",
			releaseSuccessGreenStyle.Render("SUCCESSFULLY"))

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

		// When viewport is focused, all buttons are inactive
		isFocused := i == m.releaseButtonIndex && !m.releaseViewportFocus

		switch btn {
		case ReleaseButtonAbort:
			label = "Abort"
			if isFocused {
				style = releaseButtonDangerStyle
			} else {
				style = releaseButtonInactiveStyle
			}
		case ReleaseButtonRetry:
			label = "Retry"
			if isFocused {
				style = releaseButtonActiveStyle
			} else {
				style = releaseButtonInactiveStyle
			}
		case ReleaseButtonCreateMR:
			envName := ""
			if m.releaseState != nil {
				envName = m.releaseState.Environment.Name
			}
			label = fmt.Sprintf("Create MR to %s", envName)
			if isFocused {
				style = releaseButtonActiveStyle
			} else {
				style = releaseButtonInactiveStyle
			}
		case ReleaseButtonComplete:
			label = "Complete"
			if isFocused {
				style = releaseButtonActiveStyle
			} else {
				style = releaseButtonInactiveStyle
			}
		case ReleaseButtonOpen:
			label = "Open"
			if isFocused {
				style = releaseButtonActiveStyle
			} else {
				style = releaseButtonInactiveStyle
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
		yesBtn = releaseButtonDangerStyle.Render("Yes")
		cancelBtn = releaseButtonInactiveStyle.Render("Cancel")
	} else {
		yesBtn = releaseButtonInactiveStyle.Render("Yes")
		cancelBtn = releaseButtonActiveStyle.Render("Cancel")
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

	// Create release state
	state := &ReleaseState{
		SelectedMRIIDs:  mrIIDs,
		MRBranches:      branches,
		Environment:     *m.selectedEnv,
		Version:         m.versionInput.Value(),
		ProjectID:       m.selectedProject.ID,
		CurrentStep:     ReleaseStepCheckoutRoot,
		LastSuccessStep: ReleaseStepIdle,
		MergedBranches:  []string{},
		WorkDir:         workDir,
	}

	m.releaseState = state
	m.screen = screenRelease
	m.releaseOutputBuffer = []string{}
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

		cmds := NewReleaseCommands(workDir, state.Version, &state.Environment, patterns, state.MRBranches)

		var command string
		var output string
		var err error

		executor := NewGitExecutor(workDir, m.program) // Pass program for real-time output

		switch step {
		case ReleaseStepCheckoutRoot:
			command = cmds.Step1CheckoutRoot()
			output, err = executor.RunCommand(command)

		case ReleaseStepMergeBranches:
			// Check if we need to continue a merge
			if DetectMergeConflict(workDir) {
				command = "git merge --continue"
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

			// Step 4.4: Get next v-number and create commit
			vNumber, verr := GetNextVersionNumber(workDir, state.Environment.BranchName, state.Version)
			if verr != nil {
				return releaseStepCompleteMsg{step: step, err: verr, output: output1 + output2 + output3}
			}

			title, body := BuildCommitMessage(state.Version, state.Environment.BranchName, vNumber, state.MRBranches)
			commitCmd := fmt.Sprintf("git add -A && git commit -m %q -m %q", title, body)
			output4, err4 := executor.RunCommand(commitCmd)

			output = output1 + output2 + output3 + output4
			err = err4

		case ReleaseStepPushAndCreateMR:
			command = cmds.Step6Push()
			output, err = executor.RunCommand(command)

		default:
			return releaseStepCompleteMsg{step: step, err: nil}
		}

		executor.Close()
		return releaseStepCompleteMsg{step: step, err: err, output: output}
	}
}

// handleReleaseStepComplete processes step completion
func (m *model) handleReleaseStepComplete(msg releaseStepCompleteMsg) (tea.Model, tea.Cmd) {
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
		state.ErrorOutput = GetLast500Lines(msg.output)
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
		nextStep = ReleaseStepWaitForMR
		state.CurrentStep = nextStep
		// Don't continue automatically - wait for user button press

	case ReleaseStepPushAndCreateMR:
		// Now create the MR via GitLab API
		state.CurrentStep = ReleaseStepComplete
		SaveReleaseState(state)
		m.updateReleaseButtons()

		// Create MR asynchronously
		return m, m.createGitLabMR()

	default:
		SaveReleaseState(state)
		m.updateReleaseButtons()
		return m, nil
	}

	SaveReleaseState(state)
	m.updateReleaseButtons()

	// Continue to next step if not waiting
	if nextStep != ReleaseStepWaitForMR && nextStep != ReleaseStepComplete {
		m.releaseRunning = true
		nextCmd = tea.Batch(m.spinner.Tick, m.executeReleaseStep(nextStep))
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
	} else {
		m.releaseState.CreatedMRURL = msg.url
		m.releaseState.CreatedMRIID = msg.iid
		m.releaseState.CurrentStep = ReleaseStepComplete
		m.appendReleaseOutput("")
		m.appendReleaseOutput(fmt.Sprintf("Merge request created: %s", msg.url))
	}

	SaveReleaseState(m.releaseState)
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
	m.releaseRunning = false

	// Go back to main screen
	m.screen = screenMain

	return m, nil
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
	m.releaseRunning = false

	// Go back to main screen
	m.screen = screenMain

	return m, nil
}

// resumeRelease resumes from saved release state
func (m *model) resumeRelease(state *ReleaseState) tea.Cmd {
	m.releaseState = state
	m.screen = screenRelease
	m.releaseOutputBuffer = []string{}

	// Add info about resuming
	m.appendReleaseOutput("Resuming previous release...")
	m.appendReleaseOutput("")

	// If there was an error, show it
	if state.ErrorOutput != "" {
		lines := strings.Split(state.ErrorOutput, "\n")
		for _, line := range lines {
			m.appendReleaseOutput(line)
		}
	}

	m.initReleaseScreen()
	m.updateReleaseButtons()

	// If there's an error, wait for retry
	if state.LastError != nil {
		return nil
	}

	// Otherwise continue from current step
	if state.CurrentStep == ReleaseStepWaitForMR || state.CurrentStep == ReleaseStepComplete {
		return nil
	}

	m.releaseRunning = true
	return tea.Batch(m.spinner.Tick, m.executeReleaseStep(state.CurrentStep))
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
