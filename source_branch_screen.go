package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// initSourceBranchInput initializes the source branch text input with default value
// Returns a command to trigger the initial remote check
func (m *model) initSourceBranchInput() tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = "e.g. release/rpb-1.0.0-root"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))

	// Set default value based on version
	version := m.versionInput.Value()
	if version != "" {
		ti.SetValue("release/rpb-" + version + "-root")
	}

	m.sourceBranchInput = ti
	m.sourceBranchError = ""
	m.sourceBranchVersion = version // Track version used for this source branch

	// Trigger initial remote check if we have a valid branch name
	branchName := ti.Value()
	if branchName != "" && m.validateSourceBranch(branchName) {
		m.sourceBranchRemoteStatus = "checking"
		m.sourceBranchLastCheckTime = time.Now()
		m.sourceBranchCheckedName = branchName
		return tea.Batch(m.checkSourceBranchRemote(branchName), m.spinner.Tick)
	}

	return nil
}

// validateSourceBranch checks if the version is present in the branch name
func (m model) validateSourceBranch(branchName string) bool {
	if branchName == "" {
		return true // Empty is valid (no error shown yet)
	}
	version := m.versionInput.Value()
	if version == "" {
		return true // No version to validate against
	}
	return strings.Contains(branchName, version)
}

// updateSourceBranch handles key events on the source branch input screen
func (m model) updateSourceBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+u":
		// Go back to version input
		m.screen = screenVersion
		m.sourceBranchError = ""
		return m, nil
	case "enter":
		// Validate and proceed
		branchName := m.sourceBranchInput.Value()
		if branchName == "" {
			m.sourceBranchError = "Source branch name is required"
			return m, nil
		}
		if !m.validateSourceBranch(branchName) {
			m.sourceBranchError = "Branch name must contain version: " + m.versionInput.Value()
			return m, nil
		}
		// Branch name is valid - proceed to root merge screen
		m.sourceBranchError = ""
		m.screen = screenRootMerge
		// Preserve previous selection: 0 = Yes (rootMergeSelection=true), 1 = No (rootMergeSelection=false)
		if m.rootMergeSelection {
			m.rootMergeButtonIndex = 0
		} else {
			m.rootMergeButtonIndex = 1
		}
		return m, nil
	}

	// Handle text input updates
	var cmd tea.Cmd
	oldValue := m.sourceBranchInput.Value()
	m.sourceBranchInput, cmd = m.sourceBranchInput.Update(msg)
	newValue := m.sourceBranchInput.Value()

	// Track version when user modifies the source branch
	if oldValue != newValue {
		m.sourceBranchVersion = m.versionInput.Value()
	}

	// Clear error when user types valid input
	if m.sourceBranchError != "" && m.validateSourceBranch(m.sourceBranchInput.Value()) {
		m.sourceBranchError = ""
	} else if !m.validateSourceBranch(m.sourceBranchInput.Value()) && m.sourceBranchInput.Value() != "" {
		m.sourceBranchError = "Branch name must contain version: " + m.versionInput.Value()
		// Clear remote status when invalid
		m.sourceBranchRemoteStatus = ""
	}

	// Trigger remote check if needed (with throttle)
	if oldValue != newValue && m.shouldCheckSourceBranch(newValue) {
		m.sourceBranchRemoteStatus = "checking"
		m.sourceBranchLastCheckTime = time.Now()
		m.sourceBranchCheckedName = newValue
		checkCmd := m.checkSourceBranchRemote(newValue)
		return m, tea.Batch(cmd, checkCmd, m.spinner.Tick)
	}

	return m, cmd
}

// viewSourceBranch renders the source branch input screen
func (m model) viewSourceBranch() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	// Content height (same as other screens)
	contentHeight := m.height - 4

	// Total rendered height for sidebar/content (content height + 2 for border)
	totalHeight := contentHeight + 2

	// Build triple sidebar (same as confirmation step)
	sidebar := m.renderTripleSidebar(sidebarW, totalHeight)

	// Build content - source branch input
	contentContent := m.renderSourceBranchInput(contentWidth - 4)

	// Render content with border
	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	// Combine sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Help footer
	helpText := "enter: confirm • C+u: go back • /: commands • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderSourceBranchInput renders the source branch input content area
func (m model) renderSourceBranchInput(width int) string {
	var sb strings.Builder

	// Step title
	sb.WriteString(envTitleStepStyle.Render("[4]") + envTitleStyle.Render(" Source branch "))
	sb.WriteString("\n\n")

	// Prompt
	prompt := "Specify source branch where we will accumulate releasing commits."
	sb.WriteString(envPromptStyle.Render(prompt))
	sb.WriteString("\n")

	// Sub-prompt
	subPrompt := "If the branch does not exist locally and remotely it will be created from root branch."
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Render(subPrompt))
	sb.WriteString("\n\n")

	// Source branch input field
	sb.WriteString(versionInputStyle.Render("Source branch: "))
	sb.WriteString(m.sourceBranchInput.View())

	// Show error or status
	if m.sourceBranchError != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorTitleStyle.Render(m.sourceBranchError))
	} else if m.sourceBranchInput.Value() != "" && m.validateSourceBranch(m.sourceBranchInput.Value()) {
		// Show status when branch name is valid
		sb.WriteString("\n\n")
		sb.WriteString(m.renderSourceBranchStatus())
	}

	return sb.String()
}

