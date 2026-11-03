// INPUT: Owner-scoped queue identity and explicit user stop/resume decisions.
// OUTPUT: Durable dispatch gates shared by live dispatch and startup recovery; queue discovery that skips unreadable journals.
// POS: Input queue journal control records; reading snapshots never resumes work.
package workspace

import (
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const inputQueueActionPause = "dispatch_pause"

// SetDispatchPaused persists the user's gate independently of queue contents.
func (s *InputQueueStore) SetDispatchPaused(location InputQueueLocation, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendActionLocked(location, map[string]any{
		"action": inputQueueActionPause, "paused": paused,
		"scope": string(location.Scope), "owner_user_id": location.OwnerUserID,
		"timestamp": time.Now().UnixMilli(),
	})
}

// DispatchPaused reads the durable gate; callers fail closed on read errors.
func (s *InputQueueStore) DispatchPaused(location InputQueueLocation) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.inputQueueRowsLocked(location)
	if err != nil {
		return false, err
	}
	replay := newInputQueueReplay(location)
	for _, row := range rows {
		replay.apply(row)
	}
	return replay.paused, nil
}

func (r *inputQueueReplay) pause(row map[string]any) {
	item := protocol.InputQueueItem{Scope: protocol.InputQueueScope(stringFromAny(row["scope"])), OwnerUserID: stringFromAny(row["owner_user_id"])}
	if !inputQueueItemMatchesLocationScope(r.location, item) || !inputQueueItemMatchesLocationOwner(r.location, item) {
		return
	}
	r.paused, _ = row["paused"].(bool)
}

// LocationsForWorkspace discovers queues even before a session meta was created.
// The journal's identity must match the canonical physical directory and owner.
// An unreadable session journal (for example a symlink) is skipped and reported
// in the joined error, so one bad session never hides the remaining queues.
func (s *InputQueueStore) LocationsForWorkspace(owner, workspace string) ([]InputQueueLocation, error) {
	root, err := s.files.ForOwner(owner).openWorkspaceRoot(workspace, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	sessions, err := root.OpenRootNoSymlink(".agents/sessions")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer sessions.Close()
	entries, err := fs.ReadDir(sessions.FS(), ".")
	if err != nil {
		return nil, err
	}
	var locations []InputQueueLocation
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		rows, err := readJSONLAtRoot(sessions, filepath.Join(entry.Name(), "input_queue.jsonl"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("read input queue journal %s: %w", entry.Name(), err))
			continue
		}
		seen := map[protocol.InputQueueScope]bool{}
		for _, row := range rows {
			item, ok := inputQueueItemFromAny(row["item"])
			if !ok || item.SessionKey == "" || item.OwnerUserID != owner || entry.Name() != encodeSessionDirName(item.SessionKey) {
				continue
			}
			scope := protocol.NormalizeInputQueueScope(string(item.Scope))
			if seen[scope] {
				continue
			}
			seen[scope] = true
			locations = append(locations, InputQueueLocation{OwnerUserID: owner, WorkspacePath: workspace, SessionKey: item.SessionKey, Scope: scope, RoomID: item.RoomID, ConversationID: item.ConversationID})
		}
	}
	return locations, errors.Join(failures...)
}

// ClaimDispatch retains the item durably until downstream acceptance is proven.
// A surviving claim after restart is an unknown outcome, never a fresh send.
func (s *InputQueueStore) ClaimDispatch(location InputQueueLocation, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.snapshotLocked(location)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		if item.DispatchPaused {
			return errors.New("queue dispatch is paused")
		}
		item.DispatchPending = true
		return s.appendActionLocked(location, map[string]any{"action": inputQueueActionUpdate, "item": item, "timestamp": time.Now().UnixMilli()})
	}
	return errors.New("queue dispatch item no longer exists")
}

// ClaimFirstDispatchable selects without deleting; caller holds its dispatch gate.
func (s *InputQueueStore) ClaimFirstDispatchable(location InputQueueLocation) (*protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	items, err := s.Snapshot(location)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range items {
		if item.DispatchPaused || protocol.ShouldGuideRunningRound(item.DeliveryPolicy) {
			continue
		}
		if err := s.ClaimDispatch(location, item.ID); err != nil {
			return nil, nil, err
		}
		return &item, items, nil
	}
	return nil, items, nil
}
