// INPUT: 可选 Relay base URL、Control 签发的 Relay user token 与 typed M1 请求。
// OUTPUT: 带 Bearer、Idempotency-Key 和有界响应解码的 Relay HTTP/WSS 结果。
// POS: Nexus Server 访问 Nexus Relay 的唯一 HTTP/WSS adapter；不承载 Room 业务。
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	relayAPIBase          = "/api/relay/v1"
	defaultRequestTimeout = 5 * time.Second
	// 100 条 64 KiB Message 在 JSON HTML 转义的最坏情况下约为 38 MiB。
	maxResponseBytes      = 48 << 20
	maxErrorResponseBytes = 64 << 10
	maxPageSize           = 100
	maxCommandIDBytes     = 128
)

// Client 是 Relay M1 typed HTTP client。
type Client struct {
	baseURL    string
	httpClient *http.Client
	wsClient   *http.Client
}

// RemoteError 是 Relay 返回的结构化失败。
type RemoteError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *RemoteError) Error() string {
	if e == nil {
		return "Relay 请求失败"
	}
	return fmt.Sprintf("Relay 请求失败: %s (%s)", e.Message, e.Code)
}

type responseEnvelope struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

// NewClient 创建一个独立 Relay HTTP client。未配置 URL 时调用方不应构造它。
func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("NEXUS_RELAY_URL 必须是完整的 http(s) URL")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("NEXUS_RELAY_URL 只能是不含凭据、查询参数或 fragment 的 http(s) URL")
	}
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		wsClient: &http.Client{
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Watch 订阅一个 Relay stream 的提交水位提示；调用方收到提示后必须走 Difference 恢复。
func (c *Client) Watch(
	ctx context.Context,
	token string,
	streamID string,
	streamEpoch string,
	handle func(StreamUpdated) error,
) error {
	if c == nil || c.wsClient == nil || c.baseURL == "" {
		return errors.New("Relay client 未配置")
	}
	streamID, err := requireResourceID(streamID, "stream_id")
	if err != nil {
		return err
	}
	streamEpoch, err = requireResourceID(streamEpoch, "stream_epoch")
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" || handle == nil {
		return errors.New("Relay WSS 缺少 token 或处理函数")
	}
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = "/ws/relay"
	parsed.RawQuery = url.Values{
		"stream_id":    {streamID},
		"stream_epoch": {streamEpoch},
	}.Encode()
	header := http.Header{"Authorization": {"Bearer " + token}}
	connection, response, err := websocket.Dial(ctx, parsed.String(), &websocket.DialOptions{
		HTTPClient: c.wsClient,
		HTTPHeader: header,
	})
	if err != nil {
		if response != nil {
			return readRemoteError(response)
		}
		return fmt.Errorf("连接 Nexus Relay WSS: %w", err)
	}
	defer connection.CloseNow()
	connection.SetReadLimit(16 << 10)
	for {
		var update StreamUpdated
		if err = wsjson.Read(ctx, connection, &update); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("读取 Nexus Relay WSS: %w", err)
		}
		if update.Type != "stream.updated" || update.StreamID != streamID ||
			update.StreamEpoch != streamEpoch || update.HighWaterSeq < 0 {
			return errors.New("Nexus Relay WSS 返回无效水位提示")
		}
		if err = handle(update); err != nil {
			return err
		}
	}
}

func readRemoteError(response *http.Response) error {
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
	if err != nil {
		return fmt.Errorf("读取 Nexus Relay 错误响应: %w", err)
	}
	if len(payload) > maxErrorResponseBytes {
		return errors.New("Nexus Relay 错误响应超过上限")
	}
	var envelope responseEnvelope
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("连接 Nexus Relay WSS: HTTP %d", response.StatusCode)
	}
	return &RemoteError{
		StatusCode: response.StatusCode,
		Code:       strings.TrimSpace(envelope.Code),
		Message:    strings.TrimSpace(envelope.Message),
		RequestID:  strings.TrimSpace(envelope.RequestID),
	}
}

// Bootstrap 幂等获取当前 Deployment 的默认协作空间。
func (c *Client) Bootstrap(ctx context.Context, token string) (Bootstrap, error) {
	var result Bootstrap
	err := c.do(ctx, http.MethodPost, "/bootstrap", nil, token, "", nil, &result)
	if err == nil {
		result.Conversation.StreamEpoch, err = responseStreamEpoch(result.Conversation.StreamEpoch, "")
	}
	return result, err
}

