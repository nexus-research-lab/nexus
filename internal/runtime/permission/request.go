// INPUT: runtime 权限请求、session route 与当前 pending 集合。
// OUTPUT: 可阻塞等待响应，向 round/Room 投影状态并按请求身份稳定重放的 pending 生命周期。
// POS: runtime permission 的请求登记、重连恢复与响应收口入口。
package permission

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// RouteContext 描述运行时 session 到前端路由会话的映射。
type RouteContext struct {
	DispatchSessionKey string
	RoomID             string
	ConversationID     string
	AgentID            string
	MessageID          string
	RoundID            string
	AgentRoundID       string
}

// PendingRequest 表示一个会阻塞 runtime、等待用户响应的请求。
type PendingRequest struct {
	RequestID                string
	SessionKey               string
	DispatchSessionKey       string
	ToolName                 string
	ToolInput                map[string]any
	ConfigurationSecretSlots []secretinput.Slot
	ToolUseID                string
	Suggestions              []sdkpermission.Update
	CreatedAt                time.Time
	ExpiresAt                time.Time
	Route                    RouteContext
	ResponseCh               chan sdkpermission.Decision
	finalizeOnce             sync.Once
}

func (c *Context) newPendingRequest(sessionKey string, request sdkpermission.Request) *PendingRequest {
	route := c.resolveRouteContext(sessionKey)
	now := time.Now()
	toolName := strings.TrimSpace(request.ToolName)
	toolInput := secretinput.RedactConfigurationToolInput(toolName, request.Input)
	return &PendingRequest{
		RequestID:          fmt.Sprintf("perm_%d", now.UnixNano()),
		SessionKey:         sessionKey,
		DispatchSessionKey: firstNonEmpty(route.DispatchSessionKey, sessionKey),
		ToolName:           toolName,
		ToolInput:          toolInput,
		ConfigurationSecretSlots: secretinput.SlotsFromToolInput(
			toolName,
			toolInput,
		),
		ToolUseID:   strings.TrimSpace(request.ToolUseID),
		Suggestions: slices.Clone(request.PermissionSuggestions),
		CreatedAt:   now,
		ExpiresAt:   now.Add(c.requestTimeout),
		Route:       route,
		ResponseCh:  make(chan sdkpermission.Decision, 1),
	}
}

func (c *Context) resolveRouteContext(sessionKey string) RouteContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	route := c.sessionRoutes[sessionKey].route
	if route.DispatchSessionKey == "" {
		route.DispatchSessionKey = sessionKey
	}
	return route
}

func (c *Context) replayPendingRequestsToSender(sessionKey string, sender Sender) {
	if sender == nil || sender.IsClosed() {
		return
	}
	dispatchSessionKey := c.ResolveDispatchSessionKey(sessionKey)
	c.mu.RLock()
	requests := make([]*PendingRequest, 0)
	for _, pending := range c.pendingRequests {
		if pending.DispatchSessionKey == dispatchSessionKey {
			requests = append(requests, pending)
		}
	}
	c.mu.RUnlock()

	slices.SortFunc(requests, comparePendingRequests)
	for _, pending := range requests {
		c.dispatchPendingRequestToSender(pending, sender)
	}
}

// PendingRequestIDsForRoom 返回 Room 订阅恢复所需的权威人工交互快照。
// conversationID 为空时覆盖整个 Room，否则只返回指定会话。
func (c *Context) PendingRequestIDsForRoom(roomID string, conversationID string) []string {
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
	if roomID == "" {
		return []string{}
	}

	c.mu.RLock()
	requests := make([]*PendingRequest, 0)
	for _, pending := range c.pendingRequests {
		if strings.TrimSpace(pending.Route.RoomID) != roomID {
			continue
		}
		if conversationID != "" && strings.TrimSpace(pending.Route.ConversationID) != conversationID {
			continue
		}
		requests = append(requests, pending)
	}
	c.mu.RUnlock()

	slices.SortFunc(requests, comparePendingRequests)
	requestIDs := make([]string, 0, len(requests))
	for _, pending := range requests {
		requestIDs = append(requestIDs, pending.RequestID)
	}
	return requestIDs
}

func comparePendingRequests(left *PendingRequest, right *PendingRequest) int {
	if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
		return order
	}
	return strings.Compare(left.RequestID, right.RequestID)
}

func (c *Context) dispatchPendingRequest(pending *PendingRequest) {
	if pending == nil {
		return
	}
	event := buildPermissionEvent(pending)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.broadcastPermissionEvent(ctx, pending, event)
}

