package channel

import (
	"context"
	"errors"
	"net/http"
	"strings"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
)

// Ingress 接口抽象通道入站服务。
type Ingress interface {
	Accept(context.Context, channelspkg.IngressRequest) (*channelspkg.IngressResult, error)
}

// Handlers 封装通道域 HTTP handlers。
type Handlers struct {
	api     *gatewayshared.API
	ingress Ingress
}

// New 创建通道 handlers。
func New(api *gatewayshared.API, ingress Ingress) *Handlers {
	return &Handlers{
		api:     api,
		ingress: ingress,
	}
}

func (h *Handlers) HandleChannelIngress(writer http.ResponseWriter, request *http.Request) {
	h.handleChannelIngressByName(writer, request, "")
}

func (h *Handlers) HandleInternalChannelIngress(writer http.ResponseWriter, request *http.Request) {
	h.handleChannelIngressByName(writer, request, channelspkg.ChannelTypeInternal)
}

func (h *Handlers) HandleDiscordChannelIngress(writer http.ResponseWriter, request *http.Request) {
	h.handleChannelIngressByName(writer, request, channelspkg.ChannelTypeDiscord)
}

func (h *Handlers) HandleTelegramChannelIngress(writer http.ResponseWriter, request *http.Request) {
	h.handleChannelIngressByName(writer, request, channelspkg.ChannelTypeTelegram)
}

func (h *Handlers) handleChannelIngressByName(
	writer http.ResponseWriter,
	request *http.Request,
	channelName string,
) {
	if h.ingress == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "channel ingress is not configured")
		return
	}

	var payload channelspkg.IngressRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if strings.TrimSpace(channelName) != "" {
		payload.Channel = channelName
	}

	result, err := h.ingress.Accept(request.Context(), payload)
	if err != nil {
		if isChannelIngressClientError(err) || gatewayshared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

func isChannelIngressClientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, channelspkg.ErrIngressChannelRequired) || errors.Is(err, channelspkg.ErrIngressRefRequired) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "content is required") ||
		strings.Contains(message, "agent_id 与 session_key 不一致") ||
		strings.Contains(message, "channel 与 session_key 不一致") ||
		strings.Contains(message, "仅支持 agent session_key") ||
		strings.Contains(message, "requires")
}
