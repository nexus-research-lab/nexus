// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：pretty_handler_test.go
// @Date   ：2026/04/16 20:10:00
// @Author ：leemysw
// 2026/04/16 20:10:00   Create
// =====================================================

package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestPrettyHandlerFormatsSDKSummary(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := New(Options{
		Service: "nexus",
		Level:   "debug",
		Format:  "text",
		Output:  buffer,
	}).With("component", "chat")

	logger.Debug("Agent ",
		"sdk_summary", "stream content_block_delta(thinking_delta)",
		"sdk_message_type", "stream_event",
		"session_key", "agent:nexus:ws:dm:test",
		"stream_preview", "正在判断天气查询",
	)

	output := buffer.String()
	if !strings.Contains(output, "DBG [nexus/chat] [AGENT] Agent · stream content_block_delta(thinking_delta)") {
		t.Fatalf("pretty 日志未包含摘要: %s", output)
	}
	if strings.Contains(output, "sdk_summary=") {
		t.Fatalf("sdk_summary 不应重复出现在字段区: %s", output)
	}
	if !strings.Contains(output, `session_key=agent:nexus:ws:dm:test`) {
		t.Fatalf("pretty 日志未包含 session_key: %s", output)
	}
	if strings.Contains(output, `stream_preview=`) {
		t.Fatalf("stream_preview 应由 agent 摘要吸收，不应重复输出: %s", output)
	}
}

func TestPrettyHandlerSupportsNestedGroups(t *testing.T) {
	buffer := &bytes.Buffer{}
	handler := newPrettyHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug}, false)
	logger := slog.New(handler)

	logger.Debug("测试分组字段", slog.Group("sdk", slog.String("event", "message_start")))

	output := buffer.String()
	if !strings.Contains(output, "sdk.event=message_start") {
		t.Fatalf("pretty 日志未展开分组字段: %s", output)
	}
}

func TestPrettyHandlerColorizesStdoutStyleOutput(t *testing.T) {
	buffer := &bytes.Buffer{}
	handler := newPrettyHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug}, true)
	logger := slog.New(handler)

	logger.Debug("Agent ",
		"sdk_summary", "assistant snapshot(thinking,text)",
		"sdk_message_type", "assistant",
	)

	output := buffer.String()
	if !strings.Contains(output, "\033[") {
		t.Fatalf("彩色 pretty 日志应包含 ANSI 颜色码: %q", output)
	}
	if !strings.Contains(output, "[AGENT]") {
		t.Fatalf("彩色 pretty 日志应包含 AGENT 标记: %s", output)
	}
}
