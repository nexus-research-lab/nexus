// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：access_handler.go
// @Date   ：2026/04/20 18:30:00
// @Author ：leemysw
// 2026/04/20 18:30:00   Create
// =====================================================

package logx

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// accessCollapseHandler 包裹 tint，专门压缩 HTTP access log：把 method/status/duration/bytes/path 折叠进 message，
// 同时缩短 request_id、隐藏 localhost 的 remote_ip，避免终端一行折成两行。
type accessCollapseHandler struct {
	inner slog.Handler
}

// Enabled 透传。
func (h *accessCollapseHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle 检查 record 是否具备 HTTP 访问日志特征；匹配则重写 message + 丢掉冗余字段。
func (h *accessCollapseHandler) Handle(ctx context.Context, record slog.Record) error {
	method, status, path, durationMs, bytesWritten, remoteIP, requestID, matched := extractHTTPFields(record)
	if !matched {
		// 即使不是 HTTP 行，也顺手缩短 request_id。
		return h.inner.Handle(ctx, rewriteRequestID(record))
	}

	bytesLabel := formatBytes(bytesWritten)
	durationLabel := fmt.Sprintf("%dms", durationMs)
	newMessage := fmt.Sprintf("%-6s %d  %5s  %6s  %s",
		strings.ToUpper(method), status, durationLabel, bytesLabel, path)

	next := slog.NewRecord(record.Time, record.Level, newMessage, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "method", "status", "duration_ms", "bytes", "path":
			return true
		case "remote_ip":
			if strings.TrimSpace(remoteIP) == "" || remoteIP == "127.0.0.1" || remoteIP == "::1" {
				return true
			}
		case "request_id":
			if short := shortenRequestID(requestID); short != "" {
				next.AddAttrs(slog.String("rid", short))
			}
			return true
		}
		next.AddAttrs(attr)
		return true
	})
	return h.inner.Handle(ctx, next)
}

// WithAttrs 在绑定阶段就缩短 request_id，避免 tint 渲染出全长 UUID。
func (h *accessCollapseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	rewritten := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Key == "request_id" {
			if short := shortenRequestID(attr.Value.String()); short != "" {
				rewritten = append(rewritten, slog.String("rid", short))
			}
			continue
		}
		rewritten = append(rewritten, attr)
	}
	return &accessCollapseHandler{inner: h.inner.WithAttrs(rewritten)}
}

func (h *accessCollapseHandler) WithGroup(name string) slog.Handler {
	return &accessCollapseHandler{inner: h.inner.WithGroup(name)}
}

// extractHTTPFields 扫一遍 attrs，判断是否构成访问日志并抽出关键字段。
func extractHTTPFields(record slog.Record) (method string, status int, path string, durationMs int64, bytesWritten int64, remoteIP string, requestID string, matched bool) {
	hasMethod, hasStatus, hasPath := false, false, false
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "method":
			method = attr.Value.String()
			hasMethod = true
		case "status":
			status = int(asInt64(attr.Value))
			hasStatus = true
		case "path":
			path = attr.Value.String()
			hasPath = true
		case "duration_ms":
			durationMs = asInt64(attr.Value)
		case "bytes":
			bytesWritten = asInt64(attr.Value)
		case "remote_ip":
			remoteIP = attr.Value.String()
		case "request_id":
			requestID = attr.Value.String()
		}
		return true
	})
	matched = hasMethod && hasStatus && hasPath
	return
}

func asInt64(value slog.Value) int64 {
	switch value.Kind() {
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return int64(value.Uint64())
	case slog.KindFloat64:
		return int64(value.Float64())
	case slog.KindDuration:
		return value.Duration().Milliseconds()
	default:
		return 0
	}
}

// rewriteRequestID 对非 HTTP 行也做 request_id 缩短。
func rewriteRequestID(record slog.Record) slog.Record {
	needs := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "request_id" && len(attr.Value.String()) > 8 {
			needs = true
			return false
		}
		return true
	})
	if !needs {
		return record
	}
	next := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "request_id" {
			if short := shortenRequestID(attr.Value.String()); short != "" {
				next.AddAttrs(slog.String("rid", short))
			}
			return true
		}
		next.AddAttrs(attr)
		return true
	})
	return next
}

func shortenRequestID(requestID string) string {
	trimmed := strings.TrimSpace(requestID)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}

func formatBytes(total int64) string {
	switch {
	case total >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(total)/float64(1<<20))
	case total >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(total)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", total)
	}
}
