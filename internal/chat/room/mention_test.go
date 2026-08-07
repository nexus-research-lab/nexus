package room

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResolveMentionAgentIDsKeepsAllMentionedMembers(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
	}

	for _, content := range []string{
		"@Amy @Devin 分别给一个建议",
		"@Amy 输出完后 @Devin 再回答",
		"@Amy 让 @Devin 查天气并在公区回复",
	} {
		got := ResolveMentionAgentIDs(content, aliases)
		want := []string{"agent-amy", "agent-devin"}
		if !sameStringSet(got, want) {
			t.Fatalf("Room @ 目标解析不应按自然语言裁剪: content=%q got=%v want=%v", content, got, want)
		}
	}
}

func TestResolveMentionAgentIDsPreservesTextOrder(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
		"sam":   "agent-sam",
	}

	got := ResolveMentionAgentIDs("@sam 先来，然后 @Amy 收一下，最后 @Devin 总结", aliases)
	want := []string{"agent-sam", "agent-amy", "agent-devin"}
	if !sameStringSlice(got, want) {
		t.Fatalf("Room @ 目标解析应按文本顺序返回: got=%v want=%v", got, want)
	}
}

func TestResolveMentionAgentIDsIgnoresBacktickCode(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
		"Jim":   "agent-jim",
		"Sam":   "agent-sam",
	}

	got := ResolveMentionAgentIDs(
		"首位投票 @Jim，结束用 `@Sam`。\n```text\n@Devin 这里只是示例\n```\n最后交回 @Amy",
		aliases,
	)
	want := []string{"agent-jim", "agent-amy"}
	if !sameStringSlice(got, want) {
		t.Fatalf("代码区里的 @ 不应触发 Room 唤醒: got=%v want=%v", got, want)
	}
}

func TestResolveMentionMatchesKeepsUnicodeRuneSpans(t *testing.T) {
	aliases := map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"}
	content := "请 @阿梅 看一下，再请 @Devin 总结。"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 2 {
		t.Fatalf("应解析两处 mention: %#v", matches)
	}
	if got := []rune(content)[matches[0].StartRune:matches[0].EndRune]; string(got) != "@阿梅" {
		t.Fatalf("中文 mention span 应使用 rune 偏移: got=%q span=%+v", string(got), matches[0])
	}
	if matches[1].AgentID != "agent-devin" || matches[1].StartRune <= matches[0].EndRune {
		t.Fatalf("mention 顺序或目标不正确: %#v", matches)
	}
}

func TestResolveMentionMatchesAcceptsParenthesizedAgentID(t *testing.T) {
	aliases := map[string]string{"生活方案制定员": "agent-plan"}
	content := "@生活方案制定员（c742e12ab802）请承接本次咨询"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 1 || matches[0].AgentID != "agent-plan" {
		t.Fatalf("名字后的全角括号不应阻断 mention: %#v", matches)
	}
	runes := []rune(content)
	if got := string(runes[matches[0].StartRune:matches[0].EndRune]); got != "@生活方案制定员" {
		t.Fatalf("括号中的 agent id 不应进入 mention span: got=%q span=%+v", got, matches[0])
	}
}

func TestResolveMentionMatchesDoesNotSplitIdentifierSuffix(t *testing.T) {
	aliases := map[string]string{"Amy": "agent-amy"}
	if matches := ResolveMentionMatches("请检查 @Amy_name 的结果", aliases); len(matches) != 0 {
		t.Fatalf("连接符后的标识符不应截断成 mention: %#v", matches)
	}
}

func TestResolveMentionMatchesAcceptsHanTextAfterASCIIAliasWithoutSplittingLongerAlias(t *testing.T) {
	aliases := map[string]string{
		"Agent1":  "agent-1",
		"Agent10": "agent-10",
	}
	content := "@Agent10以上为前期资料；@Agent1以上为最终结果。"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 2 {
		t.Fatalf("英文成员名紧跟汉字正文应解析两处 mention: %#v", matches)
	}
	if matches[0].AgentID != "agent-10" || matches[1].AgentID != "agent-1" {
		t.Fatalf("最长别名与文本顺序应保持稳定: %#v", matches)
	}
	runes := []rune(content)
	if got := string(runes[matches[0].StartRune:matches[0].EndRune]); got != "@Agent10" {
		t.Fatalf("较长别名 mention span 不正确: got=%q span=%+v", got, matches[0])
	}
	if got := string(runes[matches[1].StartRune:matches[1].EndRune]); got != "@Agent1" {
		t.Fatalf("无分隔符 mention span 不正确: got=%q span=%+v", got, matches[1])
	}
}

