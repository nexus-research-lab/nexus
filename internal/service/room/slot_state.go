// INPUT: Room Agent slot 的 runtime、Goal、cursor 与 delivery 子状态。
// OUTPUT: 各领域独立同步的 slot 状态快照、普通输入 drain 与客户端绑定。
// POS: 单个 Room Agent 执行槽的状态所有者。
package room

import (
	"context"
	"slices"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (slot *activeRoomSlot) bindConversationState(conversationID string, state *roomConversationState) {
	if slot == nil {
		return
	}
	slot.conversation.mu.Lock()
	if slot.conversation.state == nil || slot.conversation.state == state {
		// slot 的 conversation 归属只在首次注册时建立；后续 ACK/cleanup
		// 必须继续使用同一 shard，不能被迟到的 location 覆盖。
		slot.conversation.id = strings.TrimSpace(conversationID)
		slot.conversation.state = state
	}
	slot.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) clearConversationState(expected *roomConversationState) {
	if slot == nil {
		return
	}
	slot.conversation.mu.Lock()
	if expected == nil || slot.conversation.state == expected {
		slot.conversation.id = ""
		slot.conversation.state = nil
	}
	slot.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) conversationBinding() (string, *roomConversationState) {
	if slot == nil {
		return "", nil
	}
	slot.conversation.mu.RLock()
	defer slot.conversation.mu.RUnlock()
	return slot.conversation.id, slot.conversation.state
}

func (s *RealtimeService) finishSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	s.forgetRoomSlotGuidance(slot)
	slot.closeDone()
}

func (slot *activeRoomSlot) getStatus() string {
	if slot == nil {
		return ""
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return slot.runtime.status
}

func (slot *activeRoomSlot) setStatus(status string) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.status = status
	slot.runtime.mu.Unlock()
}

// setErrorMessage 保存 slot 的终态原因，供 root round 收口时重放给前端。
func (slot *activeRoomSlot) setErrorMessage(message string) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.errorMessage = strings.TrimSpace(message)
	slot.runtime.mu.Unlock()
}

// getErrorMessage 读取 slot 的终态原因。
func (slot *activeRoomSlot) getErrorMessage() string {
	if slot == nil {
		return ""
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.runtime.errorMessage)
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
	slot.runtime.mu.Lock()
	defer slot.runtime.mu.Unlock()
	if sessionID == "" || sessionID == strings.TrimSpace(slot.runtime.sdkSessionID) {
		return false
	}
	slot.runtime.sdkSessionID = sessionID
	return true
}

func (slot *activeRoomSlot) clearSDKSessionID() bool {
	if slot == nil {
		return false
	}
	slot.runtime.mu.Lock()
	defer slot.runtime.mu.Unlock()
	if strings.TrimSpace(slot.runtime.sdkSessionID) == "" {
		return false
	}
	slot.runtime.sdkSessionID = ""
	return true
}

func (slot *activeRoomSlot) getSDKSessionID() string {
	if slot == nil {
		return ""
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.runtime.sdkSessionID)
}

func (slot *activeRoomSlot) drainQueuedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	if len(slot.delivery.queuedInputs) == 0 {
		return nil
	}
	inputs := slices.Clone(slot.delivery.queuedInputs)
	slot.delivery.queuedInputs = nil
	return inputs
}

func (slot *activeRoomSlot) setDeliveryMetadata(
	replyRoute protocol.RoomReplyRoute,
	replySourceMessage string,
	handoffID string,
) {
	if slot == nil {
		return
	}
	slot.delivery.mu.Lock()
	slot.delivery.replyRoute = replyRoute
	slot.delivery.replySourceMessage = strings.TrimSpace(replySourceMessage)
	slot.delivery.handoffID = strings.TrimSpace(handoffID)
	slot.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) replyRoute() protocol.RoomReplyRoute {
	if slot == nil {
		return protocol.RoomReplyRoute{}
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	return slot.delivery.replyRoute
}

func (slot *activeRoomSlot) replySourceMessage() string {
	if slot == nil {
		return ""
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	return slot.delivery.replySourceMessage
}

func (slot *activeRoomSlot) handoffID() string {
	if slot == nil {
		return ""
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	return slot.delivery.handoffID
}

func (slot *activeRoomSlot) setCursors(publicID string, publicTimestamp int64, messageID string, messageTimestamp int64) {
	if slot == nil {
		return
	}
	slot.cursor.mu.Lock()
	slot.cursor.publicID = strings.TrimSpace(publicID)
	slot.cursor.publicTimestamp = publicTimestamp
	slot.cursor.messageID = strings.TrimSpace(messageID)
	slot.cursor.messageTimestamp = messageTimestamp
	slot.cursor.mu.Unlock()
}

func (slot *activeRoomSlot) publicCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.cursor.mu.RLock()
	defer slot.cursor.mu.RUnlock()
	return slot.cursor.publicID, slot.cursor.publicTimestamp
}

func (slot *activeRoomSlot) messageCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.cursor.mu.RLock()
	defer slot.cursor.mu.RUnlock()
	return slot.cursor.messageID, slot.cursor.messageTimestamp
}

func (slot *activeRoomSlot) cursorSnapshot() (string, int64, string, int64) {
	if slot == nil {
		return "", 0, "", 0
	}
	slot.cursor.mu.RLock()
	defer slot.cursor.mu.RUnlock()
	return slot.cursor.publicID, slot.cursor.publicTimestamp, slot.cursor.messageID, slot.cursor.messageTimestamp
}

func (slot *activeRoomSlot) setClient(client runtimectx.Client) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.client = client
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getClient() runtimectx.Client {
	if slot == nil {
		return nil
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return slot.runtime.client
}

func (slot *activeRoomSlot) setResultUsageWritten() {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.resultUsageWritten = true
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) resultUsageWasWritten() bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.resultUsageWritten
}

func (slot *activeRoomSlot) setCancel(cancel context.CancelFunc) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.cancel = cancel
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) cancelRuntime() {
	if slot == nil {
		return
	}
	slot.runtime.mu.RLock()
	cancel := slot.runtime.cancel
	slot.runtime.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (slot *activeRoomSlot) setRuntimeKind(runtimeKind string) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.runtimeKind = strings.TrimSpace(runtimeKind)
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) runtimeKind() string {
	if slot == nil {
		return ""
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.runtime.runtimeKind)
}

