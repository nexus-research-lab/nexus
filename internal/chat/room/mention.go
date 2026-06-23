package room

import (
	"cmp"
	"regexp"
	"slices"
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
	content = maskBacktickCodeRegions(content)

	names := make([]string, 0, len(agentNameToID))
	for name := range agentNameToID {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	slices.SortFunc(names, func(left string, right string) int {
		return cmp.Compare(len([]rune(right)), len([]rune(left)))
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
	slices.SortStableFunc(matches, func(left mentionMatch, right mentionMatch) int {
		if result := cmp.Compare(left.start, right.start); result != 0 {
			return result
		}
		return cmp.Compare(right.length, left.length)
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

// maskBacktickCodeRegions 保留原始字节位置，把反引号代码区替换为空格，避免示例里的 @ 触发执行。
func maskBacktickCodeRegions(content string) string {
	if !strings.Contains(content, "`") {
		return content
	}
	masked := []byte(content)
	for index := 0; index < len(masked); {
		if masked[index] != '`' {
			index++
			continue
		}
		start := index
		for index < len(masked) && masked[index] == '`' {
			index++
		}
		tickCount := index - start
		closing := strings.Index(string(masked[index:]), strings.Repeat("`", tickCount))
		end := len(masked)
		if closing >= 0 {
			end = index + closing + tickCount
		}
		for cursor := start; cursor < end; cursor++ {
			masked[cursor] = ' '
		}
		index = end
	}
	return string(masked)
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
