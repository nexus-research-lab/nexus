package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
	dingchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"golang.org/x/sync/singleflight"
)

type DingTalkChannel struct {
	clientID     string
	clientSecret string
	robotCode    string
	client       *http.Client
	baseURL      string
	streamHost   string
	ownerUserID  string

	mu             sync.RWMutex
	ingress        channelcontract.IngressAcceptor
	accessToken    string
	tokenExpiresAt time.Time
	stream         *dingclient.StreamClient
	tokenFlight    singleflight.Group
}

type dingTalkAccessTokenEnvelope struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type dingTalkAPIEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestid"`
}

func NewDingTalkChannel(clientID string, clientSecret string, robotCode string, client *http.Client) *DingTalkChannel {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &DingTalkChannel{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		robotCode:    strings.TrimSpace(robotCode),
		client:       client,
		baseURL:      "https://api.dingtalk.com",
		streamHost:   "https://api.dingtalk.com",
	}
}

func (c *DingTalkChannel) WithOwner(ownerUserID string) *DingTalkChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *DingTalkChannel) WithBaseURL(baseURL string) *DingTalkChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = normalizeDingTalkBaseURL(baseURL)
	}
	return c
}

func (c *DingTalkChannel) WithStreamHost(streamHost string) *DingTalkChannel {
	if streamHost = strings.TrimSpace(streamHost); streamHost != "" {
		c.streamHost = normalizeDingTalkBaseURL(streamHost)
	}
	return c
}

func (c *DingTalkChannel) BaseURL() string {
	return c.baseURL
}

func (c *DingTalkChannel) StreamHost() string {
	return c.streamHost
}

func (c *DingTalkChannel) ChannelType() string {
	return channelcontract.ChannelTypeDingTalk
}

func (c *DingTalkChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *DingTalkChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.clientID) == "" || strings.TrimSpace(c.clientSecret) == "" {
		return fmt.Errorf("dingtalk channel is not configured")
	}

	c.mu.Lock()
	if c.stream != nil {
		c.mu.Unlock()
		return nil
	}
	stream := dingclient.NewStreamClient(
		dingclient.WithAppCredential(dingclient.NewAppCredentialConfig(c.clientID, c.clientSecret)),
		dingclient.WithOpenApiHost(c.streamHost),
	)
	stream.RegisterChatBotCallbackRouter(c.handleStreamMessage)
	c.stream = stream
	c.mu.Unlock()

	if err := stream.Start(ctx); err != nil {
		c.mu.Lock()
		if c.stream == stream {
			c.stream = nil
		}
		c.mu.Unlock()
		stream.Close()
		return err
	}
	return nil
}

func (c *DingTalkChannel) Stop(context.Context) error {
	c.mu.Lock()
	stream := c.stream
	c.stream = nil
	c.mu.Unlock()
	if stream != nil {
		stream.Close()
	}
	return nil
}

func (c *DingTalkChannel) SendDeliveryMessage(ctx context.Context, target channelcontract.DeliveryTarget, text string) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(target.To) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("dingtalk delivery target requires to")
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "http://") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "https://") {
		if err := c.sendSessionWebhookText(ctx, target.To, text); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		return channelcontract.NewDeliveryResult(normalized, nil), nil
	}
	if strings.TrimSpace(c.robotCode) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("dingtalk delivery requires robot_code")
	}
	token, err := c.accessTokenForDelivery(ctx)
	if err != nil {
		return channelcontract.DeliveryResult{}, err
	}
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 3800) {
		if err = c.sendGroupTextChunk(ctx, token, target.To, chunk); err != nil {
			c.clearAccessToken()
			return channelcontract.DeliveryResult{}, err
		}
	}
	return channelcontract.NewDeliveryResult(normalized, nil), nil
}

