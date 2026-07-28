package conversation

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoundRecoveryContextualInputsNormalizesLegacyContentFilter(t *testing.T) {
	history := []protocol.Message{{
		"role":        "assistant",
		"agent_id":    "agent-a",
		"is_complete": true,
		"content": []map[string]any{{
			"type": "text",
			"text": "[1301][系统检测到输入或生成内容可能包含不安全或敏感内容]",
		}},
		"result_summary": map[string]any{
			"subtype":         "error",
			"is_error":        true,
			"terminal_reason": "invalid_request",
			"result":          "[1301] upstream raw error",
		},
	}}

	blocks := RoundRecoveryContextualInputs(history, "agent-a")
	if len(blocks) != 1 {
		t.Fatalf("recovery blocks = %d, want 1", len(blocks))
	}
	block := blocks[0]
	if block.Metadata["terminal_reason"] != protocol.ProviderFailureContentFiltered {
		t.Fatalf("terminal reason = %q", block.Metadata["terminal_reason"])
	}
	if !strings.Contains(block.Content, "content_filtered") {
		t.Fatalf("隐藏上下文未包含稳定原因: %q", block.Content)
	}
	if strings.Contains(block.Content, "1301") || strings.Contains(block.Content, "upstream raw error") {
		t.Fatalf("隐藏上下文不应回放 Provider 原始错误: %q", block.Content)
	}
}

func TestRoundRecoveryContextualInputsUsesLatestTerminalPerAgent(t *testing.T) {
	failure := protocol.Message{
		"role":        "assistant",
		"agent_id":    "agent-a",
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype":         "error",
			"is_error":        true,
			"terminal_reason": "authentication_failed",
		},
	}
	otherAgentSuccess := protocol.Message{
		"role":        "assistant",
		"agent_id":    "agent-b",
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype":  "success",
			"is_error": false,
		},
	}
	blocks := RoundRecoveryContextualInputs([]protocol.Message{failure, otherAgentSuccess}, "agent-a")
	if len(blocks) != 1 || blocks[0].Metadata["terminal_reason"] != recoveryAuthenticationFailed {
		t.Fatalf("Agent A 应保留自己的最新失败上下文: %+v", blocks)
	}

	success := protocol.Message{
		"role":        "assistant",
		"agent_id":    "agent-a",
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype":  "success",
			"is_error": false,
		},
	}
	if got := RoundRecoveryContextualInputs([]protocol.Message{failure, success}, "agent-a"); len(got) != 0 {
		t.Fatalf("较新的成功终态应屏蔽旧失败: %+v", got)
	}
}

func TestRoundRecoveryContextualInputsDoesNotExposeUnknownReason(t *testing.T) {
	history := []protocol.Message{{
		"role":            "result",
		"is_error":        true,
		"subtype":         "error",
		"terminal_reason": "provider-secret-diagnostic",
		"errors":          []string{"credential-like raw provider detail"},
	}}
	blocks := RoundRecoveryContextualInputs(history, "")
	if len(blocks) != 1 || blocks[0].Metadata["terminal_reason"] != recoveryRuntimeError {
		t.Fatalf("未知原因应归一为 runtime_error: %+v", blocks)
	}
	if strings.Contains(blocks[0].Content, "provider-secret") ||
		strings.Contains(blocks[0].Content, "credential-like") {
		t.Fatalf("未知 Provider 原因不应进入隐藏上下文: %q", blocks[0].Content)
	}
}
