package capability

import (
	"context"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
)

// ChannelSummaryCounter 抽象频道与配对计数能力。
type ChannelSummaryCounter interface {
	CountConfiguredChannels(context.Context, string) (int, error)
	CountActivePairings(context.Context, string) (int, error)
}

// Handlers 封装 capability summary handler。
type Handlers struct {
	api        *handlershared.API
	skills     *skillspkg.Service
	connectors *connectorsvc.Service
	automation *automationsvc.Service
	channels   ChannelSummaryCounter
}

// New 创建 capability handlers。
func New(
	api *handlershared.API,
	skills *skillspkg.Service,
	connectors *connectorsvc.Service,
	automation *automationsvc.Service,
	channels ...ChannelSummaryCounter,
) *Handlers {
	var channelCounter ChannelSummaryCounter
	if len(channels) > 0 {
		channelCounter = channels[0]
	}
	return &Handlers{
		api:        api,
		skills:     skills,
		connectors: connectors,
		automation: automation,
		channels:   channelCounter,
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
	configuredChannelCount := 0
	activePairingCount := 0
	if h.channels != nil {
		ownerUserID := authsvc.OwnerUserID(request.Context())
		configuredChannelCount, err = h.channels.CountConfiguredChannels(request.Context(), ownerUserID)
		if err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
		activePairingCount, err = h.channels.CountActivePairings(request.Context(), ownerUserID)
		if err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
	}

	h.api.WriteSuccess(writer, map[string]any{
		"skills_count":                  skillCount,
		"connected_connectors_count":    connectorCount,
		"enabled_scheduled_tasks_count": scheduledTaskEnabledCount,
		"configured_channels_count":     configuredChannelCount,
		"active_pairings_count":         activePairingCount,
	})
}
