// INPUT: Room Agent slot 的 runtime、Goal、cursor 与 delivery 子状态。
// OUTPUT: 各领域独立同步的 slot 状态快照、普通输入 drain 与客户端绑定。
// POS: 单个 Room Agent 执行槽的状态所有者。
package realtime

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
	slot.mutable.conversation.mu.Lock()
	if slot.mutable.conversation.state == nil || slot.mutable.conversation.state == state {
		// slot 的 conversation 归属只在首次注册时建立；后续 ACK/cleanup
		// 必须继续使用同一 shard，不能被迟到的 location 覆盖。
		slot.mutable.conversation.id = strings.TrimSpace(conversationID)
		slot.mutable.conversation.state = state
	}
	slot.mutable.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) clearConversationState(expected *roomConversationState) {
	if slot == nil {
		return
	}
	slot.mutable.conversation.mu.Lock()
	if expected == nil || slot.mutable.conversation.state == expected {
		slot.mutable.conversation.id = ""
		slot.mutable.conversation.state = nil
	}
	slot.mutable.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) conversationBinding() (string, *roomConversationState) {
	if slot == nil {
		return "", nil
	}
	slot.mutable.conversation.mu.RLock()
	defer slot.mutable.conversation.mu.RUnlock()
	return slot.mutable.conversation.id, slot.mutable.conversation.state
}

func (s *Service) finishSlot(slot *activeRoomSlot) {
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
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.status
}

func (slot *activeRoomSlot) setStatus(status string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.status = status
	slot.mutable.runtime.mu.Unlock()
}

// setErrorMessage 保存 slot 的终态原因，供 root round 收口时重放给前端。
func (slot *activeRoomSlot) setErrorMessage(message string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.errorMessage = strings.TrimSpace(message)
	slot.mutable.runtime.mu.Unlock()
}

// getErrorMessage 读取 slot 的终态原因。
func (slot *activeRoomSlot) getErrorMessage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.errorMessage)
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
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if sessionID == "" || sessionID == strings.TrimSpace(slot.mutable.runtime.sdkSessionID) {
		return false
	}
	slot.mutable.runtime.sdkSessionID = sessionID
	return true
}

func (slot *activeRoomSlot) clearSDKSessionID() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if strings.TrimSpace(slot.mutable.runtime.sdkSessionID) == "" {
		return false
	}
	slot.mutable.runtime.sdkSessionID = ""
	return true
}

func (slot *activeRoomSlot) getSDKSessionID() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.sdkSessionID)
}

func (slot *activeRoomSlot) drainQueuedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	if len(slot.mutable.delivery.queuedInputs) == 0 {
		return nil
	}
	inputs := slices.Clone(slot.mutable.delivery.queuedInputs)
	slot.mutable.delivery.queuedInputs = nil
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
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.replyRoute = replyRoute
	slot.mutable.delivery.replySourceMessage = strings.TrimSpace(replySourceMessage)
	slot.mutable.delivery.handoffID = strings.TrimSpace(handoffID)
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) replyRoute() protocol.RoomReplyRoute {
	if slot == nil {
		return protocol.RoomReplyRoute{}
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.replyRoute
}

func (slot *activeRoomSlot) replySourceMessage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.replySourceMessage
}

func (slot *activeRoomSlot) handoffID() string {
	if slot == nil {
		return ""
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.handoffID
}

func (slot *activeRoomSlot) setCursors(publicID string, publicTimestamp int64, messageID string, messageTimestamp int64) {
	if slot == nil {
		return
	}
	slot.mutable.cursor.mu.Lock()
	slot.mutable.cursor.publicID = strings.TrimSpace(publicID)
	slot.mutable.cursor.publicTimestamp = publicTimestamp
	slot.mutable.cursor.messageID = strings.TrimSpace(messageID)
	slot.mutable.cursor.messageTimestamp = messageTimestamp
	slot.mutable.cursor.mu.Unlock()
}

func (slot *activeRoomSlot) publicCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.publicID, slot.mutable.cursor.publicTimestamp
}

func (slot *activeRoomSlot) messageCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.messageID, slot.mutable.cursor.messageTimestamp
}

func (slot *activeRoomSlot) cursorSnapshot() (string, int64, string, int64) {
	if slot == nil {
		return "", 0, "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.publicID, slot.mutable.cursor.publicTimestamp, slot.mutable.cursor.messageID, slot.mutable.cursor.messageTimestamp
}

func (slot *activeRoomSlot) setClient(client runtimectx.Client) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.client = client
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getClient() runtimectx.Client {
	if slot == nil {
		return nil
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.client
}

func (slot *activeRoomSlot) setResultUsageWritten() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.resultUsageWritten = true
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) resultUsageWasWritten() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.resultUsageWritten
}

func (slot *activeRoomSlot) setCancel(cancel context.CancelFunc) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.cancel = cancel
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) cancelRuntime() {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.RLock()
	cancel := slot.mutable.runtime.cancel
	slot.mutable.runtime.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (slot *activeRoomSlot) setRuntimeKind(runtimeKind string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.runtimeKind = strings.TrimSpace(runtimeKind)
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) runtimeKind() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.runtimeKind)
}

func (slot *activeRoomSlot) setContextWindow(window int) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.contextWindow = window
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextWindow() int {
	if slot == nil {
		return 0
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.contextWindow
}

func (slot *activeRoomSlot) setContextColdStart(coldStart bool) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.contextColdStart = coldStart
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextColdStart() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.contextColdStart
}