func (slot *activeRoomSlot) setContextWindow(window int) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.contextWindow = window
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextWindow() int {
	if slot == nil {
		return 0
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return slot.runtime.contextWindow
}

func (slot *activeRoomSlot) setContextColdStart(coldStart bool) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.contextColdStart = coldStart
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextColdStart() bool {
	if slot == nil {
		return false
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return slot.runtime.contextColdStart
}

func (slot *activeRoomSlot) beginGoalUsage() {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.usage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.goal.idForUsage) != "")
	slot.goal.usageStartedAt = time.Now()
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalUsageAccumulator(usage *goalsvc.RuntimeUsageAccumulator) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.usage = usage
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) resetGoalUsageIfInactive(snapshot goalsvc.RuntimeUsageSnapshot) bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.Lock()
	defer slot.goal.mu.Unlock()
	if slot.goal.usage != nil && slot.goal.usage.Active() {
		return false
	}
	if slot.goal.usage == nil {
		slot.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.goal.usage.Reset(snapshot)
	return true
}

func (slot *activeRoomSlot) resetGoalUsage(snapshot goalsvc.RuntimeUsageSnapshot) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	if slot.goal.usage == nil {
		slot.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.goal.usage.Reset(snapshot)
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageDelta(snapshot goalsvc.RuntimeUsageSnapshot) (protocol.GoalUsage, bool, bool) {
	if slot == nil {
		return protocol.GoalUsage{}, false, false
	}
	slot.goal.mu.Lock()
	defer slot.goal.mu.Unlock()
	if slot.goal.usage == nil {
		return protocol.GoalUsage{}, false, false
	}
	usage, ok := slot.goal.usage.Delta(snapshot)
	return usage, ok, true
}

func (slot *activeRoomSlot) closeGoalUsage() {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	if slot.goal.usage != nil {
		slot.goal.usage.Close()
	}
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageStartedAt() time.Time {
	if slot == nil {
		return time.Time{}
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.usageStartedAt
}

func (slot *activeRoomSlot) setInterruptReason(reason string) {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	slot.runtime.interruptReason = reason
	slot.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getInterruptReason() string {
	if slot == nil {
		return ""
	}
	slot.runtime.mu.RLock()
	defer slot.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.runtime.interruptReason)
}

func (slot *activeRoomSlot) doneChannel() <-chan struct{} {
	if slot == nil {
		return nil
	}
	slot.runtime.mu.Lock()
	defer slot.runtime.mu.Unlock()
	if slot.runtime.done == nil {
		slot.runtime.done = make(chan struct{})
	}
	return slot.runtime.done
}

func (slot *activeRoomSlot) closeDone() {
	if slot == nil {
		return
	}
	slot.runtime.mu.Lock()
	if slot.runtime.done == nil {
		slot.runtime.done = make(chan struct{})
	}
	slot.runtime.doneOnce.Do(func() { close(slot.runtime.done) })
	slot.runtime.mu.Unlock()
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
	slot.delivery.mu.Lock()
	slot.delivery.noReplyCandidate = true
	slot.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) suppressOutput() {
	if slot == nil {
		return
	}
	slot.delivery.mu.Lock()
	slot.delivery.suppressOutput = true
	slot.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) publicMessageWasPublished() bool {
	if slot == nil {
		return false
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	return slot.delivery.publicMessagePublished
}

func (slot *activeRoomSlot) setPendingStream(events []protocol.EventMessage) {
	if slot == nil {
		return
	}
	slot.delivery.mu.Lock()
	slot.delivery.pendingStream = slices.Clone(events)
	slot.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) markPublicMessagePublished() {
	if slot == nil {
		return
	}
	slot.delivery.mu.Lock()
	slot.delivery.publicMessagePublished = true
	slot.delivery.suppressOutput = true
	slot.delivery.pendingStream = nil
	slot.delivery.noReplyCandidate = false
	slot.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) shouldSuppressOutput() bool {
	if slot == nil {
		return false
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	return slot.delivery.suppressOutput
}

func (slot *activeRoomSlot) eventsReadyForEmission(event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	slot.delivery.mu.Lock()
	defer slot.delivery.mu.Unlock()
	if slot.delivery.suppressOutput {
		slot.delivery.pendingStream = nil
		return nil
	}
	if slot.delivery.noReplyCandidate {
		if event.EventType != protocol.EventTypeStream {
			slot.delivery.noReplyCandidate = false
		} else if roomdomain.IsNoReplyCandidateStreamEvent(event) {
			slot.delivery.pendingStream = append(slot.delivery.pendingStream, event)
			return nil
		} else {
			slot.delivery.noReplyCandidate = false
		}
	}
	if len(slot.delivery.pendingStream) == 0 {
		return []protocol.EventMessage{event}
	}
	events := slices.Clone(slot.delivery.pendingStream)
	slot.delivery.pendingStream = nil
	events = append(events, event)
	return events
}

func (slot *activeRoomSlot) markCancelled() bool {
	if slot == nil {
		return false
	}
	slot.runtime.mu.Lock()
	defer slot.runtime.mu.Unlock()
	if slot.runtime.status == "cancelled" {
		return false
	}
	slot.runtime.status = "cancelled"
	return true
}

func (slot *activeRoomSlot) rememberGoalAssistantMessage(message protocol.Message) {
	if slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.lastAssistant = protocol.Clone(message)
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) lastGoalAssistantMessage() protocol.Message {
	if slot == nil {
		return nil
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return protocol.Clone(slot.goal.lastAssistant)
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
	if !metadataLooksLikeSubagentTask(metadata) && !slot.knowsSubagentTask(taskID) {
		return
	}
	runtimeKind := slot.runtimeKind()
	slot.goal.mu.Lock()
	defer slot.goal.mu.Unlock()
	if runtimeKind != "" {
		metadata["runtime_kind"] = runtimeKind
	}
	slot.goal.subagentHistory = true
	if slot.goal.subagentTasks == nil {
		slot.goal.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.goal.subagentTasks, taskID)
			return
		}
		slot.goal.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.goal.subagentTasks, taskID)
		}
	}
}

func (slot *activeRoomSlot) knowsSubagentTask(taskID string) bool {
	if slot == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	_, ok := slot.goal.subagentTasks[strings.TrimSpace(taskID)]
	return ok
}

func (slot *activeRoomSlot) hasSubagentHistory() bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.subagentHistory
}

func (slot *activeRoomSlot) hasRunningSubagentTask() bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return len(slot.goal.subagentTasks) > 0
}

