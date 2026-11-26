// INPUT: Automation 到点写入的 main 目标定时任务，以及到点的 agent 自设跟进与 Echo。
// OUTPUT: 主窗口 inbox 的宿主事件、按 kind 查表得到的模型输入与触发分隔条元数据；空闲时独占的宿主 run
// 以延迟提交轮执行，只留一条回复或分隔条上的结局（无需提醒 / 未完成）。
// POS: 宿主事件进入主窗口的唯一入口；起轮、安全点注入与收口复用 main_inbox.go 的同一条路径。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// input_kind 写入用户行 metadata，前端据此渲染触发分隔条而非用户气泡。
const (
	scheduledTaskInputKind = "scheduled_task"
	followUpInputKind      = "follow_up"
)

// HostNoReplyMarker 是宿主触发的轮次（定时任务、跟进、Echo）选择沉默时的唯一输出。
const HostNoReplyMarker = "<nexus_no_reply/>"

// hostQuietRule 告诉模型宿主触发的轮次可以沉默；只有回复会出现在用户面前。
const hostQuietRule = "Only your final reply reaches the user. If nothing is worth telling them, reply with exactly " + HostNoReplyMarker + "."

// hostRunPurpose 标出延迟提交的宿主 run：历史投影据此隐藏它在 transcript 里的过程。
const hostRunPurpose = "host_run"

// 没有回复的结局；有回复时回复本身就是记录。前端把结局并进触发分隔条。
var hostRunOutcomes = map[string]string{
	deferredStatusQuiet:     "quiet",
	deferredStatusFailed:    "failed",
	deferredStatusCancelled: "stopped",
}

// hostEventSpec 描述一类宿主事件如何进入模型与时间线；新增宿主事件只需在表里加一行。
type hostEventSpec struct {
	prompt   func(inboxstore.Event) string
	metadata func(inboxstore.Event) map[string]string
	// quietOnly 的事件只在对话空闲时独占一轮，不在安全点插进别的轮次；它的输入由自己的起轮路径给出。
	quietOnly bool
}

var hostEventSpecs = map[string]hostEventSpec{
	inboxstore.KindScheduledTask: {prompt: scheduledTaskPrompt, metadata: scheduledTaskMetadata},
	inboxstore.KindFollowUp:      {prompt: followUpPrompt, metadata: followUpMetadata},
	inboxstore.KindEcho:          {quietOnly: true},
}

// MainWindowTask 是一次交给主窗口执行的定时任务到点。
type MainWindowTask struct {
	JobID       string
	RunID       string
	Name        string
	Instruction string
	// DedupeKey 让同一次到点重复写入只留一条事件。
	DedupeKey string
}