// PostMessage 使用调用方提供的幂等键提交一条真人消息。
func (c *Client) PostMessage(
	ctx context.Context,
	token string,
	conversationID string,
	idempotencyKey string,
	input CreateMessageInput,
) (MessageCommit, error) {
	conversationID, err := requireResourceID(conversationID, "conversation_id")
	if err != nil {
		return MessageCommit{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validCommandID(idempotencyKey) {
		return MessageCommit{}, errors.New("Idempotency-Key 必须为 1-128 字节的可见 ASCII")
	}
	var result MessageCommit
	err = c.do(
		ctx,
		http.MethodPost,
		"/conversations/"+url.PathEscape(conversationID)+"/messages",
		nil,
		token,
		idempotencyKey,
		input,
		&result,
	)
	if err == nil {
		result.StreamEpoch, err = responseStreamEpoch(result.StreamEpoch, "")
	}
	return result, err
}

// Snapshot 读取一页固定上界的 Conversation 快照。
func (c *Client) Snapshot(
	ctx context.Context,
	token string,
	conversationID string,
	options SnapshotOptions,
) (Snapshot, error) {
	conversationID, err := requireResourceID(conversationID, "conversation_id")
	if err != nil {
		return Snapshot{}, err
	}
	if err = validatePage(options.AfterMessageSeq, options.Limit); err != nil {
		return Snapshot{}, err
	}
	if (options.ThroughMessageSeq == nil) != (options.SnapshotSeq == nil) {
		return Snapshot{}, errors.New("续页必须同时提供 through_message_seq 与 snapshot_seq")
	}
	streamEpoch, err := optionalStreamEpoch(
		options.StreamEpoch,
		options.AfterMessageSeq > 0 || options.SnapshotSeq != nil,
	)
	if err != nil {
		return Snapshot{}, err
	}
	query := url.Values{
		"after_message_seq": {strconv.FormatInt(options.AfterMessageSeq, 10)},
		"limit":             {strconv.Itoa(options.Limit)},
	}
	if options.ThroughMessageSeq != nil {
		if *options.ThroughMessageSeq < 0 || *options.SnapshotSeq < 0 {
			return Snapshot{}, errors.New("Relay 快照游标不能为负数")
		}
		query.Set("through_message_seq", strconv.FormatInt(*options.ThroughMessageSeq, 10))
		query.Set("snapshot_seq", strconv.FormatInt(*options.SnapshotSeq, 10))
	}
	if streamEpoch != "" {
		query.Set("stream_epoch", streamEpoch)
	}
	var result Snapshot
	err = c.do(
		ctx,
		http.MethodGet,
		"/conversations/"+url.PathEscape(conversationID)+"/snapshot",
		query,
		token,
		"",
		nil,
		&result,
	)
	if err == nil {
		result.StreamEpoch, err = responseStreamEpoch(result.StreamEpoch, streamEpoch)
	}
	return result, err
}

// Difference 读取一页连续 Conversation sync event。
func (c *Client) Difference(
	ctx context.Context,
	token string,
	streamID string,
	options DifferenceOptions,
) (Difference, error) {
	streamID, err := requireResourceID(streamID, "stream_id")
	if err != nil {
		return Difference{}, err
	}
	if err = validatePage(options.AfterSeq, options.Limit); err != nil {
		return Difference{}, err
	}
	streamEpoch, err := optionalStreamEpoch(options.StreamEpoch, options.AfterSeq > 0)
	if err != nil {
		return Difference{}, err
	}
	query := url.Values{
		"after_seq": {strconv.FormatInt(options.AfterSeq, 10)},
		"limit":     {strconv.Itoa(options.Limit)},
	}
	if streamEpoch != "" {
		query.Set("stream_epoch", streamEpoch)
	}
	var result Difference
	err = c.do(
		ctx,
		http.MethodGet,
		"/sync-streams/"+url.PathEscape(streamID)+"/difference",
		query,
		token,
		"",
		nil,
		&result,
	)
	if err == nil {
		result.StreamEpoch, err = responseStreamEpoch(result.StreamEpoch, streamEpoch)
	}
	return result, err
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	token string,
	idempotencyKey string,
	input any,
	output any,
) error {
	if c == nil || c.httpClient == nil || c.baseURL == "" {
		return errors.New("Relay client 未配置")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("Relay user token 不能为空")
	}
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}
	endpoint := c.baseURL + relayAPIBase + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("调用 Nexus Relay: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("读取 Nexus Relay 响应: %w", err)
	}
	if len(payload) > maxResponseBytes {
		return errors.New("Nexus Relay 响应超过上限")
	}
	var envelope responseEnvelope
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("解析 Nexus Relay 响应: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || envelope.Code != "0000" {
		return &RemoteError{
			StatusCode: response.StatusCode,
			Code:       strings.TrimSpace(envelope.Code),
			Message:    strings.TrimSpace(envelope.Message),
			RequestID:  strings.TrimSpace(envelope.RequestID),
		}
	}
	if output == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("Nexus Relay 成功响应缺少 data")
	}
	if err = json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("解析 Nexus Relay data: %w", err)
	}
	return nil
}

func requireResourceID(value string, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return "", fmt.Errorf("%s 必须为 1-128 字节", name)
	}
	return value, nil
}

func validatePage(after int64, limit int) error {
	if after < 0 || limit <= 0 || limit > maxPageSize {
		return errors.New("Relay 分页要求 after >= 0 且 limit 为 1-100")
	}
	return nil
}

func optionalStreamEpoch(value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && !required {
		return "", nil
	}
	return requireResourceID(value, "stream_epoch")
}

func responseStreamEpoch(value string, expected string) (string, error) {
	value, err := requireResourceID(value, "response stream_epoch")
	if err != nil {
		return "", err
	}
	if expected != "" && value != expected {
		return "", errors.New("Nexus Relay 响应的 stream_epoch 与请求不一致")
	}
	return value, nil
}

func validCommandID(value string) bool {
	if value == "" || len(value) > maxCommandIDBytes {
		return false
	}
	for _, char := range []byte(value) {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return true
}
