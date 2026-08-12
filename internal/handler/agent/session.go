package agent

import (
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

// HandleSessionTurns 返回指定 session 的 ConversationTurn 分页。
func (h *Handlers) HandleSessionTurns(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionKeyPathParam(request)
	h.writeSessionTurns(writer, request, sessionKey)
}

// HandleSessionTurnsByQuery 返回指定 session 的 ConversationTurn 分页。
func (h *Handlers) HandleSessionTurnsByQuery(writer http.ResponseWriter, request *http.Request) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 参数缺失")
		return
	}
	h.writeSessionTurns(writer, request, sessionKey)
}

func (h *Handlers) writeSessionTurns(writer http.ResponseWriter, request *http.Request, sessionKey string) {
	limit := 0
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "limit 参数错误")
			return
		}
		limit = parsedLimit
	}
	page, err := h.sessions.GetSessionTurnsPage(request.Context(), sessionKey, sessionpkg.TurnPageRequest{
		Limit:         limit,
		BeforeRoundID: strings.TrimSpace(request.URL.Query().Get("before_round_id")),
		AroundRoundID: strings.TrimSpace(request.URL.Query().Get("around_round_id")),
		Sort:          strings.TrimSpace(request.URL.Query().Get("sort")),
		View:          strings.TrimSpace(request.URL.Query().Get("view")),
	})
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionpkg.ErrExternalSessionPairingActive) {
		h.api.WriteFailure(writer, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionMutationUnsupported) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, page)
}

// HandleSessionTurnIndexByQuery 返回指定 session 的 turn 导航索引。
func (h *Handlers) HandleSessionTurnIndexByQuery(writer http.ResponseWriter, request *http.Request) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 参数缺失")
		return
	}
	items, err := h.sessions.GetSessionTurnIndex(request.Context(), sessionKey)
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
	h.api.WriteSuccess(writer, map[string]any{"items": items})
}

// HandleSessionRoundsByQuery 返回指定 session 的完整 round 导航索引。
func (h *Handlers) HandleSessionRoundsByQuery(writer http.ResponseWriter, request *http.Request) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "session_key 参数缺失")
		return
	}
	index, err := h.sessions.GetSessionRoundIndex(request.Context(), sessionKey)
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
