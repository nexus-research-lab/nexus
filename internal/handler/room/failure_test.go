// INPUT: Room 删除的 absent、无提交证据和已提交清理失败事实。
// OUTPUT: 稳定 HTTP 状态与 FailureCore code/effect 断言。
// POS: Room Handler 失败映射回归；不执行真实删除或改变事务。
package room

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
)

func TestRoomDeleteFailureRequiresDomainCommitEvidence(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		effect protocol.FailureEffect
	}{
		{
			name:   "target already absent",
			err:    roompkg.ErrRoomNotFound,
			status: http.StatusNotFound,
			code:   "room.not_found",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "failure without commit evidence",
			err:    errors.New("cleanup or database failed"),
			status: http.StatusInternalServerError,
			code:   "room.deletion_outcome_unknown",
			effect: protocol.FailureEffectUnknown,
		},
		{
			name:   "committed cleanup failure",
			err:    &roompkg.DeletionReconcileError{},
			status: http.StatusInternalServerError,
			code:   "room.deletion_cleanup_incomplete",
			effect: protocol.FailureEffectCommitted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := roomDeleteFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf(
					"roomDeleteFailure() = status %d, code %q, effect %q",
					status,
					spec.Code,
					spec.Effect,
				)
			}
		})
	}
}
