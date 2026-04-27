package logx

import (
	"bytes"
	"context"
	"log/slog"
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
		slog.String("sdk_summary", `stream content_block_delta(text_delta) "片"`),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("写日志失败: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, `Agent s=93c96efb202a a=c5740009ac97 r=a9928342-88b`) {
		t.Fatalf("未输出前置固定上下文: %s", output)
	}
	if !strings.Contains(output, `stream content_block_delta(text_delta) "片"`) {
		t.Fatalf("未输出紧凑摘要: %s", output)
	}
	if strings.Index(output, `stream content_block_delta(text_delta) "片"`) < strings.Index(output, "r=a9928342-88b") {
		t.Fatalf("摘要仍然出现在固定字段前面: %s", output)
	}
	if strings.Contains(output, "session_key=") ||
		strings.Contains(output, "sdk_message_type=") {
		t.Fatalf("仍输出了冗余字段: %s", output)
	}
}
