// INPUT: Room/Automation 持有的 owner、workspace、结构化 session_key 与 transcript 引用。
// OUTPUT: 统一持久 tombstone、runtime admission fence、目录删除和可恢复 transcript 清理。
// POS: 跨领域 Session artifact 删除协调器；禁止业务域直接删除 .agents/sessions。
package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// DeleteSessionArtifacts 删除 Room/Automation 等内部会话产生的 workspace artifact。
// 与普通 Session API 不同，这个入口允许 meta 尚未创建或已经消失，但仍会先写入
// host-only tombstone，保证晚到 runtime writer 不能复活同一物理目录。
func (s *Service) DeleteSessionArtifacts(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	rawSessionKey string,
	cleanupSessionID string,
) (returnErr error) {
	return s.DeleteSessionArtifactsWithTranscripts(
		ctx,
		ownerUserID,
		workspacePath,
		rawSessionKey,
		[]string{cleanupSessionID},
	)
}

// DeleteSessionArtifactsWithTranscripts 删除 artifact 并把完整 transcript lineage
// 固化进 host-only tombstone，使崩溃恢复不会遗漏历史 SDK session。
func (s *Service) DeleteSessionArtifactsWithTranscripts(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	rawSessionKey string,
	cleanupSessionIDs []string,
) (returnErr error) {
	if s == nil || s.files == nil {
		return errors.New("Session artifact 删除协调器未初始化")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return errors.New("Session artifact 删除缺少 owner_user_id")
	}
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return errors.New("Session artifact 删除缺少 workspace_path")
	}
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return err
	}
	if s.runtime == nil {
		return errors.New("Session artifact 删除缺少 runtime manager")
	}

	files := s.files.ForOwner(ownerUserID)
	storageLease, configurationVersion, err := files.BeginSessionArtifactDeletionWithTranscriptIDs(
		workspacePath,
		sessionKey,
		cleanupSessionIDs,
	)
	if errors.Is(err, workspacestore.ErrSessionDeleted) {
		// 已有 host-only tombstone 的删除是幂等成功；启动/周期恢复器会继续
		// deleting 或 transcript cleanup，调用方可以安全删除自己的索引记录。
		return nil
	}
	if err != nil {
		return mapSessionStorageError(err)
	}

	runtimeLease, err := s.runtime.BeginSessionDeletion(sessionKey)
	if err != nil {
		abortErr := files.AbortSessionDeletion(storageLease)
		return errors.Join(err, abortErr)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		s.runtime.AbortSessionDeletion(runtimeLease)
		if abortErr := files.AbortSessionDeletion(storageLease); abortErr != nil {
			returnErr = errors.Join(
				returnErr,
				fmt.Errorf("撤销 Session artifact 删除 tombstone: %w", abortErr),
			)
		}
	}()

	if err = s.runtime.CloseSession(ctx, sessionKey); err != nil {
		return fmt.Errorf("关闭 Session artifact 运行态失败，未删除持久数据: %w", err)
	}
	deleted, err := files.CommitSessionDeletion(storageLease, configurationVersion)
	if deleted {
		committed = true
	}
	if err != nil {
		if deleted {
			return &DeletionReconcileError{cause: err}
		}
		return mapSessionStorageError(err)
	}
	if !deleted {
		return ErrSessionNotFound
	}
	committed = true

	for _, cleanupSessionID := range cleanupSessionIDs {
		cleanupSessionID = strings.TrimSpace(cleanupSessionID)
		if cleanupSessionID == "" {
			continue
		}
		if _, err = s.history.ForOwner(ownerUserID).DeleteTranscriptSession(
			workspacePath,
			cleanupSessionID,
		); err != nil {
			return &DeletionReconcileError{cause: err}
		}
	}
	if err = files.CompleteSessionDeletionCleanup(storageLease); err != nil {
		return &DeletionReconcileError{cause: err}
	}
	return nil
}

var _ interface {
	DeleteSessionArtifacts(context.Context, string, string, string, string) error
} = (*Service)(nil)

var _ interface {
	DeleteSessionArtifactsWithTranscripts(context.Context, string, string, string, []string) error
} = (*Service)(nil)
