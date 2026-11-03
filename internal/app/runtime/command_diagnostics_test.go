package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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
		tool, available, err := buildNexusCommandMCPTool(ctx, config.Config{}, base, stubRuntimeAgentResolver{record: agent}, nil, nil, nil, nil, nexusmcp.RoundContext{
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
