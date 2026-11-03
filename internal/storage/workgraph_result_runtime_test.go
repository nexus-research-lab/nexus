// INPUT: Explicit opt-in nxs binary and provider environment; disposable WorkGraph fixtures.
// OUTPUT: Real runtime Stop/ACK and MCP delivery/review evidence plus bounded per-round usage.
// POS: Optional provider acceptance, never part of the default gate or a full application benchmark.
package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/operation"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestWorkGraphResultRuntimeAcceptance(t *testing.T) {
	binary, model := os.Getenv("NEXUS_TEST_RESULT_RUNTIME_BINARY"), os.Getenv("NEXUS_TEST_RESULT_MODEL")
	if binary == "" || model == "" {
		t.Skip("set NEXUS_TEST_RESULT_RUNTIME_BINARY and NEXUS_TEST_RESULT_MODEL with a test provider environment")
	}
	samples := 1
	if value := os.Getenv("NEXUS_TEST_RESULT_SAMPLES"); value != "" {
		var err error
		samples, err = strconv.Atoi(value)
		if err != nil || samples < 1 || samples > 5 {
			t.Fatal("NEXUS_TEST_RESULT_SAMPLES must be 1-5")
		}
	}
	cfg := integrationDatabase(t, "sqlite")
	migrateIntegrationDatabase(t, cfg, 0)
	repository := repo.NewSQLRepository("sqlite", openIntegrationDatabase(t, cfg))
	service := svc.NewService(repository)
	for sample := range samples {
		modes := []string{"tool", "hook"}
		if sample%2 == 1 {
			modes = []string{"hook", "tool"}
		}
		for _, mode := range modes {
			t.Run(fmt.Sprintf("%d_%s", sample+1, mode), func(t *testing.T) {
				actor, _ := runtimeResultFixture(t, repository, fmt.Sprintf("provider-%d-%s", sample, mode))
				runResultProviderRound(t, service, actor, binary, model, mode, "delivery")
				snapshot, err := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if err != nil || len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 || len(snapshot.Acceptances) != 0 {
					t.Fatalf("delivery unproven: submissions/reviews/acceptances or read error=%v", err)
				}
				reviewer, _ := runtimeReviewFixture(snapshot)
				runResultProviderRound(t, service, reviewer, binary, model, mode, "review")
				snapshot, err = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if err != nil || len(snapshot.Acceptances) != 1 || snapshot.Acceptances[0].Decision != protocol.WorkAcceptanceAccepted {
					t.Fatalf("review unproven: %v", err)
				}
			})
		}
	}
}

