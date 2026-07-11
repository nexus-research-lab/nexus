package loops

import (
	"context"
	"strings"
	"testing"
)

func TestServiceLocalizesDisplayFieldsAndKeepsKickoffPrompt(t *testing.T) {
	svc := NewService()
	rawItems, err := loadCatalog()
	if err != nil {
		t.Fatalf("加载原始 catalog 失败: %v", err)
	}
	if got := len(rawItems); got != 40 {
		t.Fatalf("原始 loop catalog 数量不正确: got=%d", got)
	}
	if got := len(svc.items); got != 27 {
		t.Fatalf("可见 loop catalog 数量不正确: got=%d", got)
	}
	item, err := svc.GetLoop(context.Background(), "test-until-green", "zh-CN")
	if err != nil {
		t.Fatalf("读取 loop 失败: %v", err)
	}
	if item.Title != "测试直到全绿" {
		t.Fatalf("中文标题未生效: %q", item.Title)
	}
	if !strings.Contains(item.Description, "运行测试套件") {
		t.Fatalf("中文描述未生效: %q", item.Description)
	}
	if !strings.Contains(item.KickoffPrompt, "Start the") {
		t.Fatalf("kickoff prompt 不应本地化: %q", item.KickoffPrompt)
	}
}
