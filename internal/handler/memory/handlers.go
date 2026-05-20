package memory

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"
)

// Handlers 封装记忆 HTTP handlers。
type Handlers struct {
	api    *handlershared.API
	config config.Config
	agents *agentpkg.Service
}

// New 创建记忆 handlers。
func New(api *handlershared.API, cfg config.Config, agents *agentpkg.Service) *Handlers {
	return &Handlers{
		api:    api,
		config: cfg,
		agents: agents,
	}
}

// HandleListMemory 返回结构化记忆列表。
func (h *Handlers) HandleListMemory(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	items, err := engine.List(request.Context(), memorysvc.MemoryListOptions{
		Limit:    intQuery(request, "limit", 200),
		Statuses: splitCSV(request.URL.Query().Get("status")),
		Scope:    strings.TrimSpace(request.URL.Query().Get("scope")),
	})
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"items": items})
}

// HandleSearchMemory 执行记忆检索。
func (h *Handlers) HandleSearchMemory(writer http.ResponseWriter, request *http.Request) {
	engine, scope, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	items, err := engine.Search(request.Context(), scope, memorysvc.RecallRequest{
		Query:      query,
		MaxResults: intQuery(request, "limit", h.config.MemoryMaxResults),
	})
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"items": items})
}

// HandleRecallMemory 返回运行时注入片段。
func (h *Handlers) HandleRecallMemory(writer http.ResponseWriter, request *http.Request) {
	engine, scope, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	var payload memorysvc.RecallRequest
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	if payload.Query == "" {
		payload.Query = request.URL.Query().Get("q")
	}
	if payload.MaxResults <= 0 {
		payload.MaxResults = intQuery(request, "limit", h.config.MemoryMaxResults)
	}
	injection, err := engine.BeforeRecall(request.Context(), scope, payload)
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, injection)
}

// HandleAddMemory 手动新增记忆。
func (h *Handlers) HandleAddMemory(writer http.ResponseWriter, request *http.Request) {
	engine, scope, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	var payload memorysvc.MemoryWriteInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := engine.Add(request.Context(), scope, payload)
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateMemory 更新记忆。
func (h *Handlers) HandleUpdateMemory(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	var payload memorysvc.MemoryWriteInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := engine.Update(request.Context(), chi.URLParam(request, "entry_id"), payload)
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteMemory 删除记忆。
func (h *Handlers) HandleDeleteMemory(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	if err := engine.Delete(request.Context(), chi.URLParam(request, "entry_id")); err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"deleted": true})
}

// HandlePromoteMemory 提升候选记忆到热记忆。
func (h *Handlers) HandlePromoteMemory(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	var payload struct {
		Target string `json:"target"`
	}
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	result, err := engine.Promote(request.Context(), chi.URLParam(request, "entry_id"), payload.Target)
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleIgnoreMemory 忽略候选记忆。
func (h *Handlers) HandleIgnoreMemory(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	var payload struct {
		Note string `json:"note"`
	}
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	item, err := engine.Ignore(request.Context(), chi.URLParam(request, "entry_id"), payload.Note)
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleMemoryStats 返回记忆统计。
func (h *Handlers) HandleMemoryStats(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	stats, err := engine.Stats(request.Context())
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, stats)
}

// HandleMemorySessionSummary 返回会话摘要。
func (h *Handlers) HandleMemorySessionSummary(writer http.ResponseWriter, request *http.Request) {
	engine, _, ok := h.engineForRequest(writer, request)
	if !ok {
		return
	}
	summary, err := engine.SessionSummary(request.Context(), request.URL.Query().Get("session_key"))
	if err != nil {
		h.writeMemoryError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]string{"summary": summary})
}

func (h *Handlers) engineForRequest(writer http.ResponseWriter, request *http.Request) (*memorysvc.Engine, memorysvc.MemoryScope, bool) {
	agentID := chi.URLParam(request, "agent_id")
	agentValue, err := h.agents.GetAgent(request.Context(), agentID)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return nil, memorysvc.MemoryScope{}, false
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return nil, memorysvc.MemoryScope{}, false
	}
	scope := h.scopeFromRequest(request, agentValue.AgentID)
	engine := memorysvc.NewEngine(agentValue.WorkspacePath, h.memoryOptions())
	return engine, scope, true
}

func (h *Handlers) scopeFromRequest(request *http.Request, agentID string) memorysvc.MemoryScope {
	query := request.URL.Query()
	kind := memorysvc.ScopeKind(strings.TrimSpace(query.Get("scope_kind")))
	if kind == "" {
		kind = memorysvc.ScopeKindAgent
	}
	return memorysvc.MemoryScope{
		Kind:           kind,
		UserID:         authctx.OwnerUserID(request.Context()),
		AgentID:        agentID,
		SessionKey:     strings.TrimSpace(query.Get("session_key")),
		SessionID:      strings.TrimSpace(query.Get("session_id")),
		RoomID:         strings.TrimSpace(query.Get("room_id")),
		ConversationID: strings.TrimSpace(query.Get("conversation_id")),
	}
}

func (h *Handlers) memoryOptions() memorysvc.MemoryOptions {
	return memorysvc.MemoryOptions{
		Enabled:        h.config.MemoryEnabled,
		AutoRecall:     h.config.MemoryAutoRecall,
		AutoExtract:    h.config.MemoryAutoExtract,
		MaxResults:     h.config.MemoryMaxResults,
		ScoreThreshold: h.config.MemoryScoreThreshold,
	}.Normalize()
}

func (h *Handlers) writeMemoryError(writer http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "不能为空") || strings.Contains(err.Error(), "未找到") || strings.Contains(err.Error(), "不支持") {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
}

func intQuery(request *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}
