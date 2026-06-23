package channel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

// Ingress 接口抽象通道入站服务。
type Ingress interface {
	Accept(context.Context, channelspkg.IngressRequest) (*channelspkg.IngressResult, error)
}

// Control 接口抽象频道配置与配对授权服务。
type Control interface {
	ListChannels(context.Context, string) ([]channelspkg.ChannelConfigView, error)
	UpsertChannelConfig(context.Context, string, string, channelspkg.UpsertChannelConfigRequest) (*channelspkg.ChannelConfigView, error)
	DeleteChannelConfig(context.Context, string, string) error
	DeleteChannelAccount(context.Context, string, string, string) (*channelspkg.ChannelConfigView, error)
	StartChannelLogin(context.Context, string, string) (*channelspkg.ChannelLoginView, error)
	GetChannelLogin(context.Context, string, string, string) (*channelspkg.ChannelLoginView, error)
	SubmitChannelLoginVerifyCode(context.Context, string, string, string, channelspkg.SubmitChannelLoginVerifyCodeRequest) (*channelspkg.ChannelLoginView, error)
	ListPairings(context.Context, string, channelspkg.PairingQuery) ([]channelspkg.PairingView, error)
	CreatePairing(context.Context, string, channelspkg.CreatePairingRequest) (*channelspkg.PairingView, error)
	UpdatePairing(context.Context, string, string, channelspkg.UpdatePairingRequest) (*channelspkg.PairingView, error)
	DeletePairing(context.Context, string, string) error
	ResolveChannelOwnerByConfig(context.Context, string, string, string) (string, error)
	PrepareFeishuIngress(context.Context, []byte, http.Header) (channelspkg.FeishuIngressPreparation, error)
}

// Handlers 封装通道域 HTTP handlers。
type Handlers struct {
	api     *handlershared.API
	ingress Ingress
	control Control
}

// New 创建通道 handlers。
func New(api *handlershared.API, ingress Ingress, control ...Control) *Handlers {
	var controlService Control
	if len(control) > 0 {
		controlService = control[0]
	}
	return &Handlers{
		api:     api,
		ingress: ingress,
		control: controlService,
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

func (h *Handlers) HandleDingTalkChannelIngress(writer http.ResponseWriter, request *http.Request) {
	if h.ingress == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "channel ingress is not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	callbackRequest, ignoredReason, err := channeladapters.DecodeDingTalkIngressCallback(body)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if callbackRequest == nil {
		h.api.WriteSuccess(writer, map[string]any{
			"accepted": false,
			"ignored":  true,
			"reason":   ignoredReason,
		})
		return
	}
	h.acceptChannelIngress(writer, request, *callbackRequest)
}

func (h *Handlers) HandleFeishuChannelIngress(writer http.ResponseWriter, request *http.Request) {
	if h.ingress == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "channel ingress is not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	preparedBody := body
	ownerUserID := ""
	if h.control != nil {
		prepared, prepareErr := h.control.PrepareFeishuIngress(request.Context(), body, request.Header)
		if errors.Is(prepareErr, channelspkg.ErrFeishuCallbackUnauthorized) {
			h.api.WriteFailure(writer, http.StatusUnauthorized, "feishu callback verification failed")
			return
		}
		if prepareErr != nil {
			h.api.WriteFailure(writer, http.StatusBadRequest, prepareErr.Error())
			return
		}
		if len(prepared.Body) > 0 {
			preparedBody = prepared.Body
		}
		ownerUserID = strings.TrimSpace(prepared.OwnerUserID)
	}
	callback, err := channeladapters.DecodeFeishuIngressCallback(preparedBody)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(callback.Challenge) != "" {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(map[string]string{"challenge": callback.Challenge})
		return
	}
	if callback.Request == nil {
		h.api.WriteSuccess(writer, map[string]any{
			"accepted": false,
			"ignored":  true,
			"reason":   callback.IgnoredReason,
		})
		return
	}
	if ownerUserID != "" {
		callback.Request.OwnerUserID = ownerUserID
	} else if h.control != nil && strings.TrimSpace(callback.AppID) != "" {
		ownerUserID, ownerErr := h.control.ResolveChannelOwnerByConfig(
			request.Context(),
			channelspkg.ChannelTypeFeishu,
			"app_id",
			callback.AppID,
		)
		if ownerErr != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, ownerErr.Error())
			return
		}
		if strings.TrimSpace(ownerUserID) != "" {
			callback.Request.OwnerUserID = ownerUserID
		}
	}

	h.acceptChannelIngress(writer, request, *callback.Request)
}

func (h *Handlers) HandleWeixinPersonalChannelIngress(writer http.ResponseWriter, request *http.Request) {
	h.handleChannelIngressByName(writer, request, channelspkg.ChannelTypeWeixinPersonal)
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

	h.acceptChannelIngress(writer, request, payload)
}

func (h *Handlers) acceptChannelIngress(
	writer http.ResponseWriter,
	request *http.Request,
	payload channelspkg.IngressRequest,
) {
	result, err := h.ingress.Accept(request.Context(), payload)
	h.writeChannelIngressOutcome(writer, result, err)
}

func (h *Handlers) writeChannelIngressOutcome(writer http.ResponseWriter, result *channelspkg.IngressResult, err error) {
	if errors.Is(err, channelspkg.ErrPairingApprovalRequired) {
		h.api.WriteSuccess(writer, map[string]any{
			"accepted":         false,
			"pairing_required": true,
			"message":          err.Error(),
		})
		return
	}
	if err != nil {
		if isChannelIngressClientError(err) || handlershared.IsStructuredSessionKeyError(err) {
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
		strings.Contains(message, "配对授权") ||
		strings.Contains(message, "配对控制台") ||
		strings.Contains(message, "requires")
}
