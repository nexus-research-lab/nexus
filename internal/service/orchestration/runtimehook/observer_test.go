package runtimehook

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type recordingObserver struct {
	t        *testing.T
	actor    orchestration.ActorContext
	contexts []context.Context
	events   []string
}

type recordingCompactEvidence struct {
	actor orchestration.ActorContext
	kind  orchestration.PersistenceEvidenceKind
	ids   []string
}

func (r *recordingCompactEvidence) RecordPersistenceEvidence(_ context.Context, actor orchestration.ActorContext, kind orchestration.PersistenceEvidenceKind, id string) error {
	r.actor, r.kind = actor, kind
	r.ids = append(r.ids, id)
	return errors.New("证据落库失败")
}

func TestCompactEvidenceKeepsStablePhysicalRoundIdentity(t *testing.T) {
	var logs bytes.Buffer
	recorder := &recordingCompactEvidence{}
	observer := Observer{Provider: recorder, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	actor := orchestration.ActorContext{SessionKey: "room:group:shared", AgentRoundID: "audit-round"}
	observer.ObserveCompactBoundary(actor, "physical-session", "physical-round", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem})
	message := sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, System: &sdkprotocol.SystemMessage{Subtype: " compact_boundary "}}
	for range 2 {
		observer.ObserveCompactBoundary(actor, " physical-session ", " physical-round ", message)
	}
	if !reflect.DeepEqual(recorder.ids, []string{"runtime:physical-session:physical-round:compact-boundary", "runtime:physical-session:physical-round:compact-boundary"}) || recorder.actor != actor || recorder.kind != orchestration.PersistenceEvidenceContextBoundary {
		t.Fatalf("证据身份或重放 ID 不稳定: %#v", recorder)
	}
	if strings.Count(logs.String(), "level=ERROR") != 2 {
		t.Fatal("落库失败未记录")
	}
}

func (r *recordingObserver) record(ctx context.Context, actor orchestration.ActorContext, event string) error {
	r.t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 3*time.Second {
		r.t.Fatalf("观察事件缺少有界 deadline: %v", deadline)
	}
	if !reflect.DeepEqual(actor, r.actor) || ctx.Err() != nil {
		r.t.Fatalf("观察事件丢失 actor 或收到已取消上下文: %#v, %v", actor, ctx.Err())
	}
	r.contexts = append(r.contexts, ctx)
	r.events = append(r.events, event)
	return errors.New("观察写入失败")
}

func (r *recordingObserver) BeginRuntimeRound(ctx context.Context, actor orchestration.ActorContext) error {
	return r.record(ctx, actor, "begin")
}
func (r *recordingObserver) ObserveRuntimeMessage(ctx context.Context, actor orchestration.ActorContext, message sdkprotocol.ReceivedMessage) error {
	return r.record(ctx, actor, "message:"+string(message.Type))
}
func (r *recordingObserver) ObserveRuntimeCommands(ctx context.Context, actor orchestration.ActorContext, receipts []orchestration.RuntimeCommandFact) error {
	if len(receipts) != 1 {
		r.t.Fatalf("命令回执数量 = %d", len(receipts))
	}
	return r.record(ctx, actor, "receipts")
}
func (r *recordingObserver) ObserveRuntimeArtifacts(ctx context.Context, actor orchestration.ActorContext, message protocol.Message) error {
	return r.record(ctx, actor, "artifact:"+message["id"].(string))
}
func (r *recordingObserver) FinishRuntimeRound(ctx context.Context, actor orchestration.ActorContext, status, reason string) error {
	return r.record(ctx, actor, status+":"+reason)
}

func TestObserverPreservesIdentityDeadlineAndFailureIsolation(t *testing.T) {
	actor := orchestration.ActorContext{OwnerUserID: "owner", SessionKey: "session", AgentRoundID: "round"}
	recorder := &recordingObserver{t: t, actor: actor}
	var logs bytes.Buffer
	observer := Observer{Provider: recorder, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	observer.Begin(actor)
	observer.ObserveMessage(actor, sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem})
	observer.ObserveCommandReceipts(actor, []nexusmcp.CommandReceipt{{Domain: "execution", Operation: "assign_work", RequestID: "request-a"}})
	observer.ObserveArtifacts(actor, nil)
	observer.ObserveArtifacts(actor, protocol.Message{"id": "artifact"})
	observer.Finish(actor, "failed", "runtime exited")
	want := []string{"begin", "message:system", "receipts", "artifact:artifact", "failed:runtime exited"}
	if !reflect.DeepEqual(recorder.events, want) {
		t.Fatalf("事件 = %v", recorder.events)
	}
	for _, ctx := range recorder.contexts {
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("观察结束后没有释放上下文")
		}
	}
	if strings.Count(logs.String(), "level=WARN") != len(want) {
		t.Fatalf("失败日志不完整: %s", logs.String())
	}
	// 未装配观察能力的会话仍可正常执行。
	for _, provider := range []any{nil, struct{}{}} {
		observer := Observer{Provider: provider}
		observer.Begin(actor)
		observer.ObserveMessage(actor, sdkprotocol.ReceivedMessage{})
		observer.ObserveCommandReceipts(actor, nil)
		observer.ObserveArtifacts(actor, protocol.Message{})
		observer.Finish(actor, "completed", "")
	}
}

func TestRuntimeCommandFactsFilterAndCopyTrustedReceipts(t *testing.T) {
	receipts := []nexusmcp.CommandReceipt{
		{Domain: "goal", RequestID: "other", Operation: "create_goal"},
		{Domain: "execution", Operation: "assign_work"},
		{Domain: "execution", RequestID: "empty", Operation: " "},
		{Domain: "execution", RequestID: "request", Operation: "assign_work", Outcome: "applied", ExecutionID: "execution", WorkItemID: "work", AssignmentID: "assignment", AttemptID: "attempt", Changed: []string{"attempt:attempt"}},
	}
	facts := runtimeCommandFacts(receipts)
	if len(facts) != 1 || facts[0].RequestID != "request" || facts[0].AttemptID != "attempt" || facts[0].AssignmentID != "assignment" || facts[0].WorkItemID != "work" || facts[0].ExecutionID != "execution" || facts[0].Outcome != "applied" {
		t.Fatalf("facts=%+v", facts)
	}
	receipts[3].Changed[0] = "overwritten"
	if facts[0].Changed[0] != "attempt:attempt" {
		t.Fatal("观察事实与可变回执共享了切片")
	}
}
