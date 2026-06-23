package message

import (
	"testing"
	"time"
)

func TestNormalizeInboundMergesEnvelopeWithFallback(t *testing.T) {
	t.Parallel()

	receivedAt := time.UnixMilli(1000).UTC()
	message := NormalizeInbound(&Inbound{
		Channel:           " telegram ",
		PlatformMessageID: "msg-1",
		ThreadID:          "topic-1",
		SenderID:          "user-1",
		Text:              " envelope text ",
		Metadata: map[string]string{
			"source": "callback",
		},
	}, InboundParams{
		Channel:    "fallback-channel",
		Target:     "-1001",
		ChatType:   "group",
		Text:       "fallback text",
		ReceivedAt: receivedAt,
		Metadata: map[string]string{
			"fallback": "true",
			"source":   "fallback",
		},
	})

	if message == nil {
		t.Fatal("expected migrated inbound message")
	}
	if message.Direction != DirectionInbound ||
		message.Channel != "telegram" ||
		message.Target != "-1001" ||
		message.PlatformMessageID != "msg-1" ||
		message.ThreadID != "topic-1" ||
		message.SenderID != "user-1" ||
		message.ChatType != "group" ||
		message.Text != "envelope text" {
		t.Fatalf("migrated inbound mismatch: %+v", message)
	}
	if !message.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("fallback received_at not preserved: %s", message.ReceivedAt)
	}
	if message.Metadata["fallback"] != "true" || message.Metadata["source"] != "callback" {
		t.Fatalf("metadata merge mismatch: %+v", message.Metadata)
	}
}

func TestRuntimeMetadataProjectsInboundEnvelope(t *testing.T) {
	t.Parallel()

	metadata := RuntimeMetadata(&Inbound{
		Direction:         DirectionInbound,
		Channel:           "feishu",
		Target:            "oc_group",
		PlatformMessageID: "om_1",
		ThreadID:          "thread_1",
		ReplyToID:         "reply_1",
		SenderID:          "ou_1",
		SenderName:        "Alice",
		ChatType:          "group",
		Edited:            true,
		ReceivedAt:        time.UnixMilli(2000).UTC(),
		Metadata: map[string]string{
			"event_id": "evt_1",
		},
	})

	expected := map[string]string{
		"im.direction":           "inbound",
		"im.channel":             "feishu",
		"im.target":              "oc_group",
		"im.platform_message_id": "om_1",
		"im.thread_id":           "thread_1",
		"im.reply_to_id":         "reply_1",
		"im.sender_id":           "ou_1",
		"im.sender_name":         "Alice",
		"im.chat_type":           "group",
		"im.edited":              "true",
		"im.received_at_unix_ms": "2000",
		"im.meta.event_id":       "evt_1",
	}
	for key, want := range expected {
		if got := metadata[key]; got != want {
			t.Fatalf("metadata[%s] = %q, want %q; metadata=%+v", key, got, want, metadata)
		}
	}
}
