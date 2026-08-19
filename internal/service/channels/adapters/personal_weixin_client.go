package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"

	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

const (
	personalWeixinConfigCacheTTL       = 24 * time.Hour
	personalWeixinConfigFailureBackoff = 2 * time.Second
)

type PersonalWeixinIlinkClient struct {
	baseURL            string
	token              string
	botAgent           string
	ilinkAppID         string
	ilinkClientVersion string
	client             *http.Client
	configMu           sync.Mutex
	configCache        map[string]personalWeixinConfigCacheEntry
}

type personalWeixinConfigCacheEntry struct {
	typingTicket string
	expiresAt    time.Time
	nextRetryAt  time.Time
}

// PersonalWeixinAPIError 保留 iLink 的结构化业务错误，供投递层区分
// context_token 失效与限流、鉴权等不可安全降级的失败。
type PersonalWeixinAPIError struct {
	StatusCode int
	Ret        int
	ErrCode    int
	ErrMsg     string
}

func (e *PersonalWeixinAPIError) Error() string {
	if e == nil {
		return "personal weixin request failed"
	}
	return fmt.Sprintf(
		"personal weixin request failed: status=%d ret=%d errcode=%d errmsg=%s",
		e.StatusCode,
		e.Ret,
		e.ErrCode,
		strings.TrimSpace(e.ErrMsg),
	)
}

// IsPersonalWeixinContextExpired 只识别 iLink 明确返回的上下文失效码。
// HTTP 429 等传输/限流错误不能触发无 context_token 降级。
func IsPersonalWeixinContextExpired(err error) bool {
	var apiErr *PersonalWeixinAPIError
	if !errors.As(err, &apiErr) || apiErr == nil || apiErr.StatusCode == http.StatusTooManyRequests {
		return false
	}
	return apiErr.Ret == -2 || apiErr.ErrCode == -2
}

func NewPersonalWeixinIlinkClient(config PersonalWeixinClientConfig, client *http.Client) *PersonalWeixinIlinkClient {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &PersonalWeixinIlinkClient{
		baseURL:            normalizePersonalWeixinBaseURL(config.BaseURL),
		token:              strings.TrimSpace(config.Token),
		botAgent:           channelcontract.FirstNonEmpty(config.BotAgent, defaultPersonalWeixinBotAgent),
		ilinkAppID:         channelcontract.FirstNonEmpty(config.IlinkAppID, defaultPersonalWeixinAppID),
		ilinkClientVersion: channelcontract.FirstNonEmpty(config.IlinkClientVersion, defaultPersonalWeixinClientVersion),
		client:             client,
		configCache:        make(map[string]personalWeixinConfigCacheEntry),
	}
}

func (c *PersonalWeixinIlinkClient) StartQRCode(ctx context.Context, localTokens []string) (PersonalWeixinQRCodeResponse, error) {
	var response PersonalWeixinQRCodeResponse
	endpoint := "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(defaultPersonalWeixinBotType)
	body := map[string]any{
		"local_token_list": localTokens,
	}
	if err := c.post(ctx, endpoint, body, &response); err != nil {
		return PersonalWeixinQRCodeResponse{}, err
	}
	return response, nil
}

func (c *PersonalWeixinIlinkClient) PollQRCodeStatus(ctx context.Context, qrcode string, verifyCode string) (PersonalWeixinQRStatusResponse, error) {
	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(strings.TrimSpace(qrcode))
	if strings.TrimSpace(verifyCode) != "" {
		endpoint += "&verify_code=" + url.QueryEscape(strings.TrimSpace(verifyCode))
	}
	var response PersonalWeixinQRStatusResponse
	if err := c.get(ctx, endpoint, &response); err != nil {
		return PersonalWeixinQRStatusResponse{}, err
	}
	if response.Status == "scaned_but_redirect" && strings.TrimSpace(response.RedirectHost) != "" {
		c.baseURL = "https://" + strings.TrimSpace(response.RedirectHost)
	}
	return response, nil
}

func (c *PersonalWeixinIlinkClient) GetUpdates(
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

func (c *PersonalWeixinIlinkClient) GetConfig(
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

func (c *PersonalWeixinIlinkClient) TypingTicket(ctx context.Context, ilinkUserID string, contextToken string) (string, error) {
	ilinkUserID = strings.TrimSpace(ilinkUserID)
	if ilinkUserID == "" {
		return "", fmt.Errorf("personal weixin typing requires ilink_user_id")
	}
	now := time.Now()
	c.configMu.Lock()
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

func (c *PersonalWeixinIlinkClient) SendTyping(ctx context.Context, ilinkUserID string, typingTicket string, active bool) error {
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

func (c *PersonalWeixinIlinkClient) post(ctx context.Context, endpoint string, body any, target any) error {
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

func (c *PersonalWeixinIlinkClient) get(ctx context.Context, endpoint string, target any) error {
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

func (c *PersonalWeixinIlinkClient) urlFor(endpoint string) string {
	base := strings.TrimRight(c.baseURL, "/") + "/"
	return base + strings.TrimLeft(endpoint, "/")
}

func (c *PersonalWeixinIlinkClient) applyHeaders(request *http.Request, withAuth bool) {
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

func (c *PersonalWeixinIlinkClient) baseInfo() map[string]string {
	return map[string]string{
		"channel_version": "nexus",
		"bot_agent":       channelcontract.FirstNonEmpty(c.botAgent, defaultPersonalWeixinBotAgent),
	}
}

func readPersonalWeixinResponse(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	body = bytes.TrimSpace(body)
	var status personalWeixinAPIStatus
	if len(body) > 0 {
		_ = json.Unmarshal(body, &status)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || status.Ret != 0 || status.ErrCode != 0 {
		errMsg := strings.TrimSpace(status.ErrMsg)
		if errMsg == "" {
			errMsg = http.StatusText(response.StatusCode)
		}
		if errMsg == "" {
			errMsg = "unexpected response"
		}
		return &PersonalWeixinAPIError{
			StatusCode: response.StatusCode,
			Ret:        status.Ret,
			ErrCode:    status.ErrCode,
			ErrMsg:     errMsg,
		}
	}
	if len(body) == 0 {
		return nil
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(body, target)
}
