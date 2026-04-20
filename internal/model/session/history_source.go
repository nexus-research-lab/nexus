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
	// HistorySourceLegacy 表示旧版 messages.jsonl 历史，仅用于识别未迁移会话。
	HistorySourceLegacy = "legacy"
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
		return HistorySourceLegacy
	}
	value, ok := options[OptionHistorySource].(string)
	if !ok {
		return HistorySourceLegacy
	}
	switch strings.TrimSpace(value) {
	case HistorySourceTranscript:
		return HistorySourceTranscript
	default:
		return HistorySourceLegacy
	}
}

// IsTranscriptHistory 表示会话是否使用 transcript 作为真相源。
func IsTranscriptHistory(options map[string]any) bool {
	return ResolveHistorySource(options) == HistorySourceTranscript
}

// EnsureTranscriptHistory 校验会话是否已经迁移到 transcript + overlay 机制。
func EnsureTranscriptHistory(options map[string]any, sessionKey string) error {
	if IsTranscriptHistory(options) {
		return nil
	}
	if strings.TrimSpace(sessionKey) == "" {
		return fmt.Errorf("%w: 请先执行 `nexusctl session migrate-history`", ErrLegacyHistoryUnsupported)
	}
	return fmt.Errorf("%w: session=%s，请先执行 `nexusctl session migrate-history`", ErrLegacyHistoryUnsupported, strings.TrimSpace(sessionKey))
}
