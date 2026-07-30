package runtime

import (
	"context"
	"errors"
	"strings"
)

// ErrRuntimeSessionClosing 表示 session 正在退出，不能再注册新的运行任务。
var ErrRuntimeSessionClosing = errors.New("runtime session is closing")

var errRuntimeSessionClosing = ErrRuntimeSessionClosing

// sessionCloseTarget 保存关闭期间需要取消、等待和最终移除的 session 状态。
type sessionCloseTarget struct {
	sessionKey        string
	state             *sessionState
	ownerUserID       string
	client            Client
	roundCancels      []context.CancelFunc
	roundDone         []chan struct{}
	idleMessageCancel context.CancelFunc
	backgroundCancels []context.CancelFunc
	backgroundDone    <-chan struct{}
	closeDone         chan struct{}
}

// beginSessionCloseLocked 把 session 切换到不可再接收新任务的 closing 状态。
//
// 调用者必须持有 Manager.mu。重复关闭同一 session 时返回 false，并交回
// 第一次关闭的完成信号，避免两个清理流程同时操作同一个 client。
func (m *Manager) beginSessionCloseLocked(sessionKey string) (*sessionCloseTarget, bool, <-chan struct{}) {
	if m == nil {
		return nil, false, nil
	}
	state := m.sessions[strings.TrimSpace(sessionKey)]
	if state == nil {
		return nil, false, nil
	}
	if state.Closing {
		return nil, false, state.CloseDone
	}

	state.Closing = true
	state.CloseDone = make(chan struct{})
	return &sessionCloseTarget{
		sessionKey:        strings.TrimSpace(sessionKey),
		state:             state,
		ownerUserID:       state.OwnerUserID,
		client:            state.Client,
		roundCancels:      copyRoundCancels(state.RoundCancels),
		roundDone:         copyRoundDoneSignals(state.RoundDone),
		idleMessageCancel: state.IdleMessageCancel,
		backgroundCancels: copyBackgroundCancels(state.BackgroundTasks),
		backgroundDone:    state.BackgroundDone,
		closeDone:         state.CloseDone,
	}, true, nil
}

// finishSessionClose 删除仍属于本次关闭的 session，并唤醒并发关闭调用者。
func (m *Manager) finishSessionClose(target *sessionCloseTarget) {
	if m == nil || target == nil || target.state == nil {
		return
	}
	m.mu.Lock()
	if current := m.sessions[target.sessionKey]; current == target.state {
		delete(m.sessions, target.sessionKey)
	}
	if target.closeDone != nil {
		close(target.closeDone)
	}
	m.mu.Unlock()
}

// finishSessionCloseWhenDone 延迟移除仍有 round 的 session，防止关闭返回后
// 迟到的 round 回调重新创建后台写盘任务。
func (m *Manager) finishSessionCloseWhenDone(target *sessionCloseTarget) {
	if m == nil || target == nil {
		return
	}
	if len(target.roundDone) == 0 && target.backgroundDone == nil {
		m.finishSessionClose(target)
		return
	}
	go func() {
		_ = waitRoundDoneSignals(context.Background(), target.roundDone, nil)
		_ = waitBackgroundTasks(context.Background(), target.backgroundDone)
		m.finishSessionClose(target)
	}()
}

// waitRoundDoneForClose 等待 round 真正退出；没有外部 deadline 时使用
// 与 runtime 断连相同的宽限，避免损坏的 client 永久阻塞关闭流程。
func waitRoundDoneForClose(ctx context.Context, done []chan struct{}) error {
	if len(done) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
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
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func copyRoundDoneSignals(input map[string]chan struct{}) []chan struct{} {
	if len(input) == 0 {
		return nil
	}
	output := make([]chan struct{}, 0, len(input))
	for _, done := range input {
		if done != nil {
			output = append(output, done)
		}
	}
	return output
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
