package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// updateEnvMerge handles key events on the env merge screen
func (m model) updateEnvMerge(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		// Go back to source branch input
		m.screen = screenSourceBranch
		return m, nil
	case "up", "k":
		if m.envMergeOptionIndex > 0 {
			m.envMergeOptionIndex--
		}
		return m, nil
	case "down", "j":
		if m.envMergeOptionIndex < 1 {
			m.envMergeOptionIndex++
		}
		return m, nil
	case "enter":
		m.envMergeSelection = m.envMergeOptionIndex
		m.screen = screenRootMerge
		// Preserve previous root merge selection
		if m.rootMergeSelection {
			m.rootMergeButtonIndex = 0
		} else {
			m.rootMergeButtonIndex = 1
		}
		return m, nil
	}

	return m, nil
}

// viewEnvMerge renders the env merge screen
func (m model) viewEnvMerge() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarW := sidebarWidth(m.width)
	contentWidth := m.width - sidebarW - 4

	contentHeight := m.height - 4
	totalHeight := contentHeight + 2

	// Build quad sidebar (4 sections: MRs, Environment, Version, Source branch)
	sidebar := m.renderQuadSidebar(sidebarW, totalHeight)

	// Build content - env merge selection
	contentContent := m.renderEnvMergeContent(contentWidth - 4)

	content := contentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentContent)

	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	helpText := "↓/↑/j/k: switch • enter: confirm • C+q: back • /: commands • C+c: quit"
	help := helpStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderEnvMergeContent renders the env merge content area
func (m model) renderEnvMergeContent(width int) string {
	var sb strings.Builder

	// Step title
	sb.WriteString(envTitleStepStyle.Render("[5]") + envTitleStyle.Render(" Env merge "))
	sb.WriteString("\n\n")

	// Get env name for descriptions
	envName := ""
	if m.selectedEnv != nil {
		envName = strings.ToUpper(m.selectedEnv.Name)
	}

	// Get source branch name for descriptions
	sourceBranch := m.sourceBranchInput.Value()
	if sourceBranch == "" && m.releaseState != nil {
		sourceBranch = m.releaseState.SourceBranch
	}

	// Option styles
	selectedBullet := lipgloss.NewStyle().Foreground(currentTheme.Warning).Bold(true)
	unselectedBullet := lipgloss.NewStyle().Foreground(currentTheme.Notion)
	descStyle := lipgloss.NewStyle().Foreground(currentTheme.Foreground)
	dimDescStyle := lipgloss.NewStyle().Foreground(currentTheme.Notion)

	// Option 1: Squash merge
	if m.envMergeOptionIndex == 0 {
		sb.WriteString(selectedBullet.Render("▸ Squash merge"))
		sb.WriteString("\n")
		desc := fmt.Sprintf("  All current release changes will be accumulated in one common\n  commit from the current %s branch and just copied as independent\n  changes via \"git checkout %s -- .\". That makes the merge safe\n  from conflicts; containing commits will be mentioned in the\n  commit message.", envName, sourceBranch)
		sb.WriteString(descStyle.Render(desc))
	} else {
		sb.WriteString(unselectedBullet.Render("▹ Squash merge"))
		sb.WriteString("\n")
		desc := fmt.Sprintf("  All current release changes will be accumulated in one common\n  commit from the current %s branch and just copied as independent\n  changes via \"git checkout %s -- .\". That makes the merge safe\n  from conflicts; containing commits will be mentioned in the\n  commit message.", envName, sourceBranch)
		sb.WriteString(dimDescStyle.Render(desc))
	}

	sb.WriteString("\n\n")

	// Option 2: Regular merge
	if m.envMergeOptionIndex == 1 {
		sb.WriteString(selectedBullet.Render("▸ Regular merge"))
		sb.WriteString("\n")
		desc := fmt.Sprintf("  All selected commits will be merged straight to %s branch.\n  There is a risk of conflicts; you should resolve them manually.", envName)
		sb.WriteString(descStyle.Render(desc))
	} else {
		sb.WriteString(unselectedBullet.Render("▹ Regular merge"))
		sb.WriteString("\n")
		desc := fmt.Sprintf("  All selected commits will be merged straight to %s branch.\n  There is a risk of conflicts; you should resolve them manually.", envName)
		sb.WriteString(dimDescStyle.Render(desc))
	}

	// Show commit count section (always visible, regardless of selected option)
	sb.WriteString("\n\n")
	activeDescStyle := descStyle
	if m.envMergeOptionIndex != 1 {
		activeDescStyle = dimDescStyle
	}
	if m.envMergeCountLoading {
		sb.WriteString("  " + m.spinner.View() + " Merging commits calculation")
	} else if m.envMergeCommitCount > 0 {
		countColor := currentTheme.Warning
		if m.envMergeCommitCount > 100 {
			countColor = currentTheme.Error
		}
		countStyle := lipgloss.NewStyle().Foreground(countColor).Bold(true)
		sb.WriteString(activeDescStyle.Render(fmt.Sprintf("  To %s branch per current release will be merged ", envName)))
		sb.WriteString(countStyle.Render(fmt.Sprintf("%d", m.envMergeCommitCount)))
		sb.WriteString(activeDescStyle.Render(" new commits"))
	}

	return sb.String()
}

