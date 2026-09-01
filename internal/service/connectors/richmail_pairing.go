// INPUT: owner、固定 RichMail loopback 服务与本机客户端返回的短期配对状态。
// OUTPUT: opaque owner-bound 配对会话、配置版本 CAS 连接写入及 RichMail tools/list 快照。
// POS: RichMail 固定 Connector 的唯一配对协议边界；不接受用户自定义主机、重定向或明文 Token 回传。
package connectors

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	richMailConnectorID              = "richmail"
	richMailDefaultBaseURL           = "http://127.0.0.1:3100"
	richMailDefaultMCPURL            = richMailDefaultBaseURL + "/mcp"
	richMailPairingAttemptPrefix     = "nexus-richmail-pairing:"
	richMailPairingAttemptVersion    = 1
	richMailPairingDefaultExpiresIn  = 10 * 60
	richMailPairingMaximumExpiresIn  = 30 * 60
	richMailPairingDefaultInterval   = 2
	richMailPairingMaximumInterval   = 10
	richMailPairingMaximumBodyLength = 64 * 1024
	richMailPairingMaximumRequestID  = 4 * 1024
	richMailPairingMaximumToken      = 16 * 1024
	richMailPairingHTTPTimeout       = 8 * time.Second
)

type richMailPairingAttempt struct {
	Version                      int       `json:"version"`
	AttemptID                    string    `json:"attempt_id"`
	OwnerUserID                  string    `json:"owner_user_id"`
	ConnectorID                  string    `json:"connector_id"`
	ProviderRequestID            string    `json:"provider_request_id"`
	ExpectedConfigurationVersion int64     `json:"expected_configuration_version"`
	ExpiresAt                    time.Time `json:"expires_at"`
}

type richMailPairingProviderResult struct {
	Status string
	Token  string
}

// ErrLocalPairingUnavailable 表示本机客户端或其配对协议当前不可访问、不可解析。
var ErrLocalPairingUnavailable = errors.New("本机 Connector 配对服务不可用")

