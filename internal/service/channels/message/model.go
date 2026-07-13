package message

import (
	"strings"
	"time"
)

type Kind string

const (
	KindText    Kind = "text"
	KindMedia   Kind = "media"
	KindTyping  Kind = "typing"
	KindUnknown Kind = "unknown"
)

type Capability string

const (
	CapabilityText         Capability = "text"
	CapabilityMedia        Capability = "media"
	CapabilityTyping       Capability = "typing"
	CapabilityThread       Capability = "thread"
	CapabilityReply        Capability = "reply"
	CapabilityReceipt      Capability = "receipt"
	CapabilityDurableFinal Capability = "durable_final"
)

type CapabilitySet map[Capability]bool

func NewCapabilitySet(values ...Capability) CapabilitySet {
	result := make(CapabilitySet, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func (s CapabilitySet) Has(value Capability) bool {
	if s == nil {
		return false
	}
	return s[value]
}

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Inbound struct {
	Direction         Direction         `json:"direction"`
	Channel           string            `json:"channel"`
	Target            string            `json:"target,omitempty"`
	PlatformMessageID string            `json:"platform_message_id,omitempty"`
	ThreadID          string            `json:"thread_id,omitempty"`
	ReplyToID         string            `json:"reply_to_id,omitempty"`
	SenderID          string            `json:"sender_id,omitempty"`
	SenderName        string            `json:"sender_name,omitempty"`
	ChatType          string            `json:"chat_type,omitempty"`
	Text              string            `json:"text"`
	Edited            bool              `json:"edited,omitempty"`
	ReceivedAt        time.Time         `json:"received_at"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type InboundParams struct {
	Channel           string
	Target            string
	PlatformMessageID string
	ThreadID          string
	ReplyToID         string
	SenderID          string
	SenderName        string
	ChatType          string
	Text              string
	Edited            bool
	ReceivedAt        time.Time
	Metadata          map[string]string
}

func NewInbound(params InboundParams) *Inbound {
	text := strings.TrimSpace(params.Text)
	if text == "" {
		return nil
	}
	receivedAt := params.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	return &Inbound{
		Direction:         DirectionInbound,
		Channel:           strings.TrimSpace(params.Channel),
		Target:            strings.TrimSpace(params.Target),
		PlatformMessageID: strings.TrimSpace(params.PlatformMessageID),
		ThreadID:          strings.TrimSpace(params.ThreadID),
		ReplyToID:         strings.TrimSpace(params.ReplyToID),
		SenderID:          strings.TrimSpace(params.SenderID),
		SenderName:        strings.TrimSpace(params.SenderName),
		ChatType:          strings.TrimSpace(params.ChatType),
		Text:              text,
		Edited:            params.Edited,
		ReceivedAt:        receivedAt.UTC(),
		Metadata:          copyMetadata(params.Metadata),
	}
}

func NormalizeInbound(message *Inbound, fallback InboundParams) *Inbound {
	if message == nil {
		return NewInbound(fallback)
	}
	params := InboundParams{
		Channel:           firstNonEmpty(message.Channel, fallback.Channel),
		Target:            firstNonEmpty(message.Target, fallback.Target),
		PlatformMessageID: firstNonEmpty(message.PlatformMessageID, fallback.PlatformMessageID),
		ThreadID:          firstNonEmpty(message.ThreadID, fallback.ThreadID),
		ReplyToID:         firstNonEmpty(message.ReplyToID, fallback.ReplyToID),
		SenderID:          firstNonEmpty(message.SenderID, fallback.SenderID),
		SenderName:        firstNonEmpty(message.SenderName, fallback.SenderName),
		ChatType:          firstNonEmpty(message.ChatType, fallback.ChatType),
		Text:              firstNonEmpty(message.Text, fallback.Text),
		Edited:            message.Edited || fallback.Edited,
		ReceivedAt:        message.ReceivedAt,
		Metadata:          mergeMetadata(fallback.Metadata, message.Metadata),
	}
	if params.ReceivedAt.IsZero() {
		params.ReceivedAt = fallback.ReceivedAt
	}
	return NewInbound(params)
}

type ReceiptPart struct {
	PlatformMessageID string `json:"platform_message_id"`
	Kind              Kind   `json:"kind"`
	Index             int    `json:"index"`
	ThreadID          string `json:"thread_id,omitempty"`
	ReplyToID         string `json:"reply_to_id,omitempty"`
}

type Receipt struct {
	Channel                  string        `json:"channel"`
	Target                   string        `json:"target"`
	PrimaryPlatformMessageID string        `json:"primary_platform_message_id,omitempty"`
	PlatformMessageIDs       []string      `json:"platform_message_ids,omitempty"`
	Parts                    []ReceiptPart `json:"parts,omitempty"`
	ThreadID                 string        `json:"thread_id,omitempty"`
	ReplyToID                string        `json:"reply_to_id,omitempty"`
	SentAt                   time.Time     `json:"sent_at"`
}

type ReceiptParams struct {
	Channel   string
	Target    string
	ThreadID  string
	ReplyToID string
	SentAt    time.Time
	Parts     []ReceiptPart
}

func NewReceipt(params ReceiptParams) *Receipt {
	sentAt := params.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now().UTC()
	}

	ids := make([]string, 0, len(params.Parts))
	seen := make(map[string]struct{}, len(params.Parts))
	parts := make([]ReceiptPart, 0, len(params.Parts))
	for _, part := range params.Parts {
		messageID := strings.TrimSpace(part.PlatformMessageID)
		if messageID == "" {
			continue
		}
		if part.Kind == "" {
			part.Kind = KindUnknown
		}
		part.PlatformMessageID = messageID
		if strings.TrimSpace(part.ThreadID) == "" {
			part.ThreadID = strings.TrimSpace(params.ThreadID)
		}
		if strings.TrimSpace(part.ReplyToID) == "" {
			part.ReplyToID = strings.TrimSpace(params.ReplyToID)
		}
		part.Index = len(parts)
		parts = append(parts, part)
		if _, ok := seen[messageID]; !ok {
			seen[messageID] = struct{}{}
			ids = append(ids, messageID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return &Receipt{
		Channel:                  strings.TrimSpace(params.Channel),
		Target:                   strings.TrimSpace(params.Target),
		PrimaryPlatformMessageID: ids[0],
		PlatformMessageIDs:       ids,
		Parts:                    parts,
		ThreadID:                 strings.TrimSpace(params.ThreadID),
		ReplyToID:                strings.TrimSpace(params.ReplyToID),
		SentAt:                   sentAt,
	}
}

func TextPart(messageID string) ReceiptPart {
	return ReceiptPart{
		PlatformMessageID: strings.TrimSpace(messageID),
		Kind:              KindText,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func copyMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mergeMetadata(inputs ...map[string]string) map[string]string {
	var result map[string]string
	for _, input := range inputs {
		for key, value := range copyMetadata(input) {
			if result == nil {
				result = make(map[string]string)
			}
			result[key] = value
		}
	}
	return result
}
