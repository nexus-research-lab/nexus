// INPUT: 每轮宿主事实快照（按会话与名称）、SDK 会话代次与 compact 边界。
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
// 会话以 runtime session key 为键，与 compact 观察者共用同一个键，避免两边各拼各的键而永远对不上。
type ContextAnnouncements struct {
	mu       sync.Mutex
	sessions map[string]*announcedFacts
}

// announcedFacts 绑定 SDK 会话代次：换成新的 SDK 会话说明模型看不到旧告知，需整体重来。
type announcedFacts struct {
	generation string
	facts      map[string]string
}

// Changed 在 content 与该会话上次告知的同名快照不同时记下新值并返回 true。
// generation 为 SDK session id；首轮尚未拿到 id 时为空，随后补上 id 视为同一会话而非新会话。
func (a *ContextAnnouncements) Changed(sessionKey string, generation string, name string, content string) bool {
	if a == nil {
		return content != ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entry(sessionKey, strings.TrimSpace(generation))
	if entry.facts[name] == content {
		return false
	}
	entry.facts[name] = content
	return content != ""
}

func (a *ContextAnnouncements) entry(sessionKey string, generation string) *announcedFacts {
	if a.sessions == nil {
		a.sessions = map[string]*announcedFacts{}
	}
	entry := a.sessions[sessionKey]
	if entry == nil || (entry.generation != "" && generation != "" && entry.generation != generation) {
		entry = &announcedFacts{generation: generation, facts: map[string]string{}}
		a.sessions[sessionKey] = entry
	} else if entry.generation == "" {
		entry.generation = generation
	}
	return entry
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
