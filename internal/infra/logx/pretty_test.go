package logx

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrettyHandlerRendersSDKSummaryCompactly(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	handler := newPrettyHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug}, false)
	record := slog.NewRecord(time.Date(2026, 4, 21, 14, 58, 0, 0, time.FixedZone("CST", 8*3600)), slog.LevelDebug, "Agent ", 0)
	record.AddAttrs(
		slog.String("service", "nexus"),
		slog.String("component", "chat"),
		slog.String("session_key", "agent:c5740009ac97:ws:dm:93c96efb202a"),
		slog.String("agent_id", "c5740009ac97"),
		slog.String("round_id", "a9928342-88bb-40a1-bd5b-d1d122b61b79"),
		slog.String("sdk_summary", "stream content_block_delta(text_delta)"),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("写日志失败: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, `Agent s=93c96efb202a a=c5740009ac97 r=a9928342-88b`) {
		t.Fatalf("未输出前置固定上下文: %s", output)
	}
	if !strings.Contains(output, "stream content_block_delta(text_delta)") {
		t.Fatalf("未输出紧凑摘要: %s", output)
	}
	if strings.Index(output, "stream content_block_delta(text_delta)") < strings.Index(output, "r=a9928342-88b") {
		t.Fatalf("摘要仍然出现在固定字段前面: %s", output)
	}
	if strings.Contains(output, "session_key=") ||
		strings.Contains(output, "sdk_message_type=") {
		t.Fatalf("仍输出了冗余字段: %s", output)
	}
}

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

func TestPickAccess(t *testing.T) {
	fields := []field{
		{key: "method", value: "GET"},
		{key: "status", value: "200"},
		{key: "request", value: "health"},
	}
	access, rest := pickAccess(fields)
	if access != nil {
		t.Fatalf("缺少 path 时不应识别为 access log: %+v", access)
	}
	if !reflect.DeepEqual(rest, fields) {
		t.Fatalf("普通日志字段应保持原顺序，实际 %+v", rest)
	}
	fields = []field{
		{key: "method", value: "POST"},
		{key: "status", value: "201"},
		{key: "path", value: "/v1/tasks"},
		{key: "remote_ip", value: "10.0.0.8"},
		{key: "request_id", value: "request-1"},
	}
	access, rest = pickAccess(fields)
	if access == nil || access.method != "POST" || access.status != 201 || access.path != "/v1/tasks" {
		t.Fatalf("access log 提取错误: %+v", access)
	}
	expected := []field{{key: "request_id", value: "request-1"}, {key: "ip", value: "10.0.0.8"}}
	if !reflect.DeepEqual(rest, expected) {
		t.Fatalf("剩余字段错误: %+v", rest)
	}
}
