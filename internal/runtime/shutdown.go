// INPUT: 全部 runtime Session、在途启动事务与宿主退出请求。
// OUTPUT: 永久关闭准入、取消并等待既有执行、持久化沙箱终态及共享关闭结果。
// POS: 应用退出的 Manager 生命周期入口；必须在关闭数据库前等待完成。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrRuntimeManagerClosed 表示宿主已经开始退出，不再接受执行或后台任务。
var ErrRuntimeManagerClosed = errors.New("runtime manager is shutting down")

// Close 永久关闭运行时管理器。调用者超时只结束等待；共享回收继续完成原有
// Session 的终态写入。重复调用等待同一次关闭，不能把失败当成成功。
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.shutdownDone == nil {
		m.shutdownDone = make(chan struct{})
		var cancellations []context.CancelFunc
		for _, state := range m.sessions {
			cancellations = append(cancellations, state.Rounds.cancelFuncs()...)
			cancellations = append(cancellations, copyBackgroundCancels(state.BackgroundTasks)...)
		}
		go m.closeRuntimeSessions(cancellations, m.startupsDrained)
	}
	done := m.shutdownDone
	m.mu.Unlock()
	select {
	case <-done:
		return m.shutdownErr
	default:
	}
	select {
	case <-done:
		return m.shutdownErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) closeRuntimeSessions(cancellations []context.CancelFunc, startupsDrained <-chan struct{}) {
	for _, cancel := range cancellations {
		if cancel != nil {
			cancel()
		}
	}
	// Drain startup before retiring clients: Connect may already be writing its
	// confirmed receipt. Its write must finish before CloseSession persists the
	// terminal phase, and late factories must dispose of uninstalled clients.
	if startupsDrained != nil {
		<-startupsDrained
	}
	m.mu.RLock()
	keys := make([]string, 0, len(m.sessions))
	for key := range m.sessions {
		keys = append(keys, key)
	}
	m.mu.RUnlock()
	errorsBySession := make([]error, len(keys))
	var group sync.WaitGroup
	for index, key := range keys {
		group.Add(1)
		go func() {
			defer group.Done()
			ctx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
			err := m.CloseSession(ctx, key)
			cancel()
			// CloseSession may return before round/IO cleanup completes. Keep the
			// application database alive until its final receipt write has finished.
			m.mu.RLock()
			var done <-chan struct{}
			if state := m.sessions[key]; state != nil {
				done = state.CloseDone
			}
			m.mu.RUnlock()
			if done != nil {
				err = errors.Join(err, m.waitSessionCloseResult(context.Background(), key, done))
			}
			if err != nil {
				errorsBySession[index] = fmt.Errorf("shutdown runtime session %s: %w", key, err)
			}
		}()
	}
	group.Wait()
	m.mu.Lock()
	m.shutdownErr = errors.Join(errorsBySession...)
	close(m.shutdownDone)
	m.mu.Unlock()
}
