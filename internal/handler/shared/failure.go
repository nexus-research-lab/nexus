// INPUT: Handler 已确认的 HTTP 状态、安全文案与结构化失败事实。
// OUTPUT: 保持旧 API envelope、可选携带 FailureCore v1 的失败响应。
// POS: 新失败协议的显式 opt-in 写出边界；旧 WriteFailure 行为保持独立不变。
package shared

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FailureSpec 只接受 Handler 已经确认的公开失败事实。
// Cause 只进入内部日志，不能进入响应；Resolution 必须来自领域规则，公共层不推断动作。
type FailureSpec struct {
	Code       string
	Category   protocol.FailureCategory
	Effect     protocol.FailureEffect
	Detail     string
	Cause      error
	RetryAfter time.Duration
	Resolution *protocol.FailureResolution
}

// WriteError 写入显式选择 FailureCore v1 的失败响应。
//
// 它不会替代 WriteFailure，也不会从 error 文本推断业务 code、数据影响或恢复动作。
func (a *API) WriteError(
	writer http.ResponseWriter,
	request *http.Request,
	status int,
	spec FailureSpec,
) {
	clientDetail := strings.TrimSpace(spec.Detail)
	canceled := isCanceledFailure(spec.Cause, clientDetail) && status >= http.StatusInternalServerError
	if canceled {
		status = 499
	}

	transportRequestID := ""
	logger := a.BaseLogger()
	if request != nil {
		transportRequestID = requestID(request.Context())
		logger = logx.FromContext(request.Context())
	}

	if spec.Cause != nil || clientDetail != "" {
		fields := []any{
			"status", status,
			"failure_code", strings.TrimSpace(spec.Code),
		}
		if spec.Cause != nil {
			fields = append(fields, "err", spec.Cause)
		} else {
			fields = append(fields, "detail", clientDetail)
		}
		if status == 499 {
			logger.Debug("HTTP 请求已取消", fields...)
		} else {
			logger.Warn("HTTP 请求失败", fields...)
		}
	}

	failure := protocol.FailureCore{
		Version:            protocol.FailureCoreVersion,
		Code:               strings.TrimSpace(spec.Code),
		Category:           spec.Category,
		Effect:             spec.Effect,
		TransportRequestID: transportRequestID,
		Resolution:         normalizeFailureResolution(spec.Resolution),
	}
	if failure.Code == "" {
		failure.Code = "common.request_failed"
	}
	if canceled {
		failure.Category = protocol.FailureCategoryCanceled
	} else if failure.Category == "" {
		failure.Category = failureCategoryForStatus(status)
	}
	if failure.Effect == "" {
		failure.Effect = protocol.FailureEffectUnknown
	}
	if retryAfterMS, ok := failureRetryAfter(status, spec.RetryAfter); ok {
		failure.RetryAfterMS = retryAfterMS
		seconds := (retryAfterMS + 999) / 1000
		writer.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	}

	data := map[string]any{
		"detail":  GatewayClientErrorDetail(status, clientDetail),
		"failure": failure,
	}
	if transportRequestID != "" {
		data["request_id"] = transportRequestID
	}
	a.WriteJSON(writer, status, map[string]any{
		"code":    strconv.Itoa(status),
		"message": "failed",
		"success": false,
		"data":    data,
	})
}

func isCanceledFailure(cause error, detail string) bool {
	return errors.Is(cause, context.Canceled) ||
		errors.Is(cause, context.DeadlineExceeded) ||
		isClientCanceledDetail(detail) ||
		(cause != nil && isClientCanceledDetail(cause.Error()))
}

func failureCategoryForStatus(status int) protocol.FailureCategory {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return protocol.FailureCategoryValidation
	case http.StatusUnauthorized:
		return protocol.FailureCategoryAuthentication
	case http.StatusForbidden:
		return protocol.FailureCategoryAuthorization
	case http.StatusNotFound:
		return protocol.FailureCategoryNotFound
	case http.StatusConflict, http.StatusPreconditionFailed:
		return protocol.FailureCategoryConflict
	case http.StatusTooManyRequests:
		return protocol.FailureCategoryRateLimited
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return protocol.FailureCategoryTimeout
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return protocol.FailureCategoryUnavailable
	case 499:
		return protocol.FailureCategoryCanceled
	default:
		return protocol.FailureCategoryInternal
	}
}

func failureRetryAfter(status int, retryAfter time.Duration) (int64, bool) {
	if retryAfter <= 0 || (status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable) {
		return 0, false
	}
	milliseconds := retryAfter.Milliseconds()
	if milliseconds <= 0 {
		milliseconds = 1
	}
	return milliseconds, true
}

func normalizeFailureResolution(
	resolution *protocol.FailureResolution,
) *protocol.FailureResolution {
	if resolution == nil {
		return nil
	}
	normalized := &protocol.FailureResolution{
		Actor:  protocol.FailureRecoveryActor(strings.TrimSpace(string(resolution.Actor))),
		Action: strings.TrimSpace(resolution.Action),
	}
	if normalized.Actor == "" || normalized.Action == "" {
		return nil
	}
	return normalized
}
