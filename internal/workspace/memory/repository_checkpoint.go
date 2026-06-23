package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

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
