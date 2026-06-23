package memory

import (
	"os"
	"path/filepath"
	"strings"
)

func (r *Repository) CleanupOrphans(entryIDs map[string]struct{}, scopes map[string]struct{}) (MemoryCleanupResult, error) {
	result := MemoryCleanupResult{}
	if err := r.cleanupSessionSummaries(entryIDs, &result); err != nil {
		return MemoryCleanupResult{}, err
	}
	if err := r.cleanupCheckpoints(scopes, &result); err != nil {
		return MemoryCleanupResult{}, err
	}
	if err := r.cleanupEmptyDiaries(&result); err != nil {
		return MemoryCleanupResult{}, err
	}
	return result, nil
}

func (r *Repository) cleanupSessionSummaries(entryIDs map[string]struct{}, result *MemoryCleanupResult) error {
	sessionsDir := filepath.Join(r.workspacePath, "memory", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(sessionsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		refs := extractSessionSummaryEntryIDs(string(content))
		if len(refs) == 0 || hasAnyEntryID(refs, entryIDs) {
			continue
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		result.RemovedSessionFiles++
		result.RemovedFiles = append(result.RemovedFiles, toRelative(r.workspacePath, path))
	}
	return nil
}

func (r *Repository) cleanupCheckpoints(scopes map[string]struct{}, result *MemoryCleanupResult) error {
	checkpoints, err := r.ReadCheckpoints()
	if err != nil {
		return err
	}
	if len(checkpoints.Scopes) == 0 {
		return nil
	}
	for scope := range checkpoints.Scopes {
		if _, ok := scopes[scope]; ok {
			continue
		}
		delete(checkpoints.Scopes, scope)
		result.RemovedCheckpoints++
	}
	if result.RemovedCheckpoints == 0 {
		return nil
	}
	path := r.checkpointPath()
	if len(checkpoints.Scopes) == 0 {
		if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		result.RemovedFiles = append(result.RemovedFiles, toRelative(r.workspacePath, path))
		return nil
	}
	return r.WriteCheckpoints(checkpoints)
}

func (r *Repository) cleanupEmptyDiaries(result *MemoryCleanupResult) error {
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries, err := r.parser.Parse(string(content), toRelative(r.workspacePath, path))
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			continue
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		result.RemovedEmptyDiaries++
		result.RemovedFiles = append(result.RemovedFiles, toRelative(r.workspacePath, path))
	}
	return nil
}

func extractSessionSummaryEntryIDs(content string) []string {
	matches := sessionSummaryEntryPattern.FindAllStringSubmatch(content, -1)
	items := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		entryID := strings.TrimSpace(match[1])
		if entryID == "" {
			continue
		}
		if _, ok := seen[entryID]; ok {
			continue
		}
		seen[entryID] = struct{}{}
		items = append(items, entryID)
	}
	return items
}

func hasAnyEntryID(values []string, entryIDs map[string]struct{}) bool {
	for _, value := range values {
		if _, ok := entryIDs[value]; ok {
			return true
		}
	}
	return false
}
