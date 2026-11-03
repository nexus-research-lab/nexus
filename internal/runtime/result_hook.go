// INPUT: Physical-round result callback and SDK Stop lifecycle payload.
// OUTPUT: Warm-client routing with no ambiguous or child-round result writeback.
// POS: Runtime transport only; the service callback owns parsing, binding and durable intake.
package runtime

import (
	"context"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

// SupportsWorkResultHooks limits rollout to negotiated nxs lifecycle/ACK support.
// Missing payloads still cannot produce a result: the service parser fails closed.
func SupportsWorkResultHooks(client Client) bool {
	capable, ok := client.(interface {
		Supports(agentclient.Capability) bool
	})
	return ok && capable.Supports(agentclient.CapabilityHookResponseAck) && SupportsMessageExecutionPolicy(client)
}

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
		m.mu.Lock()
		state := m.sessions[sessionKey]
		var callback sdkhook.Callback
		if state != nil && len(state.WorkResultHooks) == 1 && input.AgentID == "" {
			for _, candidate := range state.WorkResultHooks {
				callback = candidate
			}
		}
		m.mu.Unlock()
		if callback == nil {
			return sdkhook.Output{}, nil
		}
		return callback(ctx, input, toolUseID)
	}}})
	options.Hooks.Matchers = hooks
	return options
}
