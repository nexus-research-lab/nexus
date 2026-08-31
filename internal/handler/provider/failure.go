// INPUT: Provider handler operation, stable service error and optional aggregate version precondition.
// OUTPUT: Legacy-compatible HTTP status plus FailureCore problem/effect/recovery facts and strong ETag helpers.
// POS: Provider HTTP failure projection boundary; it never classifies by error text or transport request ID.
package provider

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const providerETagPrefix = "provider-"

func writeProviderETag(writer http.ResponseWriter, version int64) {
	if writer == nil || version < 1 {
		return
	}
	writer.Header().Set("ETag", fmt.Sprintf(`"%s%d"`, providerETagPrefix, version))
	writer.Header().Set("Cache-Control", "no-store")
}

func parseProviderIfMatch(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, ",") || value == "*" || strings.HasPrefix(value, "W/") {
		return nil, errors.New("Provider If-Match 必须是单个强 ETag")
	}
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return nil, errors.New("Provider If-Match 缺少强 ETag 引号")
	}
	opaque := strings.TrimSpace(value[1 : len(value)-1])
	if !strings.HasPrefix(opaque, providerETagPrefix) {
		return nil, errors.New("Provider If-Match 不属于 Provider 配置")
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(opaque, providerETagPrefix), 10, 64)
	if err != nil || version < 1 {
		return nil, errors.New("Provider If-Match version 无效")
	}
	return &version, nil
}

func (h *Handlers) writeProviderReadFailure(
	writer http.ResponseWriter,
	request *http.Request,
	cause error,
) {
	h.api.WriteError(writer, request, http.StatusInternalServerError, providerReadFailure(cause))
}

func providerReadFailure(cause error) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "provider.read_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法读取模型服务设置",
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "provider.reload",
		},
	}
}

func providerImportPreviewFailure(cause error) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "provider.preview_import_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法读取本机模型服务配置",
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "provider.review_import_source",
		},
	}
}

func (h *Handlers) writeProviderPreconditionFailure(
	writer http.ResponseWriter,
	request *http.Request,
	cause error,
) {
	h.api.WriteError(writer, request, http.StatusBadRequest, handlershared.FailureSpec{
		Code:     "provider.precondition_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "模型服务版本条件无效",
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "provider.reload",
		},
	})
}

func (h *Handlers) writeProviderMutationFailure(
	writer http.ResponseWriter,
	request *http.Request,
	operation string,
	cause error,
	preconditioned ...bool,
) {
	status, spec := providerMutationFailure(operation, cause, preconditioned...)
	h.api.WriteError(writer, request, status, spec)
}

func providerMutationFailure(
	operation string,
	cause error,
	preconditioned ...bool,
) (int, handlershared.FailureSpec) {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "change"
	}
	spec := handlershared.FailureSpec{
		Code:     "provider." + operation + "_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "暂时无法确认模型服务操作结果",
		Cause:    cause,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "provider.reconcile",
		},
	}
	switch {
	case errors.Is(cause, providercfg.ErrMutationCommitted):
		spec.Code = "provider." + operation + "_committed"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "模型服务更改已保存，但页面暂时无法读取最新状态"
		spec.Resolution.Action = "provider.reload"
		return http.StatusBadRequest, spec
	case errors.Is(cause, providercfg.ErrProviderManagementForbidden):
		spec.Code = "provider.management_forbidden"
		spec.Category = protocol.FailureCategoryAuthorization
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "当前账号不能修改这个模型服务"
		spec.Resolution.Action = "provider.review_access"
		return http.StatusForbidden, spec
	case errors.Is(cause, providercfg.ErrProviderNotFound):
		spec.Code = "provider.not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "模型服务不存在或已被删除"
		spec.Resolution.Action = "provider.reload"
		return http.StatusNotFound, spec
	case errors.Is(cause, providercfg.ErrConfigurationVersionConflict):
		spec.Code = "provider.version_conflict"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "模型服务已在其他页面更新"
		spec.Resolution.Action = "provider.reload"
		if len(preconditioned) > 0 && preconditioned[0] {
			return http.StatusPreconditionFailed, spec
		}
		return http.StatusBadRequest, spec
	case errors.Is(cause, providercfg.ErrProviderAlreadyExists):
		spec.Code = "provider.already_exists"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "同一模型服务已经存在"
		spec.Resolution.Action = "provider.reload"
		return http.StatusBadRequest, spec
	case errors.Is(cause, providercfg.ErrProviderInUse):
		spec.Code = "provider.in_use"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "这个模型服务仍在使用中，暂时不能删除"
		spec.Resolution.Action = "provider.review_usage"
		return http.StatusBadRequest, spec
	case errors.Is(cause, providercfg.ErrInvalidInput), errors.Is(cause, providercfg.ErrModelNotFound):
		spec.Code = "provider." + operation + "_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "模型服务设置不完整或格式不正确"
		spec.Resolution.Action = "provider.review_values"
		return http.StatusBadRequest, spec
	case errors.Is(cause, providercfg.ErrMutationNotApplied):
		spec.Code = "provider." + operation + "_not_applied"
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "模型服务更改没有保存"
		spec.Resolution.Action = "provider.retry_change"
		return http.StatusBadRequest, spec
	default:
		// Existing Provider handlers returned 400 for unclassified service failures.
		// Keep that wire status while exposing the conservative unknown data effect.
		return http.StatusBadRequest, spec
	}
}
