package logx

import "testing"

func TestPreviewText(t *testing.T) {
	t.Parallel()

	if got := PreviewText("  第一行\n第二行\t第三行  ", 20); got != "第一行 第二行 第三行" {
		t.Fatalf("预览文本未压平空白: %q", got)
	}
	if got := PreviewText("一二三四五", 3); got != "一二三..." {
		t.Fatalf("预览文本未按 rune 截断: %q", got)
	}
	if got := PreviewText("  ", 10); got != "" {
		t.Fatalf("空文本应返回空字符串: %q", got)
	}
}
