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

// initConfirmViewport initializes the confirmation viewport
func (m *model) initConfirmViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4
	contentHeight := m.height - 4

	// Button takes 2 lines: 1 margin top + 1 button
	buttonHeight := 2
	viewportHeight := contentHeight - buttonHeight

	m.confirmViewport = viewport.New(contentWidth-4, viewportHeight)
	m.confirmViewport.SetContent(m.renderConfirmMarkdown(contentWidth - 4))
}

// updateConfirm handles key events on the confirmation screen
func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		// Go back to root merge screen, restore button index based on selection
		m.screen = screenRootMerge
		if m.rootMergeSelection {
			m.rootMergeButtonIndex = 0
		} else {
			m.rootMergeButtonIndex = 1
		}
		return m, nil
	case "enter":
		// Block release when source branch check is in progress
		if m.sourceBranchRemoteStatus == "checking" {
			return m, nil
		}
		// Start the release process
		return m.startRelease()
	}

	// Handle viewport scrolling
	var cmd tea.Cmd
	m.confirmViewport, cmd = m.confirmViewport.Update(msg)
	return m, cmd
}

// viewConfirm renders the confirmation screen
func (m model) viewConfirm() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Content height (same as other screens)
	contentHeight := m.height - 4

	// Total rendered height for sidebar/content (content height + 2 for border)
	totalHeight := contentHeight + 2

	// Build six sidebar (pass total rendered height)
	sidebar := m.renderSixSidebar(sidebarW, totalHeight)

	// Build content - show preparing screen when checking, otherwise show confirmation
	var contentContent string
	if m.sourceBranchRemoteStatus == "checking" {
		contentContent = m.renderConfirmPreparing(contentWidth-4, contentHeight)
	} else {
		contentContent = m.renderConfirmContent(contentWidth-4, contentHeight)
	}

	// Render content with border
	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	var helpText string
	if m.sourceBranchRemoteStatus == "checking" {
		helpText = "C+q: back • /: commands • C+c: quit"
	} else {
		helpText = "↓/↑/j/k: scroll • enter: release • C+q: back • /: commands • C+c: quit"
	}
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderTripleSidebar renders MRs, Environment, and Version sidebars stacked vertically
func (m model) renderTripleSidebar(width int, availableHeight int) string {
	// Collect branch names from selected MRs or release state
	var branches []string
	if m.releaseState != nil && len(m.releaseState.MRBranches) > 0 {
		// Use branches from release state (for resume scenario)
		branches = m.releaseState.MRBranches
	} else {
		// Get branches from list
		items := m.list.Items()
		for _, item := range items {
			if mr, ok := item.(mrListItem); ok {
				if m.selectedMRs[mr.MR().IID] {
					branches = append(branches, mr.MR().SourceBranch)
				}
			}
		}
	}

	// Each bordered box adds 2 lines for top/bottom border
	// We have 3 boxes, so total border overhead is 6 lines
	// Available content height = availableHeight - 6
	totalContentHeight := availableHeight - 6

	// Minimum heights for env and version sidebars
	minEnvContentHeight := 4     // title + blank + env + padding
	minVersionContentHeight := 4 // title + blank + version + padding
	mrsHeaderLines := 3          // title line + blank line + some spacing

	// Calculate ideal MRs content height
	idealMrsContentHeight := mrsHeaderLines + len(branches)

	// Target: each sidebar gets 1/3 of content height
	thirdContentHeight := totalContentHeight / 3

	// Start with equal distribution
	mrsContentHeight := thirdContentHeight
	envContentHeight := thirdContentHeight
	versionContentHeight := totalContentHeight - mrsContentHeight - envContentHeight

	// If branches don't fit in 1/3, try to expand MRs sidebar
	if idealMrsContentHeight > thirdContentHeight {
		// Calculate max MRs content height (leaving minimum for env and version sidebars)
		maxMrsContentHeight := totalContentHeight - minEnvContentHeight - minVersionContentHeight
		if idealMrsContentHeight <= maxMrsContentHeight {
			// All branches fit if we shrink env and version sidebars to minimum
			mrsContentHeight = idealMrsContentHeight
			// Distribute remaining space equally between env and version
			remaining := totalContentHeight - mrsContentHeight
			envContentHeight = remaining / 2
			versionContentHeight = remaining - envContentHeight
		} else {
			// Even with minimum sidebars, not all branches fit
			mrsContentHeight = maxMrsContentHeight
			envContentHeight = minEnvContentHeight
			versionContentHeight = minVersionContentHeight
		}
	}

	// Render MRs sidebar section (pass content height, not total height)
	mrsSidebar := m.renderMRsSidebarSection(width, mrsContentHeight, branches)

	// Render Environment sidebar section
	envSidebar := m.renderEnvSidebarSection(width, envContentHeight)

	// Render Version sidebar section
	versionSidebar := m.renderVersionSidebarSection(width, versionContentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, mrsSidebar, envSidebar, versionSidebar)
}