// renderEnvMergeSidebarSection renders the Env merge sidebar block with border
func (m model) renderEnvMergeSidebarSection(width int, contentHeight int) string {
	var sb strings.Builder

	sb.WriteString(envTitleStepStyle.Render("[5]") +
		envTitleStyle.Render(" Env merge "))
	sb.WriteString("\n\n")

	// Show env merge selection (live preview on env merge screen, fall back to release state)
	envMergeMode := m.envMergeSelection
	if m.screen == screenEnvMerge {
		envMergeMode = m.envMergeOptionIndex // live preview while cursor is moving
	} else if m.releaseState != nil && m.releaseState.EnvMergeMode != "" {
		if m.releaseState.EnvMergeMode == "regular" {
			envMergeMode = 1
		} else {
			envMergeMode = 0
		}
	}

	if envMergeMode == 0 {
		squashStyle := lipgloss.NewStyle().
			Foreground(currentTheme.Success).
			Bold(true)
		sb.WriteString(squashStyle.Render("Squash"))
	} else {
		regularStyle := lipgloss.NewStyle().
			Foreground(currentTheme.Warning).
			Bold(true)
		sb.WriteString(regularStyle.Render("Regular"))
	}

	// Wrap in bordered box
	content := sb.String()
	borderedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		Padding(0, 1).
		Width(width).
		Height(contentHeight).
		Render(content)

	return borderedBox
}

// calculateEnvMergeCommitCount runs git rev-list --count and sums MR commits
func (m model) calculateEnvMergeCommitCount() tea.Cmd {
	return func() tea.Msg {
		workDir, err := FindProjectRoot()
		if err != nil {
			return envMergeCommitCountMsg{count: 0, err: err}
		}

		envBranch := ""
		if m.selectedEnv != nil {
			envBranch = m.selectedEnv.BranchName
		}
		sourceBranch := m.sourceBranchInput.Value()
		baseBranch := getBaseBranch()

		// If source branch exists remotely, count divergence between env and source branch
		// If source branch is new, count divergence between env and base branch + MR commits
		sourceBranchExists := m.sourceBranchRemoteStatus == "exists-same" || m.sourceBranchRemoteStatus == "exists-diff"

		total := 0
		if sourceBranchExists {
			// Source branch exists — count commits between env branch and source branch
			cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("origin/%s..%s", envBranch, sourceBranch))
			cmd.Dir = workDir
			out, err := cmd.Output()
			if err == nil {
				total, _ = strconv.Atoi(strings.TrimSpace(string(out)))
			}
		} else {
			// Source branch is new — count base-to-env divergence + MR commits
			// First: how many commits does base branch have that env branch doesn't
			cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("origin/%s..origin/%s", envBranch, baseBranch))
			cmd.Dir = workDir
			out, err := cmd.Output()
			if err == nil {
				baseToEnvCount, _ := strconv.Atoi(strings.TrimSpace(string(out)))
				total += baseToEnvCount
			}

			// Then add MR commit counts
			items := m.list.Items()
			for _, item := range items {
				if mr, ok := item.(mrListItem); ok {
					if m.selectedMRs[mr.MR().IID] {
						total += mr.MR().CommitsCount
					}
				}
			}
		}

		return envMergeCommitCountMsg{count: total, err: nil}
	}
}
