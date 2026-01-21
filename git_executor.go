package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ActiveState/vt10x"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
)

// Style for command logging in terminal output
var commandLogStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("220"))

// VirtualTerminal wraps vt10x to provide terminal emulation
type VirtualTerminal struct {
	state *vt10x.State
	vt    *vt10x.VT
	cols  int
	rows  int
}

// emptyReader is a reader that always returns EOF
type emptyReader struct{}

func (emptyReader) Read(p []byte) (int, error) { return 0, io.EOF }

// NewVirtualTerminal creates a new virtual terminal with the given dimensions
func NewVirtualTerminal(cols, rows int) *VirtualTerminal {
	state := &vt10x.State{}
	// Use New with empty reader - we'll use Write method directly
	vt, _ := vt10x.New(state, emptyReader{}, io.Discard)
	vt.Resize(cols, rows)
	return &VirtualTerminal{
		state: state,
		vt:    vt,
		cols:  cols,
		rows:  rows,
	}
}

// Write feeds data to the virtual terminal and parses it
func (vt *VirtualTerminal) Write(data []byte) {
	// vt.Write() internally locks state
	vt.vt.Write(data)
}

// RenderScreen returns the current screen buffer as a string with ANSI colors preserved
func (vt *VirtualTerminal) RenderScreen() string {
	// First pass: find the last row with actual content
	lastContentRow := -1
	for y := vt.rows - 1; y >= 0; y-- {
		for x := 0; x < vt.cols; x++ {
			ch, _, _ := vt.state.Cell(x, y)
			if ch != ' ' && ch != 0 {
				lastContentRow = y
				break
			}
		}
		if lastContentRow >= 0 {
			break
		}
	}

	// If no content, return empty
	if lastContentRow < 0 {
		return ""
	}

	var sb strings.Builder
	var lastFg, lastBg vt10x.Color = vt10x.DefaultFG, vt10x.DefaultBG

	// Second pass: render only up to the last row with content
	for y := 0; y <= lastContentRow; y++ {
		lineContent := strings.Builder{}

		for x := 0; x < vt.cols; x++ {
			ch, fg, bg := vt.state.Cell(x, y)

			// Emit color changes
			if fg != lastFg || bg != lastBg {
				colorSeq := vt.buildColorSequence(fg, bg)
				if colorSeq != "" {
					lineContent.WriteString(colorSeq)
				}
				lastFg = fg
				lastBg = bg
			}

			// Write character (use space for null)
			if ch == 0 {
				lineContent.WriteRune(' ')
			} else {
				lineContent.WriteRune(ch)
			}
		}

		// Trim trailing spaces from each line
		line := strings.TrimRight(lineContent.String(), " ")
		sb.WriteString(line)
		if y < lastContentRow {
			sb.WriteString("\n")
		}
	}

	// Reset colors at the end
	sb.WriteString("\033[0m")

	return sb.String()
}

// buildColorSequence builds ANSI escape sequence for foreground and background colors
func (vt *VirtualTerminal) buildColorSequence(fg, bg vt10x.Color) string {
	var seq strings.Builder
	seq.WriteString("\033[")

	parts := []string{}

	// Handle foreground color
	if fg == vt10x.DefaultFG {
		parts = append(parts, "39") // Default foreground
	} else if fg.ANSI() {
		// ANSI colors 0-7: 30-37, 8-15: 90-97
		if fg < 8 {
			parts = append(parts, fmt.Sprintf("%d", 30+int(fg)))
		} else {
			parts = append(parts, fmt.Sprintf("%d", 90+int(fg)-8))
		}
	} else {
		// 256 color mode
		parts = append(parts, fmt.Sprintf("38;5;%d", int(fg)))
	}

	// Handle background color
	if bg == vt10x.DefaultBG {
		parts = append(parts, "49") // Default background
	} else if bg.ANSI() {
		// ANSI colors 0-7: 40-47, 8-15: 100-107
		if bg < 8 {
			parts = append(parts, fmt.Sprintf("%d", 40+int(bg)))
		} else {
			parts = append(parts, fmt.Sprintf("%d", 100+int(bg)-8))
		}
	} else {
		// 256 color mode
		parts = append(parts, fmt.Sprintf("48;5;%d", int(bg)))
	}

	seq.WriteString(strings.Join(parts, ";"))
	seq.WriteString("m")
	return seq.String()
}

