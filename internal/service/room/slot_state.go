package room

import (
	"slices"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *RealtimeService) finishSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.doneOnce.Do(func() {
		close(slot.Done)
	})
}

func (slot *activeRoomSlot) getStatus() string {
	if slot == nil {
		return ""
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return slot.Status
}

func (slot *activeRoomSlot) setStatus(status string) {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	slot.Status = status
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) isTerminal() bool {
	switch slot.getStatus() {
	case "finished", "error", "cancelled":
		return true
	default:
		return false
	}
}

func (slot *activeRoomSlot) setSDKSessionID(sessionID string) bool {
	if slot == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if sessionID == "" || sessionID == strings.TrimSpace(slot.SDKSessionID) {
		return false
	}
	slot.SDKSessionID = sessionID
	return true
}

func (slot *activeRoomSlot) clearSDKSessionID() bool {
	if slot == nil {
		return false
	}
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if strings.TrimSpace(slot.SDKSessionID) == "" {
		return false
	}
	slot.SDKSessionID = ""
	return true
}

func (slot *activeRoomSlot) getSDKSessionID() string {
	if slot == nil {
		return ""
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return strings.TrimSpace(slot.SDKSessionID)
}

func (slot *activeRoomSlot) setClient(client runtimectx.Client) {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	slot.Client = client
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) getClient() runtimectx.Client {
	if slot == nil {
		return nil
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return slot.Client
}

func (slot *activeRoomSlot) setInterruptReason(reason string) {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	slot.InterruptReason = reason
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) getInterruptReason() string {
	if slot == nil {
		return ""
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return strings.TrimSpace(slot.InterruptReason)
}

func normalizeRoomInterruptReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	return "Request stopped"
}

func markRoomSlotInterrupted(slot *activeRoomSlot, reason string) {
	if slot == nil {
		return
	}
	slot.setInterruptReason(normalizeRoomInterruptReason(reason))
}

func roomSlotInterruptReason(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return slot.getInterruptReason()
}

func (slot *activeRoomSlot) beginNoReplyCandidate() {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	slot.NoReplyCandidate = true
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) suppressOutput() {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	slot.SuppressOutput = true
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) shouldSuppressOutput() bool {
	if slot == nil {
		return false
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return slot.SuppressOutput
}

func (slot *activeRoomSlot) eventsReadyForEmission(event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if slot.SuppressOutput {
		slot.PendingStream = nil
		return nil
	}
	if slot.NoReplyCandidate {
		if event.EventType != protocol.EventTypeStream {
			slot.NoReplyCandidate = false
		} else if roomdomain.IsNoReplyCandidateStreamEvent(event) {
			slot.PendingStream = append(slot.PendingStream, event)
			return nil
		} else {
			slot.NoReplyCandidate = false
		}
	}
	if len(slot.PendingStream) == 0 {
		return []protocol.EventMessage{event}
	}
	events := slices.Clone(slot.PendingStream)
	slot.PendingStream = nil
	events = append(events, event)
	return events
}

func (slot *activeRoomSlot) markCancelled() bool {
	if slot == nil {
		return false
	}
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if slot.Status == "cancelled" {
		return false
	}
	slot.Status = "cancelled"
	return true
}

func (slot *activeRoomSlot) rememberGoalAssistantMessage(message protocol.Message) {
	if slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	slot.stateMu.Lock()
	slot.GoalLastAssistant = protocol.Clone(message)
	slot.stateMu.Unlock()
}

func (slot *activeRoomSlot) lastGoalAssistantMessage() protocol.Message {
	if slot == nil {
		return nil
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return protocol.Clone(slot.GoalLastAssistant)
}

func (slot *activeRoomSlot) rememberSubagentTaskMessage(message protocol.Message) {
	if slot == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(anyString(metadata["task_id"]))
	if taskID == "" {
		return
	}
	subtype := strings.TrimSpace(anyString(metadata["subtype"]))
	status := strings.TrimSpace(anyString(metadata["status"]))
	if subtype == "task_started" && !metadataLooksLikeSubagentTask(metadata) {
		return
	}
	if subtype == "task_updated" && !isTerminalSubagentTaskStatus(status) && !metadataLooksLikeSubagentTask(metadata) {
		return
	}
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if slot.SubagentTasks == nil {
		slot.SubagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_updated":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.SubagentTasks, taskID)
			return
		}
		slot.SubagentTasks[taskID] = struct{}{}
	case "task_notification":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.SubagentTasks, taskID)
		}
	}
}

func (slot *activeRoomSlot) hasRunningSubagentTask() bool {
	if slot == nil {
		return false
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return len(slot.SubagentTasks) > 0
}

func metadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	return strings.TrimSpace(anyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(anyString(metadata["agent_type"])) != "" ||
		strings.TrimSpace(anyString(metadata["task_type"])) == "local_agent"
}

func isTerminalSubagentTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "error", "stopped", "killed", "cancelled":
		return true
	default:
		return false
	}
}
