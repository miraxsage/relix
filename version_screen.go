package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Version validation regex: matches versions like 1.0, 1.0.0, 1.0.0.0
var versionRegex = regexp.MustCompile(`^\d+(\.\d+){1,3}$`)

// Styles for version input screen
var (
	versionInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("189"))
)

// initVersionInput initializes the version text input
func initVersionInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. 1.2.3"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 20
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	return ti
}

// validateVersion checks if the version string is valid
func validateVersion(version string) bool {
	if version == "" {
		return true // Empty is valid (no error shown yet)
	}
	return versionRegex.MatchString(version)
}

// updateVersion handles key events on the version input screen
func (m model) updateVersion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u":
		// Go back to environment selection
		// Sync envSelectIndex with selectedEnv to preserve selection
		if m.selectedEnv != nil {
			for i, env := range m.environments {
				if env.Name == m.selectedEnv.Name {
					m.envSelectIndex = i
					break
				}
			}
		}
		m.screen = screenEnvSelect
		m.versionError = ""
		return m, nil
	case "enter":
		// Validate and proceed
		version := m.versionInput.Value()
		if version == "" {
			m.versionError = "Version is required"
			return m, nil
		}
		if !validateVersion(version) {
			m.versionError = "Invalid version format. Use: X.Y, X.Y.Z, or X.Y.Z.W"
			return m, nil
		}
		// Version is valid - proceed to source branch screen
		m.versionError = ""
		m.screen = screenSourceBranch
		// Initialize source branch input if not done or if empty
		if m.sourceBranchInput.CharLimit == 0 || m.sourceBranchInput.Value() == "" {
			checkCmd := m.initSourceBranchInput()
			return m, checkCmd
		} else {
			// Source branch already exists - check if we need to update version in it
			currentBranch := m.sourceBranchInput.Value()
			oldVersion := m.sourceBranchVersion
			if oldVersion != "" && oldVersion != version && strings.Contains(currentBranch, oldVersion) {
				// Replace old version with new version in the branch name
				newBranch := strings.Replace(currentBranch, oldVersion, version, -1)
				m.sourceBranchInput.SetValue(newBranch)
				m.sourceBranchVersion = version
				// Trigger new remote check for updated branch name
				if m.validateSourceBranch(newBranch) {
					m.sourceBranchRemoteStatus = "checking"
					m.sourceBranchCheckedName = newBranch
					checkCmd := m.checkSourceBranchRemote(newBranch)
					m.sourceBranchInput.Focus()
					return m, tea.Batch(checkCmd, m.spinner.Tick)
				}
			}
			// Just focus the existing input
			m.sourceBranchInput.Focus()
		}
		return m, nil
	}

	// Handle text input updates
	var cmd tea.Cmd
	m.versionInput, cmd = m.versionInput.Update(msg)

	// Clear error when user types
	if m.versionError != "" && validateVersion(m.versionInput.Value()) {
		m.versionError = ""
	} else if !validateVersion(m.versionInput.Value()) && m.versionInput.Value() != "" {
		m.versionError = "Invalid version format. Use: X.Y, X.Y.Z, or X.Y.Z.W"
	}

	return m, cmd
}

// viewVersion renders the version input screen
func (m model) viewVersion() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Content height (same as environment screen)
	contentHeight := m.height - 4

	// Total rendered height for sidebar/content (content height + 2 for border)
	totalHeight := contentHeight + 2

	// Build dual sidebar (pass total rendered height)
	sidebar := m.renderDualSidebar(sidebarW, totalHeight)

	// Build content - version input
	contentContent := m.renderVersionInput(contentWidth - 4)

	// Render content with border (same as environment screen)
	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "enter: confirm • u: go back • /: commands • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderDualSidebar renders both MRs sidebar and Environment sidebar stacked vertically
func (m model) renderDualSidebar(width int, availableHeight int) string {
	// Collect branch names from selected MRs
	items := m.list.Items()
	var branches []string
	for _, item := range items {
		if mr, ok := item.(mrListItem); ok {
			if m.selectedMRs[mr.MR().IID] {
				branches = append(branches, mr.MR().SourceBranch)
			}
		}
	}

	// Each bordered box adds 2 lines for top/bottom border
	// We have 2 boxes, so total border overhead is 4 lines
	// Available content height = availableHeight - 4
	totalContentHeight := availableHeight - 4

	// MRs sidebar content: title line + blank + branches
	// Env sidebar content: title line + blank + env value
	minEnvContentHeight := 4 // title + blank + env + padding
	mrsHeaderLines := 3      // title line + blank line + some spacing

	// Calculate ideal MRs content height
	idealMrsContentHeight := mrsHeaderLines + len(branches)

	// Default is 50/50 split of content height
	halfContentHeight := totalContentHeight / 2
	mrsContentHeight := halfContentHeight
	envContentHeight := totalContentHeight - mrsContentHeight

	// If branches don't fit in half, try to expand MRs sidebar
	if idealMrsContentHeight > halfContentHeight {
		// Calculate max MRs content height (leaving minimum for env sidebar)
		maxMrsContentHeight := totalContentHeight - minEnvContentHeight
		if idealMrsContentHeight <= maxMrsContentHeight {
			// All branches fit if we shrink env sidebar to minimum
			mrsContentHeight = idealMrsContentHeight
			envContentHeight = totalContentHeight - mrsContentHeight
		} else {
			// Even with minimum env sidebar, not all branches fit
			mrsContentHeight = maxMrsContentHeight
			envContentHeight = minEnvContentHeight
		}
	}

	// Render MRs sidebar section (pass content height, not total height)
	mrsSidebar := m.renderMRsSidebarSection(width, mrsContentHeight, branches)

	// Render Environment sidebar section
	envSidebar := m.renderEnvSidebarSection(width, envContentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, mrsSidebar, envSidebar)
}

