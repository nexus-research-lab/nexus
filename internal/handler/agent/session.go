// INPUT: 鉴权后的 Session HTTP 请求、分页参数与 opaque detail ref。
// OUTPUT: Session 消息/导航 JSON 或受限大型图片字节流。
// POS: Agent Session 读取传输边界；业务归属与 generation 校验委托 session service。
package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

// HandleSessionMessageDetailByQuery 返回大型 Tool result JSON 或图片字节流。
func (h *Handlers) HandleSessionMessageDetailByQuery(
	writer http.ResponseWriter,
	request *http.Request,
) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	ref := strings.TrimSpace(request.URL.Query().Get("detail_ref"))
	if sessionKey == "" || ref == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 和 detail_ref 参数缺失")
		return
	}
	detail, err := h.sessions.GetSessionMessageDetail(request.Context(), sessionKey, ref)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) ||
		errors.Is(err, sessionpkg.ErrMessageDetailUnavailable) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在或已更新")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "消息详情读取失败")
		return
	}
	if detail.Kind == "image" {
		writer.Header().Set("Content-Type", safeMessageDetailImageMediaType(detail.MediaType))
		writer.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		writer.Header().Set("Content-Length", strconv.FormatInt(detail.ByteSize, 10))
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(detail.Payload)
		return
	}
	var content any
	if err = json.Unmarshal(detail.Payload, &content); err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "消息详情格式错误")
		return
	}
	h.api.WriteSuccess(writer, protocol.MessageDetailResponse{
		Ref:      detail.Ref,
		Kind:     detail.Kind,
		ByteSize: detail.ByteSize,
		Content:  content,
	})
}

func safeMessageDetailImageMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/avif", "image/bmp", "image/gif", "image/jpeg", "image/png", "image/webp", "image/x-icon":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "application/octet-stream"
	}
}

// HandleListAgentSessions 返回指定 agent 的 session 列表。
func (h *Handlers) HandleListAgentSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := h.sessions.ListAgentSessions(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleListSessions 返回全部 session 列表。
func (h *Handlers) HandleListSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := h.sessions.ListSessions(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleSessionMessages 返回指定 session 的历史消息分页。
func (h *Handlers) HandleSessionMessages(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionKeyPathParam(request)
	h.writeSessionMessages(writer, request, sessionKey)
}

// HandleSessionMessagesByQuery 返回指定 session 的历史消息分页。
func (h *Handlers) HandleSessionMessagesByQuery(writer http.ResponseWriter, request *http.Request) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 参数缺失")
		return
	}
	h.writeSessionMessages(writer, request, sessionKey)
}

// HandleSessionRoundsByQuery 返回指定 session 的完整 round 导航索引。
func (h *Handlers) HandleSessionRoundsByQuery(writer http.ResponseWriter, request *http.Request) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 参数缺失")
		return
	}
	index, err := h.sessions.GetSessionRoundIndexPage(
		request.Context(),
		sessionKey,
		strings.EqualFold(strings.TrimSpace(request.URL.Query().Get("defer_index")), "true"),
	)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, index)
}

func (h *Handlers) writeSessionMessages(writer http.ResponseWriter, request *http.Request, sessionKey string) {
	limit := 0
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "limit 参数错误")
			return
		}
		limit = parsedLimit
	}
	beforeRoundID := strings.TrimSpace(request.URL.Query().Get("before_round_id"))
	beforeRoundTimestamp := int64(0)
	if rawBeforeTimestamp := strings.TrimSpace(request.URL.Query().Get("before_round_timestamp")); rawBeforeTimestamp != "" {
		parsedBeforeTimestamp, parseErr := strconv.ParseInt(rawBeforeTimestamp, 10, 64)
		if parseErr != nil || parsedBeforeTimestamp <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "before_round_timestamp 参数错误")
			return
		}
		beforeRoundTimestamp = parsedBeforeTimestamp
	}
	aroundRoundID := strings.TrimSpace(request.URL.Query().Get("around_round_id"))
	aroundLimit := 0
	if rawAroundLimit := strings.TrimSpace(request.URL.Query().Get("around_limit")); rawAroundLimit != "" {
		parsedAroundLimit, parseErr := strconv.Atoi(rawAroundLimit)
		if parseErr != nil || parsedAroundLimit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "around_limit 参数错误")
			return
		}
		aroundLimit = parsedAroundLimit
	}

	page, err := h.sessions.GetSessionMessagesPage(request.Context(), sessionKey, sessionpkg.MessagePageRequest{
		Limit:                limit,
		BeforeRoundID:        beforeRoundID,
		BeforeRoundTimestamp: beforeRoundTimestamp,
		AroundRoundID:        aroundRoundID,
		AroundLimit:          aroundLimit,
		DeferIndex:           strings.EqualFold(strings.TrimSpace(request.URL.Query().Get("defer_index")), "true"),
	})
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, page)
}

