// INPUT: 宿主维护 session、nxs options、已取得的 scratch 句柄及取消上下文。
// OUTPUT: 受 Manager 准入/监督/关闭管理的一次性 AutoDream 结果与句柄交接状态。
// POS: 后台 SDK control 生命周期适配；到期判断和记忆规则仍属于 nxs。
package runtime

import (
	"context"
	"errors"
	"strings"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// TryAutoDream runs an isolated maintenance client. consumed tells the caller
// whether cleanup ownership was transferred, including when startup/control fails.
func (m *Manager) TryAutoDream(ctx context.Context, sessionKey string, options bridge.Options, lease *SandboxResourceLease) (bridge.AutoDreamResult, bool, error) {
	owner := runtimeOwnerUserID(options)
	if m == nil || owner == "" || !strings.HasPrefix(sessionKey, "memory-maintenance:") || strings.TrimPrefix(sessionKey, "memory-maintenance:") == "" || normalizedManagedRuntimeKind(options.Runtime.Kind) != bridge.RuntimeNXS {
		return bridge.AutoDreamResult{}, false, errors.New("invalid managed AutoDream identity")
	}
	if err := ctx.Err(); err != nil {
		return bridge.AutoDreamResult{}, false, err
	}
	type outcome struct {
		result   bridge.AutoDreamResult
		consumed bool
		err      error
	}
	done := make(chan outcome, 1)
	if !m.StartBackgroundTaskForOwner(sessionKey, owner, func(background context.Context) {
		run, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(background, cancel)
		defer cancel()
		defer stop()
		result, consumed, err := m.runAutoDream(run, sessionKey, options, lease)
		done <- outcome{result, consumed, err}
	}) {
		return bridge.AutoDreamResult{}, false, errors.New("AutoDream runtime admission rejected")
	}
	// Do not return the lease to the caller while the registered task still owns
	// startup or cleanup. Cancellation forces client retirement inside runAutoDream.
	result := <-done
	return result.result, result.consumed, result.err
}

func (m *Manager) runAutoDream(ctx context.Context, key string, options bridge.Options, lease *SandboxResourceLease) (result bridge.AutoDreamResult, consumed bool, err error) {
	startup, err := m.BeginClientStartup(ctx, key, runtimeOwnerUserID(options))
	if err != nil {
		return result, false, err
	}
	defer startup.Close()
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
		defer cancel()
		_, closeErr := startup.RetireCurrent(closeCtx)
		err = errors.Join(err, closeErr)
	}()
	client, err := startup.GetOrCreateWithLease(ctx, options, nil, lease)
	if err != nil {
		return result, false, err
	}
	// Use the client lifecycle directly here: ClientStartup is single-goroutine.
	// The watcher never touches its startup gate or infers resource retirement.
	stopped := make(chan struct{})
	stopWatch := context.AfterFunc(ctx, func() {
		defer close(stopped)
		client.Retire()
		closeCtx, cancel := context.WithTimeout(context.Background(), RoundIdleAbortTimeout)
		defer cancel()
		_ = client.Disconnect(closeCtx)
	})
	defer func() {
		if !stopWatch() {
			<-stopped
		}
	}()
	consumed, err = startup.BindSandboxLease(lease)
	if err != nil {
		return result, consumed, err
	}
	if err = startup.Connect(ctx); err != nil {
		return result, consumed, err
	}
	control, ok := client.(interface {
		TryAutoDream(context.Context) (bridge.AutoDreamResult, error)
	})
	if !ok {
		return result, consumed, errors.New("runtime does not support managed AutoDream")
	}
	drainCtx, cancelDrain := context.WithCancel(ctx)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		messages := client.ReceiveMessages(drainCtx)
		for {
			select {
			case <-drainCtx.Done():
				return
			case _, ok := <-messages:
				if !ok {
					return
				}
			}
		}
	}()
	defer func() { cancelDrain(); <-drained }()
	returnResult, controlErr := control.TryAutoDream(ctx)
	return returnResult, consumed, controlErr
}

func (c *agentClient) TryAutoDream(ctx context.Context) (bridge.AutoDreamResult, error) {
	session, err := c.currentSession()
	if err != nil {
		return bridge.AutoDreamResult{}, err
	}
	if !session.Supports(bridge.CapabilityAutoDream) {
		return bridge.AutoDreamResult{}, errors.New("runtime does not support AutoDream")
	}
	return session.Control().TryAutoDream(ctx)
}
