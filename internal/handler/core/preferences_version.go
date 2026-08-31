// INPUT: Preferences HTTP 读写请求、持久化 version 与已确认的失败阶段。
// OUTPUT: 强 ETag/If-Match 条件、CAS 冲突与 Problem/Impact/Recovery 失败事实。
// POS: Preferences HTTP 并发控制和失败投影边界；不创建新业务身份或改写 Service version。
package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const preferencesETagPrefix = "preferences-"

func writePreferencesETag(writer http.ResponseWriter, version int64) {
	if writer == nil || version < 1 {
		return
	}
	writer.Header().Set("ETag", fmt.Sprintf(`"%s%d"`, preferencesETagPrefix, version))
	// ETag 只作为持久 Preferences aggregate 的 PATCH 强前置条件。
	// GET 还会投影 Provider 派生默认值，不得被中间缓存当作完整表示。
	writePreferencesNoStore(writer)
}

func writePreferencesNoStore(writer http.ResponseWriter) {
	if writer != nil {
		writer.Header().Set("Cache-Control", "no-store")
	}
}

func parsePreferencesIfMatch(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, ",") || value == "*" || strings.HasPrefix(value, "W/") {
		return nil, errors.New("Preferences If-Match 必须是单个强 ETag")
	}
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return nil, errors.New("Preferences If-Match 缺少强 ETag 引号")
	}
	opaque := strings.TrimSpace(value[1 : len(value)-1])
	if !strings.HasPrefix(opaque, preferencesETagPrefix) {
		return nil, errors.New("Preferences If-Match 不属于偏好设置")
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(opaque, preferencesETagPrefix), 10, 64)
	if err != nil || version < 1 {
		return nil, errors.New("Preferences If-Match version 无效")
	}
	return &version, nil
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
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "preferences.reload",
		},
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
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "preferences.reload",
		},
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
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "preferences.reload",
		},
	})
}
