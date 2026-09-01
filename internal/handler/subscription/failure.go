// INPUT: Subscription read/mutation stage and stable service errors.
// OUTPUT: Evidence-based FailureCore status/code/category/effect projections.
// POS: Subscription HTTP failure boundary; it never classifies by error text.
package subscription

import (
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
)

func subscriptionAccessFailure(effect protocol.FailureEffect) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "subscription.admin_forbidden",
		Category: protocol.FailureCategoryAuthorization,
		Effect:   effect,
		Detail:   "当前账号没有订阅管理权限",
	}
}

func subscriptionReadFailure(cause error) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "subscription.read_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法读取订阅设置",
		Cause:    cause,
	}
}

func subscriptionBodyFailure() handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "subscription.request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "订阅设置请求格式无效",
	}
}

func subscriptionMutationFailure(
	operation string,
	cause error,
) (int, handlershared.FailureSpec) {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "change"
	}
	spec := handlershared.FailureSpec{
		Code:     "subscription." + operation + "_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "暂时无法确认订阅设置是否已经更改",
		Cause:    cause,
	}
	switch {
	case errors.Is(cause, subscriptionsvc.ErrInvalidInput):
		spec.Code = "subscription." + operation + "_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "订阅设置内容无效"
		return http.StatusBadRequest, spec
	case errors.Is(cause, subscriptionsvc.ErrMutationNotApplied):
		spec.Code = "subscription." + operation + "_not_applied"
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "订阅设置没有更改"
		return http.StatusInternalServerError, spec
	case errors.Is(cause, subscriptionsvc.ErrMutationCommitted):
		spec.Code = "subscription." + operation + "_committed"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "订阅设置已更改，但暂时无法读取最新状态"
		return http.StatusInternalServerError, spec
	default:
		return http.StatusInternalServerError, spec
	}
}