// renderSourceBranchStatus renders the status text for the source branch check
func (m model) renderSourceBranchStatus() string {
	existsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	createdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	rootSameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	rootDiffStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("189"))

	switch m.sourceBranchRemoteStatus {
	case "checking":
		return m.spinner.View() + " " + normalStyle.Render("Checking remote branch...")
	case "exists-same":
		return normalStyle.Render("Exact remote branch already ") +
			existsStyle.Render("exists") +
			normalStyle.Render(" ") +
			rootSameStyle.Render("(-> root)") +
			normalStyle.Render(" and will be used for release")
	case "exists-diff":
		return normalStyle.Render("Exact remote branch already ") +
			existsStyle.Render("exists") +
			normalStyle.Render(" ") +
			rootDiffStyle.Render("(!= root)") +
			normalStyle.Render(" and will be used for release")
	case "exists":
		return normalStyle.Render("Exact remote branch already ") +
			existsStyle.Render("exists") +
			normalStyle.Render(" and will be used for release")
	case "new":
		return normalStyle.Render("That is new branch and will be ") +
			createdStyle.Render("created") +
			normalStyle.Render(" for release from root")
	default:
		return ""
	}
}

// checkSourceBranchRemote performs an async check for the remote branch
func (m *model) checkSourceBranchRemote(branchName string) tea.Cmd {
	return func() tea.Msg {
		// Get project directory (respects -d flag)
		workDir, err := FindProjectRoot()
		if err != nil {
			return sourceBranchCheckMsg{
				branchName: branchName,
				exists:     false,
				sameAsRoot: false,
				err:        err,
			}
		}

		// Check if branch exists on remote
		// Use CombinedOutput to capture stderr and check exit code properly
		cmd := exec.Command("git", "ls-remote", "--heads", "origin", branchName)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()

		// git ls-remote returns exit code 0 even if branch not found (just empty output)
		// It only returns non-zero for actual errors (network, auth, etc.)
		if err != nil {
			// Check if it's an exit error with code 128 (common git error)
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Real git error - treat as unable to check
				return sourceBranchCheckMsg{
					branchName: branchName,
					exists:     false,
					sameAsRoot: false,
					err:        exitErr,
				}
			}
			// Other error (command not found, etc.)
			return sourceBranchCheckMsg{
				branchName: branchName,
				exists:     false,
				sameAsRoot: false,
				err:        err,
			}
		}

		// Parse output - format is: "<commit-hash>\trefs/heads/<branch-name>"
		// If empty, branch doesn't exist
		outputStr := strings.TrimSpace(string(output))

		// Check if output contains the exact branch ref
		expectedRef := "refs/heads/" + branchName
		exists := strings.Contains(outputStr, expectedRef)

		if !exists {
			return sourceBranchCheckMsg{
				branchName: branchName,
				exists:     false,
				sameAsRoot: false,
				err:        nil,
			}
		}

		// Branch exists - extract commit hash
		sourceBranchCommit := ""
		for _, line := range strings.Split(outputStr, "\n") {
			if strings.Contains(line, expectedRef) {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					sourceBranchCommit = parts[0]
				}
				break
			}
		}

		// Get the commit hash of root branch
		rootCmd := exec.Command("git", "ls-remote", "--heads", "origin", "root")
		rootCmd.Dir = workDir
		rootOutput, rootErr := rootCmd.CombinedOutput()
		rootOutputStr := strings.TrimSpace(string(rootOutput))

		if rootErr != nil || !strings.Contains(rootOutputStr, "refs/heads/root") {
			// Can't determine root status, just say it exists (without root comparison)
			return sourceBranchCheckMsg{
				branchName: branchName,
				exists:     true,
				sameAsRoot: false,
				err:        nil,
			}
		}

		// Extract root commit hash
		rootCommit := ""
		for _, line := range strings.Split(rootOutputStr, "\n") {
			if strings.Contains(line, "refs/heads/root") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					rootCommit = parts[0]
				}
				break
			}
		}

		sameAsRoot := sourceBranchCommit != "" && rootCommit != "" && sourceBranchCommit == rootCommit

		return sourceBranchCheckMsg{
			branchName: branchName,
			exists:     true,
			sameAsRoot: sameAsRoot,
			err:        nil,
		}
	}
}

// shouldCheckSourceBranch determines if we should trigger a new remote check
func (m *model) shouldCheckSourceBranch(branchName string) bool {
	if branchName == "" {
		return false
	}
	if !m.validateSourceBranch(branchName) {
		return false
	}
	// Check throttle (100ms)
	if time.Since(m.sourceBranchLastCheckTime) < 100*time.Millisecond {
		return false
	}
	// Check if branch name changed since last check
	if branchName == m.sourceBranchCheckedName && m.sourceBranchRemoteStatus != "" && m.sourceBranchRemoteStatus != "checking" {
		return false
	}
	return true
}
