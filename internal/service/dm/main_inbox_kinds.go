// INPUT: 主窗口 inbox 的事件类型。
// OUTPUT: 每类事件的一行规格：是否宿主代写、能否插进运行中的轮次、模型输入与分隔条元数据、是否受暂停闸门约束、怎样起轮。
// POS: 主窗口 inbox 唯一的类型分派点；新增事件类型只在这里加一行，派发、安全点注入与快照只查表。
package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
)

// inboxStart 是一次起轮尝试的结局，决定派发循环是否继续取下一批。
type inboxStart uint8

const (
	// inboxStartNone：没有起轮——失败已退回待处理，或已交给后台任务；本次不再取。
	inboxStartNone inboxStart = iota
	// inboxStartRound：已起轮。
	inboxStartRound
	// inboxStartNext：事件已收口但不必起轮（目标已陈旧、轮次已存在、回执已处理），继续取下一批。
	inboxStartNext
)

type inboxStartFunc func(s *Service, ctx context.Context, target mainInboxTarget, roundID string, batch []inboxstore.Event) inboxStart

// inboxKindSpec 描述一类 inbox 事件；派发、安全点注入、快照与时间线都只查这张表。
type inboxKindSpec struct {
	// host 标出宿主代写的事件：独占一轮、不与用户原话合并、不进可撤回的待发送列表。
	host bool
	// joinsRunning 的事件可在安全点作为补充插进别的轮次；宿主事件因此必须有 prompt。
	joinsRunning bool
	// prompt 是宿主事件交给模型的输入，带来源包装；用户消息为 nil，用原话。
	prompt func(inboxstore.Event) string
	// metadata 写入用户行，时间线据此渲染触发分隔条。
	metadata func(inboxstore.Event) map[string]string
	// fixedRound 让轮次 ID 由 event_id 决定：进程重启后凭轮次标记认出已经起过的轮，不重复执行。
	fixedRound bool
	// gated 的事件受暂停闸门约束：用户停止后不再自动派发，除非入队时已获准越过。
	gated bool
	start inboxStartFunc
}

// inboxKinds 在 init 中构造：起轮函数会回查本表，包级变量直接引用会形成初始化环。
var inboxKinds map[string]inboxKindSpec

func init() {
	inboxKinds = map[string]inboxKindSpec{
		inboxstore.KindUserMessage: {joinsRunning: true, start: (*Service).startUserInboxRound},
		inboxstore.KindScheduledTask: {
			host: true, joinsRunning: true, prompt: scheduledTaskPrompt, metadata: scheduledTaskMetadata,
			start: (*Service).startHostInboxRun,
		},
		inboxstore.KindFollowUp: {
			host: true, joinsRunning: true, prompt: followUpPrompt, metadata: followUpMetadata,
			start: (*Service).startHostInboxRun,
		},
		inboxstore.KindEcho:         {host: true, start: (*Service).startEchoGate},
		inboxstore.KindHostRecovery: {host: true, fixedRound: true, gated: true, start: (*Service).startRecoveryRound},
		inboxstore.KindIMReply:      {host: true, start: (*Service).startIMReplyRound},
	}
}

func inboxKindOf(event inboxstore.Event) inboxKindSpec {
	return inboxKinds[event.Kind]
}

// isHostInboxEvent 区分宿主代写的事件。
func isHostInboxEvent(event inboxstore.Event) bool {
	return inboxKindOf(event).host
}

// joinsRunningRound 标出可在安全点插进别的轮次的事件。
func joinsRunningRound(event inboxstore.Event) bool {
	return inboxKindOf(event).joinsRunning
}

// hostEventPrompt 是模型看到的宿主输入：说明来源，避免被当成用户此刻说的话。
func hostEventPrompt(event inboxstore.Event) string {
	return inboxKindOf(event).prompt(event)
}

// hostEventMetadata 写入用户行，时间线据此显示触发分隔条。
func hostEventMetadata(event inboxstore.Event) map[string]string {
	if metadata := inboxKindOf(event).metadata; metadata != nil {
		return metadata(event)
	}
	return nil
}

func isInboxCommand(event inboxstore.Event) bool {
	return !isHostInboxEvent(event) && conversationsvc.IsSlashCommandInput(event.Payload.Content)
}

// isSoloInboxEvent 标出空闲时必须单独起轮的事件：命令要独占执行，宿主事件不与用户原话混在一轮。
func isSoloInboxEvent(event inboxstore.Event) bool {
	return isInboxCommand(event) || isHostInboxEvent(event)
}

// inboxRoundID 为一批事件分配轮次 ID；fixedRound 的事件由 event_id 推出（两者同源于 NewRoundID）。
func inboxRoundID(first inboxstore.Event) string {
	if inboxKindOf(first).fixedRound {
		return strings.Replace(first.EventID, "inbox_", "round_", 1)
	}
	return protocol.NewRoundID()
}
