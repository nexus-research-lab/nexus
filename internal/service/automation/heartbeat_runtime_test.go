// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package automation

import (
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
)

func (s *Service) recordWakeRequest(agentID string, sessionKey string, wakeMode string, text *string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionKey = strings.TrimSpace(sessionKey)
	request := automationexec.HeartbeatWakeRequest{
		AgentID:    strings.TrimSpace(agentID),
		SessionKey: sessionKey,
		WakeMode:   strings.TrimSpace(wakeMode),
		Text:       strings.TrimSpace(anyStringPointer(text)),
	}
	s.wakeRequests[sessionKey] = append(s.wakeRequests[sessionKey], request)
	if state := s.heartbeatState[request.AgentID]; state != nil {
		state.PendingWake = true
	}
}