// renderSixSidebar renders all 6 sidebars: MRs, Environment, Version, Source branch, Env merge, and Root merge
func (m model) renderSixSidebar(width int, availableHeight int) string {
	var branches []string
	if m.releaseState != nil && len(m.releaseState.MRBranches) > 0 {
		branches = m.releaseState.MRBranches
	} else {
		items := m.list.Items()
		for _, item := range items {
			if mr, ok := item.(mrListItem); ok {
				if m.selectedMRs[mr.MR().IID] {
					branches = append(branches, mr.MR().SourceBranch)
				}
			}
		}
	}

	totalContentHeight := availableHeight - 12

	otherCount := 5 // env, version, source branch, env merge, root merge
	otherHeight := totalContentHeight / (otherCount + 1)
	mrsContentHeight := totalContentHeight - otherCount*otherHeight
	envContentHeight := otherHeight
	versionContentHeight := otherHeight
	sourceBranchContentHeight := otherHeight
	envMergeContentHeight := otherHeight
	rootMergeContentHeight := otherHeight

	mrsSidebar := m.renderMRsSidebarSection(width, mrsContentHeight, branches)
	envSidebar := m.renderEnvSidebarSection(width, envContentHeight)
	versionSidebar := m.renderVersionSidebarSection(width, versionContentHeight)
	sourceBranchSidebar := m.renderSourceBranchSidebarSection(width, sourceBranchContentHeight)
	envMergeSidebar := m.renderEnvMergeSidebarSection(width, envMergeContentHeight)
	rootMergeSidebar := m.renderRootMergeSidebarSection(width, rootMergeContentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, mrsSidebar, envSidebar, versionSidebar, sourceBranchSidebar, envMergeSidebar, rootMergeSidebar)
}

// renderVersionSidebarSection renders the Version sidebar block with border
// height parameter is the content height (excluding border)
func (m model) renderVersionSidebarSection(width int, contentHeight int) string {
	var sb strings.Builder

	sb.WriteString(envTitleStepStyle.Render("[3]") +
		envTitleStyle.Render(" Version "))
	sb.WriteString("\n\n")

	// Show version number (fall back to release state if input is empty)
	version := m.versionInput.Value()
	if version == "" && m.releaseState != nil {
		version = m.releaseState.Version
	}
	if version != "" {
		versionStyle := lipgloss.NewStyle().
			Foreground(currentTheme.Foreground).
			Bold(true)
		sb.WriteString(versionStyle.Render(version))
	}

	// Wrap in bordered box (Height is content height, border adds 2 more lines)
	content := sb.String()
	borderedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		Bold(true).
		Padding(0, 1).
		Width(width).
		Height(contentHeight).
		Render(content)

	return borderedBox
}

// renderConfirmContent renders the confirmation content area with viewport and button
func (m model) renderConfirmContent(width int, availableHeight int) string {
	// Render viewport
	viewportContent := m.confirmViewport.View()

	// Render button with margins
	button := buttonActiveStyle.Render("Release it")
	buttonLine := "\n" + lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(button)

	return viewportContent + buttonLine
}