func (c *Context) dispatchPendingRequestToSender(pending *PendingRequest, sender Sender) {
	if pending == nil || sender == nil || sender.IsClosed() {
		return
	}
	event := buildPermissionEvent(pending)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = sender.SendEvent(ctx, event)
}

func (c *Context) cleanupRequest(requestID string) {
	c.mu.Lock()
	if _, ok := c.pendingRequests[requestID]; ok {
		delete(c.pendingRequests, requestID)
		c.notifyPendingRequestsChangedLocked()
	}
	c.mu.Unlock()
}

func (c *Context) finalizeRequest(pending *PendingRequest, status string) {
	if pending == nil {
		return
	}
	pending.finalizeOnce.Do(func() {
		c.cleanupRequest(pending.RequestID)
		c.dispatchPermissionResolution(pending, status)
	})
}

func (c *Context) buildPermissionDecision(
	ctx context.Context,
	pending *PendingRequest,
	message map[string]any,
) sdkpermission.Decision {
	decision := strings.TrimSpace(normalizeString(message["decision"]))
	configurationSecrets := normalizeConfigurationSecrets(message["configuration_secrets"])
	delete(message, "configuration_secrets")
	defer clear(configurationSecrets)
	if decision == "allow" {
		if isRecordedHumanApprovalTool(pending.ToolName) {
			c.mu.RLock()
			recorder := c.approvalRecorder
			c.mu.RUnlock()
			if recorder == nil {
				return sdkpermission.Deny(
					"高风险控制面未装配人工批准记录器；本次操作已拒绝",
					false,
				)
			}
			err := recorder.RecordHumanToolApproval(ctx, HumanToolApproval{
				PermissionRequestID:  pending.RequestID,
				ToolName:             pending.ToolName,
				ToolInput:            cloneMap(pending.ToolInput),
				ConfigurationSecrets: configurationSecrets,
				ConfigurationSecretSlots: slices.Clone(
					pending.ConfigurationSecretSlots,
				),
				RuntimeSessionKey:  pending.SessionKey,
				DispatchSessionKey: pending.DispatchSessionKey,
				Route:              pending.Route,
				ExpiresAt:          pending.ExpiresAt,
			})
			if err != nil {
				return sdkpermission.Deny(
					"批准意图、权限或版本已经变化；请重新检查后再确认",
					false,
				)
			}
		} else if len(configurationSecrets) != 0 {
			return sdkpermission.Deny(
				"该操作未声明安全配置输入；已拒绝额外 secret",
				false,
			)
		}
		updatedInput := cloneMap(pending.ToolInput)
		if pending.ToolName == "AskUserQuestion" {
			if answers := buildQuestionAnswers(
				pending.ToolInput,
				normalizeListOfMaps(message["user_answers"]),
			); len(answers) > 0 {
				updatedInput["answers"] = answers
			}
		}
		return sdkpermission.Allow(
			updatedInput,
			deserializePermissionUpdates(message["updated_permissions"]),
		)
	}
	return sdkpermission.Deny(
		firstNonEmpty(normalizeString(message["message"]), "User denied permission"),
		normalizeBool(message["interrupt"]),
	)
}

func normalizeConfigurationSecrets(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok || len(raw) == 0 || len(raw) > 32 {
		return nil
	}
	result := make(map[string]string, len(raw))
	total := 0
	for key, item := range raw {
		key = strings.TrimSpace(key)
		text, textOK := item.(string)
		if !textOK || key == "" || len(key) > 64 || len(text) > 64<<10 {
			clear(result)
			return nil
		}
		total += len(text)
		if total > 256<<10 {
			clear(result)
			return nil
		}
		result[key] = text
	}
	return result
}

func buildPermissionEvent(pending *PendingRequest) protocol.EventMessage {
	data := buildPermissionPayload(pending)
	event := protocol.NewEvent(protocol.EventTypePermissionRequest, data)
	event.SessionKey = pending.DispatchSessionKey
	event.RoomID = strings.TrimSpace(pending.Route.RoomID)
	event.ConversationID = strings.TrimSpace(pending.Route.ConversationID)
	event.AgentID = firstNonEmpty(pending.Route.AgentID, agentIDFromSessionKey(pending.SessionKey))
	event.MessageID = strings.TrimSpace(pending.Route.MessageID)
	event.RoundID = strings.TrimSpace(pending.Route.RoundID)
	event.AgentRoundID = strings.TrimSpace(pending.Route.AgentRoundID)
	return event
}