// renderMRsSidebarSection renders the MRs sidebar block with border
// height parameter is the content height (excluding border)
func (m model) renderMRsSidebarSection(width int, contentHeight int, branches []string) string {
	var sb strings.Builder

	title := fmt.Sprintf(" MRs to release (%d) ", len(branches))
	sb.WriteString(envTitleStepStyle.Render("[1]") +
		envTitleStyle.Render(title))
	sb.WriteString("\n\n")

	// Available for branches = content height - 3 (title area: title + 2 blank lines)
	availableLinesForBranches := contentHeight - 3

	if len(branches) <= availableLinesForBranches {
		// All branches fit
		for _, branch := range branches {
			branchName := truncateWithEllipsis(branch, width-6)
			sb.WriteString(mrBranchStyle.Render(branchName))
			sb.WriteString("\n")
		}
	} else {
		// Need to show "+ N mrs" at the end
		visibleCount := availableLinesForBranches - 1 // -1 for the overflow line
		if visibleCount < 0 {
			visibleCount = 0
		}
		for i := 0; i < visibleCount && i < len(branches); i++ {
			branchName := truncateWithEllipsis(branches[i], width-6)
			sb.WriteString(mrBranchStyle.Render(branchName))
			sb.WriteString("\n")
		}
		hiddenCount := len(branches) - visibleCount
		overflowText := fmt.Sprintf("+ %d mrs", hiddenCount)
		sb.WriteString(mrBranchStyle.Foreground(lipgloss.Color("245")).Render(overflowText))
		sb.WriteString("\n")
	}

	// Wrap in bordered box (Height is content height, border adds 2 more lines)
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

// renderEnvSidebarSection renders the Environment sidebar block with border
// height parameter is the content height (excluding border)
func (m model) renderEnvSidebarSection(width int, contentHeight int) string {
	var sb strings.Builder

	sb.WriteString(envTitleStepStyle.Render("[2]") +
		envTitleStyle.Render(" Environment "))
	sb.WriteString("\n\n")

	// Show selected environment with styled background
	// Fall back to release state if selectedEnv is nil
	var envName string
	if m.selectedEnv != nil {
		envName = m.selectedEnv.Name
	} else if m.releaseState != nil {
		envName = m.releaseState.Environment.Name
	}
	if envName != "" {
		envStyle := getEnvHintStyle(envName)
		sb.WriteString(envStyle.Render(" " + envName + " "))
	}

	// Wrap in bordered box (Height is content height, border adds 2 more lines)
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

// renderVersionInput renders the version input content area
func (m model) renderVersionInput(width int) string {
	var sb strings.Builder

	// Step title
	sb.WriteString(envTitleStepStyle.Render("[3]") + envTitleStyle.Render(" Version "))
	sb.WriteString("\n\n")

	// Prompt
	prompt := "Input semantic number to version this release:"
	sb.WriteString(envPromptStyle.Render(prompt))
	sb.WriteString("\n\n")

	// Version input field
	sb.WriteString(versionInputStyle.Render("Version: "))
	sb.WriteString(m.versionInput.View())

	// Error message if any (uses same style as error modal)
	if m.versionError != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorTitleStyle.Render(m.versionError))
	}

	sb.WriteString("\n\n")

	// Hint with styled parts - show version from input or placeholder
	version := m.versionInput.Value()
	if version == "" {
		version = "<version>"
	}

	var envName, branchSuffix string
	if m.selectedEnv != nil {
		envName = m.selectedEnv.Name
		branchSuffix = m.selectedEnv.BranchName
	} else {
		envName = "ENV"
		branchSuffix = "env"
	}

	branchPart := fmt.Sprintf("release/rpb-%s-%s", version, branchSuffix)
	hint := envHintBaseStyle.Render("Release branch ") +
		getEnvBranchStyle(envName).Render(branchPart) +
		envHintBaseStyle.Render(" -> ") +
		getEnvHintStyle(envName).Render(" "+envName+" ")
	sb.WriteString(hint)

	return sb.String()
}
