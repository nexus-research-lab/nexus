package channels

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type feishuChannel struct {
	appID       string
	appSecret   string
	client      *http.Client
	baseURL     string
	ownerUserID string

	mu             sync.Mutex
	tenantToken    string
	tokenExpiresAt time.Time
}

type feishuTenantTokenEnvelope struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

type feishuMessageEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

var ErrFeishuCallbackUnauthorized = errors.New("feishu callback verification failed")

// FeishuCallbackSecurity 表示飞书事件订阅回调安全配置。
type FeishuCallbackSecurity struct {
	VerificationToken string
	EncryptKey        string
}

// FeishuIngressPreparation 表示通过通道配置校验后的飞书回调明文。
type FeishuIngressPreparation struct {
	Body        []byte
	OwnerUserID string
	AppID       string
}

// FeishuIngressCallback 表示飞书回调解析后的入站结果。
type FeishuIngressCallback struct {
	Challenge     string
	AppID         string
	Token         string
	Request       *IngressRequest
	IgnoredReason string
}

func newFeishuChannel(appID string, appSecret string, client *http.Client) *feishuChannel {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	return &feishuChannel{
		appID:     strings.TrimSpace(appID),
		appSecret: strings.TrimSpace(appSecret),
		client:    client,
		baseURL:   "https://open.feishu.cn",
	}
}

func (c *feishuChannel) WithOwner(ownerUserID string) *feishuChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *feishuChannel) ChannelType() string {
	return ChannelTypeFeishu
}

func (c *feishuChannel) Start(context.Context) error {
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" {
		return fmt.Errorf("feishu channel is not configured")
	}
	return nil
}

func (c *feishuChannel) Stop(context.Context) error {
	return nil
}

func (c *feishuChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("feishu delivery target requires to")
	}
	token, err := c.tenantAccessToken(ctx)
	if err != nil {
		return err
	}
	receiveIDType := normalizeFeishuReceiveIDType(target.AccountID)
	for _, chunk := range splitText(strings.TrimSpace(text), 4500) {
		if err = c.sendTextChunk(ctx, token, receiveIDType, target.To, chunk); err != nil {
			c.clearTenantAccessToken()
			return err
		}
	}
	return nil
}

func (c *feishuChannel) tenantAccessToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" {
		return "", fmt.Errorf("feishu channel is not configured")
	}
	now := time.Now()
	c.mu.Lock()
	if c.tenantToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.tenantToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	token, expiresAt, err := c.fetchTenantAccessToken(ctx, now)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tenantToken = token
	c.tokenExpiresAt = expiresAt
	c.mu.Unlock()
	return token, nil
}

