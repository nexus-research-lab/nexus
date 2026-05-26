package room

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResolveChatTargetAgentIDsUsesExplicitTargets(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-tom"},
		},
	}
	targets, resolution, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "没有 mention 也要给 Amy", TargetAgentIDs: []string{"agent-amy", "agent-amy", " "}},
		contextValue,
		map[string]string{"agent-amy": "Amy", "agent-tom": "Tom"},
	)
	if err != nil {
		t.Fatalf("显式 Room 目标解析失败: %v", err)
	}
	if resolution != "explicit_target" || len(targets) != 1 || targets[0] != "agent-amy" {
		t.Fatalf("显式 Room 目标解析不正确: targets=%+v resolution=%s", targets, resolution)
	}
}

func TestResolveChatTargetAgentIDsRejectsNonMemberTarget(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
		},
	}
	_, _, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "整理一下", TargetAgentIDs: []string{"agent-outsider"}},
		contextValue,
		map[string]string{"agent-amy": "Amy"},
	)
	if err == nil || !strings.Contains(err.Error(), "not a room member") {
		t.Fatalf("非成员目标应被拒绝: %v", err)
	}
}
