package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalWallClockAccounting struct {
	mu      sync.Mutex
	entries map[string]goalWallClockEntry
}

type goalWallClockEntry struct {
	goalID          string
	lastAccountedAt time.Time
}

func newGoalWallClockAccounting() *goalWallClockAccounting {
	return &goalWallClockAccounting{entries: map[string]goalWallClockEntry{}}
}

func (a *goalWallClockAccounting) markActive(sessionKey string, goalID string, now time.Time) {
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if a == nil || sessionKey == "" || goalID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[sessionKey]
	if ok && entry.goalID == goalID {
		return
	}
	a.entries[sessionKey] = goalWallClockEntry{
		goalID:          goalID,
		lastAccountedAt: now.UTC(),
	}
}

func (a *goalWallClockAccounting) clear(sessionKey string) {
	sessionKey = strings.TrimSpace(sessionKey)
	if a == nil || sessionKey == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.entries, sessionKey)
}

func (a *goalWallClockAccounting) pendingSeconds(sessionKey string, goalID string, now time.Time) int64 {
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if a == nil || sessionKey == "" || goalID == "" {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[sessionKey]
	if !ok || entry.goalID != goalID {
		a.entries[sessionKey] = goalWallClockEntry{
			goalID:          goalID,
			lastAccountedAt: now.UTC(),
		}
		return 0
	}
	elapsed := int64(now.UTC().Sub(entry.lastAccountedAt).Seconds())
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (a *goalWallClockAccounting) markAccounted(sessionKey string, goalID string, accountedSeconds int64) {
	if a == nil || accountedSeconds <= 0 {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if sessionKey == "" || goalID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[sessionKey]
	if !ok || entry.goalID != goalID {
		return
	}
	entry.lastAccountedAt = entry.lastAccountedAt.Add(time.Duration(accountedSeconds) * time.Second)
	a.entries[sessionKey] = entry
}

func (s *Service) accountActiveWallClockUsage(ctx context.Context, item protocol.Goal) (*protocol.Goal, error) {
	if s == nil || s.repo == nil || s.wallClock == nil {
		return &item, nil
	}
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		s.clearWallClockGoal(item)
		return &item, nil
	}
	now := s.nowFn()
	elapsedSeconds := s.wallClock.pendingSeconds(item.SessionKey, item.ID, now)
	if elapsedSeconds <= 0 {
		return &item, nil
	}
	expectedVersion := item.Version
	item.TimeUsedSeconds += elapsedSeconds
	item.Version++
	item.UpdatedAt = now
	updated, err := s.repo.UpdateGoal(ctx, item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	s.wallClock.markAccounted(item.SessionKey, item.ID, elapsedSeconds)
	return updated, nil
}

func (s *Service) markWallClockGoalActive(item protocol.Goal) {
	if s == nil || s.wallClock == nil {
		return
	}
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return
	}
	s.wallClock.markActive(item.SessionKey, item.ID, s.nowFn())
}

func (s *Service) clearWallClockGoal(item protocol.Goal) {
	if s == nil || s.wallClock == nil {
		return
	}
	s.wallClock.clear(item.SessionKey)
}
