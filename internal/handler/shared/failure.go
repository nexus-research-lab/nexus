// INPUT: Handler 已确认的 HTTP 状态、安全文案与结构化失败事实。
// OUTPUT: 保持旧 API envelope、可选携带 FailureCore v1 的失败响应。
// POS: 新失败协议的显式 opt-in 写出边界；旧 WriteFailure 行为保持独立不变。
package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const maxFailureSemanticKeyLength = 128

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
	canceled := errors.Is(spec.Cause, context.Canceled) && status >= http.StatusInternalServerError
	timedOut := errors.Is(spec.Cause, context.DeadlineExceeded) && status >= http.StatusInternalServerError
	if canceled {
		status = 499
	} else if timedOut {
		status = http.StatusGatewayTimeout
	}

	transportRequestID := ""
	logger := a.BaseLogger()
	if request != nil {
		transportRequestID = requestID(request.Context())
		logger = logx.FromContext(request.Context())
	}

	normalizedCode, codeValid := normalizeFailureSemanticKey(spec.Code)
	normalizedResolution, resolutionValid := normalizeFailureResolution(spec.Resolution)
	categoryValid := knownFailureCategory(spec.Category)
	effectValid := knownFailureEffect(spec.Effect)
	failure := protocol.FailureCore{
		Version:            protocol.FailureCoreVersion,
		Code:               normalizedCode,
		Category:           spec.Category,
		Effect:             spec.Effect,
		TransportRequestID: transportRequestID,
		Resolution:         normalizedResolution,
	}
	if failure.Code == "" {
		failure.Code = "common.request_failed"
	}
	if canceled {
		failure.Category = protocol.FailureCategoryCanceled
	} else if timedOut {
		failure.Category = protocol.FailureCategoryTimeout
	} else if !categoryValid {
		failure.Category = failureCategoryForStatus(status)
	}
	if !effectValid {
		failure.Effect = protocol.FailureEffectUnknown
	}

	if spec.Cause != nil || clientDetail != "" || !codeValid || !categoryValid || !effectValid || !resolutionValid {
		fields := []any{
			"status", status,
			"failure_code", failure.Code,
			"failure_category", failure.Category,
			"failure_effect", failure.Effect,
			"failure_category_valid", categoryValid,
			"failure_effect_valid", effectValid,
			"failure_code_valid", codeValid,
			"failure_resolution_valid", resolutionValid,
		}
		if spec.Cause != nil {
			// 共享 writer 无法知道第三方/Provider error 是否夹带 token、路径或正文。
			// 这里只记录可关联的结构化事实和类型；需要 cause 细节的领域必须在
			// 自己的安全边界先脱敏，再单独记录。
			fields = append(fields,
				"has_cause", true,
				"cause_type", fmt.Sprintf("%T", spec.Cause),
			)
		} else {
			// 面向用户的 detail 可能包含资源名或用户输入。日志只记录其存在，
			// 内部定位依赖稳定 code、诊断请求身份和显式 Cause。
			fields = append(fields, "has_client_detail", clientDetail != "")
		}
		if status == 499 {
			logger.Debug("HTTP 请求已取消", fields...)
		} else {
			logger.Warn("HTTP 请求失败", fields...)
		}
	}
	if retryAfterMS, ok := failureRetryAfter(status, spec.RetryAfter); ok {
		failure.RetryAfterMS = retryAfterMS
		seconds := (retryAfterMS + 999) / 1000
		writer.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	}

	data := map[string]any{
		"detail":  failureClientDetail(status, clientDetail),
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

func knownFailureCategory(category protocol.FailureCategory) bool {
	if category == "" {
		return false
	}
	switch category {
	case protocol.FailureCategoryValidation,
		protocol.FailureCategoryAuthentication,
		protocol.FailureCategoryAuthorization,
		protocol.FailureCategoryNotFound,
		protocol.FailureCategoryConflict,
		protocol.FailureCategoryRateLimited,
		protocol.FailureCategoryUnavailable,
		protocol.FailureCategoryTimeout,
		protocol.FailureCategoryCanceled,
		protocol.FailureCategoryInternal:
		return true
	default:
		return false
	}
}

func knownFailureEffect(effect protocol.FailureEffect) bool {
	switch effect {
	case protocol.FailureEffectNotApplicable,
		protocol.FailureEffectNotApplied,
		protocol.FailureEffectAccepted,
		protocol.FailureEffectCommitted,
		protocol.FailureEffectUnknown:
		return true
	default:
		return false
	}
}

func failureClientDetail(status int, detail string) string {
	if status == 499 {
		return "请求已取消"
	}
	if status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout {
		return "请求超时"
	}
	// 与旧 WriteFailure 不同，FailureSpec.Detail 是 Handler 明确确认过的
	// 用户文案。保留它才能让 HTTP/CLI 等非 Web 消费方得到具体问题，
	// 同时 Cause 继续完全隔离在响应之外。
	if detail = strings.TrimSpace(detail); detail != "" {
		return detail
	}
	return GatewayClientErrorDetail(status, "")
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

// failureEffectBeforeHandler 只在中间件已经拒绝请求、业务 Handler 尚未执行时使用。
func failureEffectBeforeHandler(request *http.Request) protocol.FailureEffect {
	if request == nil {
		return protocol.FailureEffectNotApplied
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return protocol.FailureEffectNotApplicable
	default:
		return protocol.FailureEffectNotApplied
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
) (*protocol.FailureResolution, bool) {
	if resolution == nil {
		return nil, true
	}
	normalized := &protocol.FailureResolution{
		Actor:  protocol.FailureRecoveryActor(strings.TrimSpace(string(resolution.Actor))),
		Action: "",
	}
	action, actionValid := normalizeFailureSemanticKey(resolution.Action)
	normalized.Action = action
	if !knownFailureRecoveryActor(normalized.Actor) || !actionValid || normalized.Action == "" {
		return nil, false
	}
	return normalized, true
}

// normalizeFailureSemanticKey 只接受稳定的 domain.reason 语义名。
// 用户文案、URL、命令、路径或秘密必须留在该字段之外。
func normalizeFailureSemanticKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	if len(value) > maxFailureSemanticKeyLength || value[0] < 'a' || value[0] > 'z' {
		return "", false
	}
	hasSeparator := false
	segmentStart := true
	for _, character := range value {
		switch {
		case character == '.':
			if segmentStart {
				return "", false
			}
			hasSeparator = true
			segmentStart = true
		case character >= 'a' && character <= 'z':
			segmentStart = false
		case character >= '0' && character <= '9', character == '_':
			if segmentStart {
				return "", false
			}
		default:
			return "", false
		}
	}
	if segmentStart || !hasSeparator {
		return "", false
	}
	return value, true
}

func knownFailureRecoveryActor(actor protocol.FailureRecoveryActor) bool {
	switch actor {
	case protocol.FailureRecoveryActorUser,
		protocol.FailureRecoveryActorSystem,
		protocol.FailureRecoveryActorExternal,
		protocol.FailureRecoveryActorNone:
		return true
	default:
		return false
	}
}
