// INPUT: Channel HTTP operation、稳定 service error 与 service 提供的数据影响证据。
// OUTPUT: 不暴露内部文案或资源身份的 FailureCore、HTTP 状态与领域恢复动作。
// POS: Channel 控制面的唯一失败投影；禁止通过 err.Error 文本猜测提交结果。
package channel

import (
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
)

type channelControlOperation string

const (
	channelOperationListConfigs      channelControlOperation = "list_configs"
	channelOperationSaveConfig       channelControlOperation = "save_config"
	channelOperationDeleteConfig     channelControlOperation = "delete_config"
	channelOperationDeleteAccount    channelControlOperation = "delete_account"
	channelOperationStartLogin       channelControlOperation = "start_login"
	channelOperationReadLogin        channelControlOperation = "read_login"
	channelOperationSubmitVerifyCode channelControlOperation = "submit_verify_code"
	channelOperationListPairings     channelControlOperation = "list_pairings"
	channelOperationCreatePairing    channelControlOperation = "create_pairing"
	channelOperationUpdatePairing    channelControlOperation = "update_pairing"
	channelOperationDeletePairing    channelControlOperation = "delete_pairing"
)

func (operation channelControlOperation) code(suffix string) string {
	return "channel." + string(operation) + "_" + strings.TrimSpace(suffix)
}

func (operation channelControlOperation) subject() string {
	switch operation {
	case channelOperationListConfigs, channelOperationSaveConfig, channelOperationDeleteConfig:
		return "频道配置"
	case channelOperationDeleteAccount:
		return "频道账号"
	case channelOperationStartLogin, channelOperationReadLogin, channelOperationSubmitVerifyCode:
		return "频道连接"
	case channelOperationListPairings, channelOperationCreatePairing,
		channelOperationUpdatePairing, channelOperationDeletePairing:
		return "配对设置"
	default:
		return "频道操作"
	}
}

func (operation channelControlOperation) reconcileAction() string {
	switch operation {
	case channelOperationListPairings, channelOperationCreatePairing,
		channelOperationUpdatePairing, channelOperationDeletePairing:
		return "channel.reload_pairings"
	case channelOperationStartLogin, channelOperationReadLogin, channelOperationSubmitVerifyCode:
		return "channel.reload_login"
	default:
		return "channel.reload_configs"
	}
}

func channelControlRequestFailure(operation channelControlOperation) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     operation.code("request_invalid"),
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "提交的信息格式不正确",
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "channel.review_input",
		},
	}
}

func (h *Handlers) writeChannelControlUnavailable(
	writer http.ResponseWriter,
	request *http.Request,
	operation channelControlOperation,
	readOnly bool,
) {
	effect := protocol.FailureEffectNotApplied
	if readOnly {
		effect = protocol.FailureEffectNotApplicable
	}
	h.api.WriteError(writer, request, http.StatusServiceUnavailable, handlershared.FailureSpec{
		Code:     operation.code("unavailable"),
		Category: protocol.FailureCategoryUnavailable,
		Effect:   effect,
		Detail:   "频道服务暂时不可用",
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: operation.reconcileAction(),
		},
	})
}

func (h *Handlers) writeChannelReadFailure(
	writer http.ResponseWriter,
	request *http.Request,
	operation channelControlOperation,
	cause error,
) {
	status := http.StatusInternalServerError
	code := operation.code("failed")
	category := protocol.FailureCategoryInternal
	detail := "暂时无法读取" + operation.subject()
	action := operation.reconcileAction()
	switch {
	case errors.Is(cause, channelspkg.ErrChannelNotFound),
		errors.Is(cause, channelspkg.ErrChannelLoginNotFound),
		errors.Is(cause, channelspkg.ErrPairingNotFound):
		status = http.StatusNotFound
		code = operation.code("not_found")
		category = protocol.FailureCategoryNotFound
		detail = operation.subject() + "不存在或已经结束"
	case errors.Is(cause, channelspkg.ErrChannelLoginState):
		status = http.StatusConflict
		code = operation.code("state_ambiguous")
		category = protocol.FailureCategoryConflict
		detail = "当前频道存在无法安全恢复的连接会话"
	case errors.Is(cause, channelspkg.ErrChannelLoginUnsupported):
		status = http.StatusBadRequest
		code = operation.code("unsupported")
		category = protocol.FailureCategoryValidation
		detail = "当前频道不支持这种连接方式"
	}
	h.api.WriteError(writer, request, status, handlershared.FailureSpec{
		Code:     code,
		Category: category,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   detail,
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: action,
		},
	})
}

