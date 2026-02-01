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
	case "u":
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

	// Build five sidebar (pass total rendered height)
	sidebar := m.renderFiveSidebar(sidebarW, totalHeight)

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
		helpText = "u: go back • /: commands • C+c: quit"
	} else {
		helpText = "↓/↑/j/k: scroll • enter: release • u: go back • /: commands • C+c: quit"
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

// renderFiveSidebar renders all 5 sidebars: MRs, Environment, Version, Source branch, and Root merge
func (m model) renderFiveSidebar(width int, availableHeight int) string {
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
	// We have 5 boxes, so total border overhead is 10 lines
	totalContentHeight := availableHeight - 10

	// Minimum heights for sidebars
	minEnvContentHeight := 4          // title + blank + env + padding
	minVersionContentHeight := 4      // title + blank + version + padding
	minSourceBranchContentHeight := 4 // title + blank + branch + padding
	minRootMergeContentHeight := 4    // title + blank + status + padding
	mrsHeaderLines := 3               // title line + blank line + some spacing

	// Calculate ideal MRs content height
	idealMrsContentHeight := mrsHeaderLines + len(branches)

	// Target: each sidebar gets 1/5 of content height
	fifthContentHeight := totalContentHeight / 5

	// Start with equal distribution
	mrsContentHeight := fifthContentHeight
	envContentHeight := fifthContentHeight
	versionContentHeight := fifthContentHeight
	sourceBranchContentHeight := fifthContentHeight
	rootMergeContentHeight := totalContentHeight - mrsContentHeight - envContentHeight - versionContentHeight - sourceBranchContentHeight

	// If branches don't fit in 1/5, try to expand MRs sidebar
	if idealMrsContentHeight > fifthContentHeight {
		// Calculate max MRs content height (leaving minimum for other sidebars)
		maxMrsContentHeight := totalContentHeight - minEnvContentHeight - minVersionContentHeight - minSourceBranchContentHeight - minRootMergeContentHeight
		if idealMrsContentHeight <= maxMrsContentHeight {
			// All branches fit if we shrink other sidebars to minimum
			mrsContentHeight = idealMrsContentHeight
			// Distribute remaining space equally between other sidebars
			remaining := totalContentHeight - mrsContentHeight
			envContentHeight = remaining / 4
			versionContentHeight = remaining / 4
			sourceBranchContentHeight = remaining / 4
			rootMergeContentHeight = remaining - envContentHeight - versionContentHeight - sourceBranchContentHeight
		} else {
			// Even with minimum sidebars, not all branches fit
			mrsContentHeight = maxMrsContentHeight
			envContentHeight = minEnvContentHeight
			versionContentHeight = minVersionContentHeight
			sourceBranchContentHeight = minSourceBranchContentHeight
			rootMergeContentHeight = minRootMergeContentHeight
		}
	}

	// Render MRs sidebar section (pass content height, not total height)
	mrsSidebar := m.renderMRsSidebarSection(width, mrsContentHeight, branches)

	// Render Environment sidebar section
	envSidebar := m.renderEnvSidebarSection(width, envContentHeight)

	// Render Version sidebar section
	versionSidebar := m.renderVersionSidebarSection(width, versionContentHeight)

	// Render Source branch sidebar section
	sourceBranchSidebar := m.renderSourceBranchSidebarSection(width, sourceBranchContentHeight)

	// Render Root merge sidebar section
	rootMergeSidebar := m.renderRootMergeSidebarSection(width, rootMergeContentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, mrsSidebar, envSidebar, versionSidebar, sourceBranchSidebar, rootMergeSidebar)
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
			Foreground(lipgloss.Color("189")).
			Bold(true)
		sb.WriteString(versionStyle.Render(version))
	}

	// Wrap in bordered box (Height is content height, border adds 2 more lines)
	content := sb.String()
	borderedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
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

	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	envColor := "220" // default
	if m.selectedEnv != nil {
		envColor = getEnvBranchColor(m.selectedEnv.Name)
	}
	envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(envColor))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("62"))

	var sb strings.Builder
	sb.WriteString(textStyle.Render("We are almost ready to release ") + envStyle.Render(version) + textStyle.Render(" of selected MRs to ") + envStyle.Render(envName) + textStyle.Render(" environment,"))
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

	// Build tag name for step 9
	tagName := fmt.Sprintf("%s-%s-v%d", strings.ToLower(envName), version, vNumber)

	// Build step 8-9 based on root merge selection
	step8And9 := ""
	if m.rootMergeSelection {
		step8And9 = fmt.Sprintf(`8. Open new environment MR in browser for manual approval and pipeline execution

9. ~~Confirm~~ and push **%s** to remote

10. [ Merge ]()**%s** to **root**, tag **root** as **%s** and push it to remote

11. [ Merge ]()**root** to **develop** and push it to remote`, sourceBranch, sourceBranch, tagName)
	} else {
		step8And9 = fmt.Sprintf(`8. Open new environment MR in browser for manual approval and pipeline execution

9. ~~Confirm~~ and tag **%s** as **%s**, then push it to remote`, sourceBranch, tagName)
	}

	markdown := fmt.Sprintf(`[ We are ready ]()to release **%s v%d** of selected MRs to **%s** environment!

This release will go through the following steps:

%s

2. Merge selected MRs branches to it and ~~resolve conflicts~~ with your participation

3. Create environment release branch **release/rpb-%s-%s** from current **%s**

4. Copy new composed MRs' content from **%s** via `+"`git checkout -- .`"+` to **release/rpb-%s-%s** as a new independent ordinal commit with its next number **v%d** from previous within version **%s**

5. Exclude from release commit files matching patterns from app settings (restore from env branch or remove)

6. ~~Confirm~~ and push **release/rpb-%s-%s** to remote

7. Create new merge request from **release/rpb-%s-%s** to **%s**

%s

*ATTENTION!* ~~If there are existing local branches under mentioned names~~ *%s* ~~or~~ *release/rpb‑%s‑%s*~~, then they will be removed and recreated with pointer at current root or remote source branch and current environment branch respectively~~

If you agree, press enter and release it.
`,
		version, vNumber, envName,
		step1Text,
		version, envBranch, envBranch,
		sourceBranch, version, envBranch, vNumber, version,
		version, envBranch,
		version, envBranch, envBranch,
		step8And9,
		sourceBranchNB, versionNB, envBranchNB,
	)

	style := styles.DarkStyleConfig
	envColor := "226" // default
	if m.selectedEnv != nil {
		envColor = getEnvBranchColor(m.selectedEnv.Name)
		if m.selectedEnv.Name == "PROD" {
			envColor = "203"
		}
	}
	style.Document.StylePrimitive.Color = stringPtr("189")
	style.Strong.Color = stringPtr(envColor)
	style.Code.BackgroundColor = stringPtr("237")
	style.Code.Color = stringPtr("252")
	style.Emph.Color = stringPtr("203")
	style.Emph.Italic = boolPtr(false)
	style.Strikethrough.Color = stringPtr("9")
	style.Strikethrough.CrossedOut = boolPtr(false)
	style.LinkText.Color = stringPtr("255")
	style.LinkText.BackgroundColor = stringPtr("62")
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
