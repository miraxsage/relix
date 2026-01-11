package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
)

// Style for command logging in terminal output
var commandLogStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("33"))

// GitExecutor handles git command execution via PTY
type GitExecutor struct {
	workDir string
	ptyFile *os.File
	cmd     *exec.Cmd
	program *tea.Program // For sending messages back to UI
	cols    uint16
	rows    uint16
}

// NewGitExecutor creates a new git executor for the given directory
func NewGitExecutor(workDir string, program *tea.Program) *GitExecutor {
	return &GitExecutor{
		workDir: workDir,
		program: program,
		cols:    80,
		rows:    24,
	}
}

// RunCommand executes a shell command via PTY and streams output
func (g *GitExecutor) RunCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = g.workDir

	// Send command header to UI immediately if program is set (before PTY starts)
	// Format: empty line, styled command, empty line, then output
	if g.program != nil {
		g.program.Send(releaseOutputMsg{line: ""})
		g.program.Send(releaseOutputMsg{line: commandLogStyle.Render(command)})
		g.program.Send(releaseOutputMsg{line: ""})
	}

	// Start PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: g.rows, Cols: g.cols})
	if err != nil {
		return "", fmt.Errorf("failed to start PTY: %w", err)
	}

	g.ptyFile = ptmx
	g.cmd = cmd

	// Read output and send to UI
	var outputBuilder strings.Builder
	scanner := bufio.NewScanner(ptmx)
	for scanner.Scan() {
		line := scanner.Text()
		outputBuilder.WriteString(line + "\n")
		if g.program != nil {
			g.program.Send(releaseOutputMsg{line: line})
		}
	}

	// Wait for command to finish
	err = cmd.Wait()
	output := outputBuilder.String()

	// Send empty line after output for visual separation
	if g.program != nil {
		g.program.Send(releaseOutputMsg{line: ""})
	}

	g.ptyFile.Close()
	g.ptyFile = nil
	g.cmd = nil

	return output, err
}