// mainWindowRooms 解析主智能体唯一的主窗口对话；不存在时创建。
type mainWindowRooms interface {
	EnsureDirectRoom(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

// SetMainWindowRooms 注入主窗口对话解析器。
func (s *Service) SetMainWindowRooms(rooms mainWindowRooms) {
	s.mainWindowRooms = rooms
}

// AppendMainWindowTask 把定时任务写进主智能体的主窗口 inbox 并唤醒；返回主窗口 session key。
func (s *Service) AppendMainWindowTask(ctx context.Context, agentID string, task MainWindowTask) (string, error) {
	if s.mainWindowRooms == nil || s.inbox == nil {
		return "", errors.New("main window inbox is not configured")
	}
	conversation, err := s.mainWindowRooms.EnsureDirectRoom(ctx, agentID)
	if err != nil {
		return "", err
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(conversation.Conversation.ID, agentID, protocol.RoomTypeDM)
	target, ok, err := s.resolveMainInboxTarget(ctx, sessionKey, agentID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("agent %s has no main window", agentID)
	}
	if _, _, err = s.inbox.Append(ctx, inboxstore.Event{
		EventID:     newInboxEventID(),
		OwnerUserID: target.owner(),
		InboxKey:    target.sessionKey,
		Kind:        inboxstore.KindScheduledTask,
		DedupeKey:   strings.TrimSpace(task.DedupeKey),
		Payload: inboxstore.Payload{
			Content: strings.TrimSpace(task.Instruction),
			Task: &inboxstore.TaskSource{
				JobID: strings.TrimSpace(task.JobID),
				RunID: strings.TrimSpace(task.RunID),
				Name:  strings.TrimSpace(task.Name),
			},
		},
	}); err != nil {
		return "", err
	}
	s.wakeMainInbox(target)
	return target.sessionKey, nil
}

// mainInboxHostRequest 让宿主事件以内部轮次运行：不算用户输入、不排队、沿用主窗口会话的权限。
// 它是延迟提交轮：照常公告分隔条与运行态、工具可用，但过程不推送；结束时只提交最终回复或记下结局。
func (s *Service) mainInboxHostRequest(target mainInboxTarget, roundID string, event inboxstore.Event) Request {
	return Request{
		SessionKey:           target.sessionKey,
		AgentID:              target.agent.AgentID,
		Content:              hostEventPrompt(event),
		RoundID:              roundID,
		DeliveryPolicy:       protocol.ChatDeliveryPolicyQueue,
		BroadcastUserMessage: true,
		Internal:             true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Synthetic: true,
			Purpose:   hostRunPurpose,
			Metadata:  hostEventMetadata(event),
		},
		DeferredAssistant: &DeferredAssistantHooks{
			Announce:       true,
			KeepTranscript: true,
			Admit:          admitHostRun,
			Complete: func(ctx context.Context, outcome DeferredAssistantOutcome) {
				s.finishHostRun(ctx, target, roundID, outcome)
			},
		},
		hostTrigger: true,
	}
}

// admitHostRun 只拦沉默：失败与中断在准入之前已分流，其余回复原样提交。
func admitHostRun(_ context.Context, candidate DeferredAssistantCandidate) (protocol.Message, bool, error) {
	if isHostNoReply(candidate.Message) {
		return nil, false, nil
	}
	message := protocol.Clone(candidate.Message)
	if textutil.AnyString(message["message_id"]) == "" {
		message["message_id"] = protocol.NewAssistantMessageID()
	}
	return message, true, nil
}

// isHostNoReply 只认整条输出恰为沉默标记或为空；正文里偶然出现标记不吞掉正常回复。
func isHostNoReply(message protocol.Message) bool {
	text := strings.TrimSpace(messageutil.ExtractAssistantDisplayText(message))
	return text == "" || text == HostNoReplyMarker
}

// finishHostRun 为没有回复的宿主 run 结局留下一条记录。
func (s *Service) finishHostRun(ctx context.Context, target mainInboxTarget, roundID string, outcome DeferredAssistantOutcome) {
	if outcome.Error != nil {
		s.LoggerFor(ctx).Warn("宿主 run 未完成", "session_key", target.sessionKey, "round_id", roundID, "err", outcome.Error)
	}
	value, ok := hostRunOutcomes[outcome.Status]
	if !ok {
		return
	}
	message := hostRunOutcomeMessage(target, roundID, value)
	ctx = runtimehost.ContextWithExactOwner(ctx, target.owner())
	if err := s.History.ForOwner(target.owner()).AppendOverlayMessage(target.agent.WorkspacePath, target.sessionKey, message); err != nil {
		s.LoggerFor(ctx).Error("写入宿主 run 结局失败", "session_key", target.sessionKey, "round_id", roundID, "err", err)
		return
	}
	event := protocol.NewEvent(protocol.EventTypeMessage, protocol.Clone(message))
	event.SessionKey = target.sessionKey
	event.AgentID = target.agent.AgentID
	event.RoundID = roundID
	event.MessageID = textutil.AnyString(message["message_id"])
	event.DeliveryMode = protocol.DeliveryModeDurable
	s.broadcastEventWithTimeout(ctx, target.sessionKey, event)
}

func hostRunOutcomeMessage(target mainInboxTarget, roundID string, outcome string) protocol.Message {
	return protocol.Message{
		"message_id":  "host_outcome_" + roundID,
		"session_key": target.sessionKey,
		"agent_id":    target.agent.AgentID,
		"round_id":    roundID,
		"role":        "system",
		"content":     outcome,
		"timestamp":   time.Now().UnixMilli(),
		"metadata":    map[string]any{"subtype": messageutil.SystemMessageSubtypeHostRunOutcome, "outcome": outcome},
	}
}

// isHostInboxEvent 区分宿主代写的事件。
func isHostInboxEvent(event inboxstore.Event) bool {
	_, ok := hostEventSpecs[event.Kind]
	return ok
}

// isQuietOnlyInboxEvent 标出只在对话空闲时出现的事件（Echo）。
func isQuietOnlyInboxEvent(event inboxstore.Event) bool {
	return hostEventSpecs[event.Kind].quietOnly
}

// hostEventPrompt 是模型看到的宿主输入：说明来源，避免被当成用户此刻说的话。
func hostEventPrompt(event inboxstore.Event) string {
	return hostEventSpecs[event.Kind].prompt(event)
}

// hostEventMetadata 写入用户行，时间线据此显示触发分隔条。
func hostEventMetadata(event inboxstore.Event) map[string]string {
	return hostEventSpecs[event.Kind].metadata(event)
}

func scheduledTaskPrompt(event inboxstore.Event) string {
	return fmt.Sprintf(
		"<scheduled-task name=%q>\nThe scheduled task is due. Carry out its instruction now; the user did not type this. "+hostQuietRule+"\n\n%s\n</scheduled-task>",
		hostEventTaskName(event),
		event.Payload.Content,
	)
}

func scheduledTaskMetadata(event inboxstore.Event) map[string]string {
	metadata := map[string]string{"input_kind": scheduledTaskInputKind, "task_name": hostEventTaskName(event)}
	if task := event.Payload.Task; task != nil {
		metadata["job_id"] = task.JobID
		metadata["run_id"] = task.RunID
	}
	return metadata
}

func hostEventTaskName(event inboxstore.Event) string {
	if event.Payload.Task == nil {
		return ""
	}
	return event.Payload.Task.Name
}

// decorateHostTriggerMessage 把用户行标成宿主触发：不算用户输入，前端渲染为分隔条。
func decorateHostTriggerMessage(message protocol.Message, metadata map[string]string) protocol.Message {
	if message == nil || len(metadata) == 0 {
		return message
	}
	row := make(map[string]any, len(metadata))
	for key, value := range metadata {
		row[key] = value
	}
	message["metadata"] = row
	message["is_synthetic"] = true
	return message
}
