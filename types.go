package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
)

// Screen represents the current screen state
type screen int

const (
	screenLoading screen = iota
	screenAuth
	screenError
	screenMain
	screenEnvSelect
	screenVersion
	screenSourceBranch
	screenRootMerge
	screenConfirm
	screenRelease
)

// Environment represents a deployment environment
type Environment struct {
	Name       string // Display name: DEVELOP, TEST, STAGE, PROD
	BranchName string // Branch suffix: develop, testing, stable, master
}

// Credentials stored in keyring
type Credentials struct {
	GitLabURL string `json:"gitlab_url"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

// Messages for tea.Msg
type authResultMsg struct {
	err error
}

type checkCredsMsg struct {
	creds *Credentials
}

// ListItem represents a list item for the main screen
type listItem struct {
	title, desc string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

// MergeRequest represents a GitLab merge request
type MergeRequest struct {
	ID           int       `json:"id"`
	IID          int       `json:"iid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        string    `json:"state"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	CreatedAt    time.Time `json:"created_at"`
	Draft        bool      `json:"draft"`
	Author       struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	WebURL                      string `json:"web_url"`
	UserNotesCount              int    `json:"user_notes_count"`
	ChangesCount                string `json:"changes_count"`
	HasConflicts                bool   `json:"has_conflicts"`
	BlockingDiscussionsResolved bool   `json:"blocking_discussions_resolved"`
}

// MergeRequestDetails contains additional MR details
type MergeRequestDetails struct {
	MergeRequest
	DiffStats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"diff_stats"`
	CommitsCount        int `json:"-"`
	DiscussionsTotal    int `json:"-"`
	DiscussionsResolved int `json:"-"`
}

// mrListItem represents a merge request in the list
type mrListItem struct {
	mr *MergeRequestDetails
}

func (i mrListItem) Title() string { return i.mr.Title }
func (i mrListItem) Description() string {
	created := humanize.Time(i.mr.CreatedAt)
	return "@" + i.mr.Author.Username + " • " + created
}
func (i mrListItem) FilterValue() string      { return i.mr.Title }
func (i mrListItem) MR() *MergeRequestDetails { return i.mr }

// fetchMRsMsg is sent when MRs are fetched
type fetchMRsMsg struct {
	mrs []*MergeRequestDetails
	err error
}

// Project represents a GitLab project
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	NameWithNamespace string `json:"name_with_namespace"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
}

// fetchProjectsMsg is sent when projects are fetched
type fetchProjectsMsg struct {
	projects []Project
	err      error
}

// AppConfig represents the application configuration saved to file
type AppConfig struct {
	SelectedProjectID        int    `json:"selected_project_id"`
	SelectedProjectPath      string `json:"selected_project_path"`
	SelectedProjectName      string `json:"selected_project_name"`
	SelectedProjectShortName string `json:"selected_project_short_name"`

	// Release settings
	ExcludePatterns string `json:"exclude_patterns"` // File patterns to exclude from release, one per line
}

// ReleaseStep represents a step in the release process
type ReleaseStep int

const (
	ReleaseStepIdle            ReleaseStep = iota
	ReleaseStepCheckoutRoot                // Step 1: checkout/create source branch from root or remote
	ReleaseStepMergeBranches               // Step 2: git merge origin/{branch} for each MR
	ReleaseStepCheckoutEnv                 // Step 3: git checkout {env} && git pull && git checkout -B release/rpb-{ver}-{env}
	ReleaseStepCopyContent                 // Step 4: git rm -rf . && git checkout content && exclude files
	ReleaseStepCommit                      // Step 4b: git add -A && git commit (separate so retry doesn't redo file ops)
	ReleaseStepPushBranches                // Step 5: git push source branch and env release branch to remote
	ReleaseStepWaitForMR                   // Step 6: waiting for user to press "Create MR" button
	ReleaseStepPushAndCreateMR             // Step 7: create GitLab MR (branches already pushed)
	ReleaseStepMergeToRoot                 // Step 6b: merge source branch to root (if root merge enabled)
	ReleaseStepMergeToDevelop              // Step 6c: merge root to develop (if root merge enabled)
	ReleaseStepComplete                    // Done
)

// ReleaseError holds error details for a failed step
type ReleaseError struct {
	Step    ReleaseStep `json:"step"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

// ReleaseState represents the persistent state of an in-progress release
type ReleaseState struct {
	// Selection info
	SelectedMRIIDs       []int       `json:"selected_mr_iids"`
	MRBranches           []string    `json:"mr_branches"`            // Source branches in merge order
	Environment          Environment `json:"environment"`
	Version              string      `json:"version"`
	SourceBranch         string      `json:"source_branch"`          // Source branch for accumulating MRs (e.g. release/rpb_1.0.0_root)
	SourceBranchIsRemote bool        `json:"source_branch_is_remote"` // Whether source branch exists on remote (determines checkout strategy)
	RootMerge            bool        `json:"root_merge"`             // Whether to merge release to root and root to develop
	ProjectID            int         `json:"project_id"`

	// Progress tracking
	CurrentStep     ReleaseStep `json:"current_step"`
	LastSuccessStep ReleaseStep `json:"last_success_step"`
	CurrentMRIndex  int         `json:"current_mr_index"` // For step 2: which MR we're merging
	MergedBranches  []string    `json:"merged_branches"`  // Successfully merged branches

	// Error info
	LastError   *ReleaseError `json:"last_error,omitempty"`
	ErrorOutput string        `json:"error_output,omitempty"` // Last 5000 lines of terminal output on error

	// Terminal output (saved after each operation for resume)
	TerminalOutput []string `json:"terminal_output,omitempty"`

	// Created MR info (after step 6)
	CreatedMRURL string `json:"created_mr_url,omitempty"`
	CreatedMRIID int    `json:"created_mr_iid,omitempty"`

	// Working directory
	WorkDir string `json:"work_dir"` // Project root path
}

// ReleaseButton represents an action button in the release screen
type ReleaseButton int

const (
	ReleaseButtonAbort ReleaseButton = iota
	ReleaseButtonRetry
	ReleaseButtonCreateMR
	ReleaseButtonComplete
	ReleaseButtonOpen
)

// Bubble Tea messages for release execution
type releaseOutputMsg struct {
	line string
}

type releaseScreenMsg struct {
	content string
}

type releaseCommandStartMsg struct {
	command string
}

type releaseCommandEndMsg struct {
	firstOutputLine string // First non-empty line of output, or empty if no output
}

type releaseStepCompleteMsg struct {
	step   ReleaseStep
	err    error
	output string
}

type existingReleaseMsg struct {
	state *ReleaseState
}

type releaseMRCreatedMsg struct {
	url string
	iid int
	err error
}

type setProgramMsg struct {
	program *tea.Program
}

// sourceBranchCheckMsg is sent when the source branch remote check completes
type sourceBranchCheckMsg struct {
	branchName   string // The branch name that was checked
	exists       bool   // Whether the remote branch exists
	sameAsRoot   bool   // If exists, whether it points to same commit as root
	err          error  // Error if check failed
}
