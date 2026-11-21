// INPUT: 显式轮次边界与属于该轮的 SDK 消息；历史回放的消息另带 transcript 位置。
// OUTPUT: 该轮的 durable / ephemeral / stream 投影；带位置的 durable 行按 transcript 时间与行号定序。
// POS: 实时续接与历史回放共用的唯一轮次归约器；轮次身份只来自边界事件，不从消息内容推断。
package message

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ProjectionVersion 是归约规则的版本。同一批事件在规则变化后会投影出不同的行，
// 改动 Reducer / Processor 的 durable 输出时必须提升它，持久化读模型据此重建。
const ProjectionVersion = 1

// TranscriptLineField 是 durable 行上的 transcript 游标：产生该行的最后一个 transcript 条目的行号。
const TranscriptLineField = "transcript_line"

// RoundBoundaryKind 说明一个轮次由什么事件开启。
type RoundBoundaryKind string

const (
	// RoundBoundaryUserInput 是用户（或宿主代用户）提交的输入。
	RoundBoundaryUserInput RoundBoundaryKind = "user_input"
	// RoundBoundaryRuntimeWake 是 runtime 空闲后自行续跑。
	RoundBoundaryRuntimeWake RoundBoundaryKind = RuntimeWakePurpose
	// RoundBoundaryImplicit 是没有任何输入事件就出现的 runtime 输出（断链或被截断的 transcript）。
	RoundBoundaryImplicit RoundBoundaryKind = "implicit"
)

// RoundBoundary 是开启一个轮次的显式事件，携带该轮全部身份。
type RoundBoundary struct {
	Kind         RoundBoundaryKind
	RoundID      string
	AgentRoundID string
	// ParentID 是触发该轮的 durable user message id。
	ParentID  string
	SessionID string
}

// ImplicitRoundBoundary 为没有输入事件的 runtime 输出建立轮次，身份由首个条目派生。
func ImplicitRoundBoundary(roundID string, sessionID string) RoundBoundary {
	return RoundBoundary{
		Kind:      RoundBoundaryImplicit,
		RoundID:   roundID,
		ParentID:  DerivedUserMessageID(roundID),
		SessionID: sessionID,
	}
}

// DerivedUserMessageID 是边界事件没有独立 user message id 时的稳定派生规则。
func DerivedUserMessageID(roundID string) string {
	return "msg_user_" + roundID
}

// TranscriptPosition 是一条消息在 transcript 中的稳定位置。
type TranscriptPosition struct {
	Line      int
	Timestamp int64
}

// Reducer 把「轮次边界 + 消息」事件流归约为产品消息。实时链路在收到 SDK 消息时调用，
// 历史回放按 transcript 顺序调用；两者共用同一份状态机，因此同一批事件得到同一批行。
type Reducer struct {
	scope     MessageContext
	boundary  RoundBoundary
	processor *Processor
}

// NewReducer 创建尚未开启任何轮次的归约器；scope 只提供会话级身份。
func NewReducer(scope MessageContext) *Reducer {
	return &Reducer{scope: scope}
}

// BeginRound 开启一个轮次。上一轮未收口的 assistant 分段状态随之丢弃，不会并入新轮次。
func (r *Reducer) BeginRound(boundary RoundBoundary) {
	ctx := r.scope
	ctx.RoundID = boundary.RoundID
	ctx.AgentRoundID = boundary.AgentRoundID
	ctx.ParentID = boundary.ParentID
	r.boundary = boundary
	r.processor = NewProcessor(ctx, strings.TrimSpace(boundary.SessionID))
}

// Active 表示已有轮次可以接收消息。
func (r *Reducer) Active() bool {
	return r.processor != nil
}

// Boundary 返回当前轮次的边界事件。
func (r *Reducer) Boundary() RoundBoundary {
	return r.boundary
}

// Reduce 归约当前轮次的一条实时消息；durable 行使用宿主接收时间。
func (r *Reducer) Reduce(message sdkprotocol.ReceivedMessage) Output {
	return r.processor.Process(message)
}

// ReduceAt 归约一条来自 transcript 的消息，并把它的位置写到产出的 durable 行上。
func (r *Reducer) ReduceAt(message sdkprotocol.ReceivedMessage, position TranscriptPosition) Output {
	output := r.processor.Process(message)
	for index, row := range output.DurableMessages {
		positioned := protocol.Clone(row)
		positioned["timestamp"] = position.Timestamp
		positioned[TranscriptLineField] = position.Line
		output.DurableMessages[index] = positioned
	}
	return output
}

// CurrentMessageID 返回当前 assistant 分段的 message_id。
func (r *Reducer) CurrentMessageID() string {
	return r.processor.CurrentMessageID()
}

// SessionID 返回当前轮次登记的 SDK session_id。
func (r *Reducer) SessionID() string {
	return r.processor.SessionID()
}

// FinalizeInterruptedAssistant 把被中断轮次里已展示的 assistant 片段收口为 durable 行。
func (r *Reducer) FinalizeInterruptedAssistant() protocol.Message {
	return r.processor.FinalizeInterruptedAssistant()
}
