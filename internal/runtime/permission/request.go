package permission

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

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

// PendingRequest 表示一个待确认权限请求。
type PendingRequest struct {
	RequestID          string
	SessionKey         string
	DispatchSessionKey string
	ToolName           string
	ToolInput          map[string]any
	ToolUseID          string
	Suggestions        []sdkpermission.Update
	ExpiresAt          time.Time
	Route              RouteContext
	ResponseCh         chan sdkpermission.Decision
	finalizeOnce       sync.Once
}

func (c *Context) newPendingRequest(sessionKey string, request sdkpermission.Request) *PendingRequest {
	route := c.resolveRouteContext(sessionKey)
	now := time.Now()
	return &PendingRequest{
		RequestID:          fmt.Sprintf("perm_%d", now.UnixNano()),
		SessionKey:         sessionKey,
		DispatchSessionKey: firstNonEmpty(route.DispatchSessionKey, sessionKey),
		ToolName:           strings.TrimSpace(request.ToolName),
		ToolInput:          cloneMap(request.Input),
		ToolUseID:          strings.TrimSpace(request.ToolUseID),
		Suggestions:        slices.Clone(request.PermissionSuggestions),
		ExpiresAt:          now.Add(c.requestTimeout),
		Route:              route,
		ResponseCh:         make(chan sdkpermission.Decision, 1),
	}
}

func (c *Context) resolveRouteContext(sessionKey string) RouteContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	route := c.sessionRoutes[sessionKey]
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

	for _, pending := range requests {
		c.dispatchPendingRequestToSender(pending, sender)
	}
}

func (c *Context) dispatchPendingRequest(pending *PendingRequest) {
	if pending == nil {
		return
	}
	event := buildPermissionEvent(pending)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.BroadcastEvent(ctx, pending.DispatchSessionKey, event)
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
	delete(c.pendingRequests, requestID)
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
	pending *PendingRequest,
	message map[string]any,
) sdkpermission.Decision {
	decision := strings.TrimSpace(normalizeString(message["decision"]))
	if decision == "allow" {
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
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]sdkpermission.Update, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
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
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]sdkpermission.RuleValue, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
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
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if ok {
			result = append(result, payload)
		}
	}
	return result
}

func normalizeStringSlice(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(normalizeString(item))
		if value != "" {
			result = append(result, value)
		}
	}
	return result
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
