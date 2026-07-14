// INPUT: durable active Goals、continuation dispatcher 与恢复节拍。
// OUTPUT: 经最终校验投递的隐藏续跑及可停止的自动恢复循环。
// POS: Goal 进程恢复和 idle 自动续跑的调度入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const goalAutoResumeInterval = 10 * time.Second

type activeGoalContinuationSuppressedKey struct{}

// ContinuationDispatcher 把系统规划出的隐藏 Goal 续跑交给运行时执行。
type ContinuationDispatcher interface {
	ShouldDeferGoalContinuation(context.Context, string) bool
	DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error
}

type continuationTargetChecker interface {
	GoalContinuationTargetMissing(context.Context, string) (bool, error)
}

// WithActiveGoalContinuationSuppressed 延后本次 Goal mutation 触发的隐藏续跑。
func WithActiveGoalContinuationSuppressed(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, activeGoalContinuationSuppressedKey{}, true)
}

// SetContinuationDispatcher 注入 idle Goal 续跑投递器，用于 active Goal 立即续跑。
func (s *Service) SetContinuationDispatcher(dispatcher ContinuationDispatcher) {
	s.continuations = dispatcher
}

// StartAutoResume 启动 durable Goal 恢复循环。
func (s *Service) StartAutoResume(ctx context.Context, dispatcher ContinuationDispatcher) (func(), error) {
	s.SetContinuationDispatcher(dispatcher)
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
		if err := s.dispatchContinuationForGoal(ctx, item, dispatcher); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) maybeDispatchActiveGoalContinuation(ctx context.Context, item protocol.Goal) {
	if activeGoalContinuationSuppressed(ctx) {
		return
	}
	s.DispatchActiveGoalContinuation(ctx, item)
}

// DispatchActiveGoalContinuation 显式触发 active Goal 的隐藏续跑。
func (s *Service) DispatchActiveGoalContinuation(ctx context.Context, item protocol.Goal) {
	if s == nil || s.continuations == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return
	}
	_ = s.dispatchContinuationForGoal(ctx, item, s.continuations)
}

func (s *Service) dispatchContinuationForGoal(ctx context.Context, item protocol.Goal, dispatcher ContinuationDispatcher) error {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil
	}
	cleared, err := s.clearMissingGoalContinuationTarget(ctx, item, dispatcher)
	if err != nil || cleared {
		return err
	}
	if dispatcher.ShouldDeferGoalContinuation(ctx, item.SessionKey) {
		return nil
	}
	plan, err := s.planAutoResumeContinuation(ctx, item.SessionKey)
	if err != nil || plan == nil {
		return err
	}
	cleared, err = s.clearMissingGoalContinuationTarget(ctx, plan.Goal, dispatcher)
	if err != nil || cleared {
		return err
	}
	plan, err = ValidateContinuationForDispatch(ctx, s, *plan, func(plan protocol.GoalContinuation) bool {
		return dispatcher.ShouldDeferGoalContinuation(ctx, plan.Goal.SessionKey)
	})
	if err != nil || plan == nil {
		return err
	}
	return s.dispatchPreparedContinuation(ctx, *plan, dispatcher)
}

func (s *Service) planAutoResumeContinuation(ctx context.Context, sessionKey string) (*protocol.GoalContinuation, error) {
	plan, err := s.PlanContinuationForSession(ctx, sessionKey, "")
	if errors.Is(err, ErrGoalNotFound) || errors.Is(err, ErrGoalVersionStale) || errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return plan, err
}

func (s *Service) dispatchPreparedContinuation(
	ctx context.Context,
	plan protocol.GoalContinuation,
	dispatcher ContinuationDispatcher,
) error {
	err := dispatcher.DispatchGoalContinuation(ctx, plan)
	if err == nil {
		return nil
	}
	if IsExpectedMutationError(err) {
		return nil
	}
	if errors.Is(err, ErrGoalContinuationTargetMissing) {
		_, cleanupErr := s.deleteGoal(ctx, plan.Goal, protocol.GoalUpdateSourceSystem)
		return cleanupErr
	}
	_, failureErr := s.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, err.Error(), plan.Goal.ObjectiveRevision())
	if IsExpectedMutationError(failureErr) {
		return nil
	}
	return failureErr
}

func (s *Service) clearMissingGoalContinuationTarget(
	ctx context.Context,
	item protocol.Goal,
	dispatcher ContinuationDispatcher,
) (bool, error) {
	checker, ok := dispatcher.(continuationTargetChecker)
	if !ok {
		return false, nil
	}
	missing, err := checker.GoalContinuationTargetMissing(ctx, item.SessionKey)
	if err != nil || !missing {
		return false, err
	}
	return s.deleteGoal(ctx, item, protocol.GoalUpdateSourceSystem)
}

func activeGoalContinuationSuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	suppressed, _ := ctx.Value(activeGoalContinuationSuppressedKey{}).(bool)
	return suppressed
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
