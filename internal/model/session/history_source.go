// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：history_source.go
// @Date   ：2026/04/19 16:24:00
// @Author ：leemysw
// 2026/04/19 16:24:00   Create
// =====================================================

package session

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// OptionHistorySource 表示会话历史真相源配置项。
	OptionHistorySource = "history_source"
	// HistorySourceTranscript 表示历史来自 cc transcript，Nexus 仅保留 overlay。
	HistorySourceTranscript = "transcript"
)

var (
	// ErrLegacyHistoryUnsupported 表示运行时已经不再支持旧版 messages.jsonl 历史链路。
	ErrLegacyHistoryUnsupported = errors.New("legacy session history is no longer supported")
)

// ResolveHistorySource 返回会话历史真相源。
func ResolveHistorySource(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	value, ok := options[OptionHistorySource].(string)
	if !ok {
		return ""
	}
	if strings.TrimSpace(value) == HistorySourceTranscript {
		return HistorySourceTranscript
	}
	return ""
}

// IsTranscriptHistory 表示会话是否使用 transcript 作为真相源。
func IsTranscriptHistory(options map[string]any) bool {
	return ResolveHistorySource(options) == HistorySourceTranscript
}

// EnsureTranscriptHistory 校验会话是否符合 transcript + overlay 机制。
func EnsureTranscriptHistory(options map[string]any, sessionKey string) error {
	if IsTranscriptHistory(options) {
		return nil
	}
	if sessionKey == "" {
		return ErrLegacyHistoryUnsupported
	}
	return fmt.Errorf("%w: session=%s", ErrLegacyHistoryUnsupported, sessionKey)
}
