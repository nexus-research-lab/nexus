// INPUT: Room Handler 已确认的删除领域错误与提交事实。
// OUTPUT: 保持既有 HTTP 状态的 FailureCore 映射；没有提交证据时保守为 unknown。
// POS: Room 删除 HTTP 失败投影，不改变 service 事务、资源身份或删除顺序。
package room

import (
	"errors"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
)

func roomDeleteFailure(err error) (int, handlershared.FailureSpec) {
	status := http.StatusInternalServerError
	spec := handlershared.FailureSpec{
		Code:     "room.deletion_outcome_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "Room 删除结果暂时无法确认",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "room.refresh_directory",
		},
	}

	switch {
	case errors.Is(err, roompkg.ErrRoomNotFound):
		status = http.StatusNotFound
		spec.Code = "room.not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "Room 不存在或已经删除"
	case roompkg.RoomDeletionCommitted(err):
		spec.Code = "room.deletion_cleanup_incomplete"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "Room 已删除，但关联内容没有全部清理完成"
	}
	return status, spec
}