// StartLocalPairing 请求本机 RichMail 创建一次需要用户在客户端内批准的配对。
func (s *Service) StartLocalPairing(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*LocalPairingStartResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, err := requireRichMailConnector(connectorID)
	if err != nil {
		return nil, err
	}
	state, err := s.GetConfigurationState(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	providerRequestID, expiresIn, interval, err := s.requestRichMailPairing(ctx)
	if err != nil {
		return nil, err
	}
	attemptID, err := newRichMailPairingAttemptID()
	if err != nil {
		return nil, err
	}
	attempt := richMailPairingAttempt{
		Version:                      richMailPairingAttemptVersion,
		AttemptID:                    attemptID,
		OwnerUserID:                  ownerUserID,
		ConnectorID:                  entry.ConnectorID,
		ProviderRequestID:            providerRequestID,
		ExpectedConfigurationVersion: state.ConfigurationVersion,
		ExpiresAt:                    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
	}
	attemptToken, err := s.encryptRichMailPairingAttempt(attempt)
	if err != nil {
		return nil, err
	}
	return &LocalPairingStartResult{
		ConnectorID:  entry.ConnectorID,
		AttemptToken: richMailPairingAttemptPrefix + attemptToken,
		Endpoint:     entry.MCPServerURL,
		ExpiresIn:    expiresIn,
		Interval:     interval,
	}, nil
}

// PollLocalPairing 读取 RichMail 审批结果；成功时以启动版本 CAS 保存 Token。
// ACK 丢失后的相同 attempt 重试只在连接中存在同一 attempt_id 时返回已连接。
func (s *Service) PollLocalPairing(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	attemptToken string,
) (*LocalPairingPollResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, err := requireRichMailConnector(connectorID)
	if err != nil {
		return nil, err
	}
	attempt, err := s.openRichMailPairingAttempt(
		ctx, ownerUserID, entry, attemptToken,
	)
	if err != nil {
		return nil, err
	}
	state, err := s.GetConfigurationState(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	if state.ConfigurationVersion != attempt.ExpectedConfigurationVersion {
		return s.reconcileCompletedRichMailPairing(ctx, ownerUserID, entry, attempt)
	}
	providerResult, err := s.pollRichMailPairing(ctx, attempt.ProviderRequestID)
	if err != nil {
		return nil, err
	}
	switch providerResult.Status {
	case localPairingStatusPending:
		return &LocalPairingPollResult{
			Status:  localPairingStatusPending,
			Message: "等待在 RichMail 中批准连接",
		}, nil
	case localPairingStatusDenied:
		return &LocalPairingPollResult{
			Status:  localPairingStatusDenied,
			Message: "RichMail 未批准本次连接",
		}, nil
	case localPairingStatusExpired:
		return &LocalPairingPollResult{
			Status:  localPairingStatusExpired,
			Message: "RichMail 配对请求已过期",
		}, nil
	case localPairingStatusConnected:
		if strings.TrimSpace(providerResult.Token) == "" {
			return nil, fmt.Errorf("%w: RichMail 已批准连接但未返回 Token", ErrLocalPairingUnavailable)
		}
	default:
		return nil, fmt.Errorf("%w: RichMail 返回了未知配对状态", ErrLocalPairingUnavailable)
	}

	credentials, err := json.Marshal(map[string]string{
		"token":              providerResult.Token,
		"pairing_attempt_id": attempt.AttemptID,
	})
	if err != nil {
		return nil, err
	}
	if _, err = s.upsertConnectionAtVersion(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: string(credentials),
		AuthType:    entry.AuthType,
	}, &attempt.ExpectedConfigurationVersion); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &LocalPairingPollResult{
		Status:    localPairingStatusConnected,
		Message:   "RichMail 已连接",
		Connector: &info,
	}, nil
}

func requireRichMailConnector(connectorID string) (CatalogEntry, error) {
	entry, ok := getConnector(strings.TrimSpace(connectorID))
	if !ok {
		return CatalogEntry{}, errors.New("未知连接器")
	}
	if entry.ConnectorID != richMailConnectorID || entry.AuthType != "local_pairing" {
		return CatalogEntry{}, errors.New("当前连接器不支持本机配对")
	}
	if entry.Status != "available" {
		return CatalogEntry{}, errors.New("连接器暂不可用")
	}
	return entry, nil
}

func (s *Service) reconcileCompletedRichMailPairing(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	attempt richMailPairingAttempt,
) (*LocalPairingPollResult, error) {
	snapshot, err := s.LoadActiveConnection(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.Extra["pairing_attempt_id"] != attempt.AttemptID {
		return nil, ErrConfigurationConflict
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &LocalPairingPollResult{
		Status:    localPairingStatusConnected,
		Message:   "RichMail 已连接",
		Connector: &info,
	}, nil
}

func (s *Service) requestRichMailPairing(
	ctx context.Context,
) (requestID string, expiresIn int, interval int, err error) {
	payload, _, statusCode, err := s.doRichMailPairingRequest(
		ctx, http.MethodPost, "/mcp/auth/request", nil,
	)
	if err != nil {
		return "", 0, 0, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return "", 0, 0, richMailPairingHTTPError(statusCode, false)
	}
	requestID = richMailFirstString(payload, "requestId", "request_id")
	if requestID == "" || len(requestID) > richMailPairingMaximumRequestID ||
		strings.ContainsAny(requestID, "\r\n") {
		return "", 0, 0, fmt.Errorf("%w: RichMail 未返回配对请求 ID", ErrLocalPairingUnavailable)
	}
	expiresIn = normalizedRichMailPairingExpiresIn(
		richMailFirstInt(payload, "expiresIn", "expires_in"),
	)
	interval = normalizedRichMailPairingInterval(
		richMailFirstInt(payload, "interval", "pollInterval", "poll_interval"),
	)
	return requestID, expiresIn, interval, nil
}

func (s *Service) pollRichMailPairing(
	ctx context.Context,
	requestID string,
) (richMailPairingProviderResult, error) {
	payload, headers, statusCode, err := s.doRichMailPairingRequest(
		ctx,
		http.MethodGet,
		"/mcp/auth/poll",
		url.Values{"requestId": []string{requestID}},
	)
	if err != nil {
		return richMailPairingProviderResult{}, err
	}
	if statusCode == http.StatusAccepted {
		return richMailPairingProviderResult{Status: localPairingStatusPending}, nil
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		status := richMailPairingStatusForHTTP(statusCode)
		if status != "" {
			return richMailPairingProviderResult{Status: status}, nil
		}
		return richMailPairingProviderResult{}, richMailPairingHTTPError(statusCode, true)
	}

	status := normalizedRichMailPairingStatus(
		richMailFirstString(payload, "status", "state"),
	)
	token := richMailBearerToken(payload, headers)
	if status == "" && token != "" {
		status = localPairingStatusConnected
	}
	if status == "" {
		return richMailPairingProviderResult{}, fmt.Errorf("%w: RichMail 配对响应缺少状态", ErrLocalPairingUnavailable)
	}
	return richMailPairingProviderResult{
		Status: status,
		Token:  token,
	}, nil
}

func (s *Service) doRichMailPairingRequest(
	ctx context.Context,
	method string,
	requestPath string,
	query url.Values,
) (map[string]any, http.Header, int, error) {
	baseURL, err := validateRichMailBaseURL(s.richMailBaseURL)
	if err != nil {
		return nil, nil, 0, err
	}
	target := *baseURL
	target.Path = requestPath
	target.RawQuery = query.Encode()
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader("{}")
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	client := *s.httpClient
	client.Timeout = richMailPairingHTTPTimeout
	if client.Transport == nil {
		client.Transport = richMailLoopbackTransport()
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: 无法连接本机 RichMail，请确认客户端已启动 MCP 服务", ErrLocalPairingUnavailable)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, richMailPairingMaximumBodyLength+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: 读取 RichMail 配对响应失败", ErrLocalPairingUnavailable)
	}
	if len(encoded) > richMailPairingMaximumBodyLength {
		return nil, nil, 0, fmt.Errorf("%w: RichMail 配对响应过大", ErrLocalPairingUnavailable)
	}
	payload := map[string]any{}
	if len(strings.TrimSpace(string(encoded))) > 0 {
		decoder := json.NewDecoder(strings.NewReader(string(encoded)))
		decoder.UseNumber()
		if err = decoder.Decode(&payload); err != nil {
			return nil, nil, 0, fmt.Errorf("%w: RichMail 配对响应格式不正确", ErrLocalPairingUnavailable)
		}
	}
	return richMailResponsePayload(payload), response.Header.Clone(), response.StatusCode, nil
}

func richMailLoopbackTransport() http.RoundTripper {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	cloned := transport.Clone()
	cloned.Proxy = nil
	return cloned
}

func validateRichMailBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "http" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("RichMail 本机服务地址无效")
	}
	host := strings.TrimSpace(parsed.Hostname())
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || strings.TrimSpace(parsed.Port()) == "" {
		return nil, errors.New("RichMail 服务只允许连接本机 loopback 地址")
	}
	parsed.Path = ""
	return parsed, nil
}

