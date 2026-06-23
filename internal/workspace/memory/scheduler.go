package memory

import (
	"slices"
	"strings"
	"time"
)

// ScheduleDecision 描述一次自动抽取调度判断。
type ScheduleDecision struct {
	Checkpoint    memoryScopeCheckpoint
	ShouldCapture bool
	Reason        string
}

// MemoryScheduler 负责 checkpoint 去重与轻量抽取节流。
type MemoryScheduler struct {
	repository *Repository
	everyTurns int
	idleAfter  time.Duration
}

// NewMemoryScheduler 创建记忆调度器。
func NewMemoryScheduler(repository *Repository) *MemoryScheduler {
	return &MemoryScheduler{
		repository: repository,
		everyTurns: 5,
		idleAfter:  10 * time.Minute,
	}
}

// Advance 记录本轮 checkpoint，并判断是否需要抽取记忆。
func (s *MemoryScheduler) Advance(scopeKey string, roundID string, now time.Time, highImpact bool) (ScheduleDecision, error) {
	if s == nil || s.repository == nil {
		return ScheduleDecision{Reason: "scheduler_unavailable"}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	scopeKey = strings.TrimSpace(scopeKey)
	roundID = strings.TrimSpace(roundID)
	checkpoints, err := s.repository.ReadCheckpoints()
	if err != nil {
		return ScheduleDecision{}, err
	}
	checkpoint := checkpoints.Scopes[scopeKey]
	if roundID != "" && slices.Contains(checkpoint.RoundIDs, roundID) {
		return ScheduleDecision{
			Checkpoint:    checkpoint,
			ShouldCapture: false,
			Reason:        "duplicate_round",
		}, nil
	}
	checkpoint.TurnCount++
	if roundID != "" {
		checkpoint.LastRoundID = roundID
		checkpoint.RoundIDs = pruneRoundIDs(append(checkpoint.RoundIDs, roundID))
	}
	idleReady := checkpoint.LastExtractAt.IsZero() || now.Sub(checkpoint.LastExtractAt) >= s.idleAfter
	turnReady := checkpoint.TurnCount == 1 || checkpoint.TurnCount%s.everyTurns == 0
	shouldCapture := highImpact || idleReady || turnReady
	reason := "scheduler_wait"
	if shouldCapture {
		checkpoint.LastExtractAt = now
		reason = "captured"
	}
	checkpoints.Scopes[scopeKey] = checkpoint
	if err := s.repository.WriteCheckpoints(checkpoints); err != nil {
		return ScheduleDecision{}, err
	}
	return ScheduleDecision{
		Checkpoint:    checkpoint,
		ShouldCapture: shouldCapture,
		Reason:        reason,
	}, nil
}