func (c *feishuChannel) fetchTenantAccessToken(ctx context.Context, now time.Time) (string, time.Time, error) {
	payload, err := json.Marshal(map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", time.Time{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return "", time.Time{}, err
	}
	var envelope feishuTenantTokenEnvelope
	if err = decodeFeishuEnvelope(response, &envelope); err != nil {
		return "", time.Time{}, err
	}
	if envelope.Code != 0 {
		return "", time.Time{}, fmt.Errorf("feishu tenant_access_token failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	token := strings.TrimSpace(envelope.TenantAccessToken)
	if token == "" {
		return "", time.Time{}, fmt.Errorf("feishu tenant_access_token returned empty token")
	}
	expiresIn := envelope.Expire
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	if expiresIn > 600 {
		expiresIn -= 300
	}
	return token, now.Add(time.Duration(expiresIn) * time.Second), nil
}

func (c *feishuChannel) sendTextChunk(ctx context.Context, token string, receiveIDType string, receiveID string, text string) error {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"receive_id": strings.TrimSpace(receiveID),
		"msg_type":   "text",
		"content":    string(content),
	})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") +
		"/open-apis/im/v1/messages?receive_id_type=" +
		url.QueryEscape(receiveIDType)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	var envelope feishuMessageEnvelope
	if err = decodeFeishuEnvelope(response, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("feishu send message failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return nil
}

func (c *feishuChannel) clearTenantAccessToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tenantToken = ""
	c.tokenExpiresAt = time.Time{}
}

func decodeFeishuEnvelope(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("feishu request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err = json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

func normalizeFeishuReceiveIDType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "chat", "group", "chat_id":
		return "chat_id"
	case "open_id", "union_id", "user_id", "email":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return strings.TrimSpace(value)
	}
}

func feishuCallbackSignature(timestamp string, nonce string, encryptKey string, body []byte) string {
	hash := sha256.Sum256([]byte(timestamp + nonce + encryptKey + string(body)))
	return fmt.Sprintf("%x", hash[:])
}

func verifyFeishuCallbackSignature(raw []byte, header http.Header, encryptKey string) error {
	key := strings.TrimSpace(encryptKey)
	if key == "" {
		return nil
	}
	timestamp := strings.TrimSpace(header.Get("X-Lark-Request-Timestamp"))
	nonce := strings.TrimSpace(header.Get("X-Lark-Request-Nonce"))
	signature := strings.ToLower(strings.TrimSpace(header.Get("X-Lark-Signature")))
	if timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("%w: missing feishu signature headers", ErrFeishuCallbackUnauthorized)
	}
	expected := feishuCallbackSignature(timestamp, nonce, key, raw)
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
		return fmt.Errorf("%w: invalid feishu signature", ErrFeishuCallbackUnauthorized)
	}
	return nil
}

func verifyFeishuCallbackToken(callback FeishuIngressCallback, verificationToken string) error {
	expected := strings.TrimSpace(verificationToken)
	if expected == "" {
		return nil
	}
	actual := strings.TrimSpace(callback.Token)
	if actual == "" {
		return fmt.Errorf("%w: missing feishu verification token", ErrFeishuCallbackUnauthorized)
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("%w: invalid feishu verification token", ErrFeishuCallbackUnauthorized)
	}
	return nil
}

func feishuEncryptEnvelope(raw []byte) (string, bool, error) {
	var envelope struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", false, err
	}
	encrypt := strings.TrimSpace(envelope.Encrypt)
	return encrypt, encrypt != "", nil
}

func decryptFeishuCallback(raw []byte, encryptKey string) ([]byte, error) {
	encrypt, encrypted, err := feishuEncryptEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if !encrypted {
		return raw, nil
	}
	return decryptFeishuEncryptedPayload(encrypt, encryptKey)
}

func decryptFeishuEncryptedPayload(encrypt string, encryptKey string) ([]byte, error) {
	if strings.TrimSpace(encryptKey) == "" {
		return nil, fmt.Errorf("%w: missing feishu encrypt key", ErrFeishuCallbackUnauthorized)
	}
	buf, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypt))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid feishu encrypted payload", ErrFeishuCallbackUnauthorized)
	}
	if len(buf) < aes.BlockSize {
		return nil, fmt.Errorf("%w: feishu encrypted payload too short", ErrFeishuCallbackUnauthorized)
	}
	key := sha256.Sum256([]byte(strings.TrimSpace(encryptKey)))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	iv := buf[:aes.BlockSize]
	cipherText := append([]byte(nil), buf[aes.BlockSize:]...)
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: invalid feishu encrypted payload length", ErrFeishuCallbackUnauthorized)
	}
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(cipherText, cipherText)
	start := bytes.IndexByte(cipherText, '{')
	end := bytes.LastIndexByte(cipherText, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("%w: decrypted feishu payload is not json", ErrFeishuCallbackUnauthorized)
	}
	return bytes.TrimSpace(cipherText[start : end+1]), nil
}