// HandleCreateSession 创建 session。
func (h *Handlers) HandleCreateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionpkg.CreateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.sessions.CreateSession(request.Context(), payload)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionMutationUnsupported) || handlershared.IsClientMessageError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateSession 更新 session。
func (h *Handlers) HandleUpdateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionpkg.UpdateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.sessions.UpdateSession(
		request.Context(),
		sessionKeyPathParam(request),
		payload,
	)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionMutationUnsupported) || handlershared.IsClientMessageError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSessionRuntimeSettings 返回当前 Session 的显式运行时覆盖。
func (h *Handlers) HandleSessionRuntimeSettings(
	writer http.ResponseWriter,
	request *http.Request,
) {
	settings, err := h.sessions.GetRuntimeSettings(
		request.Context(),
		sessionKeyPathParam(request),
	)
	if h.writeSessionRuntimeSettingsError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, settings)
}

// HandleUpdateSessionRuntimeSettings 更新当前 Session 的显式运行时覆盖。
func (h *Handlers) HandleUpdateSessionRuntimeSettings(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload protocol.SessionRuntimeSettings
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	settings, err := h.sessions.UpdateRuntimeSettings(
		request.Context(),
		sessionKeyPathParam(request),
		payload,
	)
	if h.writeSessionRuntimeSettingsError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, settings)
}

// HandleSessionLocalDirectories 返回当前 Session 的本机挂载目录。
func (h *Handlers) HandleSessionLocalDirectories(
	writer http.ResponseWriter,
	request *http.Request,
) {
	directories, err := h.sessions.GetLocalDirectories(
		request.Context(),
		sessionKeyPathParam(request),
	)
	if h.writeSessionLocalDirectoriesError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, directories)
}

// HandleUpdateSessionLocalDirectories 更新当前 Session 的本机挂载目录。
func (h *Handlers) HandleUpdateSessionLocalDirectories(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload protocol.SessionLocalDirectories
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	directories, err := h.sessions.UpdateLocalDirectories(
		request.Context(),
		sessionKeyPathParam(request),
		payload,
	)
	if h.writeSessionLocalDirectoriesError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, directories)
}

func (h *Handlers) writeSessionLocalDirectoriesError(
	writer http.ResponseWriter,
	err error,
) bool {
	if err == nil {
		return false
	}
	switch {
	case handlershared.IsStructuredSessionKeyError(err):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, sessionpkg.ErrSessionNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, sessionpkg.ErrLocalDirectoriesUnavailable):
		h.api.WriteFailure(writer, http.StatusForbidden, err.Error())
	case errors.Is(err, sessionpkg.ErrSessionMutationUnsupported),
		errors.Is(err, sessionpkg.ErrInvalidLocalDirectories),
		handlershared.IsClientMessageError(err):
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
	return true
}

func (h *Handlers) writeSessionRuntimeSettingsError(
	writer http.ResponseWriter,
	err error,
) bool {
	if err == nil {
		return false
	}
	switch {
	case handlershared.IsStructuredSessionKeyError(err):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, sessionpkg.ErrSessionNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, sessionpkg.ErrSessionMutationUnsupported),
		errors.Is(err, sessionpkg.ErrInvalidRuntimeSettings),
		handlershared.IsClientMessageError(err):
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
	return true
}

// HandleDeleteSession 删除 session。
func (h *Handlers) HandleDeleteSession(writer http.ResponseWriter, request *http.Request) {
	err := h.sessions.DeleteSession(
		request.Context(),
		sessionKeyPathParam(request),
	)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// sessionKeyPathParam 统一还原 URL path 中经过编码的结构化 session_key。
func sessionKeyPathParam(request *http.Request) string {
	raw := strings.TrimSpace(chi.URLParam(request, "session_key"))
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(decoded)
}
