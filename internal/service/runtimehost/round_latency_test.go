package runtimehost

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func streamMessage(event map[string]any) sdkprotocol.ReceivedMessage {
	return sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: event},
	}
}

func blockStart(blockType string) sdkprotocol.ReceivedMessage {
	return streamMessage(map[string]any{
		"type":          "content_block_start",
		"content_block": map[string]any{"type": blockType},
	})
}

func decodeLatencyLogs(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(buffer)
	for decoder.More() {
		record := map[string]any{}
		if err := decoder.Decode(&record); err != nil {
			t.Fatalf("decode log: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func TestRoundLatencyReportsOnceAtFirstAnswer(t *testing.T) {
	var buffer bytes.Buffer
	latency := newRoundLatency(slog.New(slog.NewJSONHandler(&buffer, nil)))
	subagent := "toolu_parent"

	latency.Observe(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "init"})
	latency.Observe(streamMessage(map[string]any{"type": "message_start"}))
	latency.Observe(blockStart("thinking"))
	subagentText := blockStart("text")
	subagentText.ParentToolUseID = &subagent
	latency.Observe(subagentText)
	if buffer.Len() != 0 {
		t.Fatalf("子智能体输出不应结束主线程计时: %s", buffer.String())
	}
	latency.Observe(blockStart("tool_use"))
	latency.Observe(blockStart("text"))
	latency.Report()

	records := decodeLatencyLogs(t, &buffer)
	if len(records) != 1 {
		t.Fatalf("expected one latency log, got %d: %v", len(records), records)
	}
	for _, key := range []string{"system_init_ms", "message_start_ms", "first_thinking_ms", "first_answer_ms", "thinking_ms"} {
		if _, ok := records[0][key]; !ok {
			t.Fatalf("missing %s in %v", key, records[0])
		}
	}
}

func TestRoundLatencyReportsPartialMarksWithoutAnswer(t *testing.T) {
	var buffer bytes.Buffer
	latency := newRoundLatency(slog.New(slog.NewJSONHandler(&buffer, nil)))

	latency.Observe(streamMessage(map[string]any{"type": "message_start"}))
	latency.Report()

	records := decodeLatencyLogs(t, &buffer)
	if len(records) != 1 {
		t.Fatalf("expected one latency log, got %d", len(records))
	}
	if _, ok := records[0]["message_start_ms"]; !ok {
		t.Fatalf("missing message_start_ms in %v", records[0])
	}
	if _, ok := records[0]["first_answer_ms"]; ok {
		t.Fatalf("unexpected first_answer_ms in %v", records[0])
	}
}
