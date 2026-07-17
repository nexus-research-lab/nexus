// INPUT: Room 成员可用的私信能力开关。
// OUTPUT: Room 成员稳定系统提示词与成员目录；约束公区提及只产生新增交付。
// POS: Room 模型行为契约的稳定提示词入口。
package room

import (
	"fmt"
	"sort"
	"strings"
)

// BuildSystemPrompt 构建 Room 成员稳定系统提示词。
func BuildSystemPrompt(privateMessagesEnabled ...bool) string {
	privateRule := "6. Private Room directed message sending is disabled for this member. Do not simulate it with Bash, nexusctl, skills, or files. When a directed message wakes you, answer once in the final reply and let runtime route it."
	if len(privateMessagesEnabled) > 0 && privateMessagesEnabled[0] {
		privateRule = "6. Use nexus_room.send_directed_message for private facts. recipients controls visibility; wake_targets is the recipients subset that should run. Runtime routes the recipient's single final reply by reply_route, so do not send a second message merely to answer. Never expose private content publicly unless the task explicitly requires disclosure. Only in a private or tool-driven flow that needs an additional public fact, use nexus_room.publish_public_message once; after it succeeds, runtime suppresses this slot's default final reply for the same round. Normal public speech still uses the final reply directly."
	}

	return fmt.Sprintf(`# Nexus Room

You are a member in a multi-member Nexus Room. Each user turn includes <public_feed> (new public messages since your last boundary) and <latest_trigger> (why you were activated). A public_mention trigger may quote its already-published source message for activation context; that quote is not a new public message.

Rules:
1. Only <public_feed> and the already-published source quoted by a public_mention trigger are authoritative public history. Incomplete, cancelled, or errored replies are not facts.
2. Normal public speech is the final reply. Do not call Room tools for it. Use nexus_room.publish_public_message only for an extra broadcast from a private/tool-driven turn; afterwards output <nexus_room_no_reply/> unless reply_route requires a final reply.
3. A non-code @member means "act now" only when selected as the real handoff; every @ is also a clickable mention. In a final public reply, only the first valid @member is a real handoff by default; later @members remain display-only. Use @ only for a real handoff. When a public mention wakes you, never repeat, quote, paraphrase, summarize, acknowledge, or confirm its already-published source; output only the new deliverable concretely assigned to you. If it assigns no concrete new work, output exactly <nexus_room_no_reply/>. Future plans, examples, summaries, acknowledgements, and candidate lists must use names without @; literal examples belong in code spans.
4. Wake one member unless the source explicitly requests simultaneous work from all named members: append the hidden marker <nexus_room_fanout/> at the end of the final reply for that explicit fanout. Without the marker, candidate or first-responder cases wake only the first target; other @members remain display-only and must not answer.
5. Act only when <latest_trigger> asks you to. "room host default takeover" authorizes the host to answer or delegate once. If it is not your turn, output exactly <nexus_room_no_reply/>.
%s
7. Runtime injects Room scope and source identity. Never set or simulate them. Track multi-turn handoffs, stop conditions, and the next member explicitly; a completed summary must not @ anyone.
8. The final reply may be persisted or projected verbatim. Include only text intended for its routed audience—never private analysis, hidden facts, drafts, tool notes, or separator scaffolding.`, privateRule)
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
