package automationmcp

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

func TestHeartbeatOrdinaryAgentCannotTargetAnotherAgent(t *testing.T) {
	svc := &stubService{}
	for _, name := range []string{"get_heartbeat", "update_heartbeat", "wake_heartbeat"} {
		args := map[string]any{"agent_id": "agent-2"}
		if name == "update_heartbeat" {
			args["enabled"] = true
		}
		_, isError := callTool(t, svc, contract.ServerContext{
			CurrentAgentID:    "agent-1",
			SourceContextType: "agent",
		}, name, args)
		if !isError {
			t.Fatalf("%s should reject another Agent target", name)
		}
	}
}

func TestHeartbeatTrustedMainPrivateDMCouldUpdateOwnerScopedAgent(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "main-agent",
		IsMainAgent:       true,
		SourceContextType: "agent",
	}, "update_heartbeat", map[string]any{
		"agent_id":      "agent-2",
		"enabled":       true,
		"every_seconds": 90,
	})
	if isError {
		t.Fatalf("trusted main heartbeat update failed: %s", extractText(t, result))
	}
	if svc.heartbeatStatus == nil ||
		svc.heartbeatStatus.AgentID != "agent-2" ||
		!svc.heartbeatStatus.Enabled ||
		svc.heartbeatStatus.EverySeconds != 90 ||
		svc.heartbeatStatus.ConfigurationVersion != 2 {
		t.Fatalf("heartbeat status = %+v", svc.heartbeatStatus)
	}
}

func TestHeartbeatMainAgentInRoomCannotTargetAnotherAgent(t *testing.T) {
	svc := &stubService{}
	for _, name := range []string{"get_heartbeat", "update_heartbeat", "wake_heartbeat"} {
		args := map[string]any{"agent_id": "agent-2"}
		if name == "update_heartbeat" {
			args["enabled"] = true
		}
		_, isError := callTool(t, svc, contract.ServerContext{
			CurrentAgentID:    "main-agent",
			IsMainAgent:       true,
			SourceContextType: "room",
		}, name, args)
		if !isError {
			t.Fatalf("%s should reject another Agent target inside a Room", name)
		}
	}
}

func TestHeartbeatBackgroundMainAgentRemainsCurrentAgentReadOnly(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "main-agent",
		IsMainAgent:       true,
		SourceContextType: "agent_automation",
	}
	result, isError := callTool(t, svc, sctx, "get_heartbeat", map[string]any{"agent_id": "agent-2"})
	if !isError || !strings.Contains(extractText(t, result), "cannot list") {
		t.Fatalf("background main cross-Agent read should fail: %+v", result)
	}
	tools := listTools(t, svc, sctx)
	for _, item := range tools {
		switch item["name"] {
		case "update_heartbeat", "wake_heartbeat":
			t.Fatalf("background context exposed heartbeat write tool: %+v", item)
		}
	}
}
