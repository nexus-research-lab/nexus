package channels

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPersonalWeixinBaseURL       = "https://ilinkai.weixin.qq.com"
	defaultPersonalWeixinBotType       = "3"
	defaultPersonalWeixinAppID         = "bot"
	defaultPersonalWeixinClientVersion = "132099"
	defaultPersonalWeixinBotAgent      = "Nexus/0.1.0"

	personalWeixinMessageTypeUser = 1
	personalWeixinMessageTypeBot  = 2
	personalWeixinMessageStateEnd = 2
	personalWeixinItemTypeText    = 1
	personalWeixinTypingActive    = 1
	personalWeixinTypingCancel    = 2

	personalWeixinConfigCacheTTL       = 24 * time.Hour
	personalWeixinConfigFailureBackoff = 2 * time.Second
)

type personalWeixinChannel struct {
	token       string
	accountID   string
	userID      string
	ownerUserID string
	client      *personalWeixinIlinkClient

	mu      sync.RWMutex
	ingress IngressAcceptor
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type personalWeixinClientConfig struct {
	BaseURL            string
	Token              string
	AccountID          string
	UserID             string
	BotAgent           string
	IlinkAppID         string
	IlinkClientVersion string
}

type personalWeixinIlinkClient struct {
	baseURL            string
	token              string
	botAgent           string
	ilinkAppID         string
	ilinkClientVersion string
	client             *http.Client
	configMu           sync.Mutex
	configCache        map[string]personalWeixinConfigCacheEntry
}

type weixinQRCodeResponse struct {
	QRCode             string `json:"qrcode"`
	QRCodeImageContent string `json:"qrcode_img_content"`
}

type weixinQRStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token,omitempty"`
	IlinkBotID   string `json:"ilink_bot_id,omitempty"`
	BaseURL      string `json:"baseurl,omitempty"`
	IlinkUserID  string `json:"ilink_user_id,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}

type personalWeixinGetUpdatesResponse struct {
	Ret                  int                     `json:"ret,omitempty"`
	ErrCode              int                     `json:"errcode,omitempty"`
	ErrMsg               string                  `json:"errmsg,omitempty"`
	Messages             []personalWeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf        string                  `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMS int                     `json:"longpolling_timeout_ms,omitempty"`
}