func (h *Handlers) writeChannelMutationFailure(
	writer http.ResponseWriter,
	request *http.Request,
	operation channelControlOperation,
	cause error,
) {
	status, spec := channelMutationFailure(operation, cause)
	h.api.WriteError(writer, request, status, spec)
}

func channelMutationFailure(
	operation channelControlOperation,
	cause error,
) (int, handlershared.FailureSpec) {
	spec := handlershared.FailureSpec{
		Code:     operation.code("result_unknown"),
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "暂时无法确认" + operation.subject() + "是否已经改变",
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: operation.reconcileAction(),
		},
	}

	switch {
	case errors.Is(cause, channelspkg.ErrChannelControlInvalid):
		spec.Code = operation.code("invalid")
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "提交的" + operation.subject() + "信息不完整或无效"
		spec.Resolution.Action = "channel.review_input"
		return http.StatusBadRequest, spec
	case errors.Is(cause, channelspkg.ErrChannelCredentialStoreUnavailable):
		spec.Code = operation.code("credential_store_unavailable")
		spec.Category = protocol.FailureCategoryUnavailable
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "当前设备尚不能安全保存频道凭据"
		spec.Resolution.Action = "channel.review_host_settings"
		return http.StatusServiceUnavailable, spec
	case errors.Is(cause, channelspkg.ErrChannelConfigRequired):
		spec.Code = operation.code("config_required")
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "请先保存频道配置，再开始连接"
		spec.Resolution.Action = "channel.review_config"
		return http.StatusConflict, spec
	case errors.Is(cause, channelspkg.ErrChannelNotFound),
		errors.Is(cause, channelspkg.ErrChannelAccountNotFound),
		errors.Is(cause, channelspkg.ErrPairingNotFound),
		errors.Is(cause, channelspkg.ErrChannelLoginNotFound):
		spec.Code = operation.code("not_found")
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = operation.subject() + "不存在或已经结束"
		return http.StatusNotFound, spec
	case errors.Is(cause, channelspkg.ErrChannelControlVersionConflict):
		spec.Code = operation.code("version_conflict")
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = operation.subject() + "已在其他位置更新，本次操作没有应用"
		return http.StatusConflict, spec
	case errors.Is(cause, channelspkg.ErrChannelLoginUnsupported):
		spec.Code = operation.code("unsupported")
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "当前频道不支持这种连接方式"
		spec.Resolution.Action = "channel.review_connection_method"
		return http.StatusBadRequest, spec
	case errors.Is(cause, channelspkg.ErrChannelLoginState) && operation == channelOperationStartLogin:
		spec.Code = operation.code("state_changed")
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "已有频道连接正在进行，本次未再启动新连接"
		return http.StatusConflict, spec
	case errors.Is(cause, channelspkg.ErrChannelLoginState),
		errors.Is(cause, channelspkg.ErrChannelLoginAuthorizationCommit):
		spec.Code = operation.code("state_changed")
		spec.Category = protocol.FailureCategoryConflict
		// 登录可能刚好在另一路径完成。没有 exact final view 时不能声称未写入。
		spec.Effect = protocol.FailureEffectUnknown
		spec.Detail = "频道连接状态已经变化，暂时无法确认本次操作结果"
		return http.StatusConflict, spec
	}

	if effect, ok := channelspkg.ChannelControlMutationEffect(cause); ok {
		switch effect {
		case channelspkg.ControlMutationNotApplied:
			spec.Code = operation.code("not_applied")
			spec.Effect = protocol.FailureEffectNotApplied
			spec.Detail = operation.subject() + "没有改变，原有数据仍然保留"
			spec.Resolution.Action = "channel.retry_after_review"
		case channelspkg.ControlMutationCommitted:
			spec.Code = operation.code("committed")
			spec.Effect = protocol.FailureEffectCommitted
			spec.Detail = operation.subject() + "已保存，但后续连接状态需要刷新确认"
		case channelspkg.ControlMutationUnknown:
			// 保留默认 unknown；只能读取 exact aggregate 对账，不能建议重放写入。
		}
	}
	return http.StatusInternalServerError, spec
}
