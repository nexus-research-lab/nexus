// INPUT: 持锁宿主、固定进程仓储与分页游标。
// OUTPUT: 逐条原登记恢复结果；失败保留栅栏并可继续处理后续条目。
// POS: 启动恢复批次，不重放任务，不自动清除策略或 scratch 的独立 unknown。
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxProcessRecoveryScanner interface {
	PendingProcessKeys(context.Context, string, int) ([]protocol.SandboxProcessKey, bool, error)
}

type SandboxProcessRecoveryItem struct {
	Key      protocol.SandboxProcessKey
	Snapshot protocol.SandboxProcessSnapshot
	Err      error
}

type SandboxProcessRecoveryBatch struct {
	Items      []SandboxProcessRecoveryItem
	NextCursor string
	HasMore    bool
}

// RecoverPendingSandboxProcesses 必须在开放产品任务准入前由宿主调用。
// 每次至多处理 256 条；错误项仍持久 pending，游标越过它以免饿死其他记录。
// 重试失败项必须重新扫描原 key；任何结果都不授权重放原任务。
func (m *Manager) RecoverPendingSandboxProcesses(ctx context.Context, ownership SandboxProcessRecoveryOwnership, after string, limit int) (SandboxProcessRecoveryBatch, error) {
	return m.recoverPendingSandboxProcesses(ctx, ownership, after, limit, supervision.Recover)
}

func (m *Manager) recoverPendingSandboxProcesses(ctx context.Context, ownership SandboxProcessRecoveryOwnership, after string, limit int, native sandboxProcessRecoveryFunc) (SandboxProcessRecoveryBatch, error) {
	result := SandboxProcessRecoveryBatch{NextCursor: after}
	if ownership == nil || limit < 1 || limit > 256 || (after != "" && !validSandboxLaunchID(after)) {
		return result, errors.New("invalid process recovery batch admission")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.RLock()
		config := m.sandboxSupervisor
		scanner, ok := m.sandboxReceiptStore.(sandboxProcessRecoveryScanner)
		m.mu.RUnlock()
		if config == nil || !ok {
			return errors.New("process recovery requires a configured durable scanner")
		}
		if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
			return err
		}
		keys, more, err := scanner.PendingProcessKeys(ctx, after, limit)
		if err != nil {
			return err
		}
		result.HasMore = more
		var failures []error
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				result.HasMore = true
				return errors.Join(append(failures, err)...)
			}
			if !validSandboxLaunchID(key.LaunchID) || key.LaunchID <= result.NextCursor {
				result.HasMore = true
				return errors.Join(append(failures, errors.New("invalid recovery scan order"))...)
			}
			snapshot, err := m.recoverSandboxProcessOwned(ctx, key, appRoot, native)
			result.Items = append(result.Items, SandboxProcessRecoveryItem{Key: key, Snapshot: snapshot, Err: err})
			result.NextCursor = key.LaunchID
			if err != nil {
				failures = append(failures, fmt.Errorf("recover launch %s: %w", key.LaunchID, err))
			}
		}
		return errors.Join(failures...)
	})
	return result, err
}
