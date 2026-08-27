package websocket

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
)

func TestAppServerGoalRPCErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int64
		wantReason string
	}{
		{
			name:       "goal conflict",
			err:        goalsvc.ErrGoalConflict,
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonConflict,
		},
		{
			name:       "version stale",
			err:        fmt.Errorf("concurrent update: %w", goalsvc.ErrGoalVersionStale),
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonVersionStale,
		},
		{
			name:       "objective revision stale",
			err:        goalsvc.ErrGoalRevisionStale,
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonRevisionStale,
		},
		{
			name:       "execution binding conflict",
			err:        fmt.Errorf("binding read: %w", goalsvc.ErrGoalExecutionBindingConflict),
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonExecutionBindingConflict,
		},
		{
			name:     "invalid state",
			err:      goalsvc.ErrGoalInvalidState,
			wantCode: goalappserver.AppServerRPCInvalidRequestCode,
		},
		{
			name:     "unknown internal error",
			err:      errors.New("repository unavailable"),
			wantCode: goalappserver.AppServerRPCInternalErrorCode,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := appServerGoalRPCError(test.err)
			if result.Code != test.wantCode {
				t.Fatalf("code = %d, want %d", result.Code, test.wantCode)
			}
			if test.wantReason == "" {
				if result.Data != nil {
					t.Fatalf("data = %#v, want nil", result.Data)
				}
				return
			}
			data, ok := result.Data.(goalappserver.AppServerRPCErrorData)
			if !ok || data.ReasonCode != test.wantReason {
				t.Fatalf("data = %#v, want reason_code %q", result.Data, test.wantReason)
			}
		})
	}
}

func TestChatErrorDetailExplainsSubscriptionQuota(t *testing.T) {
	message := chatErrorDetail(subscriptionsvc.QuotaExceededError{
		UsedTokens:  200000,
		LimitTokens: 200000,
	})
	for _, want := range []string{
		"当前账号本月的订阅额度已全部用尽",
		"新的 Agent 请求",
		"输出长度限制",
		"升级套餐",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("额度错误提示缺少 %q: %q", want, message)
		}
	}
}

func TestChatErrorDetailExplainsRuntimeFailures(t *testing.T) {
	tests := []struct {
		name  string
		err   string
		wants []string
	}{
		{
			name: "missing Claude command",
			err:  `client: backend executable "process backend" not found: process: cli executable "claude.exe" not found`,
			wants: []string{
				"Claude Code",
				"NEXUS_CLAUDE_COMMAND_PATH",
				"command -v claude",
				"claude doctor",
				"brew install --cask claude-code",
				"winget install Anthropic.ClaudeCode",
				"~/.local/bin/claude",
				"/opt/homebrew/bin/claude",
			},
		},
		{
			name:  "missing nxs command",
			err:   `client: backend executable "process backend" not found: process: cli executable "nxs" not found`,
			wants: []string{"nxs runtime", "sidecar", "NEXUS_NXS_COMMAND_PATH"},
		},
		{
			name:  "provider config",
			err:   "provider=default 配置不完整: auth_token, model",
			wants: []string{"供应商", "API Key", "Base URL", "模型"},
		},
		{
			name:  "missing default model",
			err:   "provider=default 未配置默认模型",
			wants: []string{"默认对话模型", "设置 → 常规", "设置 → 供应商"},
		},
		{
			name:  "provider authentication",
			err:   "client: runtime startup failed: Failed to authenticate. API Error: 401",
			wants: []string{"鉴权失败", "API Key", "保存并测试"},
		},
		{
			name:  "unsupported responses api",
			err:   "provider=ri 的 api_format=responses 暂不可用于 claude Agent runtime",
			wants: []string{"Responses API", "nxs Agent runtime", "Settings"},
		},
		{
			name:  "provider overload",
			err:   `client: runtime startup failed: provider_error=server_overload stderr="API error: 529 {\"type\":\"overloaded_error\"}": context deadline exceeded`,
			wants: []string{"模型请求暂时受限", "LLM Provider"},
		},
		{
			name:  "provider rate limit",
			err:   `client: runtime startup failed: provider_error=rate_limit stderr="API error: 429 rate_limit_error": context deadline exceeded`,
			wants: []string{"模型请求暂时受限", "LLM Provider"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := chatErrorDetail(errors.New(test.err))
			for _, want := range test.wants {
				if !strings.Contains(message, want) {
					t.Fatalf("chatErrorDetail() = %q, want substring %q", message, want)
				}
			}
		})
	}
}

func TestChatErrorDetailUsesRuntimeNeutralFallback(t *testing.T) {
	message := chatErrorDetail(errors.New("client: runtime startup failed: context deadline exceeded"))
	if !strings.Contains(message, "设置 → 运行时") {
		t.Fatalf("兜底提示应覆盖当前 runtime: %q", message)
	}
	if strings.Contains(message, "Claude Code") {
		t.Fatalf("兜底提示不应把 nxs 失败误报成 Claude Code: %q", message)
	}
}

func TestCommandCatalogErrorUsesRuntimeStartupGuidance(t *testing.T) {
	message := (&Handler{}).errorEventDetail(
		"command_catalog_error",
		errors.New(`client: cli executable "nxs" not found`),
	)
	if !strings.Contains(message, "nxs runtime") ||
		!strings.Contains(message, "NEXUS_NXS_COMMAND_PATH") {
		t.Fatalf("command catalog startup guidance = %q", message)
	}
}

func TestNewGatewayErrorEventUsesRoundID(t *testing.T) {
	event := (&Handler{}).newGatewayErrorEvent(
		"agent:agent-1:ws:dm:session-1",
		"chat_error",
		"启动失败",
		map[string]any{
			"type":     "chat",
			"round_id": "round-1",
		},
	)
	if event.RoundID != "round-1" {
		t.Fatalf("error round_id = %q, want round-1", event.RoundID)
	}
	if got := event.Data["round_id"]; got != "round-1" {
		t.Fatalf("error data.round_id = %#v, want round-1", got)
	}
	if got := event.Data["failure_code"]; got != protocol.ConversationFailureProviderUnavailable {
		t.Fatalf("error failure_code = %#v, want provider_unavailable", got)
	}
}

func TestGatewayFailureCodeUsesStructuredProductCategory(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		err       error
		want      protocol.ConversationFailureCode
	}{
		{name: "usage", errorType: "chat_error", err: subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}, want: protocol.ConversationFailureUsageLimited},
		{name: "provider config", errorType: "chat_error", err: errors.New("authentication_error"), want: protocol.ConversationFailureProviderConfiguration},
		{name: "provider capacity", errorType: "chat_error", err: errors.New("provider_error=server_overload"), want: protocol.ConversationFailureProviderUnavailable},
		{name: "validation", errorType: "invalid_session_key", err: errors.New("invalid"), want: protocol.ConversationFailureValidationFailed},
		{name: "permission", errorType: "permission_request_not_found", err: errors.New("missing"), want: protocol.ConversationFailurePermissionNotSent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := gatewayFailureCode(test.errorType, test.err); got != test.want {
				t.Fatalf("gatewayFailureCode() = %q, want %q", got, test.want)
			}
		})
	}
}