// Resize resizes the virtual terminal
func (vt *VirtualTerminal) Resize(cols, rows int) {
	// vt.Resize() internally locks state
	vt.vt.Resize(cols, rows)
	vt.cols = cols
	vt.rows = rows
}

// GitExecutor handles git command execution via PTY
type GitExecutor struct {
	workDir string
	ptyFile *os.File
	cmd     *exec.Cmd
	program *tea.Program // For sending messages back to UI
	cols    uint16
	rows    uint16
	vterm   *VirtualTerminal
	doneCh  chan struct{}
	mu      sync.Mutex
}

// NewGitExecutor creates a new git executor for the given directory
func NewGitExecutor(workDir string, program *tea.Program) *GitExecutor {
	return &GitExecutor{
		workDir: workDir,
		program: program,
		cols:    120, // Default, should be set via SetSize before running commands
		rows:    24,
	}
}

// SetSize sets the terminal size for the executor
func (g *GitExecutor) SetSize(cols, rows uint16) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cols = cols
	g.rows = rows
}

// RunCommand executes a shell command via PTY and streams output through virtual terminal
func (g *GitExecutor) RunCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = g.workDir

	// Send command header to UI immediately if program is set (before PTY starts)
	// Use special message to request smart empty line handling
	if g.program != nil {
		g.program.Send(releaseCommandStartMsg{command: command})
	}

	// Start PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: g.rows, Cols: g.cols})
	if err != nil {
		return "", fmt.Errorf("failed to start PTY: %w", err)
	}

	g.mu.Lock()
	g.ptyFile = ptmx
	g.cmd = cmd
	g.doneCh = make(chan struct{})
	g.vterm = NewVirtualTerminal(int(g.cols), int(g.rows))
	g.mu.Unlock()

	// Capture vterm reference for goroutines
	vterm := g.vterm

	// Read raw bytes from PTY and feed to virtual terminal
	var outputBuilder strings.Builder
	readDone := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				outputBuilder.Write(buf[:n])
				vterm.Write(buf[:n])
			}
			if err != nil {
				close(readDone)
				return
			}
		}
	}()

	// Throttled render loop (50ms = 20 FPS max)
	if g.program != nil {
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-g.doneCh:
					// Final render
					content := vterm.RenderScreen()
					g.program.Send(releaseScreenMsg{content: strings.TrimRight(content, " \n")})
					return
				case <-ticker.C:
					content := vterm.RenderScreen()
					// Trim trailing whitespace/newlines from vt10x screen
					content = strings.TrimRight(content, " \n")
					if content != "" {
						g.program.Send(releaseScreenMsg{content: content})
					}
				}
			}
		}()
	}

	// Wait for command to finish
	err = cmd.Wait()
	output := outputBuilder.String()

	// Wait for read goroutine to finish
	<-readDone

	// Signal done to render loop
	g.mu.Lock()
	close(g.doneCh)
	g.ptyFile.Close()
	g.ptyFile = nil
	g.cmd = nil
	g.vterm = nil
	g.mu.Unlock()

	// Small delay to allow final render
	time.Sleep(60 * time.Millisecond)

	return output, err
}

