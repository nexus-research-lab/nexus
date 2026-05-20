package memory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type memoryCheckpoints struct {
	Scopes map[string]memoryScopeCheckpoint `json:"scopes"`
}

type memoryScopeCheckpoint struct {
	TurnCount     int       `json:"turn_count"`
	LastRoundID   string    `json:"last_round_id,omitempty"`
	LastExtractAt time.Time `json:"last_extract_at,omitempty"`
	RoundIDs      []string  `json:"round_ids,omitempty"`
}

func (r *Repository) ListEntries(limit int) ([]*Entry, error) {
	capacityHint := limit
	if capacityHint <= 0 {
		capacityHint = 128
	}
	items := make([]*Entry, 0, capacityHint)
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, err := r.parser.Parse(string(content), toRelative(r.workspacePath, path))
		if err != nil {
			return nil, err
		}
		for index := len(parsed) - 1; index >= 0; index-- {
			items = append(items, parsed[index])
			if limit > 0 && len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

func (r *Repository) FindEntry(entryID string) (*Entry, error) {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return nil, errors.New("entry_id 不能为空")
	}
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		entries, err := r.parser.Parse(string(content), toRelative(r.workspacePath, path))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.ID == entryID {
				return entry, nil
			}
		}
	}
	return nil, errors.New("未找到条目: " + entryID)
}

func (r *Repository) DeleteEntry(entryID string) error {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return errors.New("entry_id 不能为空")
	}
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relativePath := toRelative(r.workspacePath, path)
		entries, err := r.parser.Parse(string(content), relativePath)
		if err != nil {
			return err
		}
		next := make([]*Entry, 0, len(entries))
		found := false
		for _, entry := range entries {
			if entry.ID == entryID {
				found = true
				continue
			}
			next = append(next, entry)
		}
		if !found {
			continue
		}
		if len(next) == 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
			return nil
		}
		return os.WriteFile(path, []byte(renderEntries(next)), 0o644)
	}
	return errors.New("未找到条目: " + entryID)
}

func (r *Repository) ReadStableContext(maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 3200
	}
	type rootFile struct {
		name  string
		title string
	}
	files := []rootFile{
		{name: "USER.md", title: "USER"},
		{name: "MEMORY.md", title: "MEMORY"},
	}
	lines := make([]string, 0, len(files)*4)
	total := 0
	for _, file := range files {
		path := filepath.Join(r.workspacePath, file.name)
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		block := "# " + file.title + "\n" + trimmed
		if total+len(block) > maxChars {
			remaining := maxChars - total
			if remaining <= 0 {
				break
			}
			block = truncateRunes(block, remaining)
		}
		lines = append(lines, block)
		total += len(block)
		if total >= maxChars {
			break
		}
	}
	return strings.Join(lines, "\n\n"), nil
}

func (r *Repository) AppendSessionSummary(sessionKey string, content string) (string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	content = strings.TrimSpace(content)
	if sessionKey == "" || content == "" {
		return "", nil
	}
	relativePath := filepath.ToSlash(filepath.Join("memory", "sessions", safeMemoryFilename(sessionKey)+".md"))
	targetPath := filepath.Join(r.workspacePath, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	existing := ""
	if current, err := os.ReadFile(targetPath); err == nil {
		existing = strings.TrimRight(string(current), "\n")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	next := strings.TrimSpace(content) + "\n"
	if existing != "" {
		next = existing + "\n\n" + next
	}
	return relativePath, os.WriteFile(targetPath, []byte(next), 0o644)
}

func (r *Repository) ReadSessionSummary(sessionKey string) (string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return "", nil
	}
	targetPath := filepath.Join(r.workspacePath, "memory", "sessions", safeMemoryFilename(sessionKey)+".md")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func (r *Repository) ReadCheckpoints() (memoryCheckpoints, error) {
	path := r.checkpointPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return memoryCheckpoints{Scopes: map[string]memoryScopeCheckpoint{}}, nil
		}
		return memoryCheckpoints{}, err
	}
	var checkpoints memoryCheckpoints
	if err := json.Unmarshal(content, &checkpoints); err != nil {
		return memoryCheckpoints{}, err
	}
	if checkpoints.Scopes == nil {
		checkpoints.Scopes = map[string]memoryScopeCheckpoint{}
	}
	return checkpoints, nil
}

func (r *Repository) WriteCheckpoints(checkpoints memoryCheckpoints) error {
	if checkpoints.Scopes == nil {
		checkpoints.Scopes = map[string]memoryScopeCheckpoint{}
	}
	path := r.checkpointPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(checkpoints, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func (r *Repository) checkpointPath() string {
	return filepath.Join(r.workspacePath, "memory", "checkpoints.json")
}

func (r *Repository) CheckpointCount() (int, error) {
	checkpoints, err := r.ReadCheckpoints()
	if err != nil {
		return 0, err
	}
	return len(checkpoints.Scopes), nil
}

func pruneRoundIDs(values []string) []string {
	const maxRoundIDs = 80
	clean := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	if len(clean) > maxRoundIDs {
		clean = clean[len(clean)-maxRoundIDs:]
	}
	return clean
}

func roundIDProcessed(values []string, roundID string) bool {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" {
		return false
	}
	for _, value := range values {
		if value == roundID {
			return true
		}
	}
	return false
}

func safeMemoryFilename(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	builder := strings.Builder{}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_' || char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "session"
	}
	if len(result) > 96 {
		result = result[:96]
	}
	return result
}

func joinScopeParts(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 {
		return "unknown"
	}
	return strings.Join(clean, ":")
}

func sortMemoryItems(items []MemoryItem) {
	sort.SliceStable(items, func(i int, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].AccessCount != items[j].AccessCount {
			return items[i].AccessCount > items[j].AccessCount
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}
