package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// updateRootMerge handles key events on the root merge screen
func (m model) updateRootMerge(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u":
		// Go back to source branch input
		m.screen = screenSourceBranch
		return m, nil
	case "left", "h":
		if m.rootMergeButtonIndex > 0 {
			m.rootMergeButtonIndex--
		}
		return m, nil
	case "right", "l":
		if m.rootMergeButtonIndex < 1 {
			m.rootMergeButtonIndex++
		}
		return m, nil
	case "tab":
		// Cycle through buttons
		if m.rootMergeButtonIndex < 1 {
			m.rootMergeButtonIndex++
		} else {
			m.rootMergeButtonIndex = 0
		}
		return m, nil
	case "shift+tab":
		// Cycle through buttons in reverse
		if m.rootMergeButtonIndex > 0 {
			m.rootMergeButtonIndex--
		} else {
			m.rootMergeButtonIndex = 1
		}
		return m, nil
	case "enter":
		// Save selection and proceed to confirmation screen
		m.rootMergeSelection = m.rootMergeButtonIndex == 0 // 0 = Yes, 1 = No
		m.screen = screenConfirm
		m.initConfirmViewport()
		return m, nil
	}

	return m, nil
}

// viewRootMerge renders the root merge screen
func (m model) viewRootMerge() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Content height (same as other screens)
	contentHeight := m.height - 4

	// Total rendered height for sidebar/content (content height + 2 for border)
	totalHeight := contentHeight + 2

	// Build quad sidebar (4 sections: MRs, Environment, Version, Source branch)
	sidebar := m.renderQuadSidebar(sidebarW, totalHeight)

	// Build content - root merge selection
	contentContent := m.renderRootMergeContent(contentWidth - 4)

	// Render content with border
	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "tab/h/l: switch • enter: confirm • u: go back • /: commands • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderRootMergeContent renders the root merge content area
func (m model) renderRootMergeContent(width int) string {
	var sb strings.Builder

	// Step title
	sb.WriteString(envTitleStepStyle.Render("[5]") + envTitleStyle.Render(" Root merge "))
	sb.WriteString("\n\n")

	// Prompt
	prompt := "Whether should this release be merged to root branch and then root branch to develop?"
	sb.WriteString(envPromptStyle.Render(prompt))
	sb.WriteString("\n")

	// Show the merge flow with source branch
	sourceBranch := m.sourceBranchInput.Value()
	if sourceBranch == "" && m.releaseState != nil {
		sourceBranch = m.releaseState.SourceBranch
	}
	flowText := sourceBranch + " -> root -> develop"
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("105")).Render(flowText))
	sb.WriteString("\n\n")

	// Buttons
	var yesBtn, noBtn string
	if m.rootMergeButtonIndex == 0 {
		yesBtn = buttonDangerStyle.Render("Yes, merge it")
		noBtn = buttonStyle.Render("No")
	} else {
		yesBtn = buttonStyle.Render("Yes, merge it")
		noBtn = buttonActiveStyle.Render("No")
	}
	sb.WriteString(yesBtn + "  " + noBtn)

	return sb.String()
}

