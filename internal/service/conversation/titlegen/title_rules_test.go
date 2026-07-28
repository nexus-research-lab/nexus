package titlegen

import "testing"

func TestDefaultConversationTitlesIncludeNewSessionPlaceholders(t *testing.T) {
	for _, title := range []string{
		"",
		"New session",
		"Untitled conversation",
		"新会话",
		"未命名对话",
		"Smoke · 对话 1",
	} {
		if !isDefaultConversationTitle(title, "Smoke") {
			t.Fatalf("默认会话标题未被识别: %q", title)
		}
	}
	if isDefaultConversationTitle("登录超时排查", "Smoke") {
		t.Fatal("语义标题不应被识别为默认标题")
	}
}