func (slot *activeRoomSlot) setSubagentTasks(tasks map[string]struct{}) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.subagentTasks = tasks
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) markGoalToolProgress() {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.toolProgress = true
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) hasGoalToolProgress() bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.toolProgress
}

func (slot *activeRoomSlot) goalContext() string {
	if slot == nil {
		return ""
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.context
}

func (slot *activeRoomSlot) goalIDForUsage() string {
	if slot == nil {
		return ""
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.idForUsage
}

func (slot *activeRoomSlot) goalSessionKey() string {
	if slot == nil {
		return ""
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.sessionKey
}

func (slot *activeRoomSlot) setGoalContext(contextText string) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.context = contextText
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalBinding(sessionKey string, goalID string) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.sessionKey = strings.TrimSpace(sessionKey)
	slot.goal.idForUsage = strings.TrimSpace(goalID)
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalRuntimeIgnored(ignored bool) {
	if slot == nil {
		return
	}
	slot.goal.mu.Lock()
	slot.goal.runtimeIgnored = ignored
	slot.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalRuntimeIgnored() bool {
	if slot == nil {
		return false
	}
	slot.goal.mu.RLock()
	defer slot.goal.mu.RUnlock()
	return slot.goal.runtimeIgnored
}

func metadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	taskType := strings.ToLower(strings.TrimSpace(anyString(metadata["task_type"])))
	if taskType == "local_shell" {
		return false
	}
	if taskType != "" {
		return taskType == "local_agent"
	}
	return strings.TrimSpace(anyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(anyString(metadata["agent_type"])) != ""
}

func isTerminalSubagentTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "error", "stopped", "killed", "cancelled":
		return true
	default:
		return false
	}
}
