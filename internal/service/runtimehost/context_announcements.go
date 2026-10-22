// INPUT: 每轮宿主事实快照（按会话与名称）与 SDK compact 边界。
// OUTPUT: 事实是否需要重新告知模型的判定。
// POS: 对齐 Claude Code 的 *_delta attachment：易变事实不进 system prompt，只在变化时作为本轮上下文追加一次；
// compact 会把早先的告知摘要掉，边界之后整体重新告知。
package runtimehost

import (
	"strings"
	"sync"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ContextAnnouncements 记录每个会话已告知模型的事实快照；宿主重启后清空，只会多告知一次，不会漏告知。
type ContextAnnouncements struct {
	mu       sync.Mutex
	sessions map[string]map[string]string
}

// Changed 在 content 与该会话上次告知的同名快照不同时记下新值并返回 true。
func (a *ContextAnnouncements) Changed(sessionKey string, name string, content string) bool {
	if a == nil {
		return content != ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	announced := a.sessions[sessionKey]
	if announced[name] == content {
		return false
	}
	if announced == nil {
		if a.sessions == nil {
			a.sessions = map[string]map[string]string{}
		}
		announced = map[string]string{}
		a.sessions[sessionKey] = announced
	}
	announced[name] = content
	return content != ""
}

// ForgetAfterCompact 在 compact 边界后清空该会话的告知记录。
func (a *ContextAnnouncements) ForgetAfterCompact(sessionKey string, incoming sdkprotocol.ReceivedMessage) {
	if a == nil || incoming.Type != sdkprotocol.MessageTypeSystem || incoming.System == nil ||
		strings.TrimSpace(incoming.System.Subtype) != "compact_boundary" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, sessionKey)
}
