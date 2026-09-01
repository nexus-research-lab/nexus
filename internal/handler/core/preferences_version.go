// INPUT: Preferences HTTP 读写请求、持久化 version 与已确认的失败阶段。
// OUTPUT: 强 ETag/If-Match 条件、CAS 冲突与 Problem/Impact/Recovery 失败事实。
// POS: Preferences HTTP 并发控制和失败投影边界；不创建新业务身份或改写 Service version。
package core

import (
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const preferencesETagPrefix = "preferences-"

func writePreferencesETag(writer http.ResponseWriter, version int64) {
	handlershared.WriteStrongETag(writer, preferencesETagPrefix, version)
}

func writePreferencesNoStore(writer http.ResponseWriter) {
	// GET 还会投影 Provider 派生默认值，不得被中间缓存当作完整表示。
	if writer != nil {
		writer.Header().Set("Cache-Control", "no-store")
	}
}

func parsePreferencesIfMatch(value string) (*int64, error) {
	return handlershared.ParseStrongIfMatch(
		value,
		preferencesETagPrefix,
		"Preferences",
		"偏好设置",
	)
}

func (h *Handlers) writePreferencesReadFailure(
	writer http.ResponseWriter,
	request *http.Request,
	cause error,
) {
	h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     "preferences.read_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法读取最新偏好设置",
		Cause:    cause,
	})
}

func (h *Handlers) writePreferencesVersionConflict(
	writer http.ResponseWriter,
	request *http.Request,
	cause error,
) {
	h.api.WriteError(writer, request, http.StatusPreconditionFailed, handlershared.FailureSpec{
		Code:     "preferences.version_conflict",
		Category: protocol.FailureCategoryConflict,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "偏好设置版本与服务端最新版本不一致",
		Cause:    cause,
	})
}

func (h *Handlers) writePreferencesPostCommitFailure(
	writer http.ResponseWriter,
	request *http.Request,
	cause error,
) {
	// Preferences 文件已经发布，随后的 Provider/runtime 投影失败可能已被
	// version 条件回滚，也可能因并发新写而跳过；不根据 error 文本猜测。
	h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     "preferences.projection_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "偏好设置的后续同步未完成",
		Cause:    cause,
	})
}
