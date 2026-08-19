// INPUT: logical run、物理 round、运行目标与权限阻塞信号。
// OUTPUT: 精确停止旧物理 attempt，并在其 observer 完成后开放安全续跑。
// POS: automation attempt 生命周期屏障；禁止带临时 callback 的续跑落入普通消息队列。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

const permissionAttemptDrainTimeout = 20 * time.Second

type physicalAttemptKey struct {
	runID   string
	roundID string
}

type physicalAttempt struct {
	done     chan struct{}
	stopDone chan struct{}
	stopOnce sync.Once
	stopErr  error
}

type physicalAttemptIdentity struct {
	job        automationdomain.ScheduledTask
	runID      string
	sessionKey string
	roundID    string
}

func newPhysicalAttemptKey(runID string, roundID string) physicalAttemptKey {
	return physicalAttemptKey{
		runID:   strings.TrimSpace(runID),
		roundID: strings.TrimSpace(roundID),
	}
}

func (s *Service) registerPhysicalAttempt(runID string, roundID string) func() {
	key := newPhysicalAttemptKey(runID, roundID)
	if key.runID == "" || key.roundID == "" {
		return func() {}
	}
	attempt := &physicalAttempt{
		done:     make(chan struct{}),
		stopDone: make(chan struct{}),
	}
	s.attemptMu.Lock()
	s.physicalAttempts[key] = attempt
	s.attemptMu.Unlock()

	var completeOnce sync.Once
	return func() {
		completeOnce.Do(func() {
			s.attemptMu.Lock()
			if s.physicalAttempts[key] == attempt {
				delete(s.physicalAttempts, key)
			}
			s.attemptMu.Unlock()
			close(attempt.done)
		})
	}
}

func (s *Service) physicalAttempt(runID string, roundID string) *physicalAttempt {
	key := newPhysicalAttemptKey(runID, roundID)
	if key.runID == "" || key.roundID == "" {
		return nil
	}
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	return s.physicalAttempts[key]
}

func (s *Service) stopBlockedPhysicalAttempt(
	job automationdomain.ScheduledTask,
	runID string,
	sessionKey string,
	roundID string,
) {
	attempt := s.physicalAttempt(runID, roundID)
	if attempt == nil {
		return
	}
	s.startPhysicalAttemptStop(attempt, physicalAttemptIdentity{
		job:        job,
		runID:      strings.TrimSpace(runID),
		sessionKey: strings.TrimSpace(sessionKey),
		roundID:    strings.TrimSpace(roundID),
	})
}

func (s *Service) drainPermissionBlockedAttempt(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
) error {
	attempt := s.physicalAttempt(run.RunID, run.RoundID)
	if attempt == nil {
		// 进程重启后内存中的旧 runtime 已不存在，持久 run 可直接按恢复协议继续。
		return nil
	}
	s.startPhysicalAttemptStop(attempt, physicalAttemptIdentity{
		job:        job,
		runID:      strings.TrimSpace(run.RunID),
		sessionKey: strings.TrimSpace(run.SessionKey),
		roundID:    strings.TrimSpace(run.RoundID),
	})

	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), permissionAttemptDrainTimeout)
	defer cancel()
	select {
	case <-attempt.done:
		return nil
	case <-attempt.stopDone:
		if attempt.stopErr != nil {
			select {
			case <-attempt.done:
				return nil
			default:
			}
			return fmt.Errorf("stop blocked automation attempt: %w", attempt.stopErr)
		}
	case <-waitCtx.Done():
		return fmt.Errorf("wait for blocked automation attempt stop: %w", waitCtx.Err())
	}

	select {
	case <-attempt.done:
		return nil
	case <-waitCtx.Done():
		return fmt.Errorf("wait for blocked automation attempt observer: %w", waitCtx.Err())
	}
}

func (s *Service) startPhysicalAttemptStop(attempt *physicalAttempt, identity physicalAttemptIdentity) {
	if attempt == nil {
		return
	}
	attempt.stopOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(
				backgroundContextForJobOwner(identity.job),
				permissionAttemptDrainTimeout,
			)
			defer cancel()
			attempt.stopErr = s.interruptPhysicalAttempt(ctx, identity)
			if attempt.stopErr != nil {
				s.loggerFor(ctx).Warn("权限阻塞后的物理 attempt 停止失败",
					"job_id", identity.job.JobID,
					"run_id", identity.runID,
					"round_id", identity.roundID,
					"session_key", identity.sessionKey,
					"err", attempt.stopErr,
				)
			}
			close(attempt.stopDone)
		}()
	})
}

func (s *Service) interruptPhysicalAttempt(ctx context.Context, identity physicalAttemptIdentity) error {
	if identity.sessionKey == "" || identity.roundID == "" {
		return errors.New("blocked automation attempt is missing session or round identity")
	}
	switch protocol.ParseSessionKey(identity.sessionKey).Kind {
	case protocol.SessionKeyKindAgent:
		runner, ok := s.dm.(dmInterruptRunner)
		if !ok || runner == nil {
			return errors.New("automation DM runner cannot stop an exact round")
		}
		err := runner.HandleInterrupt(ctx, dmsvc.InterruptRequest{
			SessionKey: identity.sessionKey,
			RoundID:    identity.roundID,
		})
		if errors.Is(err, dmsvc.ErrTargetDMRoundNotRunning) {
			return nil
		}
		return err
	case protocol.SessionKeyKindRoom:
		runner, ok := s.room.(roomInterruptRunner)
		if !ok || runner == nil {
			return errors.New("automation Room runner cannot stop an exact round")
		}
		err := runner.HandleInterrupt(ctx, roomrealtime.InterruptRequest{
			SessionKey: identity.sessionKey,
			RoundID:    identity.roundID,
		})
		if errors.Is(err, roomrealtime.ErrTargetRoomRoundNotRunning) {
			return nil
		}
		return err
	default:
		return errors.New("blocked automation attempt has an unsupported session target")
	}
}
