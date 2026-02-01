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
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
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

	case ReleaseButtonPushRoot:
		return m.startPushRootBranches()

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

	// Check if hint text wraps (for ReleaseStepWaitForRootPush)
	hintExtraLines := 0
	if m.releaseState != nil && m.releaseState.CurrentStep == ReleaseStepWaitForRootPush {
		hintText := m.renderRootPushHint()
		hintWidth := lipgloss.Width(hintText)
		if hintWidth > width {
			// Calculate how many extra lines the hint takes when wrapped
			hintExtraLines = (hintWidth - 1) / width
		}
	}

	// Skip empty line margin when status takes more than minimum 3 lines or hint text wraps
	if statusLines > 3 || hintExtraLines > 0 {
		sb.WriteString("\n")
	} else {
		sb.WriteString("\n\n")
	}

	// Horizontal line
	line := releaseHorizontalLineStyle.Render(strings.Repeat("─", width))
	sb.WriteString(line)
	sb.WriteString("\n")

	// Calculate dynamic viewport height based on status height
	// Base calculation: height - 11 (status 3 + top padding 1 + empty after status 1 + line 1 + border 2 + empty before buttons 1 + buttons 1 = 10, plus 1 empty at bottom)
	// If status takes more than 3 lines or hint wraps, shrink viewport accordingly
	extraStatusLines := statusLines - 3
	viewportHeight := height - 11 - extraStatusLines - hintExtraLines
	if statusLines > 3 || hintExtraLines > 0 {
		viewportHeight++ // Compensate for skipped empty line margin
	}
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
		status = fmt.Sprintf("Release is %s\nNow do next step - create its merge request to %s\nPress %s",
			releaseSuccessGreenStyle.Render(" SUCCESSFULLY COMPOSED "),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name),
			releaseTextActiveStyle.Render("Create MR to "+state.Environment.Name),
		)

	case ReleaseStepPushAndCreateMR:
		status = fmt.Sprintf("%s %s %s\nPushing env branch and creating merge request to %s...",
			m.spinner.View(),
			getReleaseEnvStyle(state.Environment.Name).Render("RELEASING"),
			releasePercentStyle.Render("99%"),
			getReleaseEnvStyle(state.Environment.Name).Render(state.Environment.Name))

	case ReleaseStepWaitForRootPush:
		hintText := m.renderRootPushHint()
		status = fmt.Sprintf("Merge request is %s\nNow push release branch to root and develop:\n%s",
			releaseSuccessGreenStyle.Render(" CREATED "),
			hintText,
		)

	case ReleaseStepPushRootBranches:
		status = fmt.Sprintf("%s %s %s\nPushing root branches...",
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

	// Add recovery metadata to terminal output
	m.appendRecoveryMetadata(workDir, state)

	m.initReleaseScreen()

	// Save initial state (includes recovery metadata in terminal output)
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

			// Step 4.3: Exclude files - restore from env branch or remove if not exists
			excluded, _ := GetExcludedFiles(workDir, patterns)
			var output3 string
			if len(excluded) > 0 {
				for _, file := range excluded {
					output3 += fmt.Sprintf("Excluding: %s\n", releaseOrangeStyle.Render(file))
					// Try to restore file from environment branch (keeps it unchanged)
					checkoutCmd := fmt.Sprintf("git checkout origin/%s -- %q 2>/dev/null", state.Environment.BranchName, file)
					_, checkoutErr := executor.RunCommand(checkoutCmd)
					if checkoutErr != nil {
						// File doesn't exist in env branch - remove it completely
						executor.RunCommand(fmt.Sprintf("rm -rf %q", file))
						executor.RunCommand(fmt.Sprintf("git rm -rf --cached %q 2>/dev/null || true", file))
					}
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

				// Merge source branch to root
				mergeRootCmd := cmds.StepMergeToRoot()
				output2, err2 := executor.RunCommand(mergeRootCmd)
				if err2 != nil {
					return releaseStepCompleteMsg{step: step, err: err2, output: output + output2}
				}
				output += output2

				// Create tag on root (after merge)
				tagCmd := fmt.Sprintf("git tag %s", tagName)
				output3, err3 := executor.RunCommand(tagCmd)
				if err3 != nil {
					return releaseStepCompleteMsg{step: step, err: err3, output: output + output3}
				}
				output += output3

				// Push root with tags
				pushRootCmd := "git push origin root --tags"
				output4, err4 := executor.RunCommand(pushRootCmd)
				if err4 != nil {
					return releaseStepCompleteMsg{step: step, err: err4, output: output + output4}
				}
				output += output4

				// Merge root to develop and push
				mergeDevelopCmd := cmds.StepMergeToDevelop()
				output5, err5 := executor.RunCommand(mergeDevelopCmd)
				if err5 != nil {
					return releaseStepCompleteMsg{step: step, err: err5, output: output + output5}
				}
				output += output5
			} else {
				// No RootMerge: tag source branch, push with tags

				// Create tag on source branch
				tagCmd := fmt.Sprintf("git tag %s", tagName)
				output1, err1 := executor.RunCommand(tagCmd)
				if err1 != nil {
					return releaseStepCompleteMsg{step: step, err: err1, output: output1}
				}

				// Push source branch with tags
				pushCmd := fmt.Sprintf("git push origin %s --tags", state.SourceBranch)
				output2, err2 := executor.RunCommand(pushCmd)
				if err2 != nil {
					return releaseStepCompleteMsg{step: step, err: err2, output: output1 + output2}
				}

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
		// After commit, wait for user to click "Create MR" button
		// Push will happen when user clicks the button
		nextStep = ReleaseStepWaitForMR
		state.CurrentStep = nextStep

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

	case ReleaseStepPushRootBranches:
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
		// Focus on "Push root branches" button (index 1: Abort=0, PushRoot=1)
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
		m.updateReleaseButtons()
		return m, nil
	}

	m.releaseState.CreatedMRURL = msg.url
	m.releaseState.CreatedMRIID = msg.iid
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

	// Focus on "Push root branches" button (index 1: Abort=0, PushRoot=1)
	m.releaseButtonIndex = 1

	// Open MR URL in Safari (with fallback to default browser)
	return m, openInSafariWithFallback(msg.url)
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

		// Delete remote branches if requested
		if deleteRemote {
			cmds := NewReleaseCommandsWithSourceBranch(workDir, version, &m.releaseState.Environment, nil, nil, m.releaseState.SourceBranch, m.releaseState.SourceBranchIsRemote)
			exec := NewGitExecutor(workDir, nil)

			// Delete env release branch (e.g. release/rpb-1.0.0-dev)
			envBranchCmd := fmt.Sprintf("cd %q && git push origin --delete %s", workDir, cmds.EnvReleaseBranch())
			exec.RunCommand(envBranchCmd)

			// Delete source/root branch only if it was newly created (not pre-existing on remote)
			if !m.releaseState.SourceBranchIsRemote {
				rootBranchCmd := fmt.Sprintf("cd %q && git push origin --delete %s", workDir, cmds.ReleaseRootBranch())
				exec.RunCommand(rootBranchCmd)
			}

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
			textStyle.Render(", finally all pushed to remote"),
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

	m.releaseState.CurrentStep = ReleaseStepPushRootBranches
	m.updateReleaseButtons()
	SaveReleaseState(m.releaseState)

	m.releaseRunning = true
	return m, tea.Batch(m.spinner.Tick, m.executeReleaseStep(ReleaseStepPushRootBranches))
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
		// Focus on "Push root branches" button (index 1: Abort=0, PushRoot=1)
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
