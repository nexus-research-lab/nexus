package capability

import (
	"net/http"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
)

// Handlers 封装 capability summary handler。
type Handlers struct {
	api        *gatewayshared.API
	skills     *skillspkg.Service
	connectors *connectorsvc.Service
	automation *automationsvc.Service
}

// New 创建 capability handlers。
func New(
	api *gatewayshared.API,
	skills *skillspkg.Service,
	connectors *connectorsvc.Service,
	automation *automationsvc.Service,
) *Handlers {
	return &Handlers{
		api:        api,
		skills:     skills,
		connectors: connectors,
		automation: automation,
	}
}

// HandleCapabilitySummary 返回能力摘要。
func (h *Handlers) HandleCapabilitySummary(writer http.ResponseWriter, request *http.Request) {
	skillCount, err := h.skills.CountSkills(request.Context(), skillspkg.Query{})
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	connectorCount, err := h.connectors.GetConnectedCount(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	scheduledTaskEnabledCount, err := h.automation.CountEnabledTasks(request.Context(), "")
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	h.api.WriteSuccess(writer, map[string]any{
		"skills_count":                  skillCount,
		"connected_connectors_count":    connectorCount,
		"enabled_scheduled_tasks_count": scheduledTaskEnabledCount,
	})
}
