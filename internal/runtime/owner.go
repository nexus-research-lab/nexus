// INPUT: owner 身份、启动租约与待关闭的 runtime session。
// OUTPUT: owner epoch 栅栏、合并回收任务与锁外进程清理。
// POS: Manager 跨 session 启动和权限撤销之间的安全生命周期边界。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type ownerLifecycle struct {
	epoch    uint64
	startups map[*ownerStartupLease]struct{}
	reap     *ownerReapFlight
}

type ownerStartupLease struct {
	ownerUserID string
	sessionKey  string
	epoch       uint64
}

type ownerReapFlight struct {
	startupsDone   chan struct{}
	done           chan struct{}
	pendingStartup map[*ownerStartupLease]struct{}
	closingDone    []<-chan struct{}
	err            error
}

type ownerReapPlan struct {
	ownerUserID string
	flight      *ownerReapFlight
	reaper      OwnerProcessReaper
}

type ownerClosingSet struct {
	sessionKeys map[string]struct{}
	done        []<-chan struct{}
	hasOpen     bool
}

func (m *Manager) ensureOwnerLifecycleLocked(ownerUserID string) *ownerLifecycle {
	lifecycle := m.owners[ownerUserID]
	if lifecycle == nil {
		lifecycle = &ownerLifecycle{startups: make(map[*ownerStartupLease]struct{})}
		m.owners[ownerUserID] = lifecycle
	}
	return lifecycle
}

func (m *Manager) beginOwnerStartupLocked(
	ownerUserID string,
	sessionKey string,
) (*ownerStartupLease, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, nil
	}
	lifecycle := m.ensureOwnerLifecycleLocked(ownerUserID)
	if lifecycle.reap != nil {
		return nil, ErrRuntimeSessionClosing
	}
	lease := &ownerStartupLease{
		ownerUserID: ownerUserID,
		sessionKey:  strings.TrimSpace(sessionKey),
		epoch:       lifecycle.epoch,
	}
	lifecycle.startups[lease] = struct{}{}
	return lease, nil
}

func (m *Manager) releaseOwnerStartupLocked(lease *ownerStartupLease) {
	if lease == nil {
		return
	}
	lifecycle := m.owners[lease.ownerUserID]
	if lifecycle == nil {
		return
	}
	delete(lifecycle.startups, lease)
	flight := lifecycle.reap
	if flight == nil {
		return
	}
	if _, pending := flight.pendingStartup[lease]; !pending {
		return
	}
	delete(flight.pendingStartup, lease)
	if len(flight.pendingStartup) == 0 {
		close(flight.startupsDone)
	}
}

func (m *Manager) validateOwnerStartupLocked(lease *ownerStartupLease) error {
	if lease == nil {
		return nil
	}
	lifecycle := m.owners[lease.ownerUserID]
	if lifecycle == nil || lifecycle.epoch != lease.epoch {
		return agentclient.ErrAborted
	}
	if lifecycle.reap != nil {
		return ErrRuntimeSessionClosing
	}
	if _, ok := lifecycle.startups[lease]; !ok {
		return agentclient.ErrAborted
	}
	return nil
}

func (m *Manager) ownerReapActiveLocked(ownerUserID string) bool {
	ownerUserID = strings.TrimSpace(ownerUserID)
	return ownerUserID != "" && m.owners[ownerUserID] != nil && m.owners[ownerUserID].reap != nil
}

func (m *Manager) ownerClosingSetLocked(ownerUserID string) ownerClosingSet {
	closing := ownerClosingSet{sessionKeys: make(map[string]struct{})}
	for sessionKey, state := range m.sessions {
		if state == nil || state.OwnerUserID != ownerUserID {
			continue
		}
		if !state.Closing {
			closing.hasOpen = true
			continue
		}
		closing.sessionKeys[sessionKey] = struct{}{}
		if state.CloseDone != nil {
			closing.done = append(closing.done, state.CloseDone)
		}
	}
	return closing
}

func ownerCanReap(
	lifecycle *ownerLifecycle,
	closing ownerClosingSet,
	excludedStartup *ownerStartupLease,
) bool {
	if lifecycle == nil || closing.hasOpen {
		return false
	}
	for startup := range lifecycle.startups {
		if startup == excludedStartup {
			continue
		}
		if _, closingSession := closing.sessionKeys[startup.sessionKey]; !closingSession {
			return false
		}
	}
	return true
}

