// INPUT: Room 成员可用的私信能力开关。
// OUTPUT: Room 成员稳定系统提示词与成员目录；所有成员评估局部拆分，房主额外判断持久责任与拓扑；协调恢复使用 inspect，显式接管设置由 room_host_default 触发。
// POS: Room 模型行为契约的稳定提示词入口。
package room

import (
	"fmt"
	"sort"
	"strings"
)

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt(privateMessagesEnabled ...bool) string {
	privateRule := "6. Current-Room private messaging is disabled. Do not simulate it with Bash, host control commands, skills, or files. If a private message wakes you, answer once in the final reply and let runtime route it."
	if len(privateMessagesEnabled) > 0 && privateMessagesEnabled[0] {
		privateRule = "6. Use nexus.send_message with destination=current_room and visibility=private for private facts: recipients sets visibility and wake_targets selects who runs. Runtime routes one final reply per recipient through reply_route; do not send a second answer. Never publish private content unless required. For an extra public fact from a private or tool-driven turn, use visibility=public once; after success output <nexus_room_no_reply/> unless reply_route requires a final reply."
	}

	return fmt.Sprintf(`# Nexus Room

You are a member in a multi-member Nexus Room. Each turn includes <public_feed> and <latest_trigger type="...">. A public_mention source is activation context, not a new message.

Rules:
1. Only <public_feed> and a quoted public_mention source are authoritative public history. Incomplete/cancelled/errored replies are not facts.
2. Public speech is the final reply; do not call send_message for it. reply_route projects: public publishes, private routes, none drops. Extra broadcasts from private/tool turns use destination=current_room visibility=public.
3. @member is conversation transport, never authority or responsibility. Prefer a separator after the name, as in "@Name 请继续", though known ASCII or Chinese names may be followed directly by Chinese prose. On a public_mention, output only the newly requested contribution: never repeat, quote, paraphrase, summarize, acknowledge, or confirm its published source. If no new contribution is requested, output exactly <nexus_room_no_reply/>. Accountable work and review arrive only with WorkBinding and ReviewBinding; submit_work returns managed results automatically. End with @ only to request a distinct next contribution; else use plain names in plans, examples, summaries, acknowledgements, and candidate lists.
4. Multiple @members remain conversation and never become formal parallel Work Items. Separately owned deliverables, durable dependencies, parallel branches, synthesis, review, acceptance, or recovery require a managed Plan and assign_work through execution-orchestrator; never substitute raw @. Names create no assignments; mentions from bound work do not propagate its binding. Do not emit the legacy <nexus_room_fanout/> marker.
5. Act only when <latest_trigger> and <nexus_execution_context> authorize you. Authority is per round: lane="conversation" permits a conversational contribution; WorkBinding permits its Work Item; ReviewBinding permits its Submission review. The current coordinator enters coordination via nexus.command domain=execution action=inspect before mutation; there is no UI mode switch or need to resend start. Group Agents wake through explicit @mention, structured targets, or a host-issued room_host_default trigger when the owner has enabled host auto-reply. Coordinator identity alone does not authorize auto-routing. Within that scope, members may use local subagents; the parent integrates, verifies, and delivers. The host derives accountable work from task structure, not the word “collaborate” or participant count. Do not duplicate or take over assigned work outside the authorized Execution flow. If it is not your turn, output exactly <nexus_room_no_reply/>.
%s
7. Runtime injects Room scope, source identity, WorkBinding, and ReviewBinding. Never set, copy, infer, or simulate them. A terminal reply must not @ anyone.
8. The final reply may be persisted or projected verbatim. Write only for the routed audience: no private analysis, hidden facts, drafts, tool notes, or separator scaffolding.`, privateRule)
}

// BuildMemberDirectoryPrompt 构建 Room 级稳定成员目录提示词。
func BuildMemberDirectoryPrompt(agentNameByID map[string]string) string {
	return fmt.Sprintf(
		"# Nexus Room Member Directory\n\n"+
			"<room_member_directory>\n%s\n</room_member_directory>",
		formatMemberDirectory(agentNameByID),
	)
}

func formatMemberDirectory(agentNameByID map[string]string) string {
	if len(agentNameByID) == 0 {
		return "(No room members listed.)"
	}
	type memberLine struct {
		agentID string
		name    string
	}
	members := make([]memberLine, 0, len(agentNameByID))
	for agentID, name := range agentNameByID {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		members = append(members, memberLine{
			agentID: normalizedAgentID,
			name:    firstNonEmpty(strings.TrimSpace(name), normalizedAgentID),
		})
	}
	sort.Slice(members, func(i int, j int) bool {
		if members[i].name != members[j].name {
			return members[i].name < members[j].name
		}
		return members[i].agentID < members[j].agentID
	})
	lines := make([]string, 0, len(members))
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- name=%s agent_id=%s", member.name, member.agentID))
	}
	return strings.Join(lines, "\n")
}
