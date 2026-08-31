package shared

import (
	"net/http"
	"testing"
)

func TestIsClientMessageTextAllowsDefaultModelHint(t *testing.T) {
	if !IsClientMessageText("默认模型仍使用 Provider kimi-code；请先在设置中切换默认模型") {
		t.Fatal("包含具体操作指引的默认模型保护提示应被判定为客户端可展示文案")
	}
}

func TestIsClientMessageTextRejectsPartialDefaultModelHint(t *testing.T) {
	for _, message := range []string{
		"内部连接仍使用旧地址",
		"请先在服务端查看日志",
	} {
		if IsClientMessageText(message) {
			t.Fatalf("不完整的默认模型提示不应被判定为客户端可展示文案: %q", message)
		}
	}
}

func TestGatewayClientErrorDetailPreservesDefaultModelHint(t *testing.T) {
	detail := "默认模型仍使用 Provider kimi-code；请先在设置中切换默认模型"
	got := GatewayClientErrorDetail(http.StatusBadRequest, detail)
	if got != detail {
		t.Fatalf("BadRequest 下的默认模型保护提示应直接返回给客户端，got=%q want=%q", got, detail)
	}
}
