// INPUT: owner-scoped Room 标识、可选 expected configuration_version 与提交后清理依赖。
// OUTPUT: 数据库优先的 Room 删除，以及可区分“未提交”和“已提交待 reconcile”的错误。
// POS: Room 删除业务事务边界；持久身份撤销成功前不得触碰 runtime、artifact 或 Goal。
package room

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// DeletionReconcileError 表示 Room 数据库删除已经提交，但外围运行态清理未完全成功。
type DeletionReconcileError struct {
	cause error
}

func (e *DeletionReconcileError) Error() string {
	return fmt.Sprintf("Room 数据已删除，但关联运行态清理需要 reconcile: %v", e.cause)
}

func (e *DeletionReconcileError) Unwrap() error {
	return e.cause
}

// RoomDeletionCommitted 判断删除错误是否发生在数据库提交之后。
func RoomDeletionCommitted(err error) bool {
	var committed *DeletionReconcileError
	return errors.As(err, &committed)
}

// DeleteRoom 删除房间，并在数据库提交后清理所有 conversation 运行态与文件。
func (s *Service) DeleteRoom(ctx context.Context, roomID string) error {
	return s.deleteRoom(ctx, roomID, nil)
}

// DeleteRoomAtVersion 仅在 configuration_version 仍等于计划版本时删除房间。
func (s *Service) DeleteRoomAtVersion(
	ctx context.Context,
	roomID string,
	expectedConfigurationVersion int64,
) error {
	if expectedConfigurationVersion < 1 {
		return errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.deleteRoom(ctx, roomID, &expectedConfigurationVersion)
}

func (s *Service) deleteRoom(
	ctx context.Context,
	roomID string,
	expectedConfigurationVersion *int64,
) error {
	roomID = strings.TrimSpace(roomID)
	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return err
	}

	ownerUserID := authctx.OwnerUserID(ctx)
	var deleted bool
	if expectedConfigurationVersion != nil {
		deleted, err = s.repository.DeleteRoomAtVersion(
			ctx,
			ownerUserID,
			roomID,
			*expectedConfigurationVersion,
		)
	} else {
		deleted, err = s.repository.DeleteRoom(ctx, ownerUserID, roomID)
	}
	if err != nil {
		return err
	}
	if !deleted {
		return ErrRoomNotFound
	}

	cleanupCtx := context.WithoutCancel(ctx)
	cleanupErr := errors.Join(
		wrapRoomDeletionCleanup("关闭 Room runtime session", s.closeConversationRuntimeSessions(
			cleanupCtx,
			roomContexts,
			true,
			nil,
		)),
		wrapRoomDeletionCleanup("清理 Room artifact", s.cleanupConversationArtifacts(
			cleanupCtx,
			roomContexts,
			true,
			nil,
		)),
		wrapRoomDeletionCleanup("清理 Room Goal", s.cleanupGoalsForRoomContexts(
			cleanupCtx,
			roomContexts,
		)),
	)
	if cleanupErr != nil {
		return &DeletionReconcileError{cause: cleanupErr}
	}
	return nil
}

func wrapRoomDeletionCleanup(stage string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", stage, err)
}
