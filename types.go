package main

// Screen represents the current screen state
type screen int

const (
	screenAuth screen = iota
	screenError
	screenMain
)

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
	ID           int    `json:"id"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Author       struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	WebURL                       string `json:"web_url"`
	UserNotesCount               int    `json:"user_notes_count"`
	ChangesCount                 string `json:"changes_count"`
	HasConflicts                 bool   `json:"has_conflicts"`
	BlockingDiscussionsResolved  bool   `json:"blocking_discussions_resolved"`
}

// MergeRequestDetails contains additional MR details
type MergeRequestDetails struct {
	MergeRequest
	DiffStats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"diff_stats"`
	CommitsCount       int `json:"-"`
	DiscussionsTotal   int `json:"-"`
	DiscussionsResolved int `json:"-"`
}

// mrListItem represents a merge request in the list
type mrListItem struct {
	mr *MergeRequestDetails
}

func (i mrListItem) Title() string       { return i.mr.Title }
func (i mrListItem) Description() string { return "@" + i.mr.Author.Username }
func (i mrListItem) FilterValue() string { return i.mr.Title }
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
}
