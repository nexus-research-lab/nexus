// INPUT: Echo 用户级设置 HTTP 请求、Preferences revision 与服务阶段错误。
// OUTPUT: 带 ETag/CAS 的 owner-scoped 开关及 Problem/Impact/Recovery 失败事实。
// POS: Echo 的 HTTP 并发控制和失败投影边界；不改写业务 revision。
package echo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	echosvc "github.com/nexus-research-lab/nexus/internal/service/echo"
)

const echoETagPrefix = "echo-"

type echoService interface {
	GetSettings(context.Context) (echodomain.Settings, error)
	UpdateSettings(context.Context, echodomain.Settings) (echodomain.Settings, error)
	UpdateSettingsAtVersion(context.Context, echodomain.Settings, int64) (echodomain.Settings, error)
}

// Handlers 封装 Echo HTTP handlers。
type Handlers struct {
	api  *handlershared.API
	echo echoService
}

// New 创建 Echo handlers。
func New(api *handlershared.API, service echoService) *Handlers {
	return &Handlers{api: api, echo: service}
}

// HandleGetEcho 返回当前用户的 Echo 全局开关。
func (h *Handlers) HandleGetEcho(writer http.ResponseWriter, request *http.Request) {
	writeEchoNoStore(writer)
	settings, err := h.echo.GetSettings(request.Context())
	if err != nil {
		h.writeReadError(writer, request, err)
		return
	}
	writeEchoETag(writer, settings.Version)
	h.api.WriteSuccess(writer, settings)
}

// HandleUpdateEcho 更新当前用户的 Echo 全局开关。
func (h *Handlers) HandleUpdateEcho(writer http.ResponseWriter, request *http.Request) {
	writeEchoNoStore(writer)
	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	if !h.api.BindJSONError(writer, request, &payload, handlershared.FailureSpec{
		Code:     "echo.request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "主动跟进设置格式不正确",
	}) {
		return
	}
	if payload.Enabled == nil {
		h.api.WriteError(writer, request, http.StatusUnprocessableEntity, handlershared.FailureSpec{
			Code:     "echo.enabled_required",
			Category: protocol.FailureCategoryValidation,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "需要选择是否启用主动跟进",
		})
		return
	}
	expectedVersion, err := parseEchoIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.api.WriteError(writer, request, http.StatusBadRequest, handlershared.FailureSpec{
			Code:     "echo.precondition_invalid",
			Category: protocol.FailureCategoryValidation,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "主动跟进设置版本条件无效",
			Cause:    err,
		})
		return
	}
	input := echodomain.Settings{Enabled: *payload.Enabled}
	var settings echodomain.Settings
	if expectedVersion == nil {
		settings, err = h.echo.UpdateSettings(request.Context(), input)
	} else {
		settings, err = h.echo.UpdateSettingsAtVersion(
			request.Context(), input, *expectedVersion,
		)
	}
	if err != nil {
		h.writeUpdateError(writer, request, err)
		return
	}
	writeEchoETag(writer, settings.Version)
	h.api.WriteSuccess(writer, settings)
}

func (h *Handlers) writeReadError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     "echo.read_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法读取主动跟进设置",
		Cause:    err,
	})
}

func (h *Handlers) writeUpdateError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	if errors.Is(err, echosvc.ErrSettingsVersionConflict) {
		h.api.WriteError(writer, request, http.StatusPreconditionFailed, handlershared.FailureSpec{
			Code:     "echo.version_conflict",
			Category: protocol.FailureCategoryConflict,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "主动跟进设置已在其他页面更新",
			Cause:    err,
		})
		return
	}
	if echosvc.SettingsUpdateCommitted(err) {
		h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "echo.cleanup_incomplete",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectCommitted,
			Detail:   "主动跟进已关闭，但在途跟进还没有全部停止",
			Cause:    err,
		})
		return
	}
	h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     "echo.update_result_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "还无法确认主动跟进设置是否已更新",
		Cause:    err,
	})
}

func writeEchoETag(writer http.ResponseWriter, version int64) {
	if writer == nil || version < 1 {
		return
	}
	writer.Header().Set("ETag", fmt.Sprintf(`"%s%d"`, echoETagPrefix, version))
	writeEchoNoStore(writer)
}

func writeEchoNoStore(writer http.ResponseWriter) {
	if writer != nil {
		writer.Header().Set("Cache-Control", "no-store")
	}
}

func parseEchoIfMatch(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, ",") || value == "*" || strings.HasPrefix(value, "W/") {
		return nil, errors.New("Echo If-Match 必须是单个强 ETag")
	}
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return nil, errors.New("Echo If-Match 缺少强 ETag 引号")
	}
	opaque := strings.TrimSpace(value[1 : len(value)-1])
	if !strings.HasPrefix(opaque, echoETagPrefix) {
		return nil, errors.New("Echo If-Match 不属于主动跟进设置")
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(opaque, echoETagPrefix), 10, 64)
	if err != nil || version < 1 {
		return nil, errors.New("Echo If-Match version 无效")
	}
	return &version, nil
}
