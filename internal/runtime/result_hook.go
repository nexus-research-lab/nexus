// INPUT: Physical-round result callback and SDK Stop lifecycle payload.
// OUTPUT: Warm-client routing with no ambiguous or child-round result writeback.
// POS: Runtime transport only; the service callback owns parsing, binding and durable intake.
package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SupportsWorkResultHooks limits rollout to negotiated nxs lifecycle/ACK support.
// Missing payloads still cannot produce a result: the service parser fails closed.
func SupportsWorkResultHooks(client Client) bool {
	capable, ok := client.(interface {
		Supports(agentclient.Capability) bool
	})
	return ok && capable.Supports(agentclient.CapabilityHookResponseAck) && SupportsMessageExecutionPolicy(client)
}

// SetWorkResultHook keys by the agent round id the declaration must echo.
func (m *Manager) SetWorkResultHook(sessionKey, roundID string, callback sdkhook.Callback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.WorkResultHooks == nil {
		state.WorkResultHooks = make(map[string]sdkhook.Callback)
	}
	state.WorkResultHooks[roundID] = callback
}

func (m *Manager) ClearWorkResultHook(sessionKey, roundID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.sessions[sessionKey]; state != nil {
		delete(state.WorkResultHooks, roundID)
	}
}

func (m *Manager) WithWorkResultHook(options agentclient.Options, sessionKey string) agentclient.Options {
	hooks := cloneSDKHooks(options.Hooks.Matchers)
	hooks[sdkhook.EventStop] = append(hooks[sdkhook.EventStop], sdkhook.Matcher{Timeout: 10 * time.Second, Hooks: []sdkhook.Callback{func(ctx context.Context, input sdkhook.Input, toolUseID string) (sdkhook.Output, error) {
		if input.AgentID != "" {
			return sdkhook.Output{}, nil
		}
		callback, ambiguous := m.workResultHookFor(sessionKey, input.LastAssistantMessage)
		if callback != nil {
			return callback(ctx, input, toolUseID)
		}
		if ambiguous {
			// Never drop an explicit declaration silently: the model must fall back
			// to the existing result tool instead of assuming writeback happened.
			return sdkhook.Output{Decision: "block", Reason: "WorkGraph result declaration could not be matched to this round. Inspect the current work state and use submit_work/review_work explicitly; do not assume delivery or acceptance."}, nil
		}
		return sdkhook.Output{}, nil
	}}})
	options.Hooks.Matchers = hooks
	return options
}

// workResultHookFor routes by the declaration's own round_id; the callback still
// validates the full payload. A lone registration keeps the previous fast path.
func (m *Manager) workResultHookFor(sessionKey, message string) (sdkhook.Callback, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil || len(state.WorkResultHooks) == 0 {
		return nil, false
	}
	if len(state.WorkResultHooks) == 1 {
		for _, callback := range state.WorkResultHooks {
			return callback, false
		}
	}
	_, payload, found := strings.Cut(message, protocol.ExecutionResultMarker)
	if !found {
		return nil, false
	}
	var declared struct {
		RoundID string `json:"round_id"`
	}
	payload, _, _ = strings.Cut(payload, "-->")
	if json.Unmarshal([]byte(strings.TrimSpace(payload)), &declared) == nil {
		if callback := state.WorkResultHooks[declared.RoundID]; callback != nil {
			return callback, false
		}
	}
	return nil, true
}
