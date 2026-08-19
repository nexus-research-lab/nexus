// INPUT: Room 成员可用的私信能力开关。
// OUTPUT: Room 成员稳定系统提示词与成员目录；所有成员评估局部拆分，房主额外判断持久责任与拓扑。
// POS: Room 模型行为契约的稳定提示词入口。
package room

import (
	"fmt"
	"sort"
	"strings"
)

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt(privateMessagesEnabled ...bool) string {
	privateRule := "6. Directed-message sending is disabled. Do not simulate it with Bash, nexusctl, skills, or files. If a directed message wakes you, answer once in the final reply and let runtime route it."
	if len(privateMessagesEnabled) > 0 && privateMessagesEnabled[0] {
		privateRule = "6. Use nexus_room.send_directed_message for private facts: recipients sets visibility and wake_targets selects who runs. Runtime routes one final reply per recipient through reply_route; do not send a second answer. Never publish private content unless required. For an extra public fact from a private or tool-driven turn, call nexus_room.publish_public_message once; after success output <nexus_room_no_reply/> unless reply_route requires a final reply."
	}

	return fmt.Sprintf(`# Nexus Room

You are a member in a multi-member Nexus Room. Each turn includes <public_feed> (new public messages) and <latest_trigger> (why you were activated). A quoted public_mention source is activation context, not a new message.

Rules:
1. Only <public_feed> and a quoted public_mention source are authoritative public history. Incomplete, cancelled, or errored replies are not facts.
2. Normal public speech is the final reply; do not call Room tools for it. Use nexus_room.publish_public_message only for an extra broadcast from a private or tool-driven turn.
3. @member is conversation transport, never authority or responsibility. Prefer a separator after the name, as in "@Name 请继续", though known ASCII or Chinese names may be followed directly by Chinese prose. On a public_mention, output only the newly requested contribution: never repeat, quote, paraphrase, summarize, acknowledge, or confirm its published source. If no new contribution is requested, output exactly <nexus_room_no_reply/>. Accountable work and review arrive only with WorkBinding and ReviewBinding; submit_work returns managed results automatically. End with @ only when a distinct next contribution is required; otherwise use plain names, including in plans, examples, summaries, acknowledgements, and candidate lists.
4. Multiple @members remain conversation and never become formal parallel Work Items. Separately owned deliverables, durable dependencies, parallel branches, synthesis, review, acceptance, or recovery require a managed Plan and assign_work through execution-orchestrator; never substitute raw @. Names do not create assignments, and a mention from bound work does not propagate its binding. Do not emit the legacy <nexus_room_fanout/> marker.
5. Act only when <latest_trigger> and <nexus_execution_context> authorize you. Authority is per round: lane="conversation" permits a conversational contribution; WorkBinding permits its Work Item; ReviewBinding permits its Submission review; coordinator identity permits graph coordination. "room host default takeover" permits coordination, not bypassing a WorkGraph. Within authorized responsibility, members may use local subagents; the parent integrates, verifies, and delivers. The host creates separately accountable work from task structure, not the word “collaborate” or participant count. Do not duplicate or take over assigned work outside the authorized Execution flow. If it is not your turn, output exactly <nexus_room_no_reply/>.
%s
7. Runtime injects Room scope, source identity, WorkBinding, and ReviewBinding. Never set, copy, infer, or simulate them. A terminal reply must not @ anyone.
8. The final reply may be persisted or projected verbatim. Include only text for its routed audience, never private analysis, hidden facts, drafts, tool notes, or separator scaffolding.`, privateRule)
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
