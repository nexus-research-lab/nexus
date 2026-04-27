package room

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type mentionMatch struct {
	start   int
	length  int
	agentID string
}

// ResolveMentionAgentIDs 解析消息中的 @mention，并返回对应 agent_id。
func ResolveMentionAgentIDs(content string, agentNameToID map[string]string) []string {
	if strings.TrimSpace(content) == "" || len(agentNameToID) == 0 {
		return nil
	}

	names := make([]string, 0, len(agentNameToID))
	for name := range agentNameToID {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i int, j int) bool {
		return len([]rune(names[i])) > len([]rune(names[j]))
	})

	matches := make([]mentionMatch, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		pattern, err := regexp.Compile(`@` + regexp.QuoteMeta(name) + `([\s，。！？、,.!?;\-:：；]|$)`)
		if err != nil {
			continue
		}
		locations := pattern.FindAllStringIndex(content, -1)
		if len(locations) == 0 {
			continue
		}
		agentID := strings.TrimSpace(agentNameToID[name])
		if agentID == "" {
			continue
		}
		for _, location := range locations {
			matches = append(matches, mentionMatch{
				start:   location[0],
				length:  len([]rune(name)),
				agentID: agentID,
			})
		}
	}
	sort.SliceStable(matches, func(i int, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].length > matches[j].length
	})

	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, exists := seen[match.agentID]; exists {
			continue
		}
		seen[match.agentID] = struct{}{}
		result = append(result, match.agentID)
	}
	return result
}

// BuildMentionAliases 构建 Room 成员可被 @ 命中的别名表。
func BuildMentionAliases(contextValue *protocol.ConversationContextAggregate) map[string]string {
	if contextValue == nil {
		return nil
	}
	aliases := make(map[string]string, len(contextValue.MemberAgents)*3)
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID == "" {
			continue
		}
		for _, candidate := range []string{agentValue.Name, agentValue.DisplayName, agentID} {
			alias := strings.TrimSpace(candidate)
			if alias != "" {
				aliases[alias] = agentID
				aliases[strings.ToLower(alias)] = agentID
			}
		}
	}
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		if _, exists := aliases[member.MemberAgentID]; !exists {
			aliases[member.MemberAgentID] = member.MemberAgentID
		}
	}
	return aliases
}