func (c *DingTalkChannel) accessTokenForDelivery(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.clientID) == "" || strings.TrimSpace(c.clientSecret) == "" {
		return "", fmt.Errorf("dingtalk channel is not configured")
	}
	now := time.Now()
	c.mu.RLock()
	if c.accessToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	result, err, _ := c.tokenFlight.Do("refresh", func() (any, error) {
		return c.refreshAccessToken(ctx, now)
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (c *DingTalkChannel) refreshAccessToken(ctx context.Context, now time.Time) (string, error) {
	// singleflight 获胜方在真正刷新前再检查一次，避免等待期间已有 goroutine 写入新 token。
	c.mu.RLock()
	if c.accessToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	payload, err := json.Marshal(map[string]string{
		"appKey":    c.clientID,
		"appSecret": c.clientSecret,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1.0/oauth2/accessToken",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	var envelope dingTalkAccessTokenEnvelope
	if err = decodeDingTalkEnvelope(response, &envelope); err != nil {
		return "", err
	}
	if strings.TrimSpace(envelope.Code) != "" {
		return "", fmt.Errorf("dingtalk accessToken failed: code=%s message=%s", envelope.Code, strings.TrimSpace(envelope.Message))
	}
	token := strings.TrimSpace(envelope.AccessToken)
	if token == "" {
		return "", fmt.Errorf("dingtalk accessToken returned empty token")
	}
	expiresIn := envelope.ExpireIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	if expiresIn > 600 {
		expiresIn -= 300
	}
	c.mu.Lock()
	c.accessToken = token
	c.tokenExpiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	c.mu.Unlock()
	return token, nil
}

func (c *DingTalkChannel) sendGroupTextChunk(ctx context.Context, token string, conversationID string, text string) error {
	msgParam, err := json.Marshal(map[string]string{"content": text})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"robotCode":          c.robotCode,
		"openConversationId": strings.TrimSpace(conversationID),
		"msgKey":             "sampleText",
		"msgParam":           string(msgParam),
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1.0/robot/groupMessages/send",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-acs-dingtalk-access-token", strings.TrimSpace(token))

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	var envelope dingTalkAPIEnvelope
	if err = decodeDingTalkEnvelope(response, &envelope); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Code) != "" {
		return fmt.Errorf("dingtalk send message failed: code=%s message=%s", envelope.Code, strings.TrimSpace(envelope.Message))
	}
	return nil
}

func (c *DingTalkChannel) sendSessionWebhookText(ctx context.Context, webhook string, text string) error {
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 3800) {
		payload, err := json.Marshal(map[string]any{
			"msgtype": "text",
			"text": map[string]string{
				"content": chunk,
			},
		})
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(webhook), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := c.client.Do(request)
		if err != nil {
			return err
		}
		if err = channeltransport.ExpectSuccess(response); err != nil {
			return err
		}
	}
	return nil
}

func (c *DingTalkChannel) handleStreamMessage(ctx context.Context, data *dingchatbot.BotCallbackDataModel) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		return nil, nil
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return nil, nil
	}

	chatType := normalizeDingTalkConversationType(data.ConversationType)
	ref := channelcontract.FirstNonEmpty(data.ConversationId, data.SenderStaffId, data.SenderId)
	if ref == "" {
		return nil, nil
	}
	deliveryTo := channelcontract.FirstNonEmpty(data.SessionWebhook, data.ConversationId, ref)
	delivery := &channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeDingTalk,
		To:        deliveryTo,
		AccountID: strings.TrimSpace(data.ChatbotCorpId),
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeDingTalk,
		OwnerUserID:  c.ownerUserID,
		AccountID:    strings.TrimSpace(c.clientID),
		ChatType:     chatType,
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(data.MsgId),
		ReqID:        strings.TrimSpace(data.MsgId),
		ExternalName: channelcontract.FirstNonEmpty(data.ConversationTitle, data.SenderNick),
		Delivery:     delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDingTalk,
			Target:            ref,
			PlatformMessageID: strings.TrimSpace(data.MsgId),
			SenderID:          channelcontract.FirstNonEmpty(data.SenderStaffId, data.SenderId),
			SenderName:        strings.TrimSpace(data.SenderNick),
			ChatType:          chatType,
			Text:              content,
			Metadata: map[string]string{
				"conversation_title": strings.TrimSpace(data.ConversationTitle),
			},
		}),
	}); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = c.SendDeliveryMessage(ctx, *delivery, notice)
			}
			return []byte(""), nil
		}
		if strings.TrimSpace(data.SessionWebhook) != "" {
			if notifyErr := c.sendSessionWebhookText(ctx, data.SessionWebhook, "DingTalk 消息处理失败: "+TruncateError(err)); notifyErr == nil {
				return []byte(""), nil
			}
		}
		return nil, err
	}
	return []byte(""), nil
}