// Resize handles terminal resize
func (g *GitExecutor) Resize(rows, cols uint16) error {
	g.rows = rows
	g.cols = cols
	if g.ptyFile == nil {
		return nil
	}
	return pty.Setsize(g.ptyFile, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Kill terminates the running command
func (g *GitExecutor) Kill() error {
	if g.cmd != nil && g.cmd.Process != nil {
		return g.cmd.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

// Close cleans up resources
func (g *GitExecutor) Close() error {
	if g.ptyFile != nil {
		return g.ptyFile.Close()
	}
	return nil
}

// DetectMergeConflict checks if there's an unresolved merge conflict
func DetectMergeConflict(workDir string) bool {
	mergeHeadPath := filepath.Join(workDir, ".git", "MERGE_HEAD")
	_, err := os.Stat(mergeHeadPath)
	return err == nil
}

// HasUncommittedChanges checks if there are uncommitted changes in the working directory
func HasUncommittedChanges(workDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// IsBranchMerged checks if a branch is already merged into HEAD
func IsBranchMerged(workDir, branch string) (bool, error) {
	cmd := exec.Command("git", "branch", "--merged", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Remove leading spaces and asterisk
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line == branch || strings.HasSuffix(line, "/"+branch) {
			return true, nil
		}
	}
	return false, nil
}

// BranchExists checks if a local branch exists
func BranchExists(workDir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = workDir
	return cmd.Run() == nil
}

// RemoteBranchExists checks if a remote branch exists
func RemoteBranchExists(workDir, remoteBranch string) bool {
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", remoteBranch)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// FindProjectRoot finds the project root directory by looking for package.json
// If projectDirectory global is set (via -d flag), uses that instead
// Returns the root directory or error if not found
func FindProjectRoot() (string, error) {
	// If project directory was specified via command line, use it
	if projectDirectory != "" {
		return projectDirectory, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check if package.json exists in current directory
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		// Check if this is a nested package (monorepo)
		parentDir := filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(parentDir, "package.json")); err == nil {
			// Parent has package.json too, use parent as root
			return parentDir, nil
		}
		return cwd, nil
	}

	// No package.json in current dir, go up one level
	parentDir := filepath.Dir(cwd)
	if _, err := os.Stat(filepath.Join(parentDir, "package.json")); err == nil {
		return parentDir, nil
	}

	// Default to current directory
	return cwd, nil
}

// ReleaseCommands generates git commands for each release step
type ReleaseCommands struct {
	workDir         string
	version         string
	envBranch       string
	envName         string
	excludePatterns []string
	branches        []string // MR source branches to merge
}

// NewReleaseCommands creates a new command builder
func NewReleaseCommands(workDir, version string, env *Environment, patterns []string, branches []string) *ReleaseCommands {
	return &ReleaseCommands{
		workDir:         workDir,
		version:         version,
		envBranch:       env.BranchName,
		envName:         env.Name,
		excludePatterns: patterns,
		branches:        branches,
	}
}

// RootBranch returns the release root branch name
func (r *ReleaseCommands) RootBranch() string {
	return fmt.Sprintf("release/rpb-%s-root", r.version)
}

// EnvReleaseBranch returns the environment release branch name
func (r *ReleaseCommands) EnvReleaseBranch() string {
	return fmt.Sprintf("release/rpb-%s-%s", r.version, r.envBranch)
}

// Step1CheckoutRoot returns the command for step 1
func (r *ReleaseCommands) Step1CheckoutRoot() string {
	return fmt.Sprintf("git checkout root && git pull && git checkout -B %s", r.RootBranch())
}

// Step2MergeBranch returns the command to merge a specific branch
func (r *ReleaseCommands) Step2MergeBranch(branchIndex int) string {
	if branchIndex >= len(r.branches) {
		return ""
	}
	return fmt.Sprintf("git merge origin/%s", r.branches[branchIndex])
}

// Step3CheckoutEnv returns the command for step 3
// This handles the case where local env branch might not exist
func (r *ReleaseCommands) Step3CheckoutEnv() string {
	// Try to checkout local branch, if it fails try to create from remote
	return fmt.Sprintf(`git checkout %s 2>/dev/null || git checkout -b %s origin/%s && git pull && git checkout -B %s`,
		r.envBranch, r.envBranch, r.envBranch, r.EnvReleaseBranch())
}

// Step4RemoveAll returns the command to remove all files from index
func (r *ReleaseCommands) Step4RemoveAll() string {
	return "git rm -rf ."
}

// Step4CheckoutFromRoot returns the command to checkout content from root branch
func (r *ReleaseCommands) Step4CheckoutFromRoot() string {
	return fmt.Sprintf("git checkout %s -- .", r.RootBranch())
}

// Step6Push returns the command for step 6 (push only, MR is created via API)
func (r *ReleaseCommands) Step6Push() string {
	return fmt.Sprintf("git push -u origin %s", r.EnvReleaseBranch())
}

// ParseVersionNumber extracts version number from release commit title
// Format: "release:{version} {envBranch} vN"
// Returns (versionStr, vNumber, found)
func ParseVersionNumber(commitTitle, envBranch string) (string, int, bool) {
	// Pattern: release:{version} {envBranch} v{N}
	pattern := `^\s*release\s*:\s*([-0-9.\s]+)\s+.+\s+v?(\d+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(commitTitle)
	if len(matches) != 3 {
		// Try without v-number for legacy format
		patternNoV := `^\s*release\s*:\s*([-0-9.\s]+)`
		reNoV := regexp.MustCompile(patternNoV)
		matchesNoV := reNoV.FindStringSubmatch(commitTitle)
		if len(matchesNoV) == 2 {
			return matchesNoV[1], 1, true // Treat as v1 if no v-number
		}
		return "", 0, false
	}
	vNum, _ := strconv.Atoi(matches[2])
	return matches[1], vNum, true
}

// NormalizeVersion normalizes version string by removing leading zeros
// 4.05.01 -> 4.5.1
func NormalizeVersion(version string) string {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(version, -1)

	normalized := make([]string, 0, len(matches))
	for _, m := range matches {
		num, err := strconv.Atoi(m)
		if err != nil {
			continue
		}
		normalized = append(normalized, strconv.Itoa(num))
	}

	return strings.Join(normalized, ".")
}

// VersionsMatch compares two version strings ignoring leading zeros
// Returns true if versions are considered equal
func VersionsMatch(v1, v2 string) bool {
	n1 := NormalizeVersion(v1)
	n2 := NormalizeVersion(v2)

	// Also handle prefixes (4.5 matches 4.5.1)
	if strings.HasPrefix(n1, n2) || strings.HasPrefix(n2, n1) {
		return true
	}
	return n1 == n2
}

// GetNextVersionNumber parses git log and returns the next v-number to use
// Returns (vNumber, error)
func GetNextVersionNumber(workDir, envBranch, currentVersion string) (int, error) {
	cmd := exec.Command("git", "log", fmt.Sprintf("origin/%s", envBranch), "-n", "10", "--pretty=%s")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to read git log: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		version, vNum, found := ParseVersionNumber(line, envBranch)
		if !found {
			continue
		}

		// Found a release commit
		if VersionsMatch(version, currentVersion) {
			// Same version - increment
			return vNum + 1, nil
		}
		// Different version - reset to v1
		return 1, nil
	}

	// No previous release found - this is an error for MVP
	return 0, fmt.Errorf("no previous release version found in last %s-branch commits", envBranch)
}

// BuildCommitMessage builds the commit message for step 4
func BuildCommitMessage(version, envBranch string, vNumber int, branches []string) (string, string) {
	title := fmt.Sprintf("release:%s %s v%d", version, envBranch, vNumber)
	body := strings.Join(branches, "\n")
	return title, body
}

// GetExcludedFiles returns list of files matching exclusion patterns
func GetExcludedFiles(workDir string, patterns []string) ([]string, error) {
	// Get all tracked files
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := strings.Split(string(output), "\x00")
	var excluded []string

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}

			if matchesPattern(file, pattern) {
				excluded = append(excluded, file)
				break
			}
		}
	}

	return excluded, nil
}

// matchesPattern checks if a file path matches an exclusion pattern
func matchesPattern(file, pattern string) bool {
	// Handle patterns starting with "/" as relative to project root
	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
		// For absolute patterns, match from start
		return matchGlob(file, pattern)
	}

	// For patterns without "/", match at any nesting level
	// e.g., "*.ts" matches "src/foo.ts"
	parts := strings.Split(file, "/")
	for i := range parts {
		suffix := strings.Join(parts[i:], "/")
		if matchGlob(suffix, pattern) {
			return true
		}
	}

	return matchGlob(file, pattern)
}

// matchGlob performs glob matching with support for * and **
func matchGlob(path, pattern string) bool {
	// Convert glob pattern to regex
	regexPattern := "^"
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches any path including /
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					regexPattern += "(.*/|)"
					i += 2
				} else {
					regexPattern += ".*"
					i++
				}
			} else {
				// * matches anything except /
				regexPattern += "[^/]*"
			}
		case '?':
			regexPattern += "[^/]"
		case '.', '+', '^', '$', '|', '(', ')', '[', ']', '{', '}', '\\':
			regexPattern += "\\" + string(pattern[i])
		default:
			regexPattern += string(pattern[i])
		}
	}
	regexPattern += "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// DeleteLocalBranches deletes local branches created during release
func DeleteLocalBranches(workDir, version, envBranch string) error {
	rootBranch := fmt.Sprintf("release/rpb-%s-root", version)
	envReleaseBranch := fmt.Sprintf("release/rpb-%s-%s", version, envBranch)

	// Checkout to root first to avoid "cannot delete checked out branch"
	cmd := exec.Command("git", "checkout", "root")
	cmd.Dir = workDir
	cmd.Run() // Ignore errors

	// Delete root branch
	cmd = exec.Command("git", "branch", "-D", rootBranch)
	cmd.Dir = workDir
	cmd.Run() // Ignore errors, branch might not exist

	// Delete env release branch
	cmd = exec.Command("git", "branch", "-D", envReleaseBranch)
	cmd.Dir = workDir
	cmd.Run() // Ignore errors

	return nil
}

// GetLast500Lines returns the last 500 lines from a string
func GetLast500Lines(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= 500 {
		return output
	}
	return strings.Join(lines[len(lines)-500:], "\n")
}
