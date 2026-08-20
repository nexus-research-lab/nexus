// INPUT: 已验证浏览器扩展连接、runtime Session identity 与浏览器动作参数。
// OUTPUT: browser.command 请求、browser.result 回执及按 Session 保存的多标签页状态。
// POS: WebBridge 业务真相源；HTTP/WebSocket 与 MCP 只做 transport 适配。
package webbridge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// ProtocolVersion 是 Nexus 宿主与浏览器扩展的线协议版本。
	ProtocolVersion = "1"
	// WebSocketSubprotocol 防止普通网页把内部端点当作通用 WebSocket 使用。
	WebSocketSubprotocol = "nexus.webbridge.v1"
	// BrowserExtensionID 来自 Nexus 扩展 manifest 的稳定公钥。
	BrowserExtensionID = "leljaammmcdgaalkopjbeppkmjghobjf"
	// BrowserExtensionOrigin 是 WebBridge 唯一接受的浏览器扩展来源。
	BrowserExtensionOrigin = "chrome-extension://" + BrowserExtensionID

	commandTimeout = 90 * time.Second
)

var (
	// ErrNotConnected 表示 Nexus 浏览器扩展尚未连接。
	ErrNotConnected = errors.New("Nexus WebBridge 扩展未连接；请安装并启用 desktop/browser-extension")
	// ErrConnectionClosed 表示等待期间扩展连接已经关闭或被替换。
	ErrConnectionClosed = errors.New("Nexus WebBridge 扩展连接已关闭")
	// ErrCDPDisabled 表示用户尚未开启完整 CDP 访问。
	ErrCDPDisabled = errors.New("完整 CDP 访问未启用；请先在电脑操控设置中开启")
)

type client struct {
	id               uint64
	extensionVersion string
	send             func(context.Context, any) error
	close            func()
}

type commandResponse struct {
	data map[string]any
	err  error
}

type browserSession struct {
	activeTabID int64
	tabIDs      map[int64]struct{}
}

// Service 管理唯一浏览器扩展连接和各 runtime Session 的多标签页状态。
type Service struct {
	mu       sync.Mutex
	client   *client
	pending  map[string]chan commandResponse
	sessions map[string]browserSession
	sequence atomic.Uint64
	closed   bool
}

// NewService 创建 WebBridge 服务。
func NewService() *Service {
	return &Service{
		pending:  make(map[string]chan commandResponse),
		sessions: make(map[string]browserSession),
	}
}

// Attach 注册已完成握手的浏览器扩展；新连接会替换旧连接并终止旧请求。
func (s *Service) Attach(
	extensionVersion string,
	send func(context.Context, any) error,
	closeClient func(),
) (uint64, func()) {
	if s == nil || send == nil {
		return 0, func() {}
	}
	id := s.sequence.Add(1)
	next := &client{
		id:               id,
		extensionVersion: strings.TrimSpace(extensionVersion),
		send:             send,
		close:            closeClient,
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if closeClient != nil {
			closeClient()
		}
		return 0, func() {}
	}
	previous := s.client
	s.client = next
	pending := s.takePendingLocked()
	s.mu.Unlock()

	deliverError(pending, ErrConnectionClosed)
	if previous != nil && previous.close != nil {
		previous.close()
	}
	return id, func() { s.Detach(id) }
}

// Detach 仅在连接仍是当前连接时注销，避免旧连接关闭误伤新连接。
func (s *Service) Detach(clientID uint64) {
	if s == nil || clientID == 0 {
		return
	}
	s.mu.Lock()
	if s.client == nil || s.client.id != clientID {
		s.mu.Unlock()
		return
	}
	s.client = nil
	pending := s.takePendingLocked()
	s.mu.Unlock()
	deliverError(pending, ErrConnectionClosed)
}

