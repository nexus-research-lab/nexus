// INPUT: 待关闭 session 的 client、round、idle drain 与后台任务状态。
// OUTPUT: 一次性关闭快照、可等待的清理结果及失败时保留的 Session 栅栏。
// POS: owner、idle 与显式 session 关闭共用的生命周期原语。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrRuntimeSessionClosing 表示 session 正在退出，不能再注册新的运行任务。
var ErrRuntimeSessionClosing = errors.New("runtime session is closing")

// RoundIdleAbortTimeout 是 round 空闲后中断/断连的共享宽限。
const RoundIdleAbortTimeout = 5 * time.Second

// sessionCloseTarget 保存关闭期间需要取消、等待和最终移除的 session 状态。
type sessionCloseTarget struct {
	sessionKey        string
	state             *sessionState
	ownerUserID       string
	client            Client
	sandboxReceipt    *SandboxEffectivePolicyReceipt
	roundCancels      []context.CancelFunc
	roundDone         []chan struct{}
	idleMessageDrain  *idleMessageDrain
	backgroundCancels []context.CancelFunc
	backgroundDone    <-chan struct{}
	closeDone         chan struct{}
}

func (m *Manager) markSandboxReceiptRetiring(target *sessionCloseTarget) error {
	if target == nil || target.client == nil {
		return nil
	}
	return m.updateSandboxReceiptPhase(
		target.ownerUserID,
		target.sessionKey,
		target.state.StartupGeneration,
		target.client,
		target.sandboxReceipt,
		protocol.SandboxPolicyReceiptRetiring,
		"",
	)
}

func (m *Manager) finalizeSandboxReceipt(target *sessionCloseTarget, cleanupErr error) error {
	if target == nil || target.client == nil {
		return nil
	}
	phase := protocol.SandboxPolicyReceiptRetired
	reason := ""
	if cleanupErr != nil {
		phase = protocol.SandboxPolicyReceiptUnknown
		reason = sandboxReceiptReason(cleanupErr)
	}
	return m.updateSandboxReceiptPhase(
		target.ownerUserID,
		target.sessionKey,
		target.state.StartupGeneration,
		target.client,
		target.sandboxReceipt,
		phase,
		reason,
	)
}

// markSandboxReceiptUnknownAfterReaper records a conservative cleanup result
// when the owner-level process reaper fails after the bridge session itself has
// already reported a clean close. A retired receipt must not hide descendants
// that the host could not prove were collected.
func (m *Manager) markSandboxReceiptUnknownAfterReaper(target *sessionCloseTarget, reaperErr error) error {
	if target == nil || target.client == nil || reaperErr == nil {
		return nil
	}
	return m.updateSandboxReceiptPhase(
		target.ownerUserID,
		target.sessionKey,
		target.state.StartupGeneration,
		target.client,
		target.sandboxReceipt,
		protocol.SandboxPolicyReceiptUnknown,
		fmt.Sprintf("owner process reaper failed: %s", reaperErr),
	)
}

// beginSessionCloseLocked 把 session 切换到不可再接收新任务的 closing 状态。
//
// 调用者必须持有 Manager.mu。重复关闭同一 session 时返回 false，并交回
// 第一次关闭的完成信号，避免两个清理流程同时操作同一个 client。
func (m *Manager) beginSessionCloseLocked(sessionKey string) (*sessionCloseTarget, bool, <-chan struct{}) {
	state := m.sessions[strings.TrimSpace(sessionKey)]
	if state == nil {
		return nil, false, nil
	}
	if state.Closing {
		return nil, false, state.CloseDone
	}

	state.Closing = true
	state.CloseDone = make(chan struct{})
	var sandboxReceipt *SandboxEffectivePolicyReceipt
	if state.Client != nil {
		sandboxReceipt = EffectiveSandboxPolicyReceipt(state.Client)
		state.Client.Retire()
	}
	return &sessionCloseTarget{
		sessionKey:        strings.TrimSpace(sessionKey),
		state:             state,
		ownerUserID:       state.OwnerUserID,
		client:            state.Client,
		sandboxReceipt:    sandboxReceipt,
		roundCancels:      state.Rounds.cancelFuncs(),
		roundDone:         state.Rounds.doneSignals(),
		idleMessageDrain:  state.IdleMessageDrain,
		backgroundCancels: copyBackgroundCancels(state.BackgroundTasks),
		backgroundDone:    state.BackgroundDone,
		closeDone:         state.CloseDone,
	}, true, nil
}

// finishSessionClose 只删除清理成功的 session；失败保留关闭栅栏并唤醒等待者。
func (m *Manager) finishSessionClose(target *sessionCloseTarget, cleanupErr error) {
	if target == nil || target.state == nil {
		return
	}
	m.mu.Lock()
	if current := m.sessions[target.sessionKey]; current == target.state {
		if cleanupErr == nil {
			delete(m.sessions, target.sessionKey)
		} else {
			current.CloseError = cleanupErr
		}
	}
	if target.closeDone != nil {
		close(target.closeDone)
	}
	m.mu.Unlock()
}

// finishSessionCloseWhenDone 延迟移除仍有 client cleanup、round 或后台任务的
// session，防止关闭返回后新 runtime 绕过旧进程与迟到写盘的生命周期栅栏。
func (m *Manager) finishSessionCloseWhenDone(target *sessionCloseTarget, waitClient bool, cleanupErr error) {
	if target == nil {
		return
	}
	if !waitClient && target.idleMessageDrain == nil && len(target.roundDone) == 0 && target.backgroundDone == nil {
		m.finishSessionClose(target, cleanupErr)
		return
	}
	go func() {
		var disconnectErr error
		if waitClient && target.client != nil {
			disconnectErr = target.client.Disconnect(context.Background())
		}
		idleErr := waitIdleMessageDrain(context.Background(), target.idleMessageDrain)
		roundErr := waitRoundDoneSignals(context.Background(), target.roundDone, nil)
		backgroundErr := waitBackgroundTasks(context.Background(), target.backgroundDone)
		cleanupErr = errors.Join(disconnectErr, idleErr, roundErr, backgroundErr)
		cleanupErr = errors.Join(cleanupErr, m.finalizeSandboxReceipt(target, errors.Join(disconnectErr, idleErr, roundErr, backgroundErr)))
		m.finishSessionClose(target, cleanupErr)
	}()
}

// waitSessionCloseResult 返回同一次关闭的结果，失败不能因为完成信号已关闭而丢失。
func (m *Manager) waitSessionCloseResult(ctx context.Context, sessionKey string, done <-chan struct{}) error {
	if err := waitSessionClose(ctx, done); err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state := m.sessions[sessionKey]; state != nil && state.CloseDone == done {
		return state.CloseError
	}
	return nil
}

// waitRoundDoneForClose 等待 round 真正退出；没有外部 deadline 时使用
// 与 runtime 断连相同的宽限，避免损坏的 client 永久阻塞关闭流程。
func waitRoundDoneForClose(ctx context.Context, done []chan struct{}) error {
	if len(done) == 0 {
		return nil
	}
	if _, ok := ctx.Deadline(); ok {
		return waitRoundDoneSignals(ctx, done, nil)
	}
	waitCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
	defer cancel()
	return waitRoundDoneSignals(waitCtx, done, nil)
}

func waitSessionClose(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func copyBackgroundCancels(input map[uint64]context.CancelFunc) []context.CancelFunc {
	if len(input) == 0 {
		return nil
	}
	output := make([]context.CancelFunc, 0, len(input))
	for _, cancel := range input {
		if cancel != nil {
			output = append(output, cancel)
		}
	}
	return output
}
