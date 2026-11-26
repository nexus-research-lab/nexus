// INPUT: Automation 到点写入的 main 目标定时任务。
// OUTPUT: 主窗口 inbox 的 scheduled_task 事件、带来源说明的模型输入与时间线触发分隔条元数据。
// POS: 宿主事件进入主窗口的唯一入口；起轮、安全点注入与收口复用 main_inbox.go 的同一条路径。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// scheduledTaskInputKind 写入用户行 metadata.input_kind，前端据此渲染触发分隔条而非用户气泡。
const scheduledTaskInputKind = "scheduled_task"

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
	eventID := strings.Replace(protocol.NewRoundID(), "round_", "inbox_", 1)
	if _, _, err = s.inbox.Append(ctx, inboxstore.Event{
		EventID:     eventID,
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
func mainInboxHostRequest(target mainInboxTarget, roundID string, event inboxstore.Event) Request {
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
			Purpose:   scheduledTaskInputKind,
			Metadata:  hostEventMetadata(event),
		},
		hostTrigger: true,
	}
}

// isHostInboxEvent 区分宿主代写的事件；目前只有定时任务到点。
func isHostInboxEvent(event inboxstore.Event) bool {
	return event.Kind == inboxstore.KindScheduledTask
}

// hostEventPrompt 是模型看到的宿主输入：说明来源，避免被当成用户此刻说的话。
func hostEventPrompt(event inboxstore.Event) string {
	return fmt.Sprintf(
		"<scheduled-task name=%q>\nThe scheduled task is due. Carry out its instruction now; the user did not type this.\n\n%s\n</scheduled-task>",
		hostEventTaskName(event),
		event.Payload.Content,
	)
}

// hostEventMetadata 写入用户行，时间线据此显示「定时任务「名称」」分隔条。
func hostEventMetadata(event inboxstore.Event) map[string]string {
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
