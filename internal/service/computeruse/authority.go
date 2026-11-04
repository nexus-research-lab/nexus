// INPUT: host-requested target scope and runtime capabilities.
// OUTPUT: exact application authority displayed before session creation.
// POS: round-bounded CUA selection authorization; drivers never infer consent.
package computeruse

import (
	"errors"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

// TargetOptions requests an explicit subset; an empty Actions list requests the
// currently exposed runtime actions, all shown in the approval prompt.
type TargetOptions struct {
	Actions              []nexuscua.ActionKind `json:"allowed_actions"`
	AllowForegroundInput bool                  `json:"allow_foreground_input"`
	TTLSeconds           uint32                `json:"ttl_seconds"`
}

// TargetGrant is immutable approval input. The resulting session cannot exceed it.
type TargetGrant struct {
	Application          nexuscua.DiscoveredApplication `json:"application"`
	Actions              []nexuscua.ActionKind          `json:"allowed_actions"`
	AllowForegroundInput bool                           `json:"allow_foreground_input"`
	TTLSeconds           uint32                         `json:"ttl_seconds"`
	ExpiresAt            time.Time                      `json:"expires_at"`
}

func (r *Round) targetGrant(app nexuscua.DiscoveredApplication, caps nexuscua.DriverCapabilities, options TargetOptions) (TargetGrant, error) {
	available := supportedRoundActions(caps.Actions)
	requested := options.Actions
	if len(requested) == 0 {
		requested = available
	}
	allowed := make(map[nexuscua.ActionKind]bool, len(available))
	for _, kind := range available {
		allowed[kind] = true
	}
	actions := make([]nexuscua.ActionKind, 0, len(requested))
	for _, kind := range requested {
		if !allowed[kind] {
			return TargetGrant{}, errors.New("Computer Use 请求了不支持或重复的动作类别")
		}
		allowed[kind] = false
		actions = append(actions, kind)
	}
	ttl := options.TTLSeconds
	if ttl == 0 {
		ttl = defaultSessionTTL
	}
	if ttl > defaultSessionTTL {
		ttl = defaultSessionTTL
	}
	if caps.ActionLimits.MaxSessionTTLSeconds == 0 {
		return TargetGrant{}, errors.New("Runtime 未提供有效的 session 上限")
	}
	if ttl > caps.ActionLimits.MaxSessionTTLSeconds {
		ttl = caps.ActionLimits.MaxSessionTTLSeconds
	}
	expires := time.Now().Add(time.Duration(ttl) * time.Second)
	r.mu.Lock()
	deadline := r.deadline
	r.mu.Unlock()
	if !deadline.IsZero() && deadline.Before(expires) {
		expires = deadline
	}
	if time.Until(expires) < time.Second {
		return TargetGrant{}, errors.New("Computer Use 回合授权时限已到，请开始新回合")
	}
	return TargetGrant{Application: app, Actions: actions, AllowForegroundInput: options.AllowForegroundInput,
		TTLSeconds: uint32(time.Until(expires) / time.Second), ExpiresAt: expires}, nil
}
