package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GitLabClient handles GitLab API requests
type GitLabClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewGitLabClient creates a new GitLab API client
func NewGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetUserEmails retrieves the authenticated user's emails
func (c *GitLabClient) GetUserEmails() ([]string, error) {
	url := c.baseURL + "/api/v4/user/emails"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid token: authentication failed")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error: status %d", resp.StatusCode)
	}

	var emails []struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := make([]string, len(emails))
	for i, e := range emails {
		result[i] = e.Email
	}

	return result, nil
}

// ValidateCredentials checks if the credentials are valid and email matches
func ValidateCredentials(creds Credentials) error {
	client := NewGitLabClient(creds.GitLabURL, creds.Token)

	emails, err := client.GetUserEmails()
	if err != nil {
		return err
	}

	// Check if provided email matches any of the user's emails
	for _, email := range emails {
		if strings.EqualFold(email, creds.Email) {
			return nil
		}
	}

	return fmt.Errorf("email '%s' not found in your GitLab account", creds.Email)
}

// GetOpenMergeRequests fetches open merge requests for the current user
func (c *GitLabClient) GetOpenMergeRequests() ([]*MergeRequestDetails, error) {
	// Get MRs where user is assignee or reviewer
	url := c.baseURL + "/api/v4/merge_requests?state=opened&scope=all&per_page=100"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error: status %d", resp.StatusCode)
	}

	var mrs []MergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Fetch additional details for each MR
	result := make([]*MergeRequestDetails, 0, len(mrs))
	for _, mr := range mrs {
		details, err := c.GetMergeRequestDetails(mr)
		if err != nil {
			// Skip MRs we can't get details for
			continue
		}
		result = append(result, details)
	}

	return result, nil
}

// GetMergeRequestDetails fetches detailed info for a merge request
func (c *GitLabClient) GetMergeRequestDetails(mr MergeRequest) (*MergeRequestDetails, error) {
	details := &MergeRequestDetails{MergeRequest: mr}

	// Extract project path from web URL
	// URL format: https://gitlab.com/namespace/project/-/merge_requests/123
	projectPath := extractProjectPath(mr.WebURL)
	if projectPath == "" {
		return details, nil
	}

	// Get commits count
	commitsURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/commits",
		c.baseURL, strings.ReplaceAll(projectPath, "/", "%2F"), mr.IID)
	commits, err := c.fetchJSON(commitsURL)
	if err == nil {
		if arr, ok := commits.([]interface{}); ok {
			details.CommitsCount = len(arr)
		}
	}

	// Get discussions stats
	discussionsURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/discussions",
		c.baseURL, strings.ReplaceAll(projectPath, "/", "%2F"), mr.IID)
	discussions, err := c.fetchJSON(discussionsURL)
	if err == nil {
		if arr, ok := discussions.([]interface{}); ok {
			for _, d := range arr {
				if disc, ok := d.(map[string]interface{}); ok {
					if notes, ok := disc["notes"].([]interface{}); ok && len(notes) > 0 {
						details.DiscussionsTotal++
						// Check if first note is resolvable and resolved
						if note, ok := notes[0].(map[string]interface{}); ok {
							if resolvable, ok := note["resolvable"].(bool); ok && resolvable {
								if resolved, ok := note["resolved"].(bool); ok && resolved {
									details.DiscussionsResolved++
								}
							}
						}
					}
				}
			}
		}
	}

	return details, nil
}

// fetchJSON makes a GET request and returns parsed JSON
func (c *GitLabClient) fetchJSON(url string) (interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetProjects fetches projects the user has access to
func (c *GitLabClient) GetProjects() ([]Project, error) {
	url := c.baseURL + "/api/v4/projects?membership=true&per_page=100&order_by=last_activity_at"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error: status %d", resp.StatusCode)
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return projects, nil
}

// GetProjectMergeRequests fetches open merge requests for a specific project
func (c *GitLabClient) GetProjectMergeRequests(projectID int) ([]*MergeRequestDetails, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests?state=opened&per_page=100", c.baseURL, projectID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error: status %d", resp.StatusCode)
	}

	var mrs []MergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Fetch additional details for each MR
	result := make([]*MergeRequestDetails, 0, len(mrs))
	for _, mr := range mrs {
		details, err := c.GetMergeRequestDetails(mr)
		if err != nil {
			continue
		}
		result = append(result, details)
	}

	return result, nil
}

// extractProjectPath extracts project path from MR web URL
func extractProjectPath(webURL string) string {
	// URL format: https://gitlab.com/namespace/project/-/merge_requests/123
	// or: https://gitlab.com/group/subgroup/project/-/merge_requests/123
	idx := strings.Index(webURL, "/-/merge_requests/")
	if idx == -1 {
		return ""
	}

	// Get everything after the host
	path := webURL[:idx]
	// Remove protocol and host
	if strings.HasPrefix(path, "https://") {
		path = path[8:]
	} else if strings.HasPrefix(path, "http://") {
		path = path[7:]
	}

	// Remove host part (everything before first /)
	slashIdx := strings.Index(path, "/")
	if slashIdx == -1 {
		return ""
	}

	return path[slashIdx+1:]
}