func TestResolveMentionMatchesAcceptsHanTextAfterHanAliasUsingLongestKnownName(t *testing.T) {
	aliases := map[string]string{
		"研究":  "agent-research",
		"研究员": "agent-researcher",
		"分析师": "agent-analyst",
	}
	content := "@研究员请先收集资料，@分析师随后复核。"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 2 {
		t.Fatalf("中文成员名紧跟中文正文应稳定解析: %#v", matches)
	}
	if matches[0].AgentID != "agent-researcher" || matches[1].AgentID != "agent-analyst" {
		t.Fatalf("应优先采用最长已知别名并保持文本顺序: %#v", matches)
	}
	runes := []rune(content)
	if got := string(runes[matches[0].StartRune:matches[0].EndRune]); got != "@研究员" {
		t.Fatalf("最长中文 mention span 不正确: got=%q span=%+v", got, matches[0])
	}
	if got := string(runes[matches[1].StartRune:matches[1].EndRune]); got != "@分析师" {
		t.Fatalf("第二处无空格 mention span 不正确: got=%q span=%+v", got, matches[1])
	}
}

func TestResolveMentionMatchesAcceptsCJKProseAroundKnownAliasesWithoutSpaces(t *testing.T) {
	aliases := map[string]string{
		"Researcher": "agent-researcher",
		"分析师":        "agent-analyst",
	}
	content := "请@Researcher继续收集，随后@分析师复核。"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 2 ||
		matches[0].AgentID != "agent-researcher" ||
		matches[1].AgentID != "agent-analyst" {
		t.Fatalf("@ 前后都没有空格时仍应按已知成员稳定解析: %#v", matches)
	}
}

func TestResolveMentionMatchesAcceptsAdjacentKnownAliases(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
	}
	matches := ResolveMentionMatches("@Amy@Devin请分别检查。", aliases)
	if len(matches) != 2 ||
		matches[0].AgentID != "agent-amy" ||
		matches[1].AgentID != "agent-devin" {
		t.Fatalf("相邻已知 mention 应按顺序解析: %#v", matches)
	}
}

func TestResolveMentionMatchesDoesNotTreatASCIIIdentifierContinuationAsHanBoundary(t *testing.T) {
	aliases := map[string]string{"Agent1": "agent-1"}
	for _, content := range []string{
		"@Agent10 不应命中",
		"@Agent1analysis 不应命中",
		"@Agent1_name 不应命中",
		"@Agent1-review 不应命中",
	} {
		if matches := ResolveMentionMatches(content, aliases); len(matches) != 0 {
			t.Fatalf("ASCII 标识符后缀不应截断成 mention: content=%q matches=%#v", content, matches)
		}
	}
}

func TestResolveMentionMatchesIgnoresEscapedMentionsBareURLsAndEmails(t *testing.T) {
	aliases := map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"}
	content := `\@Amy https://example.test/@Devin www.example.test/@Amy mailto:ops@Amy.test foo@Amy.test 正文 @Amy`
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 1 || matches[0].AgentID != "agent-amy" {
		t.Fatalf("转义 mention、URL 与 email 不应触发 handoff: %#v", matches)
	}
}

func TestResolveMentionMatchesKeepsOffsetsOutsideMaskedCode(t *testing.T) {
	aliases := map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"}
	content := "代码 `示例 @阿梅` 后请 @Devin 继续"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 1 {
		t.Fatalf("代码区外应只解析一处 mention: %#v", matches)
	}
	runes := []rune(content)
	if got := string(runes[matches[0].StartRune:matches[0].EndRune]); got != "@Devin" {
		t.Fatalf("代码区不应改变后续 span: got=%q span=%+v", got, matches[0])
	}
}

func TestResolveMentionMatchesIgnoresLinkDestinationsAndIdentifiers(t *testing.T) {
	aliases := map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"}
	content := "foo@Amy [链接](https://example.test/@Devin) 正文 @Amy"
	matches := ResolveMentionMatches(content, aliases)
	if len(matches) != 1 || matches[0].AgentID != "agent-amy" {
		t.Fatalf("链接 destination 和标识符不应触发 mention: %#v", matches)
	}
}

func TestResolveMentionMatchesDropsAmbiguousAlias(t *testing.T) {
	aliases := BuildMentionAliases(&protocol.ConversationContextAggregate{
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-a", Name: "同名"},
			{AgentID: "agent-b", Name: "同名"},
		},
	})
	if matches := ResolveMentionMatches("@同名", aliases); len(matches) != 0 {
		t.Fatalf("歧义 alias 不应触发 handoff: %#v", matches)
	}
}

func TestBuildMentionAliasesDropsCaseInsensitiveAmbiguity(t *testing.T) {
	aliases := BuildMentionAliases(&protocol.ConversationContextAggregate{
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-a", Name: "Amy"},
			{AgentID: "agent-b", Name: "amy"},
		},
	})
	if matches := ResolveMentionMatches("@Amy", aliases); len(matches) != 0 {
		t.Fatalf("大小写不同但同名的 alias 仍应视为歧义: %#v", matches)
	}
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]int, len(left))
	for _, item := range left {
		seen[item]++
	}
	for _, item := range right {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	return true
}

func sameStringSlice(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
