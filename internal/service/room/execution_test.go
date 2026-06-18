package room

import (
	"context"
	"slices"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestRoomRuntimeToolPolicyKeepsPrivateMessagesOptIn(t *testing.T) {
	allowedTools := roomRuntimeAllowedTools([]string{"Read"}, false)
	if !slices.Contains(allowedTools, roomPublishPublicMessageTool) {
		t.Fatalf("Room 公开通讯工具应自动加入显式白名单: %+v", allowedTools)
	}
	if slices.Contains(allowedTools, roomSendDirectedMessageTool) {
		t.Fatalf("Room 私信工具不应默认加入显式白名单: %+v", allowedTools)
	}
	allowedTools = roomRuntimeAllowedTools([]string{"Read"}, true)
	if !slices.Contains(allowedTools, roomSendDirectedMessageTool) {
		t.Fatalf("Room 私信工具开启后应加入显式白名单: %+v", allowedTools)
	}

	disallowedTools := roomRuntimeDisallowedTools(nil, false)
	if !slices.Contains(disallowedTools, roomSendDirectedMessageTool) {
		t.Fatalf("Room 私信工具默认应加入 deny: %+v", disallowedTools)
	}
	disallowedTools = roomRuntimeDisallowedTools(nil, true)
	if slices.Contains(disallowedTools, roomSendDirectedMessageTool) {
		t.Fatalf("Room 私信工具开启后不应自动加入 deny: %+v", disallowedTools)
	}

	disallowedTools = roomRuntimeDisallowedTools([]string{"nexus_room.send_directed_message"}, true)
	if slices.Contains(disallowedTools, "nexus_room.send_directed_message") {
		t.Fatalf("Room 私信开启后应移除旧的私信 deny 形态: %+v", disallowedTools)
	}
}

func TestRoomRuntimePermissionHandlerKeepsPrivateMessagesOptIn(t *testing.T) {
	called := 0
	next := func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		called++
		return sdkpermission.Deny("denied", false), nil
	}

	defaultHandler := roomRuntimePermissionHandler(next, false)
	publicDecision, err := defaultHandler(context.Background(), sdkpermission.Request{ToolName: roomPublishPublicMessageTool})
	if err != nil || publicDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("Room 公开通讯工具应默认放行: decision=%+v called=%d err=%v", publicDecision, called, err)
	}
	privateDecision, err := defaultHandler(context.Background(), sdkpermission.Request{ToolName: roomSendDirectedMessageTool})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorDeny || called != 0 {
		t.Fatalf("Room 私信工具默认应直接拒绝: decision=%+v called=%d err=%v", privateDecision, called, err)
	}

	enabledHandler := roomRuntimePermissionHandler(next, true)
	privateDecision, err = enabledHandler(context.Background(), sdkpermission.Request{ToolName: roomSendDirectedMessageTool})
	if err != nil || privateDecision.Behavior != sdkpermission.BehaviorAllow || called != 0 {
		t.Fatalf("Room 私信工具开启后应直接放行: decision=%+v called=%d err=%v", privateDecision, called, err)
	}
}