// renderConfirmPreparing renders the preparing screen while source branch check is in progress
func (m model) renderConfirmPreparing(width int, height int) string {
	version := m.versionInput.Value()
	envName := ""
	if m.selectedEnv != nil {
		envName = m.selectedEnv.Name
	}
	sourceBranch := m.sourceBranchInput.Value()
	if sourceBranch == "" && m.releaseState != nil {
		sourceBranch = m.releaseState.SourceBranch
	}

	textStyle := lipgloss.NewStyle().Foreground(currentTheme.Foreground)
	envColor := string(currentTheme.Warning) // default
	if m.selectedEnv != nil {
		envColor = getEnvBranchColor(m.selectedEnv.Name)
	}
	envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(envColor))
	highlightStyle := lipgloss.NewStyle().Foreground(currentTheme.AccentForeground).Background(currentTheme.Accent)

	var sb strings.Builder
	mrSuffix := " of selected MRs"
	if len(m.selectedMRs) == 0 {
		mrSuffix = ""
	}
	sb.WriteString(textStyle.Render("We are almost ready to release ") + envStyle.Render(version) + textStyle.Render(mrSuffix+" to ") + envStyle.Render(envName) + textStyle.Render(" environment,"))
	sb.WriteString("\n")
	sb.WriteString(textStyle.Render("but there are some ") + highlightStyle.Render(" preparing of important details ") + textStyle.Render(":"))
	sb.WriteString("\n\n")
	sb.WriteString(m.spinner.View() + " " + textStyle.Render("Checking remote branch ") + envStyle.Render(sourceBranch) + textStyle.Render("..."))

	// Wrap with padding
	content := lipgloss.NewStyle().
		Padding(1, 2).
		Render(sb.String())

	return content
}

