package shared

import (
	"net/http"
	"testing"
)

func TestGatewayClientErrorDetailPreservesDefaultModelHint(t *testing.T) {
	detail := "默认模型仍使用 Provider kimi-code；请先在设置中切换默认模型"
	got := GatewayClientErrorDetail(http.StatusBadRequest, detail)
	if got != detail {
		t.Fatalf("BadRequest 下的默认模型保护提示应直接返回给客户端，got=%q want=%q", got, detail)
	}
}
