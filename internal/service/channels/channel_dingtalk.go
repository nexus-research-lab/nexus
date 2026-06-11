package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	dingchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

type dingTalkChannel struct {
	clientID     string
	clientSecret string
	robotCode    string
	client       *http.Client
	baseURL      string
	streamHost   string
	ownerUserID  string

	mu             sync.RWMutex
	ingress        IngressAcceptor
	accessToken    string
	tokenExpiresAt time.Time
	stream         *dingclient.StreamClient
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

func newDingTalkChannel(clientID string, clientSecret string, robotCode string, client *http.Client) *dingTalkChannel {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	return &dingTalkChannel{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		robotCode:    strings.TrimSpace(robotCode),
		client:       client,
		baseURL:      "https://api.dingtalk.com",
		streamHost:   "https://api.dingtalk.com",
	}
}

func (c *dingTalkChannel) WithOwner(ownerUserID string) *dingTalkChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *dingTalkChannel) ChannelType() string {
	return ChannelTypeDingTalk
}

func (c *dingTalkChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *dingTalkChannel) Start(ctx context.Context) error {
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

func (c *dingTalkChannel) Stop(context.Context) error {
	c.mu.Lock()
	stream := c.stream
	c.stream = nil
	c.mu.Unlock()
	if stream != nil {
		stream.Close()
	}
	return nil
}

func (c *dingTalkChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("dingtalk delivery target requires to")
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "http://") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "https://") {
		return c.sendSessionWebhookText(ctx, target.To, text)
	}
	if strings.TrimSpace(c.robotCode) == "" {
		return fmt.Errorf("dingtalk delivery requires robot_code")
	}
	token, err := c.accessTokenForDelivery(ctx)
	if err != nil {
		return err
	}
	for _, chunk := range splitText(strings.TrimSpace(text), 3800) {
		if err = c.sendGroupTextChunk(ctx, token, target.To, chunk); err != nil {
			c.clearAccessToken()
			return err
		}
	}
	return nil
}

func (c *dingTalkChannel) accessTokenForDelivery(ctx context.Context) (string, error) {
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

func (c *dingTalkChannel) sendGroupTextChunk(ctx context.Context, token string, conversationID string, text string) error {
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

func (c *dingTalkChannel) sendSessionWebhookText(ctx context.Context, webhook string, text string) error {
	for _, chunk := range splitText(strings.TrimSpace(text), 3800) {
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
		if err = expectSuccess(response); err != nil {
			return err
		}
	}
	return nil
}

func (c *dingTalkChannel) handleStreamMessage(ctx context.Context, data *dingchatbot.BotCallbackDataModel) ([]byte, error) {
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
	ref := firstNonEmpty(data.ConversationId, data.SenderStaffId, data.SenderId)
	if ref == "" {
		return nil, nil
	}
	deliveryTo := strings.TrimSpace(data.ConversationId)
	if deliveryTo == "" {
		deliveryTo = ref
	}
	delivery := &DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeDingTalk,
		To:        deliveryTo,
		AccountID: strings.TrimSpace(data.ChatbotCorpId),
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, IngressRequest{
		Channel:      ChannelTypeDingTalk,
		OwnerUserID:  c.ownerUserID,
		ChatType:     chatType,
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(data.MsgId),
		ReqID:        strings.TrimSpace(data.MsgId),
		ExternalName: firstNonEmpty(data.ConversationTitle, data.SenderNick),
		Delivery:     delivery,
	}); err != nil {
		if strings.TrimSpace(data.SessionWebhook) != "" {
			_ = c.sendSessionWebhookText(ctx, data.SessionWebhook, "DingTalk 消息处理失败: "+truncateChannelError(err))
		}
		return nil, err
	}
	return []byte(""), nil
}

func (c *dingTalkChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *dingTalkChannel) clearAccessToken() {
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
func DecodeDingTalkIngressCallback(raw []byte) (*IngressRequest, string, error) {
	var payload struct {
		ConversationID     string `json:"conversationId"`
		OpenConversationID string `json:"openConversationId"`
		ConversationType   string `json:"conversationType"`
		ConversationTitle  string `json:"conversationTitle"`
		ChatbotCorpID      string `json:"chatbotCorpId"`
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
	ref := firstNonEmpty(payload.OpenConversationID, payload.ConversationID, payload.SenderStaffID, payload.SenderID)
	if ref == "" {
		return nil, "empty_ref", nil
	}
	deliveryTo := firstNonEmpty(payload.OpenConversationID, payload.ConversationID, ref)
	return &IngressRequest{
		Channel:      ChannelTypeDingTalk,
		ChatType:     normalizeDingTalkConversationType(payload.ConversationType),
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(payload.MsgID),
		ReqID:        strings.TrimSpace(payload.MsgID),
		ExternalName: firstNonEmpty(payload.ConversationTitle, payload.SenderNick),
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeDingTalk,
			To:        deliveryTo,
			AccountID: strings.TrimSpace(payload.ChatbotCorpID),
		},
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
