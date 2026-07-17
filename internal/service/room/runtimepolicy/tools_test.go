package runtimepolicy

import (
	"context"
	"slices"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestToolPolicyKeepsPrivateMessagesOptIn(t *testing.T) {
	allowedTools := AllowedTools([]string{"Read"}, false)
	if len(allowedTools) != 1 || allowedTools[0] != "Read" {
		t.Fatalf("Room 普通公区发言不应注入通讯工具: %+v", allowedTools)
	}
	if slices.Contains(allowedTools, SendDirectedMessageTool) {
		t.Fatalf("Room 私信工具不应默认加入显式白名单: %+v", allowedTools)
	}
	allowedTools = AllowedTools([]string{"Read"}, true)
	if !slices.Contains(allowedTools, SendDirectedMessageTool) {
		t.Fatalf("Room 私信工具开启后应加入显式白名单: %+v", allowedTools)
	}
	if !slices.Contains(allowedTools, PublishPublicMessageTool) {
		t.Fatalf("Room 特殊流程公区工具开启后应加入显式白名单: %+v", allowedTools)
	}

	disallowedTools := DisallowedTools(nil, false)
	if !slices.Contains(disallowedTools, SendDirectedMessageTool) {
		t.Fatalf("Room 私信工具默认应加入 deny: %+v", disallowedTools)
	}
	disallowedTools = DisallowedTools(nil, true)
	if slices.Contains(disallowedTools, SendDirectedMessageTool) {
		t.Fatalf("Room 私信工具开启后不应自动加入 deny: %+v", disallowedTools)
	}
	if slices.Contains(disallowedTools, PublishPublicMessageTool) {
		t.Fatalf("Room 特殊流程公区工具开启后不应自动加入 deny: %+v", disallowedTools)
	}

	disallowedTools = DisallowedTools([]string{"nexus_room.send_directed_message"}, true)
	if slices.Contains(disallowedTools, "nexus_room.send_directed_message") {
		t.Fatalf("Room 私信开启后应移除旧的私信 deny 形态: %+v", disallowedTools)
	}
}

func TestPermissionHandlerKeepsPrivateMessagesOptIn(t *testing.T) {
	called := 0
	next := func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		called++
		return sdkpermission.Deny("denied", false), nil
	}

	defaultHandler := PermissionHandler(next, false)
	publicDecision, err := defaultHandler(context.Background(), sdkpermission.Request{ToolName: PublishPublicMessageTool})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("普通 Room 主动公区工具应直接拒绝: decision=%+v called=%d err=%v", publicDecision, called, err)
	}
	called = 0
	privateDecision, err := defaultHandler(context.Background(), sdkpermission.Request{ToolName: SendDirectedMessageTool})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("Room 私信工具默认应直接拒绝: decision=%+v called=%d err=%v", privateDecision, called, err)
	}

	enabledHandler := PermissionHandler(next, true)
	privateDecision, err = enabledHandler(context.Background(), sdkpermission.Request{ToolName: SendDirectedMessageTool})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("Room 私信工具开启后应直接放行: decision=%+v called=%d err=%v", privateDecision, called, err)
	}
	publicDecision, err = enabledHandler(context.Background(), sdkpermission.Request{ToolName: PublishPublicMessageTool})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("特殊流程公区工具开启后应直接放行: decision=%+v called=%d err=%v", publicDecision, called, err)
	}
}