// Resolve 把扩展回执交给对应的等待调用。
func (s *Service) Resolve(requestID string, data map[string]any, message string) bool {
	if s == nil {
		return false
	}
	requestID = strings.TrimSpace(requestID)
	s.mu.Lock()
	waiter := s.pending[requestID]
	delete(s.pending, requestID)
	s.mu.Unlock()
	if waiter == nil {
		return false
	}
	response := commandResponse{data: data}
	if message = strings.TrimSpace(message); message != "" {
		response.err = errors.New(message)
	}
	waiter <- response
	return true
}

// Execute 校验模型输入，把动作发送给扩展，并维护当前 Session 的标签页状态。
func (s *Service) Execute(
	ctx context.Context,
	sessionKey string,
	sessionLabel string,
	action string,
	input map[string]any,
	allowCDP bool,
) (map[string]any, error) {
	if s == nil {
		return nil, ErrNotConnected
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "status" {
		return s.Status(), nil
	}
	if action == "cdp" && !allowCDP {
		return nil, ErrCDPDisabled
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, errors.New("WebBridge 缺少 runtime Session identity")
	}
	params, immediate, err := s.prepareParams(sessionKey, sessionLabel, action, input)
	if err != nil || immediate != nil {
		return immediate, err
	}

	callCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	requestID := fmt.Sprintf("browser-%d", s.sequence.Add(1))
	waiter := make(chan commandResponse, 1)

	s.mu.Lock()
	if s.closed || s.client == nil {
		s.mu.Unlock()
		return nil, ErrNotConnected
	}
	sender := s.client.send
	s.pending[requestID] = waiter
	s.mu.Unlock()

	message := map[string]any{
		"type":   "browser.command",
		"id":     requestID,
		"action": action,
		"params": params,
	}
	if err = sender(callCtx, message); err != nil {
		s.removePending(requestID)
		return nil, fmt.Errorf("发送 WebBridge 命令失败: %w", err)
	}

	select {
	case response := <-waiter:
		if response.err != nil {
			return nil, response.err
		}
		s.updateSession(sessionKey, action, params, response.data)
		return response.data, nil
	case <-callCtx.Done():
		s.removePending(requestID)
		return nil, fmt.Errorf("WebBridge 动作 %s 超时: %w", action, callCtx.Err())
	}
}

// Status 返回不包含浏览数据的连接状态。
func (s *Service) Status() map[string]any {
	result := map[string]any{
		"connected":        false,
		"protocol_version": ProtocolVersion,
	}
	if s == nil {
		return result
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && !s.closed {
		result["connected"] = true
		result["extension_version"] = s.client.extensionVersion
	}
	return result
}

// Close 关闭扩展连接并终止所有等待调用。
func (s *Service) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	current := s.client
	s.client = nil
	pending := s.takePendingLocked()
	s.mu.Unlock()
	deliverError(pending, ErrConnectionClosed)
	if current != nil && current.close != nil {
		current.close()
	}
}

func (s *Service) prepareParams(
	sessionKey string,
	sessionLabel string,
	action string,
	input map[string]any,
) (map[string]any, map[string]any, error) {
	params := cloneMap(input)
	for _, key := range []string{"action", "owned", "session", "group_title", "tab_ids"} {
		delete(params, key)
	}
	params["session"] = sessionKey
	params["group_title"] = sessionTitle(sessionLabel)

	s.mu.Lock()
	state, hasSession := s.sessions[sessionKey]
	s.mu.Unlock()
	tabIDs := orderedTabIDs(state.tabIDs)

	switch action {
	case "list_tabs":
		delete(params, "tab_id")
		params["tab_ids"] = tabIDs
	case "attach_active":
		delete(params, "tab_id")
	case "attach_tab":
		tabID, ok := integerValue(params["tab_id"])
		if !ok || tabID <= 0 {
			return nil, nil, errors.New("attach_tab 需要正整数 tab_id")
		}
		params["tab_id"] = tabID
	case "navigate":
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("navigate 需要非空 url")
		}
		newTab, err := optionalBool(params, "new_tab")
		if err != nil {
			return nil, nil, err
		}
		delete(params, "tab_id")
		if hasSession && state.activeTabID > 0 && !newTab {
			params["tab_id"] = state.activeTabID
		}
	case "find_tab":
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("find_tab 需要非空 url")
		}
		if _, err := optionalBool(params, "active"); err != nil {
			return nil, nil, err
		}
		delete(params, "tab_id")
		params["tab_ids"] = tabIDs
	case "close_session":
		if len(tabIDs) == 0 {
			return nil, map[string]any{"closed": 0, "tab_ids": []int64{}}, nil
		}
		params = map[string]any{"session": sessionKey, "tab_ids": tabIDs}
	case "close_tab", "close":
		if !hasSession || state.activeTabID <= 0 {
			return nil, map[string]any{"closed": false, "reason": "session has no tab"}, nil
		}
		params = map[string]any{"session": sessionKey, "tab_id": state.activeTabID}
	case "evaluate":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if stringValue(params["code"]) == "" {
			return nil, nil, errors.New("evaluate 需要非空 code")
		}
	case "network":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		command := strings.ToLower(stringValue(params["cmd"]))
		if command != "start" && command != "stop" && command != "list" && command != "detail" {
			return nil, nil, errors.New("network 的 cmd 必须是 start、stop、list 或 detail")
		}
		params["cmd"] = command
		if command == "detail" && stringValue(params["request_id"]) == "" {
			return nil, nil, errors.New("network detail 需要 request_id")
		}
		if value, exists := params["filter"]; exists {
			if _, ok := value.(string); !ok {
				return nil, nil, errors.New("network 的 filter 必须是字符串")
			}
		}
	case "snapshot":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
	case "click", "mouse_click":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizeSelector(params, action); err != nil {
			return nil, nil, err
		}
	case "fill":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizeSelector(params, action); err != nil {
			return nil, nil, err
		}
		if _, ok := params["value"].(string); !ok {
			return nil, nil, errors.New("fill 的 value 必须是字符串")
		}
	case "cdp":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if stringValue(params["method"]) == "" {
			return nil, nil, errors.New("cdp 需要非空 method")
		}
		if value, exists := params["params"]; exists {
			if _, ok := value.(map[string]any); !ok {
				return nil, nil, errors.New("cdp 的 params 必须是对象")
			}
		}
	case "key_type":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if _, ok := params["text"].(string); !ok {
			return nil, nil, errors.New("key_type 的 text 必须是字符串")
		}
	case "send_keys", "press_key":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if action == "press_key" && stringValue(params["keys"]) == "" {
			params["keys"] = params["key"]
		}
		if stringValue(params["keys"]) == "" {
			return nil, nil, fmt.Errorf("%s 需要非空 keys", action)
		}
		if repeat, exists := params["repeat"]; exists {
			value, ok := integerValue(repeat)
			if !ok || value < 1 || value > 100 {
				return nil, nil, errors.New("send_keys 的 repeat 必须在 1 到 100 之间")
			}
			params["repeat"] = value
		}
	case "screenshot":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		format := strings.ToLower(stringValue(params["format"]))
		if format != "" && format != "png" && format != "jpeg" {
			return nil, nil, errors.New("screenshot 的 format 必须是 png 或 jpeg")
		}
		if format != "" {
			params["format"] = format
		}
		if quality, exists := params["quality"]; exists {
			value, ok := integerValue(quality)
			if !ok || value < 0 || value > 100 {
				return nil, nil, errors.New("screenshot 的 quality 必须在 0 到 100 之间")
			}
			params["quality"] = value
		}
		if selector := selectorValue(params); selector != "" {
			params["selector"] = selector
		}
	case "save_as_pdf":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if format := strings.ToLower(stringValue(params["paper_format"])); format != "" {
			if format != "letter" && format != "legal" && format != "a4" && format != "a3" && format != "tabloid" {
				return nil, nil, errors.New("save_as_pdf 不支持该 paper_format")
			}
			params["paper_format"] = format
		}
		if scale, exists := params["scale"]; exists {
			value, ok := numberValue(scale)
			if !ok || value < 0.1 || value > 2 {
				return nil, nil, errors.New("save_as_pdf 的 scale 必须在 0.1 到 2 之间")
			}
		}
		for _, key := range []string{"landscape", "print_background"} {
			if _, err := optionalBool(params, key); err != nil {
				return nil, nil, err
			}
		}
	case "upload":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizeSelector(params, action); err != nil {
			return nil, nil, err
		}
		if !validStringList(params["files"]) {
			return nil, nil, errors.New("upload 的 files 必须是非空路径数组")
		}
	default:
		return nil, nil, fmt.Errorf("不支持的 WebBridge action: %s", action)
	}
	return params, nil, nil
}