func (s *Service) encryptRichMailPairingAttempt(
	attempt richMailPairingAttempt,
) (string, error) {
	if s.credentialKeyringErr != nil {
		return "", s.credentialKeyringErr
	}
	payload, err := json.Marshal(attempt)
	if err != nil {
		return "", err
	}
	return s.credentialKeyring.EncryptEnvelope(payload)
}

func (s *Service) openRichMailPairingAttempt(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	token string,
) (richMailPairingAttempt, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, richMailPairingAttemptPrefix) ||
		len(token) > maxDeviceAuthAttemptTokenLength {
		return richMailPairingAttempt{}, errors.New("RichMail 配对会话无效，请重新开始")
	}
	if s.credentialKeyringErr != nil {
		return richMailPairingAttempt{}, s.credentialKeyringErr
	}
	payload, err := s.credentialKeyring.DecryptEnvelope(
		strings.TrimPrefix(token, richMailPairingAttemptPrefix),
	)
	if err != nil {
		return richMailPairingAttempt{}, errors.New("RichMail 配对会话无效，请重新开始")
	}
	var attempt richMailPairingAttempt
	if err = json.Unmarshal(payload, &attempt); err != nil {
		return richMailPairingAttempt{}, errors.New("RichMail 配对会话格式不正确")
	}
	if err = validateRichMailPairingAttempt(attempt, time.Now().UTC()); err != nil {
		return richMailPairingAttempt{}, err
	}
	if attempt.OwnerUserID != normalizeConnectorOwnerUserID(ctx, ownerUserID) ||
		attempt.ConnectorID != entry.ConnectorID {
		return richMailPairingAttempt{}, errors.New("RichMail 配对会话与当前用户或连接器不匹配")
	}
	return attempt, nil
}