// renderQuadSidebar renders MRs, Environment, Version, and Source branch sidebars stacked vertically
func (m model) renderQuadSidebar(width int, availableHeight int) string {
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
	// We have 4 boxes, so total border overhead is 8 lines
	totalContentHeight := availableHeight - 8

	// Minimum heights for sidebars
	minEnvContentHeight := 4          // title + blank + env + padding
	minVersionContentHeight := 4      // title + blank + version + padding
	minSourceBranchContentHeight := 4 // title + blank + branch + padding
	mrsHeaderLines := 3               // title line + blank line + some spacing

	// Calculate ideal MRs content height
	idealMrsContentHeight := mrsHeaderLines + len(branches)

	// Target: each sidebar gets 1/4 of content height
	quarterContentHeight := totalContentHeight / 4

	// Start with equal distribution
	mrsContentHeight := quarterContentHeight
	envContentHeight := quarterContentHeight
	versionContentHeight := quarterContentHeight
	sourceBranchContentHeight := totalContentHeight - mrsContentHeight - envContentHeight - versionContentHeight

	// If branches don't fit in 1/4, try to expand MRs sidebar
	if idealMrsContentHeight > quarterContentHeight {
		// Calculate max MRs content height (leaving minimum for other sidebars)
		maxMrsContentHeight := totalContentHeight - minEnvContentHeight - minVersionContentHeight - minSourceBranchContentHeight
		if idealMrsContentHeight <= maxMrsContentHeight {
			// All branches fit if we shrink other sidebars to minimum
			mrsContentHeight = idealMrsContentHeight
			// Distribute remaining space equally between other sidebars
			remaining := totalContentHeight - mrsContentHeight
			envContentHeight = remaining / 3
			versionContentHeight = remaining / 3
			sourceBranchContentHeight = remaining - envContentHeight - versionContentHeight
		} else {
			// Even with minimum sidebars, not all branches fit
			mrsContentHeight = maxMrsContentHeight
			envContentHeight = minEnvContentHeight
			versionContentHeight = minVersionContentHeight
			sourceBranchContentHeight = minSourceBranchContentHeight
		}
	}

	// Render MRs sidebar section
	mrsSidebar := m.renderMRsSidebarSection(width, mrsContentHeight, branches)

	// Render Environment sidebar section
	envSidebar := m.renderEnvSidebarSection(width, envContentHeight)

	// Render Version sidebar section
	versionSidebar := m.renderVersionSidebarSection(width, versionContentHeight)

	// Render Source branch sidebar section
	sourceBranchSidebar := m.renderSourceBranchSidebarSection(width, sourceBranchContentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, mrsSidebar, envSidebar, versionSidebar, sourceBranchSidebar)
}

// renderSourceBranchSidebarSection renders the Source branch sidebar block with border
func (m model) renderSourceBranchSidebarSection(width int, contentHeight int) string {
	var sb strings.Builder

	sb.WriteString(envTitleStepStyle.Render("[4]") +
		envTitleStyle.Render(" Source branch "))
	sb.WriteString("\n\n")

	// Show source branch (fall back to release state if input is empty)
	sourceBranch := m.sourceBranchInput.Value()
	if sourceBranch == "" && m.releaseState != nil {
		sourceBranch = m.releaseState.SourceBranch
	}
	if sourceBranch != "" {
		branchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("189")).
			Bold(true)
		// Wrap text if too long (width - 6 for border and padding)
		wrappedLines := wrapText(sourceBranch, width-6)
		for i, line := range wrappedLines {
			sb.WriteString(branchStyle.Render(line))
			if i < len(wrappedLines)-1 {
				sb.WriteString("\n")
			}
		}

		// Show status indicator based on remote check
		sb.WriteString("\n")
		sb.WriteString(m.renderSourceBranchSidebarStatus())
	}

	// Wrap in bordered box
	content := sb.String()
	borderedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(width).
		Height(contentHeight).
		Render(content)

	return borderedBox
}

// renderSourceBranchSidebarStatus renders the status indicator for the sidebar
func (m model) renderSourceBranchSidebarStatus() string {
	rootSameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	rootDiffStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	newBranchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("40"))

	switch m.sourceBranchRemoteStatus {
	case "exists-same":
		return rootSameStyle.Render("-> root")
	case "exists-diff":
		return rootDiffStyle.Render("!= root")
	case "new":
		return newBranchStyle.Render("new branch")
	default:
		return ""
	}
}

// renderRootMergeSidebarSection renders the Root merge sidebar block with border
func (m model) renderRootMergeSidebarSection(width int, contentHeight int) string {
	var sb strings.Builder

	sb.WriteString(envTitleStepStyle.Render("[5]") +
		envTitleStyle.Render(" Root merge "))
	sb.WriteString("\n\n")

	// Show root merge selection (fall back to release state)
	rootMerge := m.rootMergeSelection
	if m.releaseState != nil {
		rootMerge = m.releaseState.RootMerge
	}

	if rootMerge {
		// "Accepted" with color 220
		acceptedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)
		sb.WriteString(acceptedStyle.Render("Accepted"))
	} else {
		// "Skip" with color 189
		skipStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("189"))
		sb.WriteString(skipStyle.Render("Skip"))
	}

	// Wrap in bordered box
	content := sb.String()
	borderedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(width).
		Height(contentHeight).
		Render(content)

	return borderedBox
}
