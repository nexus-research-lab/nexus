package realtime

import (
	"context"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestToolPolicyKeepsPrivateMessagesOptIn(t *testing.T) {
	allowedTools := roomAllowedTools([]string{"Read"}, false)
	if len(allowedTools) != 1 || allowedTools[0] != "Read" {
		t.Fatalf("Room 普通公区发言不应注入通讯工具: %+v", allowedTools)
	}
	if slices.Contains(allowedTools, roomSendMessageTool) {
		t.Fatalf("Room 私信工具不应默认加入显式白名单: %+v", allowedTools)
	}
	allowedTools = roomAllowedTools([]string{"Read"}, true)
	if !slices.Equal(allowedTools, []string{"Read"}) {
		t.Fatalf("Room 私信开关不能扩大 Agent 显式白名单: %+v", allowedTools)
	}

	disallowedTools := roomDisallowedTools(nil, false)
	if len(disallowedTools) != 0 {
		t.Fatalf("Room 开关不应通过 deny 改变 Session 工具面: %+v", disallowedTools)
	}
	disallowedTools = roomDisallowedTools(nil, true)
	if slices.Contains(disallowedTools, roomSendMessageTool) {
		t.Fatalf("Room 私信工具开启后不应自动加入 deny: %+v", disallowedTools)
	}

	disallowedTools = roomDisallowedTools([]string{"nexus.send_message"}, true)
	if !slices.Contains(disallowedTools, "nexus.send_message") {
		t.Fatalf("Room 私信开启后仍必须保留 Agent deny: %+v", disallowedTools)
	}
}

func TestRoomRoundToolPolicyPrefersAutomationSnapshot(t *testing.T) {
	round := &activeRoomRound{RuntimeToolPolicy: &protocol.RuntimeToolPolicy{
		AllowedTools:    []string{"WebSearch"},
		DisallowedTools: []string{"Write"},
	}}
	agentValue := &protocol.Agent{Options: protocol.Options{
		AllowedTools:    []string{"Read"},
		DisallowedTools: []string{"Bash"},
	}}
	allowed, denied, snapshotted := roomRoundToolPolicy(round, agentValue)
	if !snapshotted || !slices.Equal(allowed, []string{"WebSearch"}) || !slices.Equal(denied, []string{"Write"}) {
		t.Fatalf("Room automation snapshot 未覆盖 Agent 当前工具配置: allow=%v deny=%v snapshotted=%v", allowed, denied, snapshotted)
	}
}

func TestPermissionHandlerKeepsPrivateMessagesOptIn(t *testing.T) {
	called := 0
	next := func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		called++
		return sdkpermission.Deny("denied", false), nil
	}

	defaultHandler := withRoomPermissionPolicy(next, false, nil, nil)
	publicDecision, err := defaultHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "public"},
	})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("普通 Room 主动公区工具应直接拒绝: decision=%+v called=%d err=%v", publicDecision, called, err)
	}
	called = 0
	privateDecision, err := defaultHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "private"},
	})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("Room 私信工具默认应直接拒绝: decision=%+v called=%d err=%v", privateDecision, called, err)
	}
	crossContextDecision, err := defaultHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "contact"},
	})
	if err != nil || crossContextDecision.Behavior != sdkpermission.BehaviorDeny || called != 1 {
		t.Fatalf("跨会话发送应继续走普通权限处理器: decision=%+v called=%d err=%v", crossContextDecision, called, err)
	}
	called = 0

	enabledHandler := withRoomPermissionPolicy(next, true, nil, nil)
	privateDecision, err = enabledHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "private"},
	})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("Room 私信工具开启后应直接放行: decision=%+v called=%d err=%v", privateDecision, called, err)
	}
	publicDecision, err = enabledHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "public"},
	})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("特殊流程公区工具开启后应直接放行: decision=%+v called=%d err=%v", publicDecision, called, err)
	}

	deniedHandler := withRoomPermissionPolicy(next, true, nil, []string{"nexus_communication"})
	privateDecision, err = deniedHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "private"},
	})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("Agent broad deny 必须覆盖 Room 私信开关: decision=%+v called=%d err=%v", privateDecision, called, err)
	}

	restrictedHandler := withRoomPermissionPolicy(next, true, []string{"Read"}, nil)
	publicDecision, err = restrictedHandler(context.Background(), sdkpermission.Request{
		ToolName: roomSendMessageTool,
		Input:    map[string]any{"destination": "current_room", "visibility": "public"},
	})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("Room 私信开关不能扩大 Agent allowlist: decision=%+v called=%d err=%v", publicDecision, called, err)
	}
}