func (s *Service) updateSession(
	sessionKey string,
	action string,
	params map[string]any,
	result map[string]any,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.sessions[sessionKey]
	if !ok {
		state = browserSession{tabIDs: make(map[int64]struct{})}
	}
	switch action {
	case "attach_active", "attach_tab", "navigate", "find_tab":
		tabID, valid := integerValue(result["tab_id"])
		if !valid || tabID <= 0 {
			return
		}
		state.tabIDs[tabID] = struct{}{}
		state.activeTabID = tabID
		s.sessions[sessionKey] = state
	case "close_tab", "close":
		tabID, valid := integerValue(result["tab_id"])
		if !valid {
			tabID, _ = integerValue(params["tab_id"])
		}
		delete(state.tabIDs, tabID)
		remaining := orderedTabIDs(state.tabIDs)
		if len(remaining) == 0 {
			delete(s.sessions, sessionKey)
			return
		}
		state.activeTabID = remaining[len(remaining)-1]
		s.sessions[sessionKey] = state
	case "close_session":
		delete(s.sessions, sessionKey)
	}
}

func requireActiveTab(params *map[string]any, state browserSession, hasSession bool, action string) error {
	if !hasSession || state.activeTabID <= 0 {
		return missingTabError(action)
	}
	(*params)["tab_id"] = state.activeTabID
	return nil
}