func (c *DingTalkChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *DingTalkChannel) clearAccessToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpiresAt = time.Time{}
}

func decodeDingTalkEnvelope(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("dingtalk request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err = json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

// DecodeDingTalkIngressCallback 将钉钉 HTTP/Webhook 机器人消息转换成统一通道入口请求。
func DecodeDingTalkIngressCallback(raw []byte) (*channelcontract.IngressRequest, string, error) {
	var payload struct {
		ConversationID     string `json:"conversationId"`
		OpenConversationID string `json:"openConversationId"`
		ConversationType   string `json:"conversationType"`
		ConversationTitle  string `json:"conversationTitle"`
		ChatbotCorpID      string `json:"chatbotCorpId"`
		SessionWebhook     string `json:"sessionWebhook"`
		SenderStaffID      string `json:"senderStaffId"`
		SenderID           string `json:"senderId"`
		SenderNick         string `json:"senderNick"`
		MsgID              string `json:"msgId"`
		MsgType            string `json:"msgtype"`
		Text               struct {
			Content string `json:"content"`
		} `json:"text"`
		Content any `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", err
	}
	content := strings.TrimSpace(payload.Text.Content)
	if content == "" {
		content = dingTalkContentText(payload.Content)
	}
	if content == "" {
		return nil, "empty_text", nil
	}
	ref := channelcontract.FirstNonEmpty(payload.OpenConversationID, payload.ConversationID, payload.SenderStaffID, payload.SenderID)
	if ref == "" {
		return nil, "empty_ref", nil
	}
	deliveryTo := channelcontract.FirstNonEmpty(payload.SessionWebhook, payload.OpenConversationID, payload.ConversationID, ref)
	return &channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeDingTalk,
		AccountID:    strings.TrimSpace(payload.ChatbotCorpID),
		ChatType:     normalizeDingTalkConversationType(payload.ConversationType),
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(payload.MsgID),
		ReqID:        strings.TrimSpace(payload.MsgID),
		ExternalName: channelcontract.FirstNonEmpty(payload.ConversationTitle, payload.SenderNick),
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeDingTalk,
			To:        deliveryTo,
			AccountID: strings.TrimSpace(payload.ChatbotCorpID),
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDingTalk,
			Target:            ref,
			PlatformMessageID: strings.TrimSpace(payload.MsgID),
			SenderID:          channelcontract.FirstNonEmpty(payload.SenderStaffID, payload.SenderID),
			SenderName:        strings.TrimSpace(payload.SenderNick),
			ChatType:          normalizeDingTalkConversationType(payload.ConversationType),
			Text:              content,
			Metadata: map[string]string{
				"conversation_title": strings.TrimSpace(payload.ConversationTitle),
			},
		}),
	}, "", nil
}

func dingTalkContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if text, ok := typed["content"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func normalizeDingTalkConversationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "single", "private", "private_chat", "dm":
		return "dm"
	default:
		return "group"
	}
}

func normalizeDingTalkBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "https://api.dingtalk.com"
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return strings.TrimRight(value, "/")
	}
	return value
}
