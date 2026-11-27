package launcher

import (
	"testing"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLatestReplyPreviewUsesFinalBody(t *testing.T) {
	text := func(value string) map[string]any {
		return map[string]any{"type": "text", "text": value}
	}
	thinking := map[string]any{"type": "thinking", "thinking": "重要邮件查询超时了，需要检查结果"}
	tool := map[string]any{"type": "tool_use", "name": "listImportantMails"}
	artifact := map[string]any{"type": "workspace_file_artifact"}
	for _, test := range []struct {
		name    string
		content any
		result  map[string]any
		want    string
	}{
		{
			name: "process text before tools does not prefix final reply",
			content: []any{thinking, text("重要邮件查询超时了"), tool,
				map[string]any{"type": "tool_result", "content": "internal result"},
				thinking, text("近一周没有需要紧急处理的邮件："), text("待办邮件：0 封")},
			want: "近一周没有需要紧急处理的邮件： 待办邮件：0 封",
		},
		{
			name:    "typed blocks and trailing files preserve final body",
			content: []map[string]any{text("正在整理"), tool, text("报告已完成"), artifact},
			want:    "报告已完成",
		},
		{name: "thinking only", content: []any{thinking}},
		{name: "tools after commentary are not a final body", content: []any{text("正在查询"), tool}},
		{name: "legacy string", content: "  正文\n内容  ", want: "正文 内容"},
		{name: "result-only fallback", content: []any{thinking, tool}, result: map[string]any{"result": "最终结果"}, want: "最终结果"},
		{name: "body takes precedence over result", content: []any{text("正文")}, result: map[string]any{"result": "其他结果"}, want: "正文"},
		{name: "interrupted reply", content: []any{text("未完成")}, result: map[string]any{"subtype": "interrupted"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := []protocol.Message{{"role": "assistant", "content": test.content, "result_summary": test.result}}
			if got := messageutil.LatestReplyPreview(messages); got != test.want {
				t.Fatalf("preview = %q, want %q", got, test.want)
			}
		})
	}

	t.Run("unfinished process keeps previous body", func(t *testing.T) {
		messages := []protocol.Message{
			{"role": "assistant", "content": []any{text("上一条正文")}},
			{"role": "user", "content": "继续查询"},
			{"role": "assistant", "content": []any{text("正在查询"), tool, thinking}},
		}
		if got := messageutil.LatestReplyPreview(messages); got != "上一条正文" {
			t.Fatalf("preview = %q, want previous body", got)
		}
	})
}