func (m *Manager) beginOwnerReapLocked(
	ownerUserID string,
	excludedStartup *ownerStartupLease,
	force bool,
) (*ownerReapPlan, *ownerReapFlight) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, nil
	}
	lifecycle := m.ensureOwnerLifecycleLocked(ownerUserID)
	if lifecycle.reap != nil {
		return nil, lifecycle.reap
	}
	closing := m.ownerClosingSetLocked(ownerUserID)
	if !force && (m.ownerProcessReaper == nil || !ownerCanReap(lifecycle, closing, excludedStartup)) {
		return nil, nil
	}

	lifecycle.epoch++
	flight := &ownerReapFlight{
		startupsDone:   make(chan struct{}),
		done:           make(chan struct{}),
		pendingStartup: make(map[*ownerStartupLease]struct{}),
		closingDone:    closing.done,
	}
	for startup := range lifecycle.startups {
		if startup != excludedStartup {
			flight.pendingStartup[startup] = struct{}{}
		}
	}
	if len(flight.pendingStartup) == 0 {
		close(flight.startupsDone)
	}
	lifecycle.reap = flight
	return &ownerReapPlan{
		ownerUserID: ownerUserID,
		flight:      flight,
		reaper:      m.ownerProcessReaper,
	}, flight
}

func (m *Manager) startOwnerReap(plan *ownerReapPlan) {
	if m == nil || plan == nil || plan.flight == nil {
		return
	}
	go func() {
		<-plan.flight.startupsDone
		var reapErr error
		if plan.reaper != nil {
			reapCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
			reapErr = plan.reaper.ReapOwnerProcesses(reapCtx, plan.ownerUserID)
			cancel()
		}

		m.mu.Lock()
		plan.flight.err = reapErr
		m.mu.Unlock()

		for _, done := range plan.flight.closingDone {
			_ = waitSessionClose(context.Background(), done)
		}
		m.mu.Lock()
		if lifecycle := m.owners[plan.ownerUserID]; lifecycle != nil && lifecycle.reap == plan.flight {
			lifecycle.reap = nil
		}
		close(plan.flight.done)
		m.mu.Unlock()
	}()
}

func waitOwnerReap(ctx context.Context, flight *ownerReapFlight) error {
	if flight == nil {
		return nil
	}
	select {
	case <-flight.done:
		return flight.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CloseOwnerSessions 关闭指定 owner 的全部 runtime，并取消仍在执行的 round。
func (m *Manager) CloseOwnerSessions(ctx context.Context, ownerUserID string) (int, error) {
	if m == nil || strings.TrimSpace(ownerUserID) == "" {
		return 0, nil
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	targets := make([]*sessionCloseTarget, 0)

	m.mu.Lock()
	if lifecycle := m.owners[ownerUserID]; lifecycle != nil && lifecycle.reap != nil {
		flight := lifecycle.reap
		m.mu.Unlock()
		return 0, waitOwnerReap(ctx, flight)
	}
	for sessionKey, state := range m.sessions {
		if state == nil || state.OwnerUserID != ownerUserID {
			continue
		}
		target, started, _ := m.beginSessionCloseLocked(sessionKey)
		if started {
			targets = append(targets, target)
		}
	}
	reapPlan, reapFlight := m.beginOwnerReapLocked(ownerUserID, nil, true)
	m.mu.Unlock()

	for _, target := range targets {
		cancelSessionCloseTarget(target)
	}
	m.startOwnerReap(reapPlan)

	errs := make([]error, 0, len(targets)+1)
	for _, target := range targets {
		var disconnectErr error
		if target.client != nil {
			disconnectCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
			disconnectErr = target.client.Disconnect(disconnectCtx)
			cancel()
		}
		idleDrainErr := waitIdleMessageDrain(ctx, target.idleMessageDrain)
		backgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
		roundErr := waitRoundDoneForClose(ctx, target.roundDone)
		clientCleanupPending := errors.Is(disconnectErr, context.Canceled) ||
			errors.Is(disconnectErr, context.DeadlineExceeded)
		if clientCleanupPending || idleDrainErr != nil || backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target, clientCleanupPending)
		} else {
			m.finishSessionClose(target)
		}
		err := errors.Join(disconnectErr, idleDrainErr, backgroundErr, roundErr)
		if err != nil && !IsRuntimeTransportClosedError(err) {
			errs = append(errs, fmt.Errorf(
				"close owner runtime session %s: %w",
				target.sessionKey,
				err,
			))
		}
	}
	if reaperErr := waitOwnerReap(ctx, reapFlight); reaperErr != nil {
		errs = append(errs, fmt.Errorf("reap owner runtime processes: %w", reaperErr))
	}
	return len(targets), errors.Join(errs...)
}
