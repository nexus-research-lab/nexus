// INPUT: 已认证 session、可选 pending request ID 与 allow/deny 决策。
// OUTPUT: 复用 Web 人工审批真相源的原子 runtime 决策结果；省略 ID 时只命中唯一请求。
// POS: 非 Web transport 将显式用户决定提交给 pending runtime 的窄入口。
package permission

import (
	"context"
	"slices"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// SessionPermissionResolution 描述一次 session-scoped 决策是否命中并完成。
type SessionPermissionResolution struct {
	Found                bool
	Resolved             bool
	Persisted            bool
	PersistenceSupported bool
	RequestID            string
	MatchingRequests     int
}

// CountSessionPermissionRequests 返回当前 dispatch session 与可选请求 ID
// 精确匹配的 pending runtime 权限请求数，不执行任何决定。
func (c *Context) CountSessionPermissionRequests(sessionKey string, requestID string) int {
	if c == nil {
		return 0
	}
	sessionKey = strings.TrimSpace(sessionKey)
	requestID = strings.TrimSpace(requestID)
	if sessionKey == "" {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if requestID != "" {
		pending := c.pendingRequests[requestID]
		if pending != nil && pending.DispatchSessionKey == sessionKey {
			return 1
		}
		return 0
	}
	matchingRequests := 0
	for _, pending := range c.pendingRequests {
		if pending.DispatchSessionKey == sessionKey {
			matchingRequests++
		}
	}
	return matchingRequests
}

// ResolveSessionPermissionRequest 把非 Web transport 的显式决定提交给同一 pending request。
// persist 只接受 SDK 为这个请求给出的权限更新建议，不修改 Nexus Agent 配置。
func (c *Context) ResolveSessionPermissionRequest(
	ctx context.Context,
	sessionKey string,
	requestID string,
	decision sdkpermission.Behavior,
	persist bool,
) SessionPermissionResolution {
	if c == nil {
		return SessionPermissionResolution{}
	}
	sessionKey = strings.TrimSpace(sessionKey)
	requestID = strings.TrimSpace(requestID)
	if sessionKey == "" {
		return SessionPermissionResolution{}
	}

	c.mu.RLock()
	var pending *PendingRequest
	matchingRequests := 0
	if requestID != "" {
		pending = c.pendingRequests[requestID]
		if pending != nil && pending.DispatchSessionKey == sessionKey {
			matchingRequests = 1
		} else {
			pending = nil
		}
	} else {
		for _, candidate := range c.pendingRequests {
			if candidate.DispatchSessionKey != sessionKey {
				continue
			}
			matchingRequests++
			pending = candidate
		}
	}
	if pending == nil || matchingRequests != 1 {
		c.mu.RUnlock()
		return SessionPermissionResolution{MatchingRequests: matchingRequests}
	}
	requestID = pending.RequestID
	suggestions := slices.Clone(pending.Suggestions)
	c.mu.RUnlock()

	result := SessionPermissionResolution{
		Found:                true,
		PersistenceSupported: len(suggestions) > 0,
		RequestID:            requestID,
		MatchingRequests:     1,
	}
	message := map[string]any{
		"request_id": requestID,
		"decision":   string(decision),
		"interrupt":  false,
	}
	switch decision {
	case sdkpermission.BehaviorAllow:
		message["message"] = "User allowed permission from paired IM"
		if persist {
			if len(suggestions) == 0 {
				return result
			}
			message["updated_permissions"] = serializePermissionUpdates(suggestions)
		}
	case sdkpermission.BehaviorDeny:
		message["message"] = "User denied permission from paired IM"
	default:
		return result
	}
	result.Resolved = c.HandlePermissionResponse(ctx, sessionKey, message)
	result.Persisted = result.Resolved && persist
	return result
}