func validateRichMailPairingAttempt(
	attempt richMailPairingAttempt,
	now time.Time,
) error {
	if attempt.Version != richMailPairingAttemptVersion ||
		strings.TrimSpace(attempt.AttemptID) == "" ||
		strings.TrimSpace(attempt.OwnerUserID) == "" ||
		attempt.ConnectorID != richMailConnectorID ||
		strings.TrimSpace(attempt.ProviderRequestID) == "" ||
		attempt.ExpectedConfigurationVersion < 1 {
		return errors.New("RichMail 配对会话缺少必要绑定")
	}
	if !attempt.ExpiresAt.After(now) {
		return errors.New("RichMail 配对会话已过期，请重新开始")
	}
	return nil
}

func newRichMailPairingAttemptID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "rmp_" + hex.EncodeToString(random), nil
}

func richMailResponsePayload(root map[string]any) map[string]any {
	for _, key := range []string{"data", "result"} {
		if nested, ok := root[key].(map[string]any); ok {
			return mergeRichMailPayload(root, nested)
		}
	}
	return root
}

func mergeRichMailPayload(root map[string]any, nested map[string]any) map[string]any {
	merged := make(map[string]any, len(root)+len(nested))
	for key, value := range root {
		merged[key] = value
	}
	for key, value := range nested {
		merged[key] = value
	}
	return merged
}

func richMailFirstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func richMailFirstInt(payload map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := payload[key].(type) {
		case json.Number:
			parsed, err := strconv.Atoi(value.String())
			if err == nil {
				return parsed
			}
		case float64:
			return int(value)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func richMailBearerToken(payload map[string]any, responseHeaders http.Header) string {
	candidates := []string{
		richMailFirstString(
			payload,
			"token", "accessToken", "access_token", "authorization",
			"header", "requestHeader", "request_header",
		),
		richMailAuthorizationFromMap(payload, "headers", "requestHeaders", "request_headers"),
		responseHeaders.Get("Authorization"),
	}
	for _, candidate := range candidates {
		if token := normalizeRichMailBearerToken(candidate); token != "" {
			return token
		}
	}
	return ""
}

func richMailAuthorizationFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		headers, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		for name, value := range headers {
			text, ok := value.(string)
			if ok && strings.EqualFold(strings.TrimSpace(name), "Authorization") {
				return text
			}
		}
	}
	return ""
}

func normalizedRichMailPairingExpiresIn(value int) int {
	if value <= 0 {
		return richMailPairingDefaultExpiresIn
	}
	if value > richMailPairingMaximumExpiresIn {
		return richMailPairingMaximumExpiresIn
	}
	return value
}

func normalizedRichMailPairingInterval(value int) int {
	if value <= 0 {
		return richMailPairingDefaultInterval
	}
	if value > richMailPairingMaximumInterval {
		return richMailPairingMaximumInterval
	}
	return value
}

func normalizedRichMailPairingStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "waiting", "requested":
		return localPairingStatusPending
	case "approved", "authorized", "connected", "success", "succeeded":
		return localPairingStatusConnected
	case "denied", "rejected", "cancelled", "canceled":
		return localPairingStatusDenied
	case "expired", "timeout", "timed_out":
		return localPairingStatusExpired
	default:
		return ""
	}
}

func normalizeRichMailBearerToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > richMailPairingMaximumToken ||
		strings.ContainsAny(value, "\r\n\x00") {
		return ""
	}
	if len(value) >= len("Authorization:") &&
		strings.EqualFold(value[:len("Authorization:")], "Authorization:") {
		value = strings.TrimSpace(value[len("Authorization:"):])
	}
	if len(value) >= len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		value = strings.TrimSpace(value[len("Bearer "):])
		if value == "" || strings.ContainsAny(value, " \t") {
			return ""
		}
		return value
	}
	if strings.ContainsAny(value, " \t:") {
		return ""
	}
	return value
}

func richMailPairingStatusForHTTP(statusCode int) string {
	switch statusCode {
	case http.StatusForbidden:
		return localPairingStatusDenied
	case http.StatusNotFound, http.StatusGone, http.StatusRequestTimeout:
		return localPairingStatusExpired
	default:
		return ""
	}
}

func richMailPairingHTTPError(statusCode int, polling bool) error {
	operation := "创建"
	if polling {
		operation = "查询"
	}
	return fmt.Errorf("%w: RichMail 无法%s配对请求（HTTP %d）", ErrLocalPairingUnavailable, operation, statusCode)
}