func (slot *activeRoomSlot) beginGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.mutable.goal.idForUsage) != "")
	slot.mutable.goal.usageStartedAt = time.Now()
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalUsageAccumulator(usage *goalsvc.RuntimeUsageAccumulator) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usage = usage
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) resetGoalUsageIfInactive(snapshot goalsvc.RuntimeUsageSnapshot) bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usage != nil && slot.mutable.goal.usage.Active() {
		return false
	}
	if slot.mutable.goal.usage == nil {
		slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.mutable.goal.usage.Reset(snapshot)
	return true
}

func (slot *activeRoomSlot) resetGoalUsage(snapshot goalsvc.RuntimeUsageSnapshot) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.usage == nil {
		slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.mutable.goal.usage.Reset(snapshot)
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageDelta(snapshot goalsvc.RuntimeUsageSnapshot) (protocol.GoalUsage, bool, bool) {
	if slot == nil {
		return protocol.GoalUsage{}, false, false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usage == nil {
		return protocol.GoalUsage{}, false, false
	}
	usage, ok := slot.mutable.goal.usage.Delta(snapshot)
	return usage, ok, true
}

func (slot *activeRoomSlot) closeGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.usage != nil {
		slot.mutable.goal.usage.Close()
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageStartedAt() time.Time {
	if slot == nil {
		return time.Time{}
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.usageStartedAt
}

func (slot *activeRoomSlot) setInterruptReason(reason string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.interruptReason = reason
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getInterruptReason() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.interruptReason)
}

func (slot *activeRoomSlot) doneChannel() <-chan struct{} {
	if slot == nil {
		return nil
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if slot.mutable.runtime.done == nil {
		slot.mutable.runtime.done = make(chan struct{})
	}
	return slot.mutable.runtime.done
}

func (slot *activeRoomSlot) closeDone() {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	if slot.mutable.runtime.done == nil {
		slot.mutable.runtime.done = make(chan struct{})
	}
	slot.mutable.runtime.doneOnce.Do(func() { close(slot.mutable.runtime.done) })
	slot.mutable.runtime.mu.Unlock()
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
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.noReplyCandidate = true
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) suppressOutput() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.suppressOutput = true
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) publicMessageWasPublished() bool {
	if slot == nil {
		return false
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.publicMessagePublished
}

func (slot *activeRoomSlot) setPendingStream(events []protocol.EventMessage) {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.pendingStream = slices.Clone(events)
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) markPublicMessagePublished() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.publicMessagePublished = true
	slot.mutable.delivery.suppressOutput = true
	slot.mutable.delivery.pendingStream = nil
	slot.mutable.delivery.noReplyCandidate = false
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) shouldSuppressOutput() bool {
	if slot == nil {
		return false
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.suppressOutput
}

func (slot *activeRoomSlot) eventsReadyForEmission(event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	if slot.mutable.delivery.suppressOutput {
		slot.mutable.delivery.pendingStream = nil
		return nil
	}
	if slot.mutable.delivery.noReplyCandidate {
		if event.EventType != protocol.EventTypeStream {
			slot.mutable.delivery.noReplyCandidate = false
		} else if roomdomain.IsNoReplyCandidateStreamEvent(event) {
			slot.mutable.delivery.pendingStream = append(slot.mutable.delivery.pendingStream, event)
			return nil
		} else {
			slot.mutable.delivery.noReplyCandidate = false
		}
	}
	if len(slot.mutable.delivery.pendingStream) == 0 {
		return []protocol.EventMessage{event}
	}
	events := slices.Clone(slot.mutable.delivery.pendingStream)
	slot.mutable.delivery.pendingStream = nil
	events = append(events, event)
	return events
}

func (slot *activeRoomSlot) markCancelled() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if slot.mutable.runtime.status == "cancelled" {
		return false
	}
	slot.mutable.runtime.status = "cancelled"
	return true
}

func (slot *activeRoomSlot) rememberGoalAssistantMessage(message protocol.Message) {
	if slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.lastAssistant = protocol.Clone(message)
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) lastGoalAssistantMessage() protocol.Message {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return protocol.Clone(slot.mutable.goal.lastAssistant)
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
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if runtimeKind != "" {
		metadata["runtime_kind"] = runtimeKind
	}
	slot.mutable.goal.subagentHistory = true
	if slot.mutable.goal.subagentTasks == nil {
		slot.mutable.goal.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.mutable.goal.subagentTasks, taskID)
			return
		}
		slot.mutable.goal.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.mutable.goal.subagentTasks, taskID)
		}
	}
}

func (slot *activeRoomSlot) knowsSubagentTask(taskID string) bool {
	if slot == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	_, ok := slot.mutable.goal.subagentTasks[strings.TrimSpace(taskID)]
	return ok
}

func (slot *activeRoomSlot) hasSubagentHistory() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.subagentHistory
}

func (slot *activeRoomSlot) hasRunningSubagentTask() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return len(slot.mutable.goal.subagentTasks) > 0
}

func (slot *activeRoomSlot) setSubagentTasks(tasks map[string]struct{}) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.subagentTasks = tasks
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) markGoalToolProgress() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.toolProgress = true
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) hasGoalToolProgress() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.toolProgress
}

func (slot *activeRoomSlot) goalContext() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.context
}

func (slot *activeRoomSlot) goalIDForUsage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.idForUsage
}

func (slot *activeRoomSlot) goalSessionKey() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.sessionKey
}

func (slot *activeRoomSlot) setGoalContext(contextText string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.context = contextText
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalBinding(sessionKey string, goalID string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.sessionKey = strings.TrimSpace(sessionKey)
	slot.mutable.goal.idForUsage = strings.TrimSpace(goalID)
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalRuntimeIgnored(ignored bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.runtimeIgnored = ignored
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalRuntimeIgnored() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.runtimeIgnored
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
