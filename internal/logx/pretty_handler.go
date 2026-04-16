// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：pretty_handler.go
// @Date   ：2026/04/16 20:06:00
// @Author ：leemysw
// 2026/04/16 20:06:00   Create
// =====================================================

package logx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// prettyHandler 提供适合本地调试的紧凑日志视图。
type prettyHandler struct {
	writer   io.Writer
	level    slog.Leveler
	attrs    []slog.Attr
	groups   []string
	mutex    *sync.Mutex
	colorize bool
}

type logField struct {
	Key   string
	Value string
}

func newPrettyHandler(writer io.Writer, options *slog.HandlerOptions, colorize bool) slog.Handler {
	var level slog.Leveler = slog.LevelInfo
	if options != nil && options.Level != nil {
		level = options.Level
	}
	return &prettyHandler{
		writer:   writer,
		level:    level,
		mutex:    &sync.Mutex{},
		colorize: colorize,
	}
}

// Enabled 判断当前级别是否应输出。
func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle 输出单条日志。
func (h *prettyHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]logField, 0, record.NumAttrs()+len(h.attrs))
	fields = appendResolvedAttrs(fields, h.attrs, h.groups)
	record.Attrs(func(attr slog.Attr) bool {
		fields = appendResolvedAttr(fields, attr, h.groups)
		return true
	})

	service, component, summary, sdkType, filtered := extractSpecialFields(fields)
	line := formatPrettyLine(record.Time, record.Level, service, component, record.Message, summary, sdkType, filtered, h.colorize)

	h.mutex.Lock()
	defer h.mutex.Unlock()
	_, err := io.WriteString(h.writer, line)
	return err
}

// WithAttrs 追加结构化字段。
func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := *h
	cloned.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cloned
}

// WithGroup 追加字段分组。
func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if strings.TrimSpace(name) == "" {
		return h
	}
	cloned := *h
	cloned.groups = append(append([]string{}, h.groups...), name)
	return &cloned
}

func appendResolvedAttrs(fields []logField, attrs []slog.Attr, groups []string) []logField {
	for _, attr := range attrs {
		fields = appendResolvedAttr(fields, attr, groups)
	}
	return fields
}

func appendResolvedAttr(fields []logField, attr slog.Attr, groups []string) []logField {
	if attr.Equal(slog.Attr{}) {
		return fields
	}
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if key := strings.TrimSpace(attr.Key); key != "" {
			nextGroups = append(append([]string{}, groups...), key)
		}
		for _, nested := range attr.Value.Group() {
			fields = appendResolvedAttr(fields, nested, nextGroups)
		}
		return fields
	}

	key := formatAttrKey(groups, attr.Key)
	value := stringifyAttrValue(attr.Value)
	if key == "" || value == "" {
		return fields
	}
	return append(fields, logField{Key: key, Value: value})
}

func extractSpecialFields(fields []logField) (string, string, string, string, []logField) {
	service := ""
	component := ""
	summary := ""
	sdkType := ""
	filtered := make([]logField, 0, len(fields))

	for _, field := range fields {
		switch field.Key {
		case "service":
			service = field.Value
		case "component":
			component = field.Value
		case "sdk_summary":
			summary = field.Value
		case "sdk_message_type":
			sdkType = field.Value
		case "sdk_message_subtype", "stream_preview", "assistant_preview", "result_preview",
			"stream_event_type", "stream_delta_type", "assistant_block_types", "result_subtype":
			continue
		default:
			filtered = append(filtered, field)
		}
	}
	return service, component, summary, sdkType, filtered
}

func formatPrettyLine(
	logTime time.Time,
	level slog.Level,
	service string,
	component string,
	message string,
	summary string,
	sdkType string,
	fields []logField,
	colorize bool,
) string {
	builder := &strings.Builder{}
	if !logTime.IsZero() {
		builder.WriteString(logTime.Format("2006-01-02 15:04:05.000"))
		builder.WriteByte(' ')
	}
	builder.WriteString(colorizeIfNeeded(formatLevel(level), colorForLevel(level), colorize))
	builder.WriteByte(' ')
	if scope := buildScope(service, component); scope != "" {
		builder.WriteByte('[')
		builder.WriteString(colorizeIfNeeded(scope, ansiBrightBlack, colorize))
		builder.WriteString("] ")
	}
	if strings.TrimSpace(sdkType) != "" {
		builder.WriteString(colorizeIfNeeded("[AGENT] ", colorForSDKType(sdkType), colorize))
	}

	builder.WriteString(strings.TrimSpace(message))
	if summary = strings.TrimSpace(summary); summary != "" {
		builder.WriteString(" · ")
		builder.WriteString(colorizeIfNeeded(summary, colorForSDKType(sdkType), colorize))
	}

	for _, field := range fields {
		builder.WriteByte(' ')
		builder.WriteString(field.Key)
		builder.WriteByte('=')
		builder.WriteString(quoteIfNeeded(field.Value))
	}
	builder.WriteByte('\n')
	return builder.String()
}

func buildScope(service string, component string) string {
	service = strings.TrimSpace(service)
	component = strings.TrimSpace(component)
	switch {
	case service != "" && component != "":
		return service + "/" + component
	case service != "":
		return service
	default:
		return component
	}
}

func formatLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DBG"
	case level < slog.LevelWarn:
		return "INF"
	case level < slog.LevelError:
		return "WRN"
	default:
		return "ERR"
	}
}

const (
	ansiReset       = "\033[0m"
	ansiBlue        = "\033[34m"
	ansiCyan        = "\033[36m"
	ansiGreen       = "\033[32m"
	ansiYellow      = "\033[33m"
	ansiRed         = "\033[31m"
	ansiMagenta     = "\033[35m"
	ansiBrightBlack = "\033[90m"
)

func colorForLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return ansiCyan
	case level < slog.LevelWarn:
		return ansiGreen
	case level < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed
	}
}

func colorForSDKType(sdkType string) string {
	switch strings.TrimSpace(sdkType) {
	case "stream_event":
		return ansiCyan
	case "assistant":
		return ansiGreen
	case "result":
		return ansiBlue
	case "system":
		return ansiMagenta
	case "tool_progress":
		return ansiYellow
	default:
		return ansiBrightBlack
	}
}

func colorizeIfNeeded(value string, color string, enabled bool) string {
	if !enabled || strings.TrimSpace(color) == "" || strings.TrimSpace(value) == "" {
		return value
	}
	return color + value + ansiReset
}

func formatAttrKey(groups []string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(groups) == 0 {
		return key
	}
	all := append(append([]string{}, groups...), key)
	return strings.Join(all, ".")
}

func stringifyAttrValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return strings.TrimSpace(value.String())
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'f', -1, 64)
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return strings.TrimSpace(fmt.Sprint(value.Any()))
	default:
		return strings.TrimSpace(value.String())
	}
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\r=\"") {
		return strconv.Quote(value)
	}
	return value
}
