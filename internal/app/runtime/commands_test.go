// commands_test.go 覆盖 app/runtime 产物工具、命令诊断校验与子代理命令测试。
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	subagentcommand "github.com/nexus-research-lab/nexus/internal/mcp/command/subagent"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

type artifactWorkspace struct{ owner, agent string }

func (s *artifactWorkspace) ValidateDeliverables(ctx context.Context, agent string, paths []string) ([]string, error) {
	s.owner, s.agent = authctx.OwnerUserID(ctx), agent
	return paths, nil
}

func TestArtifactToolBuilderBindsTheProducingRoomMember(t *testing.T) {
	service := &artifactWorkspace{}
	builder := NewArtifactToolBuilder(service)
	for _, agentID := range []string{"researcher", "designer"} {
		round := nexusmcp.RoundContext{CommandContext: runtimectx.RuntimeCommandContext{
			Agent:        &protocol.Agent{OwnerUserID: "owner", AgentID: agentID},
			AgentRoundID: agentID + "-round", CoordinatorAgentID: "coordinator",
		}}
		definitions := builder(context.Background(), round)
		if len(definitions) != 1 || definitions[0].Name != "deliver_files" {
			t.Fatal("missing built-in delivery tool")
		}
		result, err := definitions[0].Handler(context.Background(), map[string]any{"paths": []string{"report.pdf"}})
		if err != nil || result.IsError || service.owner != "owner" || service.agent != agentID || result.StructuredContent["agent_round_id"] != agentID+"-round" {
			t.Fatalf("coordinator or ambient identity replaced producer: %+v %+v", service, result)
		}
	}
}

func TestCommandDiagnosticsUseBaseLoggerAcrossRequestAndBackgroundRounds(t *testing.T) {
	for _, requestLogger := range []bool{false, true} {
		var baseOutput, requestOutput bytes.Buffer
		base := slog.New(slog.NewJSONHandler(&baseOutput, nil))
		ctx := context.Background()
		if requestLogger {
			ctx = logx.WithLogger(ctx, slog.New(slog.NewJSONHandler(&requestOutput, nil)).With("request_id", "sensitive-request-context"))
		}
		ctx = runtimectx.WithRuntimeRoundLease(ctx, "sensitive-session", "sensitive-round")
		agent := &protocol.Agent{AgentID: "sensitive-agent", OwnerUserID: "sensitive-owner"}
		tool, available, err := buildNexusCommandMCPTool(ctx, config.Config{}, base, stubRuntimeAgentResolver{record: agent}, nil, nil, nil, nil, nil, nil, nexusmcp.RoundContext{
			SessionKey: "sensitive-session", RoundID: "sensitive-round", CommandReceipts: nexusmcp.NewCommandReceiptState(),
			CommandContext: runtimectx.RuntimeCommandContext{Agent: agent},
		})
		if err != nil || !available {
			t.Fatalf("command assembly: available=%t err=%v", available, err)
		}
		result, err := tool.Handler(ctx, map[string]any{"domain": "execution", "action": "invoke", "request_id": 123})
		if err != nil || !result.IsError {
			t.Fatalf("input rejection: %+v, %v", result, err)
		}
		if requestOutput.Len() != 0 || strings.Contains(baseOutput.String(), "sensitive-") {
			t.Fatal("command diagnostics inherited raw request attributes")
		}
		lines := bytes.Split(bytes.TrimSpace(baseOutput.Bytes()), []byte{'\n'})
		if len(lines) != 2 {
			t.Fatalf("base logger records = %d, want 2", len(lines))
		}
		var finish map[string]any
		if err := json.Unmarshal(lines[1], &finish); err != nil || finish["reason_code"] != "invalid_envelope" {
			t.Fatalf("base logger finish: %+v, %v", finish, err)
		}
	}
}

func TestAutomationMalformedCommandRejectedBeforeServiceAndConfirmation(t *testing.T) {
	// No database or permission context: invalid model input must not reach either.
	for _, test := range []struct {
		request command.Request
		want    string
	}{
		{command.Request{Action: "apply", Operation: "create", Input: map[string]any{"name": "test", "instruction": "test", "schedule": map[string]any{"kind": "interval", "interval_value": 0}}}, "$.schedule.interval_value"},
		{command.Request{Action: "apply", Operation: "delete", Input: map[string]any{"job_id": "job", "instruction": "unrelated"}}, "$.instruction"},
		{command.Request{Action: "invoke", Operation: "get", Input: map[string]any{"job_id": "job"}}, "action=inspect"},
	} {
		_, err := handleAutomationCommand(context.Background(), &automationsvc.Service{}, nil, command.Actor{SourceContextType: "agent"}, test.request)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %s, got %v", test.want, err)
		}
	}
}

func TestSubagentCommandUsesSDKMetadataAndPropagatesNativeError(t *testing.T) {
	calls := 0
	handler := subagentcommand.NewHandler(command.Actor{OwnerUserID: "owner", AgentID: "agent", SessionKey: "parent", RoundID: "round", LeaseSessionKey: "parent", LeaseRoundID: "round", SourceContextType: "agent"}, func(_ context.Context, id, operation string, _ map[string]any) (map[string]any, error) {
		calls++
		if id != "sdk-tool" || operation != "spawn" {
			t.Fatalf("caller=%q operation=%q", id, operation)
		}
		return map[string]any{"is_error": true, "content": "assignment revoked"}, nil
	})
	server := newRoundScopedMCPServer(context.Background(), sdktool.NewSimpleSDKMCPServer("nexus", "1", []sdktool.Tool{command.NewTool(handler)}))
	params := map[string]any{"name": "command", "arguments": map[string]any{"domain": "subagent", "action": "invoke", "operation": "spawn", "request_id": "request-1", "input": map[string]any{"description": "task", "prompt": "evidence"}}}
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params}
	response, err := server.HandleMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || response["result"].(map[string]any)["isError"] != true {
		t.Fatalf("missing metadata accepted: %#v", response)
	}
	params["_meta"] = map[string]any{"claudecode/toolUseId": "sdk-tool"}
	response, err = server.HandleMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || response["result"].(map[string]any)["isError"] != true {
		t.Fatalf("native rejection lost: %#v", response)
	}
}
