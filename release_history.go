package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	releasesDir      = "releases"
	historyIndexFile = "index.json"
)

// getReleasesDir returns the path to the releases history directory, creating it if needed
func getReleasesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", ".relix", releasesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// generateReleaseID creates a timestamp-based unique ID
func generateReleaseID() string {
	return time.Now().Format("20060102-150405")
}

// buildReleaseTag constructs the tag string from version and env (e.g., "5.2-v13")
func buildReleaseTag(state *ReleaseState) string {
	// Tag is stored in state if root merge was done
	if state.TagName != "" {
		// Strip env prefix from tag name (e.g., "dev-1.0.0-v2" -> "1.0.0-v2")
		parts := strings.SplitN(state.TagName, "-", 2)
		if len(parts) > 1 {
			return parts[1]
		}
		return state.TagName
	}
	return state.Version
}

// SaveReleaseHistory saves a completed or aborted release to history
func SaveReleaseHistory(state *ReleaseState, status string, terminalOutput []string) error {
	dir, err := getReleasesDir()
	if err != nil {
		return err
	}

	id := generateReleaseID()
	now := time.Now()

	indexEntry := HistoryIndexEntry{
		ID:          id,
		Tag:         buildReleaseTag(state),
		Environment: state.Environment.Name,
		DateTime:    now,
		MRCount:     len(state.MRBranches),
		Status:      status,
		Version:     state.Version,
	}

	detail := &ReleaseHistoryEntry{
		HistoryIndexEntry: indexEntry,
		MRBranches:        state.MRBranches,
		MRURLs:            state.MRURLs,
		MRIIDs:            state.SelectedMRIIDs,
		MRCommitSHAs:      state.MRCommitSHAs,
		SourceBranch:      state.SourceBranch,
		EnvBranch:         state.Environment.BranchName,
		RootMerge:         state.RootMerge,
		CreatedMRURL:      state.CreatedMRURL,
		TerminalOutput:    terminalOutput,
	}

	// Save individual detail file
	detailPath := filepath.Join(dir, id+".json")
	detailData, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal detail: %w", err)
	}
	if err := os.WriteFile(detailPath, detailData, 0o644); err != nil {
		return fmt.Errorf("write detail: %w", err)
	}

	// Update index file (preserve existing entries, only start fresh if file doesn't exist)
	index, err := LoadHistoryIndex()
	if err != nil {
		// Index is corrupt - try to read raw file and back it up before overwriting
		if dir2, err2 := getReleasesDir(); err2 == nil {
			src := filepath.Join(dir2, historyIndexFile)
			if _, statErr := os.Stat(src); statErr == nil {
				os.Rename(src, src+".bak")
			}
		}
		index = nil
	}
	index = append([]HistoryIndexEntry{indexEntry}, index...)

	indexPath := filepath.Join(dir, historyIndexFile)
	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := os.WriteFile(indexPath, indexData, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}

// LoadHistoryIndex loads the history index for quick list display
func LoadHistoryIndex() ([]HistoryIndexEntry, error) {
	dir, err := getReleasesDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, historyIndexFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []HistoryIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// DeleteHistoryEntries removes the specified entries from the index and deletes their detail files
func DeleteHistoryEntries(ids map[string]bool) error {
	dir, err := getReleasesDir()
	if err != nil {
		return err
	}

	// Load current index
	index, err := LoadHistoryIndex()
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}

	// Filter out deleted entries
	filtered := make([]HistoryIndexEntry, 0, len(index))
	for _, entry := range index {
		if !ids[entry.ID] {
			filtered = append(filtered, entry)
		}
	}

	// Write back filtered index
	indexPath := filepath.Join(dir, historyIndexFile)
	indexData, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := os.WriteFile(indexPath, indexData, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	// Delete detail files (best effort)
	for id := range ids {
		os.Remove(filepath.Join(dir, id+".json"))
	}

	return nil
}

// LoadHistoryDetail loads full details for a specific release
func LoadHistoryDetail(id string) (*ReleaseHistoryEntry, error) {
	dir, err := getReleasesDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, err
	}

	var entry ReleaseHistoryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}