func runResultProviderRound(t *testing.T, service *svc.Service, actor svc.ActorContext, binary, model, mode, kind string, task ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	manager := runtime.NewManager()
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := manager.Close(cleanup); err != nil {
			t.Error("runtime cleanup failed")
		}
	}()
	key := actor.ExecutionID + ":" + kind
	scope := contract.Context{OwnerUserID: actor.OwnerUserID, AgentID: actor.AgentID, Role: actor.Role, ScopeKind: actor.ScopeKind, ScopeSessionKey: actor.SessionKey, RuntimeSessionKey: key, ExecutionID: actor.ExecutionID, WorkBinding: actor.WorkBinding, ReviewBinding: actor.ReviewBinding, RuntimeRoundID: actor.RuntimeRoundID, AgentRoundID: actor.AgentRoundID, RoomID: actor.RoomID, ConversationID: actor.ConversationID}
	operations := operation.BuildAll(service, scope)
	var commandCalls atomic.Int64
	tool := command.NewTool(func(ctx context.Context, request command.Request) (any, error) {
		commandCalls.Add(1)
		return command.HandleSemantic(ctx, command.Actor{}, command.DomainExecution, "get_execution", operations, request)
	})
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1", []sdktool.Tool{tool})
	environment := map[string]string{
		"NEXUS_CONFIG_DIR": t.TempDir(), "NEXUS_DISABLE_AUTO_MEMORY": "1", "NEXUS_API_PROVIDER": "anthropic-compatible",
		"ANTHROPIC_BASE_URL": os.Getenv("ANTHROPIC_BASE_URL"), "ANTHROPIC_AUTH_TOKEN": os.Getenv("ANTHROPIC_AUTH_TOKEN"), "ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
	}
	options := manager.WithWorkResultHook(bridge.Options{
		CLIPath: binary, CWD: t.TempDir(), Env: environment, Model: model,
		System:  bridge.SystemOptions{Text: "You are validating a synthetic WorkGraph responsibility. Use only supplied evidence and the bound WorkGraph. Be concise. No other work or external actions are needed."},
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS, MaxTurns: 8, Thinking: &bridge.ThinkingConfig{Type: bridge.ThinkingModeDisabled}},
		Tools:   bridge.ToolOptions{Available: []string{}, Allow: []string{"mcp__nexus__command"}},
		Skills:  bridge.SkillOptions{Mode: bridge.SkillModeNone}, SettingSources: []string{},
		MCP: bridge.MCPOptions{SDKServers: map[string]sdkmcp.SDKMCPServer{"nexus": server}, StrictConfig: true},
	}, key)
	client, err := manager.GetOrCreate(ctx, key, options)
	if err != nil {
		t.Fatal("runtime initialization failed: " + err.Error())
	}
	if err := client.Connect(ctx); err != nil {
		t.Fatal("runtime connection failed: " + err.Error())
	}
	if !runtime.SupportsWorkResultHooks(client) {
		t.Fatal("runtime lacks negotiated Stop/ACK prerequisites")
	}
	contextText, err := service.RuntimeContext(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	prompt := contextText + "\nThe exact acceptance criterion is verified. The supplied artifact is fixture:arithmetic, containing 2 + 2 = 4. Check it directly from this content; no file or network access is needed."
	operationName := "submit_work"
	if kind == "delivery" {
		prompt += "\nDeliver this bound responsibility with a concise result_summary, result_refs containing fixture:arithmetic, and evidence that 2+2 equals 4. Do not review it yourself."
	} else {
		operationName = "review_work"
		prompt += "\nReview the bound submission against the supplied artifact and the exact criterion verified. Make your explicit decision with evidence. Do not continue coordinating other work."
	}
	if len(task) > 0 {
		prompt = contextText + "\n" + task[0]
	}
	if mode == "hook" {
		manager.SetWorkResultHook(key, actor.AgentRoundID, runtimehook.ResultCallback(service, actor, nil))
		defer manager.ClearWorkResultHook(key, actor.AgentRoundID)
		prompt += "\n" + runtimehook.ResultContract(actor)
	} else {
		op, ok := command.FindOperation(operations, operationName)
		if !ok {
			t.Fatal("missing canonical operation")
		}
		schema, _ := json.Marshal(op.InputSchema)
		prompt += "\nReport the result using nexus.command with domain=execution, action=invoke, operation=" + operationName + ", a new valid request_id and the following exact input schema (bound locators may be omitted): " + string(schema) + ". Finish with a short reply after the receipt."
	}
	messages := client.ReceiveMessages(ctx)
	start := time.Now()
	if err := client.Query(ctx, prompt); err != nil {
		t.Fatal("runtime query failed")
	}
	for {
		select {
		case <-ctx.Done():
			evidence, _ := json.Marshal(map[string]any{"mode": mode, "kind": kind, "model": model, "elapsed_ms": time.Since(start).Milliseconds(), "turn_budget": 8, "command_calls": commandCalls.Load(), "has_usage": false, "outcome": "timeout"})
			t.Log(string(evidence))
			t.Fatal("provider round timed out")
		case message, ok := <-messages:
			if !ok {
				t.Fatal("runtime ended without terminal receipt")
			}
			if message.Result == nil {
				continue
			}
			result := message.Result
			usage, hasUsage := sdkprotocol.TokenUsageFromResult(*result)
			usage.Raw = nil
			evidence, _ := json.Marshal(map[string]any{"mode": mode, "kind": kind, "model": model, "elapsed_ms": time.Since(start).Milliseconds(), "turn_budget": 8, "turns": result.NumTurns, "command_calls": commandCalls.Load(), "usage": usage, "has_usage": hasUsage, "runtime_error": result.IsError})
			t.Log(string(evidence))
			if result.IsError {
				detail := strings.Join(result.Errors, "; ")
				for _, credential := range []string{environment["ANTHROPIC_AUTH_TOKEN"], environment["ANTHROPIC_API_KEY"]} {
					if credential != "" {
						detail = strings.ReplaceAll(detail, credential, "[redacted]")
					}
				}
				t.Fatalf("provider/runtime rejected round: %s %s", result.Subtype, detail)
			}
			if mode == "hook" && commandCalls.Load() != 0 {
				t.Log("hook-enabled round fell back to explicit commands; retain its full usage and timing in the comparison")
			}
			return
		}
	}
}
