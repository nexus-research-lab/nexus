// INPUT: Personal profile/password operation stage and stable auth service errors.
// OUTPUT: Evidence-based FailureCore projections that never expose credential details.
// POS: Personal settings HTTP failure boundary; recovery copy remains a Web concern.
package auth

import (
	"errors"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func passwordBodyFailure() handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "auth.password_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "密码修改请求格式无效",
	}
}

func passwordMutationFailure(cause error) (int, handlershared.FailureSpec) {
	spec := handlershared.FailureSpec{
		Code:     "auth.password_change_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "暂时无法确认密码是否已经修改",
		Cause:    cause,
	}
	switch {
	case errors.Is(cause, authsvc.ErrInvalidCredentials):
		spec.Code = "auth.current_password_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "当前密码不正确"
		return http.StatusUnprocessableEntity, spec
	case errors.Is(cause, authsvc.ErrPasswordChangeNotApplied):
		spec.Code = "auth.password_change_not_applied"
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "密码没有修改"
		return http.StatusConflict, spec
	case errors.Is(cause, authsvc.ErrPasswordChangeCommitted):
		spec.Code = "auth.password_change_committed"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "密码已修改，但暂时无法刷新登录状态"
		return http.StatusInternalServerError, spec
	case errors.Is(cause, authsvc.ErrPasswordChangeInvalidInput):
		spec.Code = "auth.password_change_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "新密码不符合要求"
		return http.StatusBadRequest, spec
	default:
		return http.StatusInternalServerError, spec
	}
}

func passwordAccessFailure(
	code string,
	category protocol.FailureCategory,
) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     code,
		Category: category,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "当前登录方式不支持修改密码",
	}
}

func passwordServiceUnavailableFailure() handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "auth.password_service_unavailable",
		Category: protocol.FailureCategoryUnavailable,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "密码服务暂时不可用",
	}
}

func passwordReceiptReadFailure(
	code string,
	category protocol.FailureCategory,
	cause error,
) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     code,
		Category: category,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法核对密码修改结果",
		Cause:    cause,
	}
}

func passwordSettlementFailure(cause error) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "auth.password_settlement_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "暂时无法确认这次密码修改是否已关闭",
		Cause:    cause,
	}
}
