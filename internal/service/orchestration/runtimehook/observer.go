// INPUT: 可信 actor、运行事件与可选 Execution 观察能力。
// OUTPUT: 受独立三秒超时约束的观察调用及失败日志。
// POS: DM 与 Room 共用的运行观察边界，观察失败不阻断会话。
package runtimehook

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

// Observer 将两种会话的运行观察汇入同一超时和错误处理边界。
type Observer struct {
	Provider any
	Logger   *slog.Logger
}

type executionRuntimeGraphObserver interface {
	BeginRuntimeRound(context.Context, orchestrationsvc.ActorContext) error
	ObserveRuntimeMessage(context.Context, orchestrationsvc.ActorContext, sdkprotocol.ReceivedMessage) error
	ObserveRuntimeCommands(context.Context, orchestrationsvc.ActorContext, []orchestrationsvc.RuntimeCommandFact) error
	FinishRuntimeRound(context.Context, orchestrationsvc.ActorContext, string, string) error
}

type executionRuntimeArtifactObserver interface {
	ObserveRuntimeArtifacts(context.Context, orchestrationsvc.ActorContext, protocol.Message) error
}

type persistenceEvidenceRecorder interface {
	RecordPersistenceEvidence(context.Context, orchestrationsvc.ActorContext, orchestrationsvc.PersistenceEvidenceKind, string) error
}

// ObserveCompactBoundary 以物理会话和 Agent round 生成稳定证据 ID，重放不会新增边界。
// 仅活动 round 调用；idle 消息观察不能隐式改变这一证据范围。
func (o Observer) ObserveCompactBoundary(actor orchestrationsvc.ActorContext, sessionKey, agentRoundID string, incoming sdkprotocol.ReceivedMessage) {
	if incoming.Type != sdkprotocol.MessageTypeSystem || incoming.System == nil || strings.TrimSpace(incoming.System.Subtype) != "compact_boundary" {
		return
	}
	recorder, ok := o.Provider.(persistenceEvidenceRecorder)
	if !ok {
		return
	}
	commandID := fmt.Sprintf("runtime:%s:%s:compact-boundary", strings.TrimSpace(sessionKey), strings.TrimSpace(agentRoundID))
	if err := recorder.RecordPersistenceEvidence(context.Background(), actor, orchestrationsvc.PersistenceEvidenceContextBoundary, commandID); err != nil {
		o.Logger.Error("记录 Execution context boundary 失败", "session_key", sessionKey, "agent_round_id", agentRoundID, "err", err)
	}
}

func (o Observer) Begin(actor orchestrationsvc.ActorContext) {
	observer, ok := o.Provider.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.BeginRuntimeRound(ctx, actor); err != nil {
		o.Logger.Warn("记录 AgentRun 开始失败", "err", err)
	}
}

func (o Observer) ObserveMessage(
	actor orchestrationsvc.ActorContext,
	message sdkprotocol.ReceivedMessage,
) {
	observer, ok := o.Provider.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeMessage(ctx, actor, message); err != nil {
		o.Logger.Warn("记录 Runtime NodeRun 失败", "err", err)
	}
}

func (o Observer) ObserveCommandReceipts(
	actor orchestrationsvc.ActorContext,
	receipts []nexusmcp.CommandReceipt,
) {
	observer, ok := o.Provider.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeCommands(ctx, actor, runtimeCommandFacts(receipts)); err != nil {
		o.Logger.Warn("核验 Execution command NodeRun 失败", "err", err)
	}
}

func (o Observer) ObserveArtifacts(
	actor orchestrationsvc.ActorContext,
	message protocol.Message,
) {
	observer, ok := o.Provider.(executionRuntimeArtifactObserver)
	if !ok || observer == nil || message == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeArtifacts(ctx, actor, message); err != nil {
		o.Logger.Warn("关联 Runtime Artifact 失败", "err", err)
	}
}

func (o Observer) Finish(
	actor orchestrationsvc.ActorContext,
	terminalStatus string,
	failureReason string,
) {
	observer, ok := o.Provider.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.FinishRuntimeRound(ctx, actor, terminalStatus, failureReason); err != nil {
		o.Logger.Warn("收口 AgentRun 失败", "err", err)
	}
}

// runtimeCommandFacts 只转换宿主持有的 Execution 回执，忽略其他领域和不完整身份。
func runtimeCommandFacts(receipts []nexusmcp.CommandReceipt) []orchestrationsvc.RuntimeCommandFact {
	facts := make([]orchestrationsvc.RuntimeCommandFact, 0, len(receipts))
	for _, r := range receipts {
		if r.Domain != command.DomainExecution || strings.TrimSpace(r.RequestID) == "" || strings.TrimSpace(r.Operation) == "" {
			continue
		}
		facts = append(facts, orchestrationsvc.RuntimeCommandFact{RequestID: r.RequestID, Operation: r.Operation, Outcome: r.Outcome, Message: r.Message, ReasonCode: r.ReasonCode, ExecutionID: r.ExecutionID, WorkItemID: r.WorkItemID, AssignmentID: r.AssignmentID, AttemptID: r.AttemptID, Changed: slices.Clone(r.Changed)})
	}
	return facts
}
