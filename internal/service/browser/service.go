// INPUT: 已验证浏览器扩展连接、runtime Session identity 与浏览器动作参数。
// OUTPUT: browser.command 请求、browser.result 回执及按 Session 保存的多标签页状态。
// POS: Browser 业务真相源；HTTP/WebSocket 与 MCP 只做 transport 适配。
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// ProtocolVersion 是 Nexus 宿主与浏览器扩展的线协议版本。
	ProtocolVersion = "2"
	// WebSocketSubprotocol 防止普通网页把内部端点当作通用 WebSocket 使用。
	WebSocketSubprotocol = "nexus.browser.v1"
	// BrowserExtensionID 来自 Nexus 扩展 manifest 的稳定公钥。
	BrowserExtensionID = "leljaammmcdgaalkopjbeppkmjghobjf"
	// BrowserExtensionOrigin 是 Browser 唯一接受的浏览器扩展来源。
	BrowserExtensionOrigin = "chrome-extension://" + BrowserExtensionID

	commandTimeout = 90 * time.Second
)

var (
	// ErrNotConnected 表示 Nexus 浏览器扩展尚未连接。
	ErrNotConnected = errors.New("Nexus Browser 扩展未连接；请安装并启用 desktop/browser-extension")
	// ErrConnectionClosed 表示等待期间扩展连接已经关闭或被替换。
	ErrConnectionClosed = errors.New("Nexus Browser 扩展连接已关闭")
	// ErrCDPDisabled 表示用户尚未开启完整 CDP 访问。
	ErrCDPDisabled = errors.New("完整 CDP 访问未启用；请先在 Browser 设置中开启")
)

// SupportedActions 返回 Browser service 接受的稳定 action 名称。
func SupportedActions() []string {
	return []string{
		"status", "navigate", "find_tab", "list_tabs", "attach_active", "attach_tab",
		"back", "forward", "reload", "history", "evaluate", "page_content", "wait_for", "wait_for_url",
		"network", "console", "dialog", "snapshot", "click", "fill", "check", "uncheck",
		"select_option", "mouse_click", "double_click", "hover", "mouse_move", "drag", "scroll",
		"cdp", "clipboard", "key_type", "send_keys", "press_key", "screenshot", "save_as_pdf", "upload",
		"download", "downloads", "close_tab", "close_session", "close",
	}
}

type client struct {
	id                uint64
	extensionVersion  string
	browserInstance   string
	browserGeneration string
	send              func(context.Context, any) error
	close             func()
}

type commandResponse struct {
	data map[string]any
	err  error
}

type browserTab struct {
	id  int64
	ref string
}

type browserSession struct {
	activeTabRef string
	tabs         map[string]browserTab
}

// Service 管理唯一浏览器扩展连接和各 runtime Session 的多标签页状态。
type Service struct {
	mu              sync.Mutex
	client          *client
	pending         map[string]chan commandResponse
	sessions        map[string]browserSession
	browserIdentity string
	sequence        atomic.Uint64
	closed          bool
}

// NewService 创建 Browser 服务。
func NewService() *Service {
	return &Service{
		pending:  make(map[string]chan commandResponse),
		sessions: make(map[string]browserSession),
	}
}

