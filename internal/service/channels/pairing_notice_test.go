package channels

import (
	"strings"
	"testing"
)

func TestPairingApprovalNoticeTextIncludesPairingID(t *testing.T) {
	text := pairingApprovalNoticeText(&pairingApprovalError{
		PairingID: "pair_123",
		Message:   "IM 对象尚未配对授权，请先在配对控制台批准",
	})
	if !strings.Contains(text, "配对控制台") || !strings.Contains(text, "pair_123") {
		t.Fatalf("配对提醒文案不完整: %q", text)
	}
	if strings.Contains(text, "处理失败") {
		t.Fatalf("配对提醒不应包含处理失败: %q", text)
	}
}

func TestPairingApprovalNoticeTextIgnoresOtherErrors(t *testing.T) {
	if got := pairingApprovalNoticeText(nil); got != "" {
		t.Fatalf("nil error should not produce notice: %q", got)
	}
}
