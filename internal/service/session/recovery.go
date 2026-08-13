// INPUT: host-only owner Session lifecycle records、runtime Manager、跨域引用协调器与 transcript store。
// OUTPUT: 重启后 deleting/deleted 删除的确定性提交、任务停用/引用清理和永久 admission fence。
// POS: Session 删除崩溃恢复边界；在 Channel/Automation 等后台入口启动前至少执行一次。
package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// DeletionRecoveryScanError 表示宿主无法枚举权威删除记录，服务启动应 fail closed。
type DeletionRecoveryScanError struct {
	cause error
}

func (e *DeletionRecoveryScanError) Error() string {
	return fmt.Sprintf("扫描 Session 删除恢复记录失败: %v", e.cause)
}

func (e *DeletionRecoveryScanError) Unwrap() error {
	return e.cause
}

// SessionDeletionRecoveryScanFailed 判断恢复错误是否发生在安装 runtime fence 之前。
func SessionDeletionRecoveryScanFailed(err error) bool {
	var scan *DeletionRecoveryScanError
	return errors.As(err, &scan)
}

// ReconcilePendingDeletions 恢复宿主 state 中未完成的 Session 删除，并为所有
// 已完成 tombstone 重新安装进程期 runtime admission fence。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) (int, error) {
	if s == nil || s.files == nil {
		return 0, nil
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	records, err := s.files.ListSessionDeletionRecords()
	if err != nil {
		return 0, &DeletionRecoveryScanError{cause: err}
	}
	reconciled := 0
	errs := make([]error, 0)
	for _, item := range records {
		if err = s.reconcilePendingDeletion(ctx, item); err != nil {
			errs = append(errs, fmt.Errorf(
				"owner=%s session=%s: %w",
				item.OwnerUserID,
				item.SessionKey,
				err,
			))
			continue
		}
		if !item.CleanupComplete {
			reconciled++
		}
	}
	return reconciled, errors.Join(errs...)
}

func (s *Service) reconcilePendingDeletion(
	ctx context.Context,
	item workspacestore.PendingSessionDeletion,
) error {
	if s.runtime == nil {
		return errors.New("Session 删除恢复缺少 runtime manager")
	}
	blockKey := strings.TrimSpace(item.OwnerUserID) + "\x00" +
		strings.TrimSpace(item.SessionKey)
	_, alreadyBlocked := s.recoveryBlocked[blockKey]
	if !alreadyBlocked {
		if _, err := s.runtime.BeginSessionDeletion(item.SessionKey); err != nil {
			if errors.Is(err, runtimectx.ErrRuntimeSessionDeleted) {
				// runtime fence 按物理 session identity 全局复用；同 key 的另一个
				// owner/恢复记录或正常删除流程已经提供了同等 admission 保护。
				s.recoveryBlocked[blockKey] = struct{}{}
			} else {
				return err
			}
		} else {
			s.recoveryBlocked[blockKey] = struct{}{}
		}
	}
	if item.CleanupComplete && alreadyBlocked {
		return nil
	}
	if err := s.runtime.CloseSession(ctx, item.SessionKey); err != nil {
		return fmt.Errorf("关闭残留 runtime: %w", err)
	}
	if item.CleanupComplete {
		return nil
	}
	files := s.files.ForOwner(item.OwnerUserID)
	if !item.Committed {
		if _, err := files.CommitSessionDeletion(
			item.Lease,
			item.ConfigurationVersion,
		); err != nil {
			return fmt.Errorf("提交残留目录删除: %w", err)
		}
	}
	if s.deletion != nil {
		if err := s.deletion.CleanupSessionReferencesPreservingTasks(
			context.WithoutCancel(ctx),
			item.OwnerUserID,
			[]string{item.SessionKey},
		); err != nil {
			return fmt.Errorf("清理残留 Session 引用: %w", err)
		}
	}
	cleanupSessionIDs := item.CleanupSessionIDs
	if len(cleanupSessionIDs) == 0 && item.CleanupSessionID != "" {
		cleanupSessionIDs = []string{item.CleanupSessionID}
	}
	for _, cleanupSessionID := range cleanupSessionIDs {
		if _, err := s.history.ForOwner(item.OwnerUserID).DeleteTranscriptSession(
			item.WorkspacePath,
			cleanupSessionID,
		); err != nil {
			return fmt.Errorf("清理残留 transcript %s: %w", cleanupSessionID, err)
		}
	}
	if err := files.CompleteSessionDeletionCleanup(item.Lease); err != nil {
		return fmt.Errorf("完成删除 tombstone: %w", err)
	}
	return nil
}
