// INPUT: Skill Handler 已确认的请求阶段、服务错误与 catalog 提交证据。
// OUTPUT: 保持既有 HTTP 状态的 FailureCore 投影，区分 not_applied/committed/unknown。
// POS: Skill Marketplace HTTP 失败边界；不得从错误文案猜测提交结果或自动重放写入。
package skill

import (
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func skillRequestFailure(code string, detail string) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     code,
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   detail,
	}
}

func skillMutationFailure(
	err error,
	code string,
	status int,
	category protocol.FailureCategory,
	detail string,
) (int, handlershared.FailureSpec) {
	return projectSkillMutationFailure(
		err,
		code,
		status,
		category,
		detail,
		skillspkg.SkillMutationNeedsReconcile(err),
		skillspkg.SkillMutationApplied(err),
	)
}

func projectSkillMutationFailure(
	err error,
	code string,
	status int,
	category protocol.FailureCategory,
	detail string,
	needsReconcile bool,
	applied bool,
) (int, handlershared.FailureSpec) {
	spec := handlershared.FailureSpec{
		Code:     code,
		Category: category,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   detail,
		Cause:    err,
	}
	if !needsReconcile {
		return status, spec
	}

	spec.Category = protocol.FailureCategoryInternal
	spec.Resolution = &protocol.FailureResolution{
		Actor:  protocol.FailureRecoveryActorUser,
		Action: "skill.refresh_catalog",
	}
	if applied {
		spec.Code = code + ".reconcile_required"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "技能变更已保存，但技能目录需要刷新"
		return status, spec
	}
	spec.Code = code + ".outcome_unknown"
	spec.Effect = protocol.FailureEffectUnknown
	spec.Detail = "无法确认技能变更是否已经保存"
	return status, spec
}

func skillNotFoundMutationFailure(
	err error,
	code string,
) (int, handlershared.FailureSpec) {
	return skillMutationFailure(
		err,
		code,
		http.StatusNotFound,
		protocol.FailureCategoryNotFound,
		"技能不存在，操作没有执行",
	)
}