// Resize handles terminal resize
func (g *GitExecutor) Resize(rows, cols uint16) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.rows = rows
	g.cols = cols

	// Resize PTY if running
	if g.ptyFile != nil {
		pty.Setsize(g.ptyFile, &pty.Winsize{
			Rows: rows,
			Cols: cols,
		})
	}

	// Resize virtual terminal if running
	if g.vterm != nil {
		g.vterm.Resize(int(cols), int(rows))
	}

	return nil
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
	workDir              string
	version              string
	envBranch            string
	envName              string
	excludePatterns      []string
	branches             []string // MR source branches to merge
	sourceBranch         string   // Custom source branch name (e.g. release/rpb-1.0.0-root)
	sourceBranchIsRemote bool     // Whether source branch exists on remote
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

// NewReleaseCommandsWithSourceBranch creates a command builder with source branch configuration
func NewReleaseCommandsWithSourceBranch(workDir, version string, env *Environment, patterns []string, branches []string, sourceBranch string, sourceBranchIsRemote bool) *ReleaseCommands {
	return &ReleaseCommands{
		workDir:              workDir,
		version:              version,
		envBranch:            env.BranchName,
		envName:              env.Name,
		excludePatterns:      patterns,
		branches:             branches,
		sourceBranch:         sourceBranch,
		sourceBranchIsRemote: sourceBranchIsRemote,
	}
}

// RootBranch returns the release root branch name (source branch for accumulating MRs)
func (r *ReleaseCommands) RootBranch() string {
	// Use custom source branch if provided
	if r.sourceBranch != "" {
		return r.sourceBranch
	}
	return fmt.Sprintf("release/rpb-%s-root", r.version)
}

// EnvReleaseBranch returns the environment release branch name
func (r *ReleaseCommands) EnvReleaseBranch() string {
	return fmt.Sprintf("release/rpb-%s-%s", r.version, r.envBranch)
}

// Step1CheckoutRoot returns the command for step 1
// If source branch exists remotely, checkout from remote
// If not, create from root branch after pull
func (r *ReleaseCommands) Step1CheckoutRoot() string {
	if r.sourceBranchIsRemote {
		// Source branch exists remotely - checkout from remote to reliably use it locally
		return fmt.Sprintf("git checkout -B %s origin/%s", r.RootBranch(), r.RootBranch())
	}
	// Source branch doesn't exist - create from root after pull
	return fmt.Sprintf("git checkout root && git pull && git checkout -B %s root", r.RootBranch())
}

// Step2MergeBranch returns the command to merge a specific branch
func (r *ReleaseCommands) Step2MergeBranch(branchIndex int) string {
	if branchIndex >= len(r.branches) {
		return ""
	}
	return fmt.Sprintf("GIT_EDITOR=true git merge --no-edit origin/%s", r.branches[branchIndex])
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

// Step6PushSourceBranch returns the command to push source branch to remote
// This ensures the source branch with all merged MRs is available for the next release
func (r *ReleaseCommands) Step6PushSourceBranch() string {
	return fmt.Sprintf("git push -u origin %s", r.RootBranch())
}

// Step6Push returns the command for step 6 (push only, MR is created via API)
func (r *ReleaseCommands) Step6Push() string {
	return fmt.Sprintf("git push -u origin %s", r.EnvReleaseBranch())
}

// StepMergeToRoot returns the command to merge source branch to root and push
// This is step 6b: git checkout root && git merge <source branch> && git push
func (r *ReleaseCommands) StepMergeToRoot() string {
	return fmt.Sprintf("git checkout root && git merge %s && git push", r.RootBranch())
}

// StepMergeToDevelop returns the command to merge root to develop and push
// This is step 6c: git checkout develop && git pull && git merge root && git push
func (r *ReleaseCommands) StepMergeToDevelop() string {
	return "git checkout develop && git pull && git merge root && git push"
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
	var body strings.Builder
	for _, branch := range branches {
		body.WriteString("- " + branch + "\n")
	}
	return title, body.String()
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

// GetLastNLines returns the last n lines from a string
func GetLastNLines(output string, n int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= n {
		return output
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