func (c *Context) dispatchPermissionResolution(pending *PendingRequest, status string) {
	if pending == nil {
		return
	}
	event := protocol.NewPermissionRequestResolvedEvent(
		pending.DispatchSessionKey,
		pending.RequestID,
		status,
	)
	event.RoomID = strings.TrimSpace(pending.Route.RoomID)
	event.ConversationID = strings.TrimSpace(pending.Route.ConversationID)
	event.AgentID = firstNonEmpty(pending.Route.AgentID, agentIDFromSessionKey(pending.SessionKey))
	event.MessageID = strings.TrimSpace(pending.Route.MessageID)
	event.RoundID = strings.TrimSpace(pending.Route.RoundID)
	event.AgentRoundID = strings.TrimSpace(pending.Route.AgentRoundID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.broadcastPermissionEvent(ctx, pending, event)
}

func (c *Context) broadcastPermissionEvent(
	ctx context.Context,
	pending *PendingRequest,
	event protocol.EventMessage,
) {
	c.mu.RLock()
	broadcaster := c.roomBroadcaster
	c.mu.RUnlock()
	if broadcaster != nil && strings.TrimSpace(pending.Route.RoomID) != "" {
		roomEvent := event
		roomEvent.DeliveryMode = protocol.DeliveryModeDurable
		_ = broadcaster.Broadcast(ctx, pending.Route.RoomID, roomEvent)
	}
	_ = c.BroadcastEvent(ctx, pending.DispatchSessionKey, event)
}

func buildQuestionAnswers(input map[string]any, userAnswers []map[string]any) map[string]string {
	rawQuestions, _ := input["questions"].([]any)
	if len(rawQuestions) == 0 {
		return nil
	}

	answers := make(map[string]string)
	for _, row := range userAnswers {
		questionIndex := normalizeInt(row["question_index"])
		if questionIndex < 0 || questionIndex >= len(rawQuestions) {
			continue
		}
		questionPayload, _ := rawQuestions[questionIndex].(map[string]any)
		questionText := strings.TrimSpace(normalizeString(questionPayload["question"]))
		if questionText == "" {
			continue
		}
		selectedOptions := normalizeStringSlice(row["selected_options"])
		if len(selectedOptions) == 0 {
			continue
		}
		answers[questionText] = strings.Join(selectedOptions, ", ")
	}
	return answers
}

func deserializePermissionUpdates(raw any) []sdkpermission.Update {
	items := normalizeListOfMaps(raw)
	result := make([]sdkpermission.Update, 0, len(items))
	for _, payload := range items {
		updateType := normalizeString(payload["type"])
		if updateType == "" {
			continue
		}
		update := sdkpermission.Update{
			Type:        updateType,
			Behavior:    sdkpermission.Behavior(normalizeString(payload["behavior"])),
			Mode:        sdkpermission.Mode(normalizeString(payload["mode"])),
			Destination: sdkpermission.UpdateDestination(normalizeString(payload["destination"])),
		}
		update.Directories = normalizeStringSlice(payload["directories"])
		update.Rules = deserializePermissionRules(payload["rules"])
		result = append(result, update)
	}
	return result
}

func deserializePermissionRules(raw any) []sdkpermission.RuleValue {
	items := normalizeListOfMaps(raw)
	result := make([]sdkpermission.RuleValue, 0, len(items))
	for _, payload := range items {
		toolName := firstNonEmpty(normalizeString(payload["tool_name"]), normalizeString(payload["toolName"]))
		if toolName == "" {
			continue
		}
		result = append(result, sdkpermission.RuleValue{
			ToolName:    toolName,
			RuleContent: firstNonEmpty(normalizeString(payload["rule_content"]), normalizeString(payload["ruleContent"])),
		})
	}
	return result
}

func normalizeListOfMaps(raw any) []map[string]any {
	switch items := raw.(type) {
	case []map[string]any:
		return items
	case []any:
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			payload, ok := item.(map[string]any)
			if ok {
				result = append(result, payload)
			}
		}
		return result
	default:
		return nil
	}
}

func normalizeStringSlice(raw any) []string {
	switch items := raw.(type) {
	case []string:
		return slices.Clone(items)
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			value := strings.TrimSpace(normalizeString(item))
			if value != "" {
				result = append(result, value)
			}
		}
		return result
	default:
		return nil
	}
}

func cloneMap(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	return maps.Clone(raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func normalizeBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func normalizeInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func agentIDFromSessionKey(sessionKey string) string {
	parsed := protocol.ParseSessionKey(sessionKey)
	return parsed.AgentID
}
