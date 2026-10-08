package room

import (
	"testing"
)

func TestResolveMentionMatches(t *testing.T) {
	tests := []struct {
		name      string
		aliases   map[string]string
		content   string
		wantIDs   []string
		wantSpans []string
	}{
		{
			name:      "Unicode 偏移与括号 ID",
			aliases:   map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"},
			content:   "请 @阿梅（c742e12ab802）看一下，再请 @Devin 总结。",
			wantIDs:   []string{"agent-amy", "agent-devin"},
			wantSpans: []string{"@阿梅", "@Devin"},
		},
		{
			name:      "英文最长别名紧跟中文",
			aliases:   map[string]string{"Agent1": "agent-1", "Agent10": "agent-10"},
			content:   "@Agent10以上为前期资料；@Agent1以上为最终结果。",
			wantIDs:   []string{"agent-10", "agent-1"},
			wantSpans: []string{"@Agent10", "@Agent1"},
		},
		{
			name:      "中文最长别名紧跟正文",
			aliases:   map[string]string{"研究": "agent-research", "研究员": "agent-researcher", "分析师": "agent-analyst"},
			content:   "@研究员请先收集资料，@分析师随后复核。",
			wantIDs:   []string{"agent-researcher", "agent-analyst"},
			wantSpans: []string{"@研究员", "@分析师"},
		},
		{
			name:      "相邻 mention",
			aliases:   map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"},
			content:   "@Amy@Devin请分别检查。",
			wantIDs:   []string{"agent-amy", "agent-devin"},
			wantSpans: []string{"@Amy", "@Devin"},
		},
		{
			name:      "忽略转义 URL 邮箱与链接",
			aliases:   map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"},
			content:   `\@Amy https://example.test/@Devin www.example.test/@Amy mailto:ops@Amy.test foo@Amy.test [链接](https://example.test/@Devin) 正文 @Amy`,
			wantIDs:   []string{"agent-amy"},
			wantSpans: []string{"@Amy"},
		},
		{
			name:      "代码区不改变后续偏移",
			aliases:   map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"},
			content:   "代码 `示例 @阿梅` 后请 @Devin 继续",
			wantIDs:   []string{"agent-devin"},
			wantSpans: []string{"@Devin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := ResolveMentionMatches(test.content, test.aliases)
			if len(matches) != len(test.wantIDs) {
				t.Fatalf("ResolveMentionMatches() = %#v, want %v", matches, test.wantIDs)
			}
			runes := []rune(test.content)
			for i, match := range matches {
				if match.AgentID != test.wantIDs[i] {
					t.Fatalf("match[%d].AgentID = %q, want %q", i, match.AgentID, test.wantIDs[i])
				}
				if span := string(runes[match.StartRune:match.EndRune]); span != test.wantSpans[i] {
					t.Fatalf("match[%d] span = %q, want %q", i, span, test.wantSpans[i])
				}
			}
		})
	}

	for _, content := range []string{
		"@Agent10 不应命中",
		"@Agent1analysis 不应命中",
		"@Agent1_name 不应命中",
		"@Agent1-review 不应命中",
	} {
		t.Run("拒绝标识符后缀/"+content, func(t *testing.T) {
			if matches := ResolveMentionMatches(content, map[string]string{"Agent1": "agent-1"}); len(matches) != 0 {
				t.Fatalf("标识符后缀不应截断成 mention: %#v", matches)
			}
		})
	}
}
