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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

type feishuChannel struct {
	appID             string
	appSecret         string
	client            *http.Client
	baseURL           string
	ownerUserID       string
	verificationToken string
	encryptKey        string
	connectionMode    string
	replyInThread     bool

	mu             sync.Mutex
	tenantToken    string
	tokenExpiresAt time.Time
	ingress        IngressAcceptor
	cancel         context.CancelFunc
	eventClient    feishuEventClient
	eventFactory   feishuEventClientFactory
	typingReacts   map[string]string
}

type feishuEventClient interface {
	Start(context.Context) error
	Close()
}

type feishuEventClientFactory func(feishuEventClientConfig) feishuEventClient

type feishuEventClientConfig struct {
	AppID             string
	AppSecret         string
	BaseURL           string
	VerificationToken string
	EncryptKey        string
	OnReady           func()
	OnError           func(error)
	OnMessage         func(context.Context, *larkim.P2MessageReceiveV1) error
	OnReaction        func(context.Context, *larkim.P2MessageReactionCreatedV1) error
}

type feishuTenantTokenEnvelope struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

type feishuMessageEnvelope struct {
	Code int                       `json:"code"`
	Msg  string                    `json:"msg"`
	Data feishuMessageResponseData `json:"data,omitempty"`
}

type feishuMessageResponseData struct {
	MessageID  string `json:"message_id,omitempty"`
	RootID     string `json:"root_id,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	ThreadID   string `json:"thread_id,omitempty"`
	ReactionID string `json:"reaction_id,omitempty"`
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
		appID:          strings.TrimSpace(appID),
		appSecret:      strings.TrimSpace(appSecret),
		client:         client,
		baseURL:        "https://open.feishu.cn",
		connectionMode: "websocket",
		eventFactory:   newFeishuSDKEventClient,
	}
}

func (c *feishuChannel) WithOwner(ownerUserID string) *feishuChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *feishuChannel) WithEventSecurity(verificationToken string, encryptKey string) *feishuChannel {
	c.verificationToken = strings.TrimSpace(verificationToken)
	c.encryptKey = strings.TrimSpace(encryptKey)
	return c
}

func (c *feishuChannel) WithConnectionMode(mode string) *feishuChannel {
	c.connectionMode = normalizeFeishuConnectionMode(mode)
	return c
}

func (c *feishuChannel) WithReplyInThread(value string) *feishuChannel {
	c.replyInThread = normalizeFeishuReplyInThread(value)
	return c
}

func (c *feishuChannel) ChannelType() string {
	return ChannelTypeFeishu
}

func (c *feishuChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *feishuChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" {
		return fmt.Errorf("feishu channel is not configured")
	}
	if normalizeFeishuConnectionMode(c.connectionMode) == "webhook" {
		return nil
	}

	ready := make(chan struct{}, 1)
	startErr := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	client := c.eventFactory(feishuEventClientConfig{
		AppID:             c.appID,
		AppSecret:         c.appSecret,
		BaseURL:           c.baseURL,
		VerificationToken: c.verificationToken,
		EncryptKey:        c.encryptKey,
		OnReady: func() {
			select {
			case ready <- struct{}{}:
			default:
			}
		},
		OnError: func(err error) {
			if err == nil {
				return
			}
			select {
			case startErr <- err:
			default:
			}
		},
		OnMessage:  c.handleSDKMessage,
		OnReaction: c.handleSDKReaction,
	})

	c.mu.Lock()
	if c.eventClient != nil {
		cancel()
		c.mu.Unlock()
		return nil
	}
	c.cancel = cancel
	c.eventClient = client
	c.mu.Unlock()

	go func() {
		if err := client.Start(runCtx); err != nil {
			select {
			case startErr <- err:
			default:
			}
		}
	}()

	select {
	case <-ready:
		return nil
	case err := <-startErr:
		c.clearEventClient(client)
		client.Close()
		cancel()
		return err
	case <-time.After(8 * time.Second):
		return nil
	case <-ctx.Done():
		c.clearEventClient(client)
		client.Close()
		cancel()
		return ctx.Err()
	}
}

func (c *feishuChannel) clearEventClient(client feishuEventClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventClient == client {
		c.eventClient = nil
		c.cancel = nil
	}
}

func (c *feishuChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	client := c.eventClient
	c.cancel = nil
	c.eventClient = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if client != nil {
		client.Close()
	}
	return nil
}

func (c *feishuChannel) handleSDKMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil {
		return nil
	}
	raw, err := feishuSDKEventRaw(event.EventReq, event)
	if err != nil {
		return err
	}
	callback, err := DecodeFeishuIngressCallback(raw)
	if err != nil {
		return err
	}
	return c.acceptDecodedIngress(ctx, callback)
}

func (c *feishuChannel) handleSDKReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	if event == nil {
		return nil
	}
	raw, err := feishuSDKEventRaw(event.EventReq, event)
	if err != nil {
		return err
	}
	callback, err := DecodeFeishuIngressCallback(raw)
	if err != nil {
		return err
	}
	return c.acceptDecodedIngress(ctx, callback)
}

func feishuSDKEventRaw(eventReq *larkevent.EventReq, fallback any) ([]byte, error) {
	if eventReq != nil && len(bytes.TrimSpace(eventReq.Body)) > 0 {
		return bytes.TrimSpace(eventReq.Body), nil
	}
	return json.Marshal(fallback)
}

func (c *feishuChannel) acceptDecodedIngress(ctx context.Context, callback FeishuIngressCallback) error {
	ingress := c.currentIngress()
	if ingress == nil || callback.Request == nil {
		return nil
	}
	callback.Request.OwnerUserID = c.ownerUserID
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, *callback.Request); err != nil {
		if isPairingApprovalRequired(err) {
			if callback.Request.Delivery != nil {
				if notice := pairingApprovalNoticeText(err); notice != "" {
					_, _ = c.SendDeliveryMessage(requestCtx, *callback.Request.Delivery, notice)
				}
			}
			return nil
		}
		return err
	}
	return nil
}

func (c *feishuChannel) currentIngress() IngressAcceptor {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ingress
}

func (c *feishuChannel) SendDeliveryMessage(ctx context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(target.To) == "" {
		return DeliveryResult{}, fmt.Errorf("feishu delivery target requires to")
	}
	token, err := c.tenantAccessToken(ctx)
	if err != nil {
		return DeliveryResult{}, err
	}
	receiveIDType := normalizeFeishuReceiveIDType(target.AccountID)
	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range splitText(strings.TrimSpace(text), 4500) {
		messageID := ""
		if strings.TrimSpace(target.ThreadID) != "" {
			messageID, err = c.replyTextChunk(ctx, token, target.ThreadID, chunk)
		} else {
			messageID, err = c.sendTextChunk(ctx, token, receiveIDType, target.To, chunk)
		}
		if err != nil {
			c.clearTenantAccessToken()
			return DeliveryResult{}, err
		}
		if strings.TrimSpace(messageID) != "" {
			parts = append(parts, channelmessage.TextPart(messageID))
		}
	}
	return newDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  ChannelTypeFeishu,
		Target:   target.To,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *feishuChannel) SendDeliveryTyping(ctx context.Context, target DeliveryTarget, active bool) error {
	messageID := strings.TrimSpace(target.ThreadID)
	if messageID == "" {
		return nil
	}
	token, err := c.tenantAccessToken(ctx)
	if err != nil {
		return err
	}
	if active {
		reactionID, err := c.addMessageReaction(ctx, token, messageID, "Typing")
		if err != nil {
			return nil
		}
		if strings.TrimSpace(reactionID) == "" {
			return nil
		}
		c.mu.Lock()
		if c.typingReacts == nil {
			c.typingReacts = make(map[string]string)
		}
		c.typingReacts[messageID] = reactionID
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	reactionID := ""
	if c.typingReacts != nil {
		reactionID = c.typingReacts[messageID]
		delete(c.typingReacts, messageID)
	}
	c.mu.Unlock()
	if reactionID == "" {
		return nil
	}
	_ = c.deleteMessageReaction(ctx, token, messageID, reactionID)
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

func (c *feishuChannel) sendTextChunk(ctx context.Context, token string, receiveIDType string, receiveID string, text string) (string, error) {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{
		"receive_id": strings.TrimSpace(receiveID),
		"msg_type":   "text",
		"content":    string(content),
	})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") +
		"/open-apis/im/v1/messages?receive_id_type=" +
		url.QueryEscape(receiveIDType)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	var envelope feishuMessageEnvelope
	if err = decodeFeishuEnvelope(response, &envelope); err != nil {
		return "", err
	}
	if envelope.Code != 0 {
		return "", fmt.Errorf("feishu send message failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return strings.TrimSpace(envelope.Data.MessageID), nil
}

func (c *feishuChannel) replyTextChunk(ctx context.Context, token string, messageID string, text string) (string, error) {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"msg_type": "text",
		"content":  string(content),
	}
	if c.replyInThread {
		payload["reply_in_thread"] = true
	}
	endpoint := strings.TrimRight(c.baseURL, "/") +
		"/open-apis/im/v1/messages/" +
		url.PathEscape(strings.TrimSpace(messageID)) +
		"/reply"
	var envelope feishuMessageEnvelope
	if err = c.doFeishuJSON(ctx, http.MethodPost, token, endpoint, payload, &envelope); err != nil {
		return "", err
	}
	if envelope.Code != 0 {
		return "", fmt.Errorf("feishu reply message failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return strings.TrimSpace(envelope.Data.MessageID), nil
}

func (c *feishuChannel) addMessageReaction(ctx context.Context, token string, messageID string, emojiType string) (string, error) {
	payload := map[string]any{
		"reaction_type": map[string]string{
			"emoji_type": strings.TrimSpace(emojiType),
		},
	}
	endpoint := strings.TrimRight(c.baseURL, "/") +
		"/open-apis/im/v1/messages/" +
		url.PathEscape(strings.TrimSpace(messageID)) +
		"/reactions"
	var envelope feishuMessageEnvelope
	if err := c.doFeishuJSON(ctx, http.MethodPost, token, endpoint, payload, &envelope); err != nil {
		return "", err
	}
	if envelope.Code != 0 {
		return "", fmt.Errorf("feishu add reaction failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return strings.TrimSpace(envelope.Data.ReactionID), nil
}

func (c *feishuChannel) deleteMessageReaction(ctx context.Context, token string, messageID string, reactionID string) error {
	endpoint := strings.TrimRight(c.baseURL, "/") +
		"/open-apis/im/v1/messages/" +
		url.PathEscape(strings.TrimSpace(messageID)) +
		"/reactions/" +
		url.PathEscape(strings.TrimSpace(reactionID))
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	var envelope feishuMessageEnvelope
	if err = decodeFeishuEnvelope(response, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("feishu delete reaction failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return nil
}

func (c *feishuChannel) doFeishuJSON(
	ctx context.Context,
	method string,
	token string,
	endpoint string,
	payload any,
	target any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	return decodeFeishuEnvelope(response, target)
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

func newFeishuSDKEventClient(config feishuEventClientConfig) feishuEventClient {
	sdkLogger := feishuSDKLogger{}
	dispatcher := larkdispatcher.NewEventDispatcher(config.VerificationToken, config.EncryptKey)
	dispatcher.InitConfig(larkevent.WithLogger(sdkLogger), larkevent.WithLogLevel(larkcore.LogLevelWarn))
	dispatcher.OnP2MessageReceiveV1(config.OnMessage)
	if config.OnReaction != nil {
		dispatcher.OnP2MessageReactionCreatedV1(config.OnReaction)
	}
	options := []larkws.ClientOption{
		larkws.WithEventHandler(dispatcher),
		larkws.WithLogLevel(larkcore.LogLevelWarn),
		larkws.WithLogger(sdkLogger),
		larkws.WithOnReady(config.OnReady),
		larkws.WithOnError(config.OnError),
	}
	if baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"); baseURL != "" {
		options = append(options, larkws.WithDomain(baseURL))
	}
	return larkws.NewClient(config.AppID, config.AppSecret, options...)
}

func normalizeFeishuConnectionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "webhook", "http", "callback":
		return "webhook"
	default:
		return "websocket"
	}
}

func normalizeFeishuReplyInThread(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled", "enable":
		return true
	default:
		return false
	}
}

type feishuSDKLogger struct{}

func (feishuSDKLogger) Debug(context.Context, ...interface{}) {}

func (feishuSDKLogger) Info(context.Context, ...interface{}) {}

func (feishuSDKLogger) Warn(ctx context.Context, args ...interface{}) {
	detail := formatFeishuSDKLog(args...)
	if detail == "" || isFeishuSDKExpectedCloseLog(detail) {
		return
	}
	slog.Default().WarnContext(ctx, "飞书 SDK 长连接警告", "detail", detail)
}

func (feishuSDKLogger) Error(ctx context.Context, args ...interface{}) {
	detail := formatFeishuSDKLog(args...)
	if detail == "" || isFeishuSDKExpectedCloseLog(detail) {
		return
	}
	slog.Default().ErrorContext(ctx, "飞书 SDK 长连接错误", "detail", detail)
}

func formatFeishuSDKLog(args ...interface{}) string {
	return strings.TrimSpace(fmt.Sprint(args...))
}

func isFeishuSDKExpectedCloseLog(detail string) bool {
	normalized := strings.ToLower(strings.TrimSpace(detail))
	return strings.Contains(normalized, "use of closed network connection") ||
		strings.Contains(normalized, "connection is closed, receive message loop exit") ||
		strings.Contains(normalized, "websocket: close 1000")
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

	eventType := strings.TrimSpace(firstNonEmpty(payload.Header.EventType, payload.Type))
	switch eventType {
	case "im.message.receive_v1":
		callback.Request = decodeFeishuMessageIngress(payload, &callback)
	case "im.message.reaction.created_v1":
		callback.Request = decodeFeishuReactionIngress(payload, &callback)
	case "im.message.message_read_v1", "im.message.reaction.deleted_v1":
		callback.IgnoredReason = "ignored_event_type"
	default:
		callback.IgnoredReason = "unsupported_event_type"
		return callback, nil
	}
	return callback, nil
}

func decodeFeishuMessageIngress(payload feishuEventCallbackPayload, callback *FeishuIngressCallback) *IngressRequest {
	if isFeishuBotSender(payload.Event.Sender.SenderType) {
		callback.IgnoredReason = "bot_message"
		return nil
	}
	message := payload.Event.Message
	if strings.TrimSpace(message.MessageID) == "" && strings.TrimSpace(message.ChatID) == "" {
		callback.IgnoredReason = "empty_message"
		return nil
	}
	content := feishuMessageText(message)
	if content == "" {
		callback.IgnoredReason = "empty_text"
		return nil
	}

	ref := strings.TrimSpace(message.ChatID)
	accountID := "chat_id"
	if ref == "" {
		ref, accountID = feishuSenderRef(payload.Event.Sender.SenderID)
	}
	if ref == "" {
		callback.IgnoredReason = "empty_ref"
		return nil
	}
	threadID := firstNonEmpty(message.ThreadID, message.RootID)
	reqID := firstNonEmpty(message.MessageID, payload.Header.EventID)
	chatType := normalizeFeishuChatType(message.ChatType)
	senderID, _ := feishuSenderRef(payload.Event.Sender.SenderID)

	return &IngressRequest{
		Channel:      ChannelTypeFeishu,
		ChatType:     chatType,
		Ref:          ref,
		ThreadID:     threadID,
		Content:      content,
		RoundID:      firstNonEmpty(payload.Header.EventID, message.MessageID),
		ReqID:        reqID,
		ExternalName: strings.TrimSpace(message.ChatID),
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeFeishu,
			To:        ref,
			AccountID: accountID,
			ThreadID:  strings.TrimSpace(message.MessageID),
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           ChannelTypeFeishu,
			Target:            ref,
			PlatformMessageID: reqID,
			ThreadID:          threadID,
			SenderID:          senderID,
			SenderName:        strings.TrimSpace(message.ChatID),
			ChatType:          chatType,
			Text:              content,
		}),
	}
}

func decodeFeishuReactionIngress(payload feishuEventCallbackPayload, callback *FeishuIngressCallback) *IngressRequest {
	emoji := strings.TrimSpace(payload.Event.ReactionType.EmojiType)
	messageID := strings.TrimSpace(payload.Event.MessageID)
	senderID, _ := feishuSenderRef(payload.Event.UserID)
	if emoji == "" || messageID == "" || senderID == "" {
		callback.IgnoredReason = "empty_reaction"
		return nil
	}
	if isFeishuBotSender(payload.Event.OperatorType) {
		callback.IgnoredReason = "bot_reaction"
		return nil
	}
	if strings.EqualFold(emoji, "Typing") {
		callback.IgnoredReason = "typing_reaction"
		return nil
	}

	ref := strings.TrimSpace(payload.Event.ChatID)
	accountID := "chat_id"
	if ref == "" {
		ref = senderID
		accountID = "open_id"
	}
	if ref == "" {
		callback.IgnoredReason = "empty_ref"
		return nil
	}

	threadID := firstNonEmpty(payload.Event.ThreadID, payload.Event.RootID)
	reqID := strings.Join([]string{
		messageID,
		"reaction",
		emoji,
		firstNonEmpty(payload.Header.EventID, payload.Event.ActionTime),
	}, ":")
	return &IngressRequest{
		Channel:      ChannelTypeFeishu,
		ChatType:     normalizeFeishuChatType(payload.Event.ChatType),
		Ref:          ref,
		ThreadID:     threadID,
		Content:      fmt.Sprintf("[reacted with %s to message %s]", emoji, messageID),
		RoundID:      firstNonEmpty(payload.Header.EventID, reqID),
		ReqID:        reqID,
		ExternalName: strings.TrimSpace(payload.Event.ChatID),
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeFeishu,
			To:        ref,
			AccountID: accountID,
			ThreadID:  messageID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           ChannelTypeFeishu,
			Target:            ref,
			PlatformMessageID: messageID,
			ThreadID:          threadID,
			SenderID:          senderID,
			SenderName:        strings.TrimSpace(payload.Event.ChatID),
			ChatType:          normalizeFeishuChatType(payload.Event.ChatType),
			Text:              fmt.Sprintf("[reacted with %s to message %s]", emoji, messageID),
			ReplyToID:         messageID,
			Metadata: map[string]string{
				"reaction": emoji,
				"event_id": firstNonEmpty(payload.Header.EventID, payload.Event.ActionTime),
			},
		}),
	}
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
	AppID        string              `json:"app_id"`
	Sender       feishuEventSender   `json:"sender"`
	Message      feishuEventMessage  `json:"message"`
	MessageID    string              `json:"message_id"`
	ChatID       string              `json:"chat_id"`
	ChatType     string              `json:"chat_type"`
	ThreadID     string              `json:"thread_id"`
	RootID       string              `json:"root_id"`
	ParentID     string              `json:"parent_id"`
	ReactionType feishuReactionType  `json:"reaction_type"`
	OperatorType string              `json:"operator_type"`
	UserID       feishuEventSenderID `json:"user_id"`
	ActionTime   string              `json:"action_time"`
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
	RootID      string `json:"root_id"`
	ParentID    string `json:"parent_id"`
	ThreadID    string `json:"thread_id"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	CreateTime  string `json:"create_time"`
}

type feishuReactionType struct {
	EmojiType string `json:"emoji_type"`
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