// DecodeFeishuIngressCallback 将飞书事件订阅回调转换成统一通道入口请求。
func DecodeFeishuIngressCallback(raw []byte) (FeishuIngressCallback, error) {
	if _, encrypted, err := feishuEncryptEnvelope(raw); err == nil && encrypted {
		return FeishuIngressCallback{}, errors.New("encrypted feishu callback requires configured encrypt_key")
	}
	var payload feishuEventCallbackPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return FeishuIngressCallback{}, err
	}
	callback := FeishuIngressCallback{
		Challenge: strings.TrimSpace(payload.Challenge),
		AppID:     strings.TrimSpace(payload.Header.AppID),
		Token:     firstNonEmpty(payload.Header.Token, payload.Token),
	}
	if callback.AppID == "" {
		callback.AppID = strings.TrimSpace(payload.Event.AppID)
	}
	if callback.Challenge != "" || strings.EqualFold(strings.TrimSpace(payload.Type), "url_verification") {
		return callback, nil
	}

	eventType := firstNonEmpty(payload.Header.EventType, payload.Type)
	if eventType != "im.message.receive_v1" {
		callback.IgnoredReason = "unsupported_event_type"
		return callback, nil
	}
	if isFeishuBotSender(payload.Event.Sender.SenderType) {
		callback.IgnoredReason = "bot_message"
		return callback, nil
	}
	message := payload.Event.Message
	if strings.TrimSpace(message.MessageID) == "" && strings.TrimSpace(message.ChatID) == "" {
		callback.IgnoredReason = "empty_message"
		return callback, nil
	}
	content := feishuMessageText(message)
	if content == "" {
		callback.IgnoredReason = "empty_text"
		return callback, nil
	}

	ref := strings.TrimSpace(message.ChatID)
	accountID := "chat_id"
	if ref == "" {
		ref, accountID = feishuSenderRef(payload.Event.Sender.SenderID)
	}
	if ref == "" {
		callback.IgnoredReason = "empty_ref"
		return callback, nil
	}

	callback.Request = &IngressRequest{
		Channel:      ChannelTypeFeishu,
		ChatType:     normalizeFeishuChatType(message.ChatType),
		Ref:          ref,
		Content:      content,
		RoundID:      firstNonEmpty(payload.Header.EventID, message.MessageID),
		ReqID:        firstNonEmpty(message.MessageID, payload.Header.EventID),
		ExternalName: strings.TrimSpace(message.ChatID),
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeFeishu,
			To:        ref,
			AccountID: accountID,
		},
	}
	return callback, nil
}

type feishuEventCallbackPayload struct {
	Challenge string             `json:"challenge"`
	Token     string             `json:"token"`
	Type      string             `json:"type"`
	Header    feishuEventHeader  `json:"header"`
	Event     feishuEventPayload `json:"event"`
}

type feishuEventHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	AppID     string `json:"app_id"`
	Token     string `json:"token"`
}

type feishuEventPayload struct {
	AppID   string             `json:"app_id"`
	Sender  feishuEventSender  `json:"sender"`
	Message feishuEventMessage `json:"message"`
}

type feishuEventSender struct {
	SenderType string              `json:"sender_type"`
	SenderID   feishuEventSenderID `json:"sender_id"`
}

type feishuEventSenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

type feishuEventMessage struct {
	MessageID   string `json:"message_id"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

func feishuMessageText(message feishuEventMessage) string {
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(message.MessageType), "text") || strings.TrimSpace(message.MessageType) == "" {
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textPayload); err == nil && strings.TrimSpace(textPayload.Text) != "" {
			return strings.TrimSpace(textPayload.Text)
		}
	}
	return content
}

func feishuSenderRef(senderID feishuEventSenderID) (string, string) {
	if value := strings.TrimSpace(senderID.OpenID); value != "" {
		return value, "open_id"
	}
	if value := strings.TrimSpace(senderID.UserID); value != "" {
		return value, "user_id"
	}
	if value := strings.TrimSpace(senderID.UnionID); value != "" {
		return value, "union_id"
	}
	return "", ""
}

func normalizeFeishuChatType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "group", "chat":
		return "group"
	case "p2p", "private", "dm":
		return "dm"
	default:
		return "group"
	}
}

func isFeishuBotSender(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "app", "bot":
		return true
	default:
		return false
	}
}