func normalizeSelector(params map[string]any, action string) error {
	selector := selectorValue(params)
	if selector == "" {
		return fmt.Errorf("%s 需要 CSS selector 或 snapshot ref", action)
	}
	params["selector"] = selector
	return nil
}

func selectorValue(params map[string]any) string {
	if selector := stringValue(params["selector"]); selector != "" {
		return selector
	}
	return stringValue(params["ref"])
}

func optionalBool(params map[string]any, key string) (bool, error) {
	value, exists := params[key]
	if !exists {
		return false, nil
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s 必须是布尔值", key)
	}
	return result, nil
}

func validStringList(value any) bool {
	switch list := value.(type) {
	case []string:
		if len(list) == 0 {
			return false
		}
		for _, item := range list {
			if strings.TrimSpace(item) == "" {
				return false
			}
		}
		return true
	case []any:
		if len(list) == 0 {
			return false
		}
		for _, item := range list {
			if stringValue(item) == "" {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func orderedTabIDs(tabIDs map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(tabIDs))
	for tabID := range tabIDs {
		result = append(result, tabID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func sessionTitle(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "Nexus"
	}
	if len([]rune(label)) <= 80 {
		return label
	}
	return string([]rune(label)[:80])
}

func (s *Service) removePending(requestID string) {
	s.mu.Lock()
	delete(s.pending, requestID)
	s.mu.Unlock()
}

func (s *Service) takePendingLocked() []chan commandResponse {
	result := make([]chan commandResponse, 0, len(s.pending))
	for requestID, waiter := range s.pending {
		result = append(result, waiter)
		delete(s.pending, requestID)
	}
	return result
}

func deliverError(waiters []chan commandResponse, err error) {
	for _, waiter := range waiters {
		waiter <- commandResponse{err: err}
	}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		converted := int64(typed)
		return converted, float64(converted) == typed
	default:
		return 0, false
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func missingTabError(action string) error {
	return fmt.Errorf("%s 前请先 navigate、find_tab、attach_active 或 attach_tab", action)
}
