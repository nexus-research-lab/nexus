package room

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MentionMatch 是一处已经通过 Room 成员目录校验的 @mention span。
// start/end 使用 Unicode rune 偏移，前端可据此稳定替换为 Agent chip。
type MentionMatch struct {
	AgentID   string
	Label     string
	StartRune int
	EndRune   int
}

// ResolveMentionAgentIDs 解析消息中的 @mention，并按文本顺序返回去重后的 agent_id。
func ResolveMentionAgentIDs(content string, agentNameToID map[string]string) []string {
	matches := ResolveMentionMatches(content, agentNameToID)
	seen := make(map[string]struct{}, len(matches))
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, exists := seen[match.AgentID]; exists {
			continue
		}
		seen[match.AgentID] = struct{}{}
		result = append(result, match.AgentID)
	}
	return result
}

// ResolveMentionMatches 返回所有可用于 UI 标注与 handoff 的 mention span。
// 代码区和链接/标识符中的模糊 @ 不会被解析；重叠别名只保留最长匹配。
func ResolveMentionMatches(content string, agentNameToID map[string]string) []MentionMatch {
	if strings.TrimSpace(content) == "" || len(agentNameToID) == 0 {
		return nil
	}
	masked := maskMentionExcludedRegions(content)
	namesByKey := make(map[string]string, len(agentNameToID))
	ambiguous := make(map[string]struct{})
	for name, agentID := range agentNameToID {
		name = strings.TrimSpace(name)
		agentID = strings.TrimSpace(agentID)
		key := strings.ToLower(name)
		if name == "" || agentID == "" || key == "" {
			continue
		}
		if _, blocked := ambiguous[key]; blocked {
			continue
		}
		if existing, exists := namesByKey[key]; exists && existing != agentID {
			delete(namesByKey, key)
			ambiguous[key] = struct{}{}
			continue
		}
		namesByKey[key] = agentID
	}
	type mentionAlias struct {
		name    string
		agentID string
	}
	aliases := make([]mentionAlias, 0, len(namesByKey))
	for key, agentID := range namesByKey {
		aliases = append(aliases, mentionAlias{name: key, agentID: agentID})
	}
	slices.SortFunc(aliases, func(left mentionAlias, right mentionAlias) int {
		if result := cmp.Compare(len([]rune(right.name)), len([]rune(left.name))); result != 0 {
			return result
		}
		return cmp.Compare(left.name, right.name)
	})

	all := make([]MentionMatch, 0, len(aliases))
	for _, alias := range aliases {
		pattern, err := regexp.Compile(`(?i)@` + regexp.QuoteMeta(alias.name) + `([\s，。！？、,.!?;\-:：；]|$)`)
		if err != nil {
			continue
		}
		for _, location := range pattern.FindAllStringSubmatchIndex(masked, -1) {
			if len(location) < 4 || !isMentionBoundary(content, location[0]) {
				continue
			}
			matchEnd := location[1]
			if location[2] >= 0 {
				matchEnd = location[2]
			}
			if matchEnd <= location[0] || matchEnd > len(content) {
				continue
			}
			startRune := utf8.RuneCountInString(content[:location[0]])
			endRune := startRune + utf8.RuneCountInString(content[location[0]:matchEnd])
			label := strings.TrimPrefix(content[location[0]:matchEnd], "@")
			all = append(all, MentionMatch{
				AgentID:   alias.agentID,
				Label:     label,
				StartRune: startRune,
				EndRune:   endRune,
			})
		}
	}
	slices.SortStableFunc(all, func(left MentionMatch, right MentionMatch) int {
		if result := cmp.Compare(left.StartRune, right.StartRune); result != 0 {
			return result
		}
		return cmp.Compare(right.EndRune-right.StartRune, left.EndRune-left.StartRune)
	})
	result := make([]MentionMatch, 0, len(all))
	lastEnd := -1
	for _, match := range all {
		if match.StartRune < lastEnd {
			continue
		}
		result = append(result, match)
		lastEnd = match.EndRune
	}
	return result
}

func isMentionBoundary(content string, byteIndex int) bool {
	if byteIndex <= 0 || byteIndex > len(content) {
		return byteIndex == 0
	}
	previous, _ := utf8.DecodeLastRuneInString(content[:byteIndex])
	return !unicode.IsLetter(previous) && !unicode.IsDigit(previous) && previous != '_' && previous != '@'
}

func maskMentionExcludedRegions(content string) string {
	return maskMarkdownLinkDestinations(maskBacktickCodeRegions(content))
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

func maskMarkdownLinkDestinations(content string) string {
	masked := []byte(content)
	for index := 0; index+1 < len(masked); index++ {
		if masked[index] != ']' || masked[index+1] != '(' {
			continue
		}
		depth := 0
		end := -1
		for cursor := index + 1; cursor < len(masked); cursor++ {
			switch masked[cursor] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = cursor
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			continue
		}
		for cursor := index + 1; cursor <= end; cursor++ {
			masked[cursor] = ' '
		}
		index = end
	}
	return string(masked)
}

// BuildMentionAliases 构建 Room 成员可被 @ 命中的别名表。
// 同一别名指向多个成员时标记为歧义并移除，避免错误 handoff。
func BuildMentionAliases(contextValue *protocol.ConversationContextAggregate) map[string]string {
	if contextValue == nil {
		return nil
	}
	idsByAlias := make(map[string]map[string]struct{}, len(contextValue.MemberAgents)*3)
	formsByAlias := make(map[string]map[string]struct{}, len(contextValue.MemberAgents)*3)
	addAlias := func(alias string, agentID string) {
		alias = strings.TrimSpace(alias)
		agentID = strings.TrimSpace(agentID)
		key := strings.ToLower(alias)
		if alias == "" || key == "" || agentID == "" {
			return
		}
		ids := idsByAlias[key]
		if ids == nil {
			ids = make(map[string]struct{})
			idsByAlias[key] = ids
		}
		ids[agentID] = struct{}{}
		forms := formsByAlias[key]
		if forms == nil {
			forms = make(map[string]struct{})
			formsByAlias[key] = forms
		}
		forms[alias] = struct{}{}
	}
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID == "" {
			continue
		}
		for _, candidate := range []string{agentValue.Name, agentValue.DisplayName, agentID} {
			addAlias(candidate, agentID)
		}
	}
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		addAlias(member.MemberAgentID, member.MemberAgentID)
	}
	aliases := make(map[string]string, len(idsByAlias))
	for alias, agentIDs := range idsByAlias {
		if len(agentIDs) != 1 {
			continue
		}
		for agentID := range agentIDs {
			for form := range formsByAlias[alias] {
				aliases[form] = agentID
				aliases[strings.ToLower(form)] = agentID
			}
		}
	}
	return aliases
}
