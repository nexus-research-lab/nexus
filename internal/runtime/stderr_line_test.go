package runtime

import "testing"

func TestNormalizeRuntimeStderrLineDecodesWindowsGBK(t *testing.T) {
	got := NormalizeRuntimeStderrLine(string([]byte{0xc3, 0xfc, 0xc1, 0xee, 0xd0, 0xd0, 0xcc, 0xab, 0xb3, 0xa4, 0xa1, 0xa3}))
	if got != "命令行太长。" {
		t.Fatalf("GBK stderr 解码不正确: got=%q", got)
	}
}

func TestNormalizeRuntimeStderrLineKeepsUTF8(t *testing.T) {
	got := NormalizeRuntimeStderrLine("  process failed  ")
	if got != "process failed" {
		t.Fatalf("UTF-8 stderr 不应被改写: got=%q", got)
	}
}
