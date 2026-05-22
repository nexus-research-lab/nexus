package goal

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const goalAutoResumeInterval = 10 * time.Second

// ContinuationDispatcher 把系统规划出的隐藏 Goal 续跑交给运行时执行。
type ContinuationDispatcher interface {
	IsGoalSessionBusy(sessionKey string) bool
	DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error
}

// StartAutoResume 启动 durable Goal 恢复循环。
func (s *Service) StartAutoResume(ctx context.Context, dispatcher ContinuationDispatcher) (func(), error) {
	if err := s.ensureEnabled(); err != nil {
		return func() {}, nil
	}
	if !s.config.GoalAutoContinueEnabled || dispatcher == nil {
		return func() {}, nil
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runAutoResumeLoop(loopCtx, dispatcher)
	}()
	return func() {
		cancel()
		<-done
	}, nil
}

// RunAutoResumeOnce 扫描并恢复一批 active Goal。测试和启动恢复共享同一条路径。
func (s *Service) RunAutoResumeOnce(ctx context.Context, dispatcher ContinuationDispatcher) error {
	if err := s.ensureEnabled(); err != nil {
		return err
	}
	if !s.config.GoalAutoContinueEnabled || dispatcher == nil {
		return nil
	}
	items, err := s.repo.ListRunnableGoals(ctx, 50)
	if err != nil {
		return err
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
			continue
		}
		if dispatcher.IsGoalSessionBusy(item.SessionKey) {
			continue
		}
		plan, planErr := s.PlanContinuationForSession(ctx, item.SessionKey, "")
		if errors.Is(planErr, ErrGoalNotFound) ||
			errors.Is(planErr, ErrGoalVersionStale) ||
			errors.Is(planErr, sql.ErrNoRows) {
			continue
		}
		if planErr != nil {
			return planErr
		}
		if plan == nil {
			continue
		}
		if dispatchErr := dispatcher.DispatchGoalContinuation(ctx, *plan); dispatchErr != nil {
			return dispatchErr
		}
	}
	return nil
}

func (s *Service) runAutoResumeLoop(ctx context.Context, dispatcher ContinuationDispatcher) {
	_ = s.RunAutoResumeOnce(ctx, dispatcher)

	ticker := time.NewTicker(goalAutoResumeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.RunAutoResumeOnce(ctx, dispatcher)
		}
	}
}