type personalWeixinConfigResponse struct {
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

type personalWeixinAPIStatus struct {
	Ret     int    `json:"ret,omitempty"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type personalWeixinConfigCacheEntry struct {
	typingTicket string
	expiresAt    time.Time
	nextRetryAt  time.Time
}

type personalWeixinMessage struct {
	Seq          int64                       `json:"seq,omitempty"`
	MessageID    int64                       `json:"message_id,omitempty"`
	FromUserID   string                      `json:"from_user_id,omitempty"`
	ToUserID     string                      `json:"to_user_id,omitempty"`
	ClientID     string                      `json:"client_id,omitempty"`
	CreateTimeMS int64                       `json:"create_time_ms,omitempty"`
	SessionID    string                      `json:"session_id,omitempty"`
	GroupID      string                      `json:"group_id,omitempty"`
	MessageType  int                         `json:"message_type,omitempty"`
	MessageState int                         `json:"message_state,omitempty"`
	ItemList     []personalWeixinMessageItem `json:"item_list,omitempty"`
	ContextToken string                      `json:"context_token,omitempty"`
}

type personalWeixinMessageItem struct {
	Type     int                    `json:"type,omitempty"`
	TextItem personalWeixinTextItem `json:"text_item,omitempty"`
	RefMsg   *personalWeixinRefMsg  `json:"ref_msg,omitempty"`
}

type personalWeixinTextItem struct {
	Text string `json:"text,omitempty"`
}

type personalWeixinRefMsg struct {
	Title string `json:"title,omitempty"`
}

func newPersonalWeixinChannel(config personalWeixinClientConfig, client *http.Client) *personalWeixinChannel {
	ilinkClient := newPersonalWeixinIlinkClient(config, client)
	return &personalWeixinChannel{
		token:     strings.TrimSpace(config.Token),
		accountID: strings.TrimSpace(config.AccountID),
		userID:    strings.TrimSpace(config.UserID),
		client:    ilinkClient,
	}
}

func newPersonalWeixinIlinkClient(config personalWeixinClientConfig, client *http.Client) *personalWeixinIlinkClient {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	return &personalWeixinIlinkClient{
		baseURL:            normalizePersonalWeixinBaseURL(config.BaseURL),
		token:              strings.TrimSpace(config.Token),
		botAgent:           firstNonEmpty(config.BotAgent, defaultPersonalWeixinBotAgent),
		ilinkAppID:         firstNonEmpty(config.IlinkAppID, defaultPersonalWeixinAppID),
		ilinkClientVersion: firstNonEmpty(config.IlinkClientVersion, defaultPersonalWeixinClientVersion),
		client:             client,
		configCache:        make(map[string]personalWeixinConfigCacheEntry),
	}
}

func (c *personalWeixinChannel) WithOwner(ownerUserID string) *personalWeixinChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *personalWeixinChannel) ChannelType() string {
	return ChannelTypeWeixinPersonal
}

func (c *personalWeixinChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *personalWeixinChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.token) == "" {
		return nil
	}
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	c.mu.Unlock()

	go c.pollUpdates(runCtx)
	return nil
}

func (c *personalWeixinChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *personalWeixinChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("personal weixin channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("personal weixin delivery target requires to")
	}
	for _, chunk := range splitText(strings.TrimSpace(text), 4000) {
		request := map[string]any{
			"base_info": c.client.baseInfo(),
			"msg": personalWeixinMessage{
				FromUserID:   "",
				ToUserID:     strings.TrimSpace(target.To),
				ClientID:     newDeliveryID("weixin"),
				MessageType:  personalWeixinMessageTypeBot,
				MessageState: personalWeixinMessageStateEnd,
				ContextToken: strings.TrimSpace(target.ThreadID),
				ItemList: []personalWeixinMessageItem{{
					Type: personalWeixinItemTypeText,
					TextItem: personalWeixinTextItem{
						Text: chunk,
					},
				}},
			},
		}
		if err := c.client.post(ctx, "ilink/bot/sendmessage", request, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *personalWeixinChannel) SendDeliveryTyping(ctx context.Context, target DeliveryTarget, active bool) error {
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("personal weixin channel is not configured")
	}
	to := strings.TrimSpace(target.To)
	if to == "" {
		return fmt.Errorf("personal weixin typing target requires to")
	}
	ticket, err := c.client.TypingTicket(ctx, to, strings.TrimSpace(target.ThreadID))
	if err != nil {
		return err
	}
	if strings.TrimSpace(ticket) == "" {
		return nil
	}
	return c.client.SendTyping(ctx, to, ticket, active)
}

func (c *personalWeixinChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()
	getUpdatesBuf := ""
	nextTimeout := 35 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		response, err := c.client.GetUpdates(ctx, getUpdatesBuf, nextTimeout)
		if err != nil {
			if waitChannelLoginRetry(ctx, 2*time.Second) {
				continue
			}
			return
		}
		if response.LongPollingTimeoutMS > 0 {
			nextTimeout = time.Duration(response.LongPollingTimeoutMS) * time.Millisecond
		}
		if response.Ret != 0 || response.ErrCode != 0 {
			if waitChannelLoginRetry(ctx, 5*time.Second) {
				continue
			}
			return
		}
		if strings.TrimSpace(response.GetUpdatesBuf) != "" {
			getUpdatesBuf = response.GetUpdatesBuf
		}
		for _, message := range response.Messages {
			c.handleMessage(ctx, message)
		}
	}
}

func (c *personalWeixinChannel) handleMessage(ctx context.Context, message personalWeixinMessage) {
	if message.MessageType == personalWeixinMessageTypeBot {
		return
	}
	fromUserID := strings.TrimSpace(message.FromUserID)
	if fromUserID == "" {
		return
	}
	content := personalWeixinTextContent(message)
	if content == "" {
		return
	}
	ingress := c.currentIngress()
	if ingress == nil {
		return
	}
	delivery := &DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeWeixinPersonal,
		To:        fromUserID,
		AccountID: c.accountID,
		ThreadID:  strings.TrimSpace(message.ContextToken),
	}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, IngressRequest{
		Channel:      ChannelTypeWeixinPersonal,
		OwnerUserID:  c.ownerUserID,
		ChatType:     "dm",
		Ref:          fromUserID,
		ExternalName: fromUserID,
		Content:      content,
		Delivery:     delivery,
	}); err != nil {
		_ = c.SendDeliveryText(requestCtx, *delivery, "⚠️ 个人微信消息处理失败: "+truncateChannelError(err))
	}
}

func (c *personalWeixinChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *personalWeixinIlinkClient) StartQRCode(ctx context.Context, localTokens []string) (weixinQRCodeResponse, error) {
	var response weixinQRCodeResponse
	endpoint := "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(defaultPersonalWeixinBotType)
	body := map[string]any{
		"local_token_list": localTokens,
	}
	if err := c.post(ctx, endpoint, body, &response); err != nil {
		return weixinQRCodeResponse{}, err
	}
	return response, nil
}

func (c *personalWeixinIlinkClient) PollQRCodeStatus(ctx context.Context, qrcode string, verifyCode string) (weixinQRStatusResponse, error) {
	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(strings.TrimSpace(qrcode))
	if strings.TrimSpace(verifyCode) != "" {
		endpoint += "&verify_code=" + url.QueryEscape(strings.TrimSpace(verifyCode))
	}
	var response weixinQRStatusResponse
	if err := c.get(ctx, endpoint, &response); err != nil {
		return weixinQRStatusResponse{}, err
	}
	if response.Status == "scaned_but_redirect" && strings.TrimSpace(response.RedirectHost) != "" {
		c.baseURL = "https://" + strings.TrimSpace(response.RedirectHost)
	}
	return response, nil
}

func (c *personalWeixinIlinkClient) GetUpdates(
	ctx context.Context,
	getUpdatesBuf string,
	timeout time.Duration,
) (personalWeixinGetUpdatesResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	body := map[string]any{
		"get_updates_buf": strings.TrimSpace(getUpdatesBuf),
		"base_info":       c.baseInfo(),
	}
	var response personalWeixinGetUpdatesResponse
	if err := c.post(requestCtx, "ilink/bot/getupdates", body, &response); err != nil {
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return personalWeixinGetUpdatesResponse{Messages: nil, GetUpdatesBuf: getUpdatesBuf}, nil
		}
		return personalWeixinGetUpdatesResponse{}, err
	}
	return response, nil
}

func (c *personalWeixinIlinkClient) GetConfig(
	ctx context.Context,
	ilinkUserID string,
	contextToken string,
) (personalWeixinConfigResponse, error) {
	body := map[string]any{
		"ilink_user_id": strings.TrimSpace(ilinkUserID),
		"base_info":     c.baseInfo(),
	}
	if strings.TrimSpace(contextToken) != "" {
		body["context_token"] = strings.TrimSpace(contextToken)
	}
	var response personalWeixinConfigResponse
	if err := c.post(ctx, "ilink/bot/getconfig", body, &response); err != nil {
		return personalWeixinConfigResponse{}, err
	}
	return response, nil
}

func (c *personalWeixinIlinkClient) TypingTicket(ctx context.Context, ilinkUserID string, contextToken string) (string, error) {
	ilinkUserID = strings.TrimSpace(ilinkUserID)
	if ilinkUserID == "" {
		return "", fmt.Errorf("personal weixin typing requires ilink_user_id")
	}
	now := time.Now()
	c.configMu.Lock()
	if c.configCache == nil {
		c.configCache = make(map[string]personalWeixinConfigCacheEntry)
	}
	if entry, ok := c.configCache[ilinkUserID]; ok {
		if strings.TrimSpace(entry.typingTicket) != "" && now.Before(entry.expiresAt) {
			ticket := entry.typingTicket
			c.configMu.Unlock()
			return ticket, nil
		}
		if strings.TrimSpace(entry.typingTicket) == "" && now.Before(entry.nextRetryAt) {
			c.configMu.Unlock()
			return "", nil
		}
	}
	c.configMu.Unlock()

	response, err := c.GetConfig(ctx, ilinkUserID, contextToken)
	now = time.Now()
	c.configMu.Lock()
	defer c.configMu.Unlock()
	if err != nil {
		c.configCache[ilinkUserID] = personalWeixinConfigCacheEntry{
			nextRetryAt: now.Add(personalWeixinConfigFailureBackoff),
		}
		return "", nil
	}
	if response.Ret != 0 {
		c.configCache[ilinkUserID] = personalWeixinConfigCacheEntry{
			nextRetryAt: now.Add(personalWeixinConfigFailureBackoff),
		}
		return "", nil
	}
	ticket := strings.TrimSpace(response.TypingTicket)
	c.configCache[ilinkUserID] = personalWeixinConfigCacheEntry{
		typingTicket: ticket,
		expiresAt:    now.Add(personalWeixinConfigCacheTTL),
		nextRetryAt:  now.Add(personalWeixinConfigFailureBackoff),
	}
	return ticket, nil
}

func (c *personalWeixinIlinkClient) SendTyping(ctx context.Context, ilinkUserID string, typingTicket string, active bool) error {
	status := personalWeixinTypingCancel
	if active {
		status = personalWeixinTypingActive
	}
	body := map[string]any{
		"ilink_user_id": strings.TrimSpace(ilinkUserID),
		"typing_ticket": strings.TrimSpace(typingTicket),
		"status":        status,
		"base_info":     c.baseInfo(),
	}
	return c.post(ctx, "ilink/bot/sendtyping", body, nil)
}

func (c *personalWeixinIlinkClient) post(ctx context.Context, endpoint string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.urlFor(endpoint),
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	c.applyHeaders(request, true)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	return readPersonalWeixinResponse(response, target)
}

func (c *personalWeixinIlinkClient) get(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlFor(endpoint), nil)
	if err != nil {
		return err
	}
	c.applyHeaders(request, false)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	return readPersonalWeixinResponse(response, target)
}

func (c *personalWeixinIlinkClient) urlFor(endpoint string) string {
	base := strings.TrimRight(c.baseURL, "/") + "/"
	return base + strings.TrimLeft(endpoint, "/")
}

func (c *personalWeixinIlinkClient) applyHeaders(request *http.Request, withAuth bool) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("iLink-App-Id", c.ilinkAppID)
	request.Header.Set("iLink-App-ClientVersion", c.ilinkClientVersion)
	if withAuth {
		request.Header.Set("AuthorizationType", "ilink_bot_token")
		request.Header.Set("X-WECHAT-UIN", randomPersonalWeixinUIN())
		if strings.TrimSpace(c.token) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.token))
		}
	}
}

func (c *personalWeixinIlinkClient) baseInfo() map[string]string {
	return map[string]string{
		"channel_version": "nexus",
		"bot_agent":       firstNonEmpty(c.botAgent, defaultPersonalWeixinBotAgent),
	}
}

func readPersonalWeixinResponse(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("personal weixin request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var status personalWeixinAPIStatus
	if err := json.Unmarshal(body, &status); err == nil && (status.Ret != 0 || status.ErrCode != 0) {
		return fmt.Errorf(
			"personal weixin request failed: ret=%d errcode=%d errmsg=%s",
			status.Ret,
			status.ErrCode,
			strings.TrimSpace(status.ErrMsg),
		)
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(body, target)
}

func personalWeixinTextContent(message personalWeixinMessage) string {
	parts := make([]string, 0, len(message.ItemList))
	for _, item := range message.ItemList {
		if item.Type != personalWeixinItemTypeText {
			continue
		}
		if text := strings.TrimSpace(item.TextItem.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func normalizePersonalWeixinBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultPersonalWeixinBaseURL
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	return strings.TrimRight(value, "/")
}

func randomPersonalWeixinUIN() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	}
	value := binary.BigEndian.Uint32(buffer)
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(value), 10)))
}