// renderConfirmMarkdown renders the confirmation markdown content
func (m model) renderConfirmMarkdown(width int) string {
	version := m.versionInput.Value()
	envName := ""
	envBranch := ""
	if m.selectedEnv != nil {
		envName = m.selectedEnv.Name
		envBranch = m.selectedEnv.BranchName
	}

	// Get source branch name
	sourceBranch := m.sourceBranchInput.Value()
	if sourceBranch == "" && m.releaseState != nil {
		sourceBranch = m.releaseState.SourceBranch
	}

	// Get next v-number for display
	vNumber := 1
	if workDir, err := FindProjectRoot(); err == nil && envBranch != "" {
		if n, err := GetNextVersionNumber(workDir, envBranch, version); err == nil {
			vNumber = n
		}
	}

	// Determine step 1 text based on whether source branch exists remotely
	// Note: when "checking", the spinner is shown above viewport in renderConfirmContent
	step1Text := ""
	sourceBranchExists := m.sourceBranchRemoteStatus == "exists-same" || m.sourceBranchRemoteStatus == "exists-diff" || m.sourceBranchRemoteStatus == "exists"
	if sourceBranchExists {
		step1Text = fmt.Sprintf("1. Use remote branch **%s** as cumulative one", sourceBranch)
	} else {
		// Default to "Create" for new branches or when still checking
		step1Text = fmt.Sprintf("1. [ Create cumulative branch ]()**%s** from current root", sourceBranch)
	}

	// Create non-breaking versions of branch names for ATTENTION line (prevents word wrap at hyphens)
	nbHyphen := "‑" // U+2011 non-breaking hyphen
	sourceBranchNB := strings.ReplaceAll(sourceBranch, "-", nbHyphen)
	versionNB := strings.ReplaceAll(version, "-", nbHyphen)
	envBranchNB := strings.ReplaceAll(envBranch, "-", nbHyphen)

	// Build tag name
	tagName := fmt.Sprintf("%s-%s-v%d", strings.ToLower(envName), version, vNumber)

	hasMRs := len(m.selectedMRs) > 0

	// Build steps with dynamic numbering (step 1 is always the cumulative branch)
	stepNum := 2

	// Step: merge MR branches (only when MRs are selected)
	mergeStep := ""
	if hasMRs {
		mergeStep = fmt.Sprintf("\n\n%d. Merge selected MRs branches to it and ~~resolve conflicts~~ with your participation", stepNum)
		stepNum++
	}

	// Step: create env release branch
	envStep := fmt.Sprintf(`%d. Create environment release branch **release/rpb-%s-%s** from current **%s**`, stepNum, version, envBranch, envBranch)
	stepNum++

	// Step: copy content / merge
	step4And5 := ""
	if m.envMergeSelection == 1 {
		// Regular merge mode
		step4And5 = fmt.Sprintf(`%d. Merge **%s** to **release/rpb-%s-%s** via regular git merge (may require ~~conflict resolution~~)`,
			stepNum, sourceBranch, version, envBranch)
		stepNum++
	} else {
		// Squash merge mode (default)
		copyDesc := "Copy new composed MRs' content"
		if !hasMRs {
			copyDesc = fmt.Sprintf("Copy **%s** content", sourceBranch)
		} else {
			copyDesc = fmt.Sprintf("Copy new composed MRs' content from **%s**", sourceBranch)
		}
		step4And5 = fmt.Sprintf(`%d. %s via `+"`git checkout -- .`"+` to **release/rpb-%s-%s** as a new independent ordinal commit with its next number **v%d** from previous within version **%s**

%d. Exclude from release commit files matching patterns from app settings (restore from env branch or remove)`,
			stepNum, copyDesc, version, envBranch, vNumber, version,
			stepNum+1)
		stepNum += 2
	}

	// Step: push env branch
	pushStep := fmt.Sprintf(`%d. ~~Confirm~~ and push **release/rpb-%s-%s** to remote`, stepNum, version, envBranch)
	stepNum++

	// Step: create MR
	mrStep := fmt.Sprintf(`%d. Create new merge request from **release/rpb-%s-%s** to **%s**`, stepNum, version, envBranch, envBranch)
	stepNum++

	// Steps: open MR, push root branches
	step8And9 := ""
	if m.rootMergeSelection {
		step8And9 = fmt.Sprintf(`%d. Open new environment MR in browser for manual approval and pipeline execution

%d. ~~Confirm~~ and [ merge ]()**%s** to **root**, tag **root** as **%s** and push it to remote

%d. [ Merge ]()**root** to **develop** and push it to remote`,
			stepNum, stepNum+1, sourceBranch, tagName, stepNum+2)
	} else {
		step8And9 = fmt.Sprintf(`%d. Open new environment MR in browser for manual approval and pipeline execution

%d. ~~Confirm~~ and tag **%s** as **%s**, then push it to remote`,
			stepNum, stepNum+1, sourceBranch, tagName)
	}

	// Build header
	header := ""
	if hasMRs {
		header = fmt.Sprintf(`[ We are ready ]()to release **%s v%d** of selected MRs to **%s** environment!`, version, vNumber, envName)
	} else {
		header = fmt.Sprintf(`[ We are ready ]()to release **%s v%d** to **%s** environment!`, version, vNumber, envName)
	}

	markdown := fmt.Sprintf(`%s

This release will go through the following steps:

%s%s

%s

%s

%s

%s

%s

*ATTENTION!* ~~If there are existing local branches under mentioned names~~ *%s* ~~or~~ *release/rpb‑%s‑%s*~~, then they will be removed and recreated with pointer at current root or remote source branch and current environment branch respectively~~

If you agree, press enter and release it.
`,
		header,
		step1Text, mergeStep,
		envStep,
		step4And5,
		pushStep,
		mrStep,
		step8And9,
		sourceBranchNB, versionNB, envBranchNB,
	)

	style := styles.DarkStyleConfig
	envColor := string(currentTheme.Warning) // default
	if m.selectedEnv != nil {
		envColor = getEnvBranchColor(m.selectedEnv.Name)
	}
	style.Document.StylePrimitive.Color = stringPtr(string(currentTheme.Foreground))
	style.Strong.Color = stringPtr(envColor)
	style.Code.BackgroundColor = stringPtr(string(currentTheme.Muted))
	style.Code.Color = stringPtr(string(currentTheme.MutedForeground))
	style.Emph.Color = stringPtr(string(currentTheme.Error))
	style.Emph.Italic = boolPtr(false)
	style.Strikethrough.Color = stringPtr(string(currentTheme.Error))
	style.Strikethrough.CrossedOut = boolPtr(false)
	style.LinkText.Color = stringPtr(string(currentTheme.AccentForeground))
	style.LinkText.BackgroundColor = stringPtr(string(currentTheme.Accent))
	style.LinkText.Bold = boolPtr(true)
	style.H1.Prefix = ""

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return markdown
	}

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	return rendered
}