// Attach 注册已完成握手的浏览器扩展；新连接会替换旧连接并终止旧请求。
func (s *Service) Attach(
	extensionVersion string,
	browserInstance string,
	browserGeneration string,
	send func(context.Context, any) error,
	closeClient func(),
) (uint64, func()) {
	browserInstance = strings.TrimSpace(browserInstance)
	browserGeneration = strings.TrimSpace(browserGeneration)
	if s == nil || send == nil || browserInstance == "" || browserGeneration == "" {
		return 0, func() {}
	}
	id := s.sequence.Add(1)
	next := &client{
		id:                id,
		extensionVersion:  strings.TrimSpace(extensionVersion),
		browserInstance:   browserInstance,
		browserGeneration: browserGeneration,
		send:              send,
		close:             closeClient,
	}
	nextIdentity := next.browserInstance + ":" + next.browserGeneration

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if closeClient != nil {
			closeClient()
		}
		return 0, func() {}
	}
	previous := s.client
	if s.browserIdentity != "" && s.browserIdentity != nextIdentity {
		clear(s.sessions)
	}
	s.browserIdentity = nextIdentity
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
		return nil, errors.New("Browser 缺少 runtime Session identity")
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
		return nil, fmt.Errorf("发送 Browser 命令失败: %w", err)
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
		return nil, fmt.Errorf("Browser 动作 %s 超时: %w", action, callCtx.Err())
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
	requestedTabRef := stringValue(params["tab_ref"])
	for _, key := range []string{"action", "owned", "session", "group_title", "tab_id", "tab_ids", "tab_ref", "tab_refs"} {
		delete(params, key)
	}
	params["session"] = sessionKey
	params["group_title"] = sessionTitle(sessionLabel)

	s.mu.Lock()
	state, hasSession := s.sessions[sessionKey]
	s.mu.Unlock()
	tabs := orderedTabs(state.tabs)
	tabRefs := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		tabRefs = append(tabRefs, tab.ref)
	}

	switch action {
	case "list_tabs":
		delete(params, "tab_id")
		scope := strings.ToLower(stringValue(params["scope"]))
		if scope == "" {
			scope = "session"
		}
		if scope != "session" && scope != "all" {
			return nil, nil, errors.New("list_tabs 的 scope 必须是 session 或 all")
		}
		params["scope"] = scope
		if scope == "all" {
			delete(params, "tab_refs")
		} else {
			params["tab_refs"] = tabRefs
		}
	case "attach_active":
		delete(params, "tab_id")
	case "attach_tab":
		if requestedTabRef == "" {
			return nil, nil, errors.New("attach_tab 需要 list_tabs 返回的 tab_ref")
		}
		params["tab_ref"] = requestedTabRef
	case "navigate":
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("navigate 需要非空 url")
		}
		newTab, err := optionalBool(params, "new_tab")
		if err != nil {
			return nil, nil, err
		}
		delete(params, "tab_id")
		if hasSession && state.activeTabRef != "" && !newTab {
			setActiveTab(params, state)
		}
	case "find_tab":
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("find_tab 需要非空 url")
		}
		if _, err := optionalBool(params, "active"); err != nil {
			return nil, nil, err
		}
		delete(params, "tab_id")
		params["tab_refs"] = tabRefs
	case "history":
		if err := optionalString(params, "query"); err != nil {
			return nil, nil, err
		}
		for _, key := range []string{"start_time", "end_time"} {
			if err := optionalNonNegativeNumber(params, key); err != nil {
				return nil, nil, err
			}
		}
		if start, startOK := numberValue(params["start_time"]); startOK {
			if end, endOK := numberValue(params["end_time"]); endOK && start > end {
				return nil, nil, errors.New("history 的 start_time 不能晚于 end_time")
			}
		}
		if err := optionalBoundedInteger(params, "max_results", 1, 1000); err != nil {
			return nil, nil, err
		}
	case "download":
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("download 需要非空 url")
		}
		if err := optionalString(params, "file_name"); err != nil {
			return nil, nil, err
		}
		if _, err := optionalBool(params, "save_as"); err != nil {
			return nil, nil, err
		}
	case "downloads":
		command := strings.ToLower(stringValue(params["cmd"]))
		if command != "list" && command != "wait" && command != "show" {
			return nil, nil, errors.New("downloads 的 cmd 必须是 list、wait 或 show")
		}
		params["cmd"] = command
		if command == "list" {
			if err := optionalString(params, "query"); err != nil {
				return nil, nil, err
			}
			state := strings.ToLower(stringValue(params["download_state"]))
			if state != "" && state != "in_progress" && state != "complete" && state != "interrupted" {
				return nil, nil, errors.New("downloads 的 download_state 无效")
			}
			if state != "" {
				params["download_state"] = state
			}
			if err := optionalBoundedInteger(params, "max_results", 1, 1000); err != nil {
				return nil, nil, err
			}
		} else {
			if err := requiredPositiveInteger(params, "download_id"); err != nil {
				return nil, nil, err
			}
			if command == "wait" {
				if err := optionalBoundedInteger(params, "timeout_ms", 100, 80000); err != nil {
					return nil, nil, err
				}
			}
		}
	case "close_session":
		if len(tabRefs) == 0 {
			return nil, map[string]any{"closed": 0, "tab_refs": []string{}}, nil
		}
		params = map[string]any{"session": sessionKey, "tab_refs": tabRefs}
	case "close_tab", "close":
		if !hasSession || state.activeTabRef == "" {
			return nil, map[string]any{"closed": false, "reason": "session has no tab"}, nil
		}
		params = map[string]any{"session": sessionKey}
		setActiveTab(params, state)
	case "back", "forward", "reload":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
	case "evaluate":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if stringValue(params["code"]) == "" {
			return nil, nil, errors.New("evaluate 需要非空 code")
		}
		if err := optionalBoundedInteger(params, "timeout_ms", 100, 80000); err != nil {
			return nil, nil, err
		}
	case "page_content":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		format := strings.ToLower(stringValue(params["page_format"]))
		if format == "" {
			format = "text"
		}
		if format != "text" && format != "html" {
			return nil, nil, errors.New("page_content 的 page_format 必须是 text 或 html")
		}
		params["page_format"] = format
		if selector := selectorValue(params); selector != "" {
			params["selector"] = selector
		}
		if err := optionalBoundedInteger(params, "max_chars", 1, 2_000_000); err != nil {
			return nil, nil, err
		}
	case "wait_for":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizeSelector(params, action); err != nil {
			return nil, nil, err
		}
		waitState := strings.ToLower(stringValue(params["state"]))
		if waitState == "" {
			waitState = "visible"
		}
		if waitState != "attached" && waitState != "detached" && waitState != "visible" && waitState != "hidden" {
			return nil, nil, errors.New("wait_for 的 state 无效")
		}
		params["state"] = waitState
		if err := optionalString(params, "text"); err != nil {
			return nil, nil, err
		}
		if err := optionalBoundedInteger(params, "timeout_ms", 100, 80000); err != nil {
			return nil, nil, err
		}
	case "wait_for_url":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if stringValue(params["url"]) == "" {
			return nil, nil, errors.New("wait_for_url 需要非空 url")
		}
		if err := optionalBoundedInteger(params, "timeout_ms", 100, 80000); err != nil {
			return nil, nil, err
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
	case "console":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		command := strings.ToLower(stringValue(params["cmd"]))
		if command != "start" && command != "stop" && command != "list" {
			return nil, nil, errors.New("console 的 cmd 必须是 start、stop 或 list")
		}
		params["cmd"] = command
		if err := optionalString(params, "filter"); err != nil {
			return nil, nil, err
		}
		if err := optionalBoundedInteger(params, "max_results", 1, 1000); err != nil {
			return nil, nil, err
		}
	case "dialog":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		command := strings.ToLower(stringValue(params["cmd"]))
		if command != "get" && command != "accept" && command != "dismiss" {
			return nil, nil, errors.New("dialog 的 cmd 必须是 get、accept 或 dismiss")
		}
		params["cmd"] = command
		if err := optionalString(params, "prompt_text"); err != nil {
			return nil, nil, err
		}
	case "snapshot":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
	case "click", "check", "uncheck":
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
	case "select_option":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizeSelector(params, action); err != nil {
			return nil, nil, err
		}
		if value, exists := params["values"]; exists {
			if !validStringList(value) {
				return nil, nil, errors.New("select_option 的 values 必须是非空字符串数组")
			}
		} else if _, ok := params["value"].(string); !ok {
			return nil, nil, errors.New("select_option 需要 value 或 values")
		}
	case "mouse_click", "double_click":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizePointer(params, action, "x", "y"); err != nil {
			return nil, nil, err
		}
		button := strings.ToLower(stringValue(params["button"]))
		if button != "" && button != "left" && button != "middle" && button != "right" && button != "back" && button != "forward" {
			return nil, nil, fmt.Errorf("%s 的 button 无效", action)
		}
		if button != "" {
			params["button"] = button
		}
		if err := optionalBoundedInteger(params, "click_count", 1, 3); err != nil {
			return nil, nil, err
		}
	case "hover", "mouse_move":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if err := normalizePointer(params, action, "x", "y"); err != nil {
			return nil, nil, err
		}
	case "drag":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		selector := selectorValue(params)
		targetSelector := stringValue(params["target_selector"])
		if selector != "" || targetSelector != "" {
			if selector == "" || targetSelector == "" {
				return nil, nil, errors.New("drag 需要同时提供 selector 和 target_selector")
			}
			params["selector"] = selector
			params["target_selector"] = targetSelector
		} else {
			if err := requirePoint(params, "drag", "from_x", "from_y"); err != nil {
				return nil, nil, err
			}
			if err := requirePoint(params, "drag", "to_x", "to_y"); err != nil {
				return nil, nil, err
			}
		}
		if err := optionalBoundedInteger(params, "steps", 1, 100); err != nil {
			return nil, nil, err
		}
	case "scroll":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		if selector := selectorValue(params); selector != "" {
			params["selector"] = selector
		}
		if _, err := optionalPoint(params, "x", "y"); err != nil {
			return nil, nil, err
		}
		deltaProvided := false
		for _, key := range []string{"delta_x", "delta_y"} {
			provided, err := optionalFiniteNumber(params, key)
			if err != nil {
				return nil, nil, err
			}
			deltaProvided = deltaProvided || provided
		}
		if selectorValue(params) == "" && !deltaProvided {
			return nil, nil, errors.New("scroll 需要 selector 或 delta_x/delta_y")
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
	case "clipboard":
		if err := requireActiveTab(&params, state, hasSession, action); err != nil {
			return nil, nil, err
		}
		command := strings.ToLower(stringValue(params["cmd"]))
		if command != "read" && command != "write" {
			return nil, nil, errors.New("clipboard 的 cmd 必须是 read 或 write")
		}
		params["cmd"] = command
		if command == "write" {
			if _, ok := params["text"].(string); !ok {
				return nil, nil, errors.New("clipboard write 的 text 必须是字符串")
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
		if _, err := optionalBool(params, "full_page"); err != nil {
			return nil, nil, err
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
		return nil, nil, fmt.Errorf("不支持的 Browser action: %s", action)
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
		state = browserSession{tabs: make(map[string]browserTab)}
	}
	switch action {
	case "attach_active", "attach_tab", "navigate", "find_tab":
		tabID, valid := integerValue(result["tab_id"])
		tabRef := stringValue(result["tab_ref"])
		if !valid || tabID <= 0 || tabRef == "" {
			return
		}
		for existingRef, tab := range state.tabs {
			if tab.id == tabID && existingRef != tabRef {
				delete(state.tabs, existingRef)
			}
		}
		state.tabs[tabRef] = browserTab{id: tabID, ref: tabRef}
		state.activeTabRef = tabRef
		s.sessions[sessionKey] = state
	case "list_tabs":
		items, _ := result["tabs"].([]any)
		if stringValue(result["scope"]) != "session" || items == nil {
			return
		}
		next := make(map[string]browserTab, len(items))
		for _, item := range items {
			value, _ := item.(map[string]any)
			tabID, valid := integerValue(value["tab_id"])
			tabRef := stringValue(value["tab_ref"])
			if valid && tabID > 0 && tabRef != "" {
				next[tabRef] = browserTab{id: tabID, ref: tabRef}
			}
		}
		state.tabs = next
		if _, exists := next[state.activeTabRef]; !exists {
			ordered := orderedTabs(next)
			state.activeTabRef = ""
			if len(ordered) > 0 {
				state.activeTabRef = ordered[len(ordered)-1].ref
			}
		}
		if state.activeTabRef == "" {
			delete(s.sessions, sessionKey)
			return
		}
		s.sessions[sessionKey] = state
	case "close_tab", "close":
		tabRef := stringValue(params["tab_ref"])
		delete(state.tabs, tabRef)
		remaining := orderedTabs(state.tabs)
		if len(remaining) == 0 {
			delete(s.sessions, sessionKey)
			return
		}
		state.activeTabRef = remaining[len(remaining)-1].ref
		s.sessions[sessionKey] = state
	case "close_session":
		delete(s.sessions, sessionKey)
	}
}

func requireActiveTab(params *map[string]any, state browserSession, hasSession bool, action string) error {
	if !hasSession || state.activeTabRef == "" {
		return missingTabError(action)
	}
	setActiveTab(*params, state)
	return nil
}

func setActiveTab(params map[string]any, state browserSession) {
	tab, ok := state.tabs[state.activeTabRef]
	if !ok {
		return
	}
	params["tab_id"] = tab.id
	params["tab_ref"] = tab.ref
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

func optionalString(params map[string]any, key string) error {
	value, exists := params[key]
	if !exists {
		return nil
	}
	if _, ok := value.(string); !ok {
		return fmt.Errorf("%s 必须是字符串", key)
	}
	return nil
}

func optionalNonNegativeNumber(params map[string]any, key string) error {
	provided, err := optionalFiniteNumber(params, key)
	if err != nil || !provided {
		return err
	}
	value, _ := numberValue(params[key])
	if value < 0 {
		return fmt.Errorf("%s 不能小于 0", key)
	}
	return nil
}

func optionalFiniteNumber(params map[string]any, key string) (bool, error) {
	value, exists := params[key]
	if !exists {
		return false, nil
	}
	number, ok := numberValue(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return false, fmt.Errorf("%s 必须是有限数字", key)
	}
	params[key] = number
	return true, nil
}

func optionalBoundedInteger(params map[string]any, key string, minimum int64, maximum int64) error {
	value, exists := params[key]
	if !exists {
		return nil
	}
	integer, ok := integerValue(value)
	if !ok || integer < minimum || integer > maximum {
		return fmt.Errorf("%s 必须在 %d 到 %d 之间", key, minimum, maximum)
	}
	params[key] = integer
	return nil
}

func requiredPositiveInteger(params map[string]any, key string) error {
	value, ok := integerValue(params[key])
	if !ok || value <= 0 {
		return fmt.Errorf("%s 需要正整数", key)
	}
	params[key] = value
	return nil
}

func optionalPoint(params map[string]any, xKey string, yKey string) (bool, error) {
	_, hasX := params[xKey]
	_, hasY := params[yKey]
	if hasX != hasY {
		return false, fmt.Errorf("%s 和 %s 必须同时提供", xKey, yKey)
	}
	if !hasX {
		return false, nil
	}
	if _, err := optionalFiniteNumber(params, xKey); err != nil {
		return false, err
	}
	if _, err := optionalFiniteNumber(params, yKey); err != nil {
		return false, err
	}
	return true, nil
}

func requirePoint(params map[string]any, action string, xKey string, yKey string) error {
	provided, err := optionalPoint(params, xKey, yKey)
	if err != nil {
		return err
	}
	if !provided {
		return fmt.Errorf("%s 需要 %s 和 %s", action, xKey, yKey)
	}
	return nil
}

func normalizePointer(params map[string]any, action string, xKey string, yKey string) error {
	selector := selectorValue(params)
	pointProvided, err := optionalPoint(params, xKey, yKey)
	if err != nil {
		return err
	}
	if selector == "" && !pointProvided {
		return fmt.Errorf("%s 需要 selector/ref 或 %s/%s", action, xKey, yKey)
	}
	if selector != "" {
		params["selector"] = selector
	}
	return nil
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

func orderedTabs(tabs map[string]browserTab) []browserTab {
	result := make([]browserTab, 0, len(tabs))
	for _, tab := range tabs {
		result = append(result, tab)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })
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
	case json.Number:
		if converted, err := typed.Int64(); err == nil {
			return converted, true
		}
		parsed, err := typed.Float64()
		converted := int64(parsed)
		return converted, err == nil && float64(converted) == parsed
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
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
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
