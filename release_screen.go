package main

import (
	"fmt"
	"os/exec"
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

// calculateReleaseTotalSteps returns the total number of substeps for the release
func calculateReleaseTotalSteps(state *ReleaseState) int {
	total := 1                     // GitFetch
	total += 1                     // CheckoutRoot
	total += len(state.MRBranches) // MergeBranches (one per MR)
	total += 1                     // CheckoutEnv
	total += 3                     // CopyContent (checkout env-release + rm all + checkout from root)
	total += 1                     // Commit
	total += 1                     // Push env branch
	total += 1                     // Create MR (API)
	if state.RootMerge {
		total += 5 // push release-root, merge to root, tag, push root+tags, merge develop+push
	} else {
		total += 2 // tag, push with tags
	}
	return total
}

// initReleaseScreen initializes the release screen
func (m *model) initReleaseScreen() {
	if m.width == 0 || m.height == 0 {
		return
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4
	contentHeight := m.height - 4

	// Layout: status 5 + hrline 1 + viewport border 2 + empty before buttons 1 + buttons 1 + empty after buttons 1 = 11
	viewportHeight := contentHeight - 11
	if viewportHeight < 1 {
		viewportHeight = 1
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

	// Open button: available at step 8 when MR or pipeline URL exists
	if state.CurrentStep == ReleaseStepWaitForRootPush {
		if state.CreatedMRURL != "" || (m.pipelineStatus != nil && m.pipelineStatus.PipelineWebURL != "") {
			m.releaseButtons = append(m.releaseButtons, ReleaseButtonOpen)
		}
	}

	// Push root branches is available at step 8 (waiting for root push) and no error
	if state.CurrentStep == ReleaseStepWaitForRootPush && state.LastError == nil {
		m.releaseButtons = append(m.releaseButtons, ReleaseButtonPushRoot)
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

// appendRecoveryMetadata adds recovery metadata to the terminal output at release start
func (m *model) appendRecoveryMetadata(workDir string, state *ReleaseState) {
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	m.releaseOutputBuffer = append(m.releaseOutputBuffer, headerStyle.Render("Release recover metadata:"))

	// Get commit IDs for various branches
	rootCommit := GetBranchCommitID(workDir, "root")
	originRootCommit := GetBranchCommitID(workDir, "origin/root")
	originEnvCommit := GetBranchCommitID(workDir, "origin/"+state.Environment.BranchName)

	// Calculate max label width for alignment
	// Labels: "root:", "origin/root:", "origin/<env>:", "origin/<source>:"
	sourceBranchLabel := "origin/" + state.SourceBranch + ":"
	envBranchLabel := "origin/" + state.Environment.BranchName + ":"

	maxWidth := len("origin/root:")
	if len(envBranchLabel) > maxWidth {
		maxWidth = len(envBranchLabel)
	}
	if state.SourceBranchIsRemote && len(sourceBranchLabel) > maxWidth {
		maxWidth = len(sourceBranchLabel)
	}

	// Format string with dynamic width
	format := fmt.Sprintf("%%-%ds %%s", maxWidth)

	m.releaseOutputBuffer = append(m.releaseOutputBuffer, fmt.Sprintf(format, "root:", rootCommit))
	m.releaseOutputBuffer = append(m.releaseOutputBuffer, fmt.Sprintf(format, "origin/root:", originRootCommit))
	m.releaseOutputBuffer = append(m.releaseOutputBuffer, fmt.Sprintf(format, envBranchLabel, originEnvCommit))

	// Only show source branch if it existed remotely on release start
	if state.SourceBranchIsRemote {
		originSourceCommit := GetBranchCommitID(workDir, "origin/"+state.SourceBranch)
		m.releaseOutputBuffer = append(m.releaseOutputBuffer, fmt.Sprintf(format, sourceBranchLabel, originSourceCommit))
	}
	m.releaseOutputBuffer = append(m.releaseOutputBuffer, "") // Empty line after metadata
}

// updateRelease handles key events on the release screen
func (m model) updateRelease(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle open options modal first
	if m.showOpenOptionsModal {
		return m.updateOpenOptionsModal(msg)
	}

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
			if m.releaseState != nil && m.releaseState.CurrentStep >= ReleaseStepPushAndCreateMR {
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
				if m.releaseState != nil && m.releaseState.CurrentStep >= ReleaseStepPushAndCreateMR {
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
		// Show options modal for release resources
		if m.releaseState != nil {
			return m.handleOpenAction(buildReleaseOpenOptions(m.releaseState, m.pipelineStatus))
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

	case ReleaseButtonPushRoot:
		return m.startPushRootBranches()

	case ReleaseButtonComplete:
		return m.completeRelease()

	case ReleaseButtonOpen:
		if m.releaseState != nil {
			return m.handleOpenAction(buildReleaseOpenOptions(m.releaseState, m.pipelineStatus))
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
	// Add "o: open" hint when MR URL or pipeline URL is available
	if m.releaseState != nil && (m.releaseState.CreatedMRURL != "" || (m.pipelineStatus != nil && m.pipelineStatus.PipelineWebURL != "")) {
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
	lines := make([]string, 0, height)

	// Status section - always exactly 5 visual lines
	status := m.renderReleaseStatus(width)
	statusParts := strings.Split(status, "\n")

	// Account for text wrapping: any line wider than width wraps to extra visual lines
	visualLines := 0
	for _, part := range statusParts {
		lineWidth := lipgloss.Width(part)
		if lineWidth > width && width > 0 {
			visualLines += (lineWidth + width - 1) / width
		} else {
			visualLines++
		}
	}

	// Pad status to 5 lines: max 1 empty line at top, rest at bottom
	// If 5+ lines of text, no padding at all
	if visualLines < 5 {
		topPad := 1
		bottomPad := 5 - visualLines - topPad
		for i := 0; i < topPad; i++ {
			lines = append(lines, "")
		}
		lines = append(lines, statusParts...)
		for i := 0; i < bottomPad; i++ {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, statusParts...)
	}

	// Horizontal line
	lines = append(lines, releaseHorizontalLineStyle.Render(strings.Repeat("─", width)))

	// Calculate viewport height
	// Fixed lines around viewport: hrline 1 + viewport border 2 + empty before buttons 1 + buttons 1 + empty after buttons 1 = 6
	linesBeforeViewport := len(lines)
	linesAfterViewport := 3 // empty before buttons + buttons + empty after buttons
	viewportHeight := height - linesBeforeViewport - 2 - linesAfterViewport
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Viewport with border
	vp := m.releaseViewport
	vp.Height = viewportHeight
	viewportRendered := releaseTerminalStyle.Render(vp.View())
	lines = append(lines, strings.Split(viewportRendered, "\n")...)

	// Button section: empty line, buttons
	lines = append(lines, "")
	lines = append(lines, m.renderReleaseButtons(width))

	// Pad to exact height to guarantee empty line after buttons
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

// renderReleaseStatus renders the status section
func (m model) renderReleaseStatus(width int) string {
	if m.releaseState == nil {
		return "Initializing release..."
	}

	state := m.releaseState

	// Calculate progress from substep counters
	completed := state.CompletedSubSteps
	total := state.TotalSubSteps
	if total == 0 {
		total = 1
	}
	percentage := (completed * 100) / total
	if state.CurrentStep == ReleaseStepComplete {
		percentage = 100
	}
	progressText := fmt.Sprintf("%d%% [%d/%d]", percentage, completed, state.TotalSubSteps)

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
				releasePercentStyle.Render(progressText),
				errorType,
				releaseActiveTextStyle.Render("Retry"),
			)
		} else if state.LastError.Code == "COMMIT_FAILED" {
			// Commit failed (likely linter error) - tell user to fix in release-root branch
			cmds := NewReleaseCommandsWithSourceBranch(state.WorkDir, state.Version, &state.Environment, nil, nil, state.SourceBranch, state.SourceBranchIsRemote)
			status = fmt.Sprintf("Release is %s on %s because of\n%s %s\nFix errors in %s branch, commit them and press %s",
				releaseSuspendedStyle.Render("SUSPENDED"),
				releasePercentStyle.Render(progressText),
				releaseErrorStyle.Render("ERROR"),
				state.LastError.Message,
				releaseOrangeStyle.Render(cmds.ReleaseRootBranch()),
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
				releasePercentStyle.Render(progressText),
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
			releasePercentStyle.Render(progressText))

	case ReleaseStepMergeBranches:
		branchName := ""
		if state.CurrentMRIndex < len(state.MRBranches) {
			branchName = state.MRBranches[state.CurrentMRIndex]
		}
		status = fmt.Sprintf("%s %s %s\nMerging %s MR of %d: %s",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText),
			ordinal(state.CurrentMRIndex+1),
			len(state.MRBranches),
			branchName)

	case ReleaseStepCheckoutEnv:
		status = fmt.Sprintf("%s %s %s\nCreating environment branch for %s...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name))

	case ReleaseStepCopyContent:
		status = fmt.Sprintf("%s %s %s\nCopying content from root branch...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText))

	case ReleaseStepCommit:
		status = fmt.Sprintf("%s %s %s\nCreating release commit...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText))

	case ReleaseStepPushBranches:
		status = fmt.Sprintf("%s %s %s\nPushing %s and release branch to remote...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText),
			releaseOrangeStyle.Render(state.SourceBranch))

	case ReleaseStepWaitForMR:
		status = fmt.Sprintf("Release is %s %s\nNow do next step - create its merge request to %s\nPress %s",
			releaseSuccessGreenStyle.Render(" SUCCESSFULLY COMPOSED "),
			releasePercentStyle.Render(progressText),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name),
			releaseTextActiveStyle.Render("Create MR to "+state.Environment.Name),
		)

	case ReleaseStepPushAndCreateMR:
		status = fmt.Sprintf("%s %s %s\nPushing env branch and creating merge request to %s...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name))

	case ReleaseStepWaitForRootPush:
		hintText := m.renderRootPushHint()
		pipelineStatus := m.renderPipelineStatus()
		if pipelineStatus != "" {
			status = fmt.Sprintf("Merge request is %s %s\n%s\nNow you can push release branch to root and develop:\n%s",
				releaseSuccessGreenStyle.Render(" CREATED "),
				releasePercentStyle.Render(progressText),
				pipelineStatus,
				hintText,
			)
		} else {
			status = fmt.Sprintf("Merge request is %s %s\nNow you can push release branch to root and develop:\n%s",
				releaseSuccessGreenStyle.Render(" CREATED "),
				releasePercentStyle.Render(progressText),
				hintText,
			)
		}

	case ReleaseStepPushRootBranches:
		status = fmt.Sprintf("%s %s %s\nPushing root branches...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render(progressText))

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
		case ReleaseButtonPushRoot:
			label = "Push root branches"
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
	var mrURLs []string
	var mrCommitSHAs []string
	for _, item := range items {
		if mr, ok := item.(mrListItem); ok {
			if m.selectedMRs[mr.MR().IID] {
				mrIIDs = append(mrIIDs, mr.MR().IID)
				branches = append(branches, mr.MR().SourceBranch)
				mrURLs = append(mrURLs, mr.MR().WebURL)
				mrCommitSHAs = append(mrCommitSHAs, mr.MR().SHA)
			}
		}
	}

	// Determine if source branch exists remotely based on the check status
	sourceBranchIsRemote := m.sourceBranchRemoteStatus == "exists-same" || m.sourceBranchRemoteStatus == "exists-diff" || m.sourceBranchRemoteStatus == "exists"

	// Create release state
	state := &ReleaseState{
		SelectedMRIIDs:       mrIIDs,
		MRBranches:           branches,
		MRURLs:               mrURLs,
		MRCommitSHAs:         mrCommitSHAs,
		Environment:          *m.selectedEnv,
		Version:              m.versionInput.Value(),
		SourceBranch:         m.sourceBranchInput.Value(),
		SourceBranchIsRemote: sourceBranchIsRemote,
		RootMerge:            m.rootMergeSelection,
		ProjectID:            m.selectedProject.ID,
		CurrentStep:          ReleaseStepGitFetch,
		LastSuccessStep:      ReleaseStepIdle,
		MergedBranches:       []string{},
		WorkDir:              workDir,
	}

	state.TotalSubSteps = calculateReleaseTotalSteps(state)
	state.CompletedSubSteps = 0

	m.releaseState = state
	m.screen = screenRelease
	m.releaseOutputBuffer = []string{}
	m.releaseCurrentScreen = ""

	// Add recovery metadata to terminal output
	m.appendRecoveryMetadata(workDir, state)

	m.initReleaseScreen()

	// Save initial state (includes recovery metadata in terminal output)
	SaveReleaseState(state)

	// Start execution with spinner
	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepGitFetch))
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
		case ReleaseStepGitFetch:
			output, err = executor.RunCommand(cmds.StepGitFetch())

		case ReleaseStepCheckoutRoot:
			output, err = executor.RunCommands(cmds.Step1CheckoutRoot())

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
			output, err = executor.RunCommands(cmds.Step3CheckoutEnv())

		case ReleaseStepCopyContent:
			// First, ensure we're on the env-release-branch (needed when retrying after commit failure)
			envReleaseBranch := cmds.EnvReleaseBranch()
			checkoutOutput, checkoutErr := executor.RunCommand(fmt.Sprintf("git checkout %s", envReleaseBranch))
			if checkoutErr != nil {
				return releaseStepCompleteMsg{step: step, err: checkoutErr, output: checkoutOutput}
			}
			m.program.Send(releaseSubStepDoneMsg{})

			// Step 4.1: Remove all files
			output1, err1 := executor.RunCommand(cmds.Step4RemoveAll())
			if err1 != nil {
				return releaseStepCompleteMsg{step: step, err: err1, output: checkoutOutput + output1}
			}
			m.program.Send(releaseSubStepDoneMsg{})

			// Step 4.2: Checkout from root
			output2, err2 := executor.RunCommand(cmds.Step4CheckoutFromRoot())
			if err2 != nil {
				return releaseStepCompleteMsg{step: step, err: err2, output: checkoutOutput + output1 + output2}
			}

			// Step 4.3: Exclude files - restore from env branch or remove if not exists
			excluded, _ := GetExcludedFiles(workDir, patterns)
			var output3 string
			if len(excluded) > 0 {
				for _, file := range excluded {
					output3 += fmt.Sprintf("Excluding: %s\n", releaseOrangeStyle.Render(file))
					// Try to restore file from environment branch (keeps it unchanged)
					restoreCmd := fmt.Sprintf("git checkout origin/%s -- %q 2>/dev/null", state.Environment.BranchName, file)
					_, restoreErr := executor.RunCommand(restoreCmd)
					if restoreErr != nil {
						// File doesn't exist in env branch - remove it completely
						executor.RunCommand(fmt.Sprintf("rm -rf %q", file))
						executor.RunCommand(fmt.Sprintf("git rm -rf --cached %q 2>/dev/null || true", file))
					}
				}
			}
			m.program.Send(releaseSubStepDoneMsg{})

			output = checkoutOutput + output1 + output2 + output3

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
			// Don't use "git add -A" - files are already staged from checkout
			commitCmd := fmt.Sprintf("git commit -m %q -m $'%s'", title, escapedBody)
			output, err = executor.RunCommand(commitCmd)

			// After successful commit, clean up any remaining untracked files
			if err == nil {
				cleanOutput, _ := executor.RunCommand("git clean -fd")
				output += cleanOutput
			}

		case ReleaseStepPushAndCreateMR:
			// Push env release branch to remote (for the MR)
			// Release root branch will be pushed later when user clicks "Push root branches"
			output, err = executor.RunCommand(cmds.Step6Push())
			// MR will be created via API after this step completes

		case ReleaseStepPushRootBranches:
			// Step 9: Tag and push branches
			tagName := state.TagName

			if state.RootMerge {
				// RootMerge enabled: push release root, merge to root, tag root, push root, merge to develop, push

				// Push release root branch first
				pushReleaseRootCmd := fmt.Sprintf("git push -u origin %s", state.SourceBranch)
				output1, err1 := executor.RunCommand(pushReleaseRootCmd)
				if err1 != nil {
					return releaseStepCompleteMsg{step: step, err: err1, output: output1}
				}
				output = output1
				m.program.Send(releaseSubStepDoneMsg{})

				// Merge source branch to root
				output2, err2 := executor.RunCommands(cmds.StepMergeToRoot())
				if err2 != nil {
					return releaseStepCompleteMsg{step: step, err: err2, output: output + output2}
				}
				output += output2
				m.program.Send(releaseSubStepDoneMsg{})

				// Create tag on root (after merge)
				tagCmd := fmt.Sprintf("git tag %s", tagName)
				output3, err3 := executor.RunCommand(tagCmd)
				if err3 != nil {
					return releaseStepCompleteMsg{step: step, err: err3, output: output + output3}
				}
				output += output3
				m.program.Send(releaseSubStepDoneMsg{})

				// Push root with tags
				pushRootCmd := "git push origin root --tags"
				output4, err4 := executor.RunCommand(pushRootCmd)
				if err4 != nil {
					return releaseStepCompleteMsg{step: step, err: err4, output: output + output4}
				}
				output += output4
				m.program.Send(releaseSubStepDoneMsg{})

				// Merge root to develop and push
				output5, err5 := executor.RunCommands(cmds.StepMergeToDevelop())
				if err5 != nil {
					return releaseStepCompleteMsg{step: step, err: err5, output: output + output5}
				}
				output += output5
				m.program.Send(releaseSubStepDoneMsg{})
			} else {
				// No RootMerge: tag source branch, push with tags

				// Create tag on source branch
				tagCmd := fmt.Sprintf("git tag %s", tagName)
				output1, err1 := executor.RunCommand(tagCmd)
				if err1 != nil {
					return releaseStepCompleteMsg{step: step, err: err1, output: output1}
				}
				m.program.Send(releaseSubStepDoneMsg{})

				// Push source branch with tags
				pushCmd := fmt.Sprintf("git push origin %s --tags", state.SourceBranch)
				output2, err2 := executor.RunCommand(pushCmd)
				if err2 != nil {
					return releaseStepCompleteMsg{step: step, err: err2, output: output1 + output2}
				}
				m.program.Send(releaseSubStepDoneMsg{})

				output = output1 + output2
			}

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
		// Handle error - display in terminal with pale red color (no background)
		terminalErrorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
		m.appendReleaseOutput("")
		m.appendReleaseOutput(terminalErrorStyle.Render("ERROR: " + msg.err.Error()))

		// Special handling for commit step errors (likely linter errors)
		// Reset to release-root branch so user can fix there
		if msg.step == ReleaseStepCommit {
			m.appendReleaseOutput("")
			m.appendReleaseOutput("Resetting to release-root branch for fixes...")

			// Reset staged changes and switch to release-root branch
			executor := NewGitExecutor(state.WorkDir, nil)
			executor.RunCommand("git reset")
			cmds := NewReleaseCommandsWithSourceBranch(state.WorkDir, state.Version, &state.Environment, nil, nil, state.SourceBranch, state.SourceBranchIsRemote)
			executor.RunCommand(fmt.Sprintf("git checkout %s", cmds.ReleaseRootBranch()))
			executor.Close()

			m.appendReleaseOutput(fmt.Sprintf("Switched to %s", releaseOrangeStyle.Render(cmds.ReleaseRootBranch())))

			state.LastError = &ReleaseError{
				Step:    ReleaseStepCopyContent, // Retry from copy content step
				Message: msg.err.Error(),
				Code:    "COMMIT_FAILED",
			}
		} else {
			state.LastError = &ReleaseError{
				Step:    msg.step,
				Message: msg.err.Error(),
			}
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
	case ReleaseStepGitFetch:
		state.CompletedSubSteps++
		nextStep = ReleaseStepCheckoutRoot
		state.CurrentStep = nextStep

	case ReleaseStepCheckoutRoot:
		state.CompletedSubSteps++
		nextStep = ReleaseStepMergeBranches
		state.CurrentStep = nextStep
		state.CurrentMRIndex = 0

	case ReleaseStepMergeBranches:
		state.CompletedSubSteps++
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
		state.CompletedSubSteps++
		nextStep = ReleaseStepCopyContent
		state.CurrentStep = nextStep

	case ReleaseStepCopyContent:
		// substeps already incremented via releaseSubStepDoneMsg
		nextStep = ReleaseStepCommit
		state.CurrentStep = nextStep

	case ReleaseStepCommit:
		state.CompletedSubSteps++
		// After commit, wait for user to click "Create MR" button
		// Push will happen when user clicks the button
		nextStep = ReleaseStepWaitForMR
		state.CurrentStep = nextStep

	case ReleaseStepPushAndCreateMR:
		state.CompletedSubSteps++
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

	case ReleaseStepPushRootBranches:
		// substeps already incremented via releaseSubStepDoneMsg
		// Root push completed, go to complete
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
	if nextStep != ReleaseStepWaitForMR && nextStep != ReleaseStepWaitForRootPush && nextStep != ReleaseStepComplete {
		m.releaseRunning = true
		nextCmd = tea.Batch(m.spinner.Tick, m.executeReleaseStep(nextStep))
	} else if nextStep == ReleaseStepWaitForMR {
		// Focus on "Create MR" button (index 1: Abort=0, CreateMR=1)
		m.releaseButtonIndex = 1
	} else if nextStep == ReleaseStepWaitForRootPush {
		// Focus on "Push root branches" button (index 2: Abort=0, Open=1, PushRoot=2)
		m.releaseButtonIndex = 2
	} else if nextStep == ReleaseStepComplete {
		// Release complete (including root merge if enabled)
		// Save to history immediately so it persists even if user exits with Ctrl+C
		terminalOutput := append([]string{}, m.releaseOutputBuffer...)
		if m.releaseCurrentScreen != "" {
			lines := strings.Split(m.releaseCurrentScreen, "\n")
			terminalOutput = append(terminalOutput, lines...)
		}
		SaveReleaseHistory(state, "completed", terminalOutput)

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

		cmds := NewReleaseCommands(state.WorkDir, state.Version, &state.Environment, nil, nil)
		sourceBranch := cmds.EnvReleaseBranch()
		targetBranch := state.Environment.BranchName

		// Get version number and build MR title/body
		vNumber, _ := GetNextVersionNumber(state.WorkDir, state.Environment.BranchName, state.Version)
		title, body := BuildCommitMessage(state.Version, state.Environment.BranchName, vNumber, state.MRBranches)

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
		m.updateReleaseButtons()
		return m, nil
	}

	m.releaseState.CreatedMRURL = msg.url
	m.releaseState.CreatedMRIID = msg.iid
	m.releaseState.CompletedSubSteps++ // MR created via API = 1 substep
	m.appendReleaseOutput("")
	m.appendReleaseOutput(fmt.Sprintf("Merge request created: %s", msg.url))

	// Calculate and store tag name for display
	vNumber, _ := GetNextVersionNumber(m.releaseState.WorkDir, m.releaseState.Environment.BranchName, m.releaseState.Version)
	m.releaseState.TagName = fmt.Sprintf("%s-%s-v%d",
		strings.ToLower(m.releaseState.Environment.Name),
		m.releaseState.Version,
		vNumber,
	)

	// Go to wait for root push step (user must click "Push root branches")
	m.releaseState.CurrentStep = ReleaseStepWaitForRootPush

	// Save state
	m.releaseState.TerminalOutput = make([]string, len(m.releaseOutputBuffer))
	copy(m.releaseState.TerminalOutput, m.releaseOutputBuffer)
	SaveReleaseState(m.releaseState)
	m.updateReleaseButtons()

	// Focus on "Push root branches" button (index 2: Abort=0, Open=1, PushRoot=2)
	m.releaseButtonIndex = 2

	// Start pipeline observer and open MR URL in Safari
	return m, tea.Batch(m.startPipelineObserver(), openInSafariWithFallback(msg.url))
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
	// Stop pipeline observer
	m.stopPipelineObserver()
	m.pipelineStatus = nil

	// Save to history before cleanup
	if m.releaseState != nil {
		terminalOutput := append([]string{}, m.releaseOutputBuffer...)
		if m.releaseCurrentScreen != "" {
			lines := strings.Split(m.releaseCurrentScreen, "\n")
			terminalOutput = append(terminalOutput, lines...)
		}
		SaveReleaseHistory(m.releaseState, "aborted", terminalOutput)
	}

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
		exec := NewGitExecutor(workDir, nil)
		exec.RunCommand("git reset --hard")
		exec.RunCommand("git checkout root")
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
	m.selectedMRs = make(map[int]bool)
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""
	m.mrsLoaded = false

	// Go back to home screen
	m.screen = screenHome

	return m, nil
}

// abortReleaseWithRemoteDeletion cleans up and aborts the release, optionally deleting remote branch
func (m model) abortReleaseWithRemoteDeletion(deleteRemote bool) (tea.Model, tea.Cmd) {
	// Stop pipeline observer
	m.stopPipelineObserver()
	m.pipelineStatus = nil

	// Save to history before cleanup
	if m.releaseState != nil {
		terminalOutput := append([]string{}, m.releaseOutputBuffer...)
		if m.releaseCurrentScreen != "" {
			lines := strings.Split(m.releaseCurrentScreen, "\n")
			terminalOutput = append(terminalOutput, lines...)
		}
		SaveReleaseHistory(m.releaseState, "aborted", terminalOutput)
	}

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

		// Delete remote branches if requested
		if deleteRemote {
			cmds := NewReleaseCommandsWithSourceBranch(workDir, version, &m.releaseState.Environment, nil, nil, m.releaseState.SourceBranch, m.releaseState.SourceBranchIsRemote)
			exec := NewGitExecutor(workDir, nil)

			// Delete env release branch (e.g. release/rpb-1.0.0-dev)
			exec.RunCommand(fmt.Sprintf("git push origin --delete %s", cmds.EnvReleaseBranch()))

			// Delete source/root branch only if it was newly created (not pre-existing on remote)
			if !m.releaseState.SourceBranchIsRemote {
				exec.RunCommand(fmt.Sprintf("git push origin --delete %s", cmds.ReleaseRootBranch()))
			}

			exec.Close()
		}

		// Reset to clean state
		exec2 := NewGitExecutor(workDir, nil)
		exec2.RunCommand("git reset --hard")
		exec2.RunCommand("git checkout root")
		exec2.Close()

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
	m.selectedMRs = make(map[int]bool)
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""
	m.mrsLoaded = false

	// Go back to home screen
	m.screen = screenHome

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

// renderRootPushHint returns the hint text for the root push step
func (m model) renderRootPushHint() string {
	if m.releaseState == nil {
		return ""
	}

	state := m.releaseState

	// Styles for the hint text
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// Get tag name (already calculated and stored in state)
	tagName := state.TagName

	if state.RootMerge {
		// With RootMerge: {branch} will be merged to root, tagged as {tag}, then root to develop
		return fmt.Sprintf("%s %s %s%s %s %s %s%s %s%s",
			branchStyle.Render(state.SourceBranch),
			textStyle.Render("will be merged to"),
			branchStyle.Render("root"),
			textStyle.Render(","),
			branchStyle.Render("root"),
			textStyle.Render("tagged as"),
			tagStyle.Render(tagName),
			textStyle.Render(" and merged to"),
			branchStyle.Render("develop"),
			textStyle.Render(",\nfinally all pushed to remote"),
		)
	}

	// Without RootMerge: {branch} will be tagged as {tag} pushed to remote
	return fmt.Sprintf("%s %s %s %s",
		branchStyle.Render(state.SourceBranch),
		textStyle.Render("will be tagged as"),
		tagStyle.Render(tagName),
		textStyle.Render("pushed to remote"),
	)
}

// startPushRootBranches initiates the root branch push step
func (m model) startPushRootBranches() (tea.Model, tea.Cmd) {
	if m.releaseState == nil {
		return m, nil
	}

	// Stop pipeline observer
	m.stopPipelineObserver()
	m.pipelineStatus = nil

	m.releaseState.CurrentStep = ReleaseStepPushRootBranches
	m.updateReleaseButtons()
	SaveReleaseState(m.releaseState)

	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepPushRootBranches))
}

// completeRelease finishes the release and cleans up
func (m model) completeRelease() (tea.Model, tea.Cmd) {
	// Stop pipeline observer
	m.stopPipelineObserver()
	m.pipelineStatus = nil

	// Switch to root branch as the final operation
	if m.releaseState != nil {
		workDir := m.releaseState.WorkDir
		exec := NewGitExecutor(workDir, nil)
		exec.RunCommand("git checkout root")
		exec.Close()
	}

	ClearReleaseState()
	m.releaseState = nil
	m.releaseOutputBuffer = nil
	m.releaseCurrentScreen = ""
	m.releaseRunning = false

	// Reset selections for next release
	m.selectedMRs = make(map[int]bool)
	m.selectedEnv = nil
	m.envSelectIndex = 0
	m.versionInput.SetValue("")
	m.versionError = ""
	m.mrsLoaded = false

	// Go back to home screen
	m.screen = screenHome

	return m, nil
}

// resumeRelease resumes from saved release state
func (m *model) resumeRelease(state *ReleaseState) tea.Cmd {
	// Recalculate total for backward compat with saved state
	state.TotalSubSteps = calculateReleaseTotalSteps(state)

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
		state.CurrentStep != ReleaseStepWaitForRootPush &&
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
	if state.CurrentStep == ReleaseStepWaitForRootPush {
		// Focus on "Push root branches" button (index 2: Abort=0, Open=1, PushRoot=2)
		m.releaseButtonIndex = 2
		// Start pipeline observer when resuming at this step
		return m.startPipelineObserver()
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

// startPipelineObserver starts the pipeline observer and returns the first tick command
func (m *model) startPipelineObserver() tea.Cmd {
	m.pipelineObserving = true
	m.pipelineFailNotified = false
	m.pipelineStatus = &PipelineStatus{
		Stage: PipelineStageLoading,
	}
	// Return first check immediately, with spinner tick to keep animation running
	return tea.Batch(m.spinner.Tick, m.checkPipelineStatus())
}

// stopPipelineObserver stops the pipeline observer
func (m *model) stopPipelineObserver() {
	m.pipelineObserving = false
}

// pipelineTick returns a command that triggers a pipeline check after 7 seconds
func (m *model) pipelineTick() tea.Cmd {
	return tea.Tick(7*time.Second, func(t time.Time) tea.Msg {
		return pipelineTickMsg{}
	})
}

// getEnvJobSuffix returns the job suffix for the given environment branch
func getEnvJobSuffix(envBranch string) string {
	switch envBranch {
	case "develop":
		return "dev01"
	case "testing":
		return "test01"
	case "stable":
		return "stage01"
	case "master", "main":
		return "prod01"
	default:
		return ""
	}
}

// isRelevantJob checks if a job is a Package or Deploy job for our apps
func isRelevantJob(jobName, envSuffix string) bool {
	if envSuffix == "" {
		return false
	}

	// App names to track
	apps := []string{"Main", "Admin", "JudgePersonal", "Touch"}

	// Normalize job name for comparison
	jobNameLower := strings.ToLower(jobName)

	for _, app := range apps {
		// Check Package jobs: "Package Application {App} {env}"
		packagePattern := strings.ToLower(fmt.Sprintf("Package Application %s %s", app, envSuffix))
		if jobNameLower == packagePattern {
			return true
		}

		// Check Deploy jobs: "Deploy Application {App} {env}" or "Deploy Application {App} for {env}"
		deployPattern1 := strings.ToLower(fmt.Sprintf("Deploy Application %s %s", app, envSuffix))
		deployPattern2 := strings.ToLower(fmt.Sprintf("Deploy Application %s for %s", app, envSuffix))
		if jobNameLower == deployPattern1 || jobNameLower == deployPattern2 {
			return true
		}

		// Also check "Deploy Application {App} to {env}" pattern
		deployPattern3 := strings.ToLower(fmt.Sprintf("Deploy Application %s to %s", app, envSuffix))
		if jobNameLower == deployPattern3 {
			return true
		}
	}

	return false
}

// checkPipelineStatus fetches MR and pipeline status from GitLab API
func (m *model) checkPipelineStatus() tea.Cmd {
	return func() tea.Msg {
		if m.releaseState == nil || m.creds == nil {
			return pipelineStatusMsg{err: fmt.Errorf("invalid state")}
		}

		client := NewGitLabClient(m.creds.GitLabURL, m.creds.Token)
		status := &PipelineStatus{}

		// Step 1: Fetch MR status
		mr, err := client.GetMergeRequestStatus(m.releaseState.ProjectID, m.releaseState.CreatedMRIID)
		if err != nil {
			status.Error = err
			return pipelineStatusMsg{status: status, err: err}
		}

		// Check if MR is merged
		if mr.State != "merged" {
			status.Stage = PipelineStageWaitingForMerge
			status.MRMerged = false
			return pipelineStatusMsg{status: status}
		}

		status.MRMerged = true

		// Step 2: Fetch pipelines for the merge commit
		// After MR is merged, pipelines run on target branch, not associated with MR directly
		var pipelines []Pipeline
		if mr.MergeCommitSHA != "" {
			pipelines, err = client.GetPipelinesByCommit(m.releaseState.ProjectID, mr.MergeCommitSHA)
			if err != nil {
				status.Error = err
				return pipelineStatusMsg{status: status, err: err}
			}
		}

		// Fallback: try MR pipelines API if no pipelines found by commit
		if len(pipelines) == 0 {
			pipelines, err = client.GetMergeRequestPipelines(m.releaseState.ProjectID, m.releaseState.CreatedMRIID)
			if err != nil {
				status.Error = err
				return pipelineStatusMsg{status: status, err: err}
			}
		}

		// No pipelines yet
		if len(pipelines) == 0 {
			status.Stage = PipelineStageWaitingForStart
			return pipelineStatusMsg{status: status}
		}

		// Get the latest pipeline (first in the list)
		latestPipeline := pipelines[0]
		status.PipelineID = latestPipeline.ID
		status.PipelineWebURL = latestPipeline.WebURL
		status.PipelineState = latestPipeline.Status

		// Step 3: Fetch pipeline jobs to track specific Package/Deploy jobs
		jobs, err := client.GetPipelineJobs(m.releaseState.ProjectID, latestPipeline.ID)
		if err != nil {
			status.Error = err
			return pipelineStatusMsg{status: status, err: err}
		}

		// Get the job suffix for this environment
		envSuffix := getEnvJobSuffix(m.releaseState.Environment.BranchName)

		// Count relevant jobs by status
		for _, job := range jobs {
			if !isRelevantJob(job.Name, envSuffix) {
				continue
			}

			status.TotalJobs++

			switch job.Status {
			case "success":
				status.CompletedJobs++
			case "failed", "canceled":
				status.FailedJobs++
			case "pending", "running", "preparing", "waiting_for_resource":
				status.RunningJobs++
			case "created", "manual":
				// "created" = job exists but not yet scheduled; "manual" = awaiting manual trigger
				// Neither means the job is actively running
			case "skipped":
				// Skipped jobs don't count towards failure, but keep them in total
				// to maintain consistent denominator (e.g., "1/2 failed" not "1/1")
			}
		}

		// Determine stage based on job statuses
		if status.TotalJobs == 0 {
			// No relevant jobs found yet - waiting for pipeline to start properly
			status.Stage = PipelineStageWaitingForStart
		} else if status.CompletedJobs == 0 && status.FailedJobs == 0 && status.RunningJobs == 0 {
			// All relevant jobs are manual (not triggered yet) - waiting for start
			status.Stage = PipelineStageWaitingForStart
		} else if status.FailedJobs > 0 {
			// Any failed job means pipeline failed
			status.Stage = PipelineStageFailed
		} else if status.CompletedJobs == status.TotalJobs {
			// All jobs completed successfully
			status.Stage = PipelineStageCompleted
		} else {
			// Jobs still running
			status.Stage = PipelineStageRunning
		}

		return pipelineStatusMsg{status: status}
	}
}

// sendPipelineNotification sends a macOS notification for pipeline completion
// Configure Script Editor to use "Alerts" style in System Preferences > Notifications for persistent notifications
func (m *model) sendPipelineNotification(success bool) {
	var title, message, sound string

	if success {
		title = "✅ Release Pipeline Succeeded"
		message = fmt.Sprintf("%s deployed to %s (%d/%d jobs)",
			m.releaseState.TagName,
			m.releaseState.Environment.Name,
			m.pipelineStatus.CompletedJobs,
			m.pipelineStatus.TotalJobs)
		sound = "Glass"
	} else {
		title = "❌ Release Pipeline Failed"
		message = fmt.Sprintf("%s to %s (%d/%d jobs failed)",
			m.releaseState.TagName,
			m.releaseState.Environment.Name,
			m.pipelineStatus.FailedJobs,
			m.pipelineStatus.TotalJobs)
		sound = "Sosumi"
	}

	script := fmt.Sprintf(`display notification %q with title %q sound name %q`,
		message, title, sound)
	exec.Command("osascript", "-e", script).Start()
}

// handlePipelineStatus processes the pipeline status update
func (m *model) handlePipelineStatus(msg pipelineStatusMsg) (tea.Model, tea.Cmd) {
	if !m.pipelineObserving {
		return m, nil
	}

	// Update status even on error (to show check failed)
	if msg.status != nil {
		m.pipelineStatus = msg.status
	}

	// Update buttons to show/hide Open Pipeline button based on pipeline URL
	m.updateReleaseButtons()

	// Check if we reached a terminal state — only stop on completion
	if m.pipelineStatus != nil {
		switch m.pipelineStatus.Stage {
		case PipelineStageCompleted:
			m.pipelineFailNotified = false
			m.sendPipelineNotification(true)
			return m, nil
		case PipelineStageFailed:
			// Don't stop observing on failure — user may restart the pipeline.
			// Send notification only once per failure episode.
			if !m.pipelineFailNotified {
				m.pipelineFailNotified = true
				m.sendPipelineNotification(false)
			}
		default:
			// Reset failure notification flag when pipeline recovers (e.g. restarted)
			m.pipelineFailNotified = false
		}
	}

	// Continue polling
	return m, m.pipelineTick()
}

// renderPipelineStatus returns the formatted pipeline status line
func (m *model) renderPipelineStatus() string {
	if m.pipelineStatus == nil {
		return ""
	}

	status := m.pipelineStatus

	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	var line string

	switch status.Stage {
	case PipelineStageLoading:
		line = m.spinner.View() + " " + loadingStyle.Render("Loading merge request status...")
	case PipelineStageWaitingForMerge:
		line = m.spinner.View() + " " + loadingStyle.Render("[1/4] Waiting for manual merge...")
	case PipelineStageWaitingForStart:
		line = m.spinner.View() + " " + loadingStyle.Render("[2/4] Waiting for manual pipeline start...")
	case PipelineStageRunning:
		progressText := ""
		if status.TotalJobs > 0 {
			progressText = fmt.Sprintf(" (%d/%d jobs)", status.CompletedJobs, status.TotalJobs)
		}
		line = m.spinner.View() + " " + loadingStyle.Render("[3/4] Waiting for pipeline completion..."+progressText)
	case PipelineStageCompleted:
		jobsText := ""
		if status.TotalJobs > 0 {
			jobsText = fmt.Sprintf(" (%d jobs)", status.TotalJobs)
		}
		line = successStyle.Render("[4/4] Pipeline successfully completed" + jobsText)
	case PipelineStageFailed:
		failText := "[4/4] Pipeline failed"
		if status.FailedJobs > 0 {
			failText = fmt.Sprintf("[4/4] Pipeline failed (%d/%d jobs failed)", status.FailedJobs, status.TotalJobs)
		}
		line = m.spinner.View() + " " + failedStyle.Render(failText) + " " + loadingStyle.Render("(observing)")
	}

	// Add error indicator if there was a check failure
	if status.Error != nil && status.Stage != PipelineStageCompleted && status.Stage != PipelineStageFailed {
		line += " " + loadingStyle.Render("(check failed)")
	}

	return line
}
