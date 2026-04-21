// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：pretty_handler.go
// @Date   ：2026/04/20 19:00:00
// @Author ：leemysw
// 2026/04/20 19:00:00   Create
//
// 终端日志格式参考 python-main 分支配色：
//   [ 18:16:47 ] INFO    | gateway - GET 200 0ms 119B /agent/v1/runtime/options  rid=34197658
//      灰        紫       青        method/status 各自染色          暗淡 k=v
// =====================================================

package logx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ANSI 配色：与 python-main 一致，并补 uvicorn 风格的 method/status 配色。
const (
	ansiReset       = "\033[0m"
	ansiBold        = "\033[1m"
	ansiDim         = "\033[2m"
	ansiRed         = "\033[31m"
	ansiGreen       = "\033[32m"
	ansiYellow      = "\033[33m"
	ansiBlue        = "\033[34m"
	ansiMagenta     = "\033[35m"
	ansiCyan        = "\033[36m"
	ansiWhite       = "\033[97m"
	ansiBrightBlack = "\033[90m"
)

type prettyHandler struct {
	writer   io.Writer
	level    slog.Leveler
	attrs    []slog.Attr
	groups   []string
	mutex    *sync.Mutex
	colorize bool
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

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := *h
	cloned.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cloned
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if strings.TrimSpace(name) == "" {
		return h
	}
	cloned := *h
	cloned.groups = append(append([]string{}, h.groups...), name)
	return &cloned
}

func (h *prettyHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]field, 0, record.NumAttrs()+len(h.attrs))
	fields = appendAttrs(fields, h.attrs, h.groups)
	record.Attrs(func(attr slog.Attr) bool {
		fields = appendAttr(fields, attr, h.groups)
		return true
	})

	scope, fields := pickScope(fields)
	access, fields := pickAccess(fields)
	requestID, fields := pickRequestID(fields)

	line := h.format(record.Time, record.Level, scope, record.Message, access, requestID, fields)

	h.mutex.Lock()
	defer h.mutex.Unlock()
	_, err := io.WriteString(h.writer, line)
	return err
}

func (h *prettyHandler) format(
	logTime time.Time,
	level slog.Level,
	scope string,
	message string,
	access *accessLog,
	requestID string,
	fields []field,
) string {
	builder := &strings.Builder{}

	// [ TIME ]
	builder.WriteString(h.paint("[ ", ansiWhite))
	builder.WriteString(h.paint(logTime.Format("15:04:05"), ansiBrightBlack))
	builder.WriteString(h.paint(" ]", ansiWhite))
	builder.WriteByte(' ')

	// LEVEL（紫色，跟 python-main 保持一致；按级别区分时再覆盖）
	builder.WriteString(h.paint(formatLevel(level), colorForLevel(level)))
	builder.WriteByte(' ')

	// | scope -
	builder.WriteString(h.paint("| ", ansiWhite))
	if scope == "" {
		scope = "-"
	}
	const scopeWidth = 18
	padded := scope
	if len(padded) < scopeWidth {
		padded = padded + strings.Repeat(" ", scopeWidth-len(padded))
	}
	builder.WriteString(h.paint(padded, ansiCyan))
	builder.WriteString(h.paint(" - ", ansiWhite))

	if access != nil {
		// HTTP access log: METHOD STATUS DURATION BYTES PATH
		builder.WriteString(h.paint(fmt.Sprintf("%-6s", strings.ToUpper(access.method)), colorForMethod(access.method)))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(fmt.Sprintf("%3d", access.status), colorForStatus(access.status)))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(fmt.Sprintf("%5s", access.duration), ansiBrightBlack))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(fmt.Sprintf("%6s", access.bytes), ansiBrightBlack))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(access.path, ansiWhite))
	} else {
		builder.WriteString(h.paint(strings.TrimSpace(message), colorForMessage(level)))
	}

	if requestID != "" {
		builder.WriteString("  ")
		builder.WriteString(h.paint("rid=", ansiBrightBlack))
		builder.WriteString(h.paint(requestID, ansiBrightBlack))
	}

	for _, f := range fields {
		builder.WriteByte(' ')
		builder.WriteString(h.paint(f.key+"=", ansiBrightBlack))
		valueColor := ""
		if f.key == "err" || f.key == "error" {
			valueColor = ansiRed
		}
		builder.WriteString(h.paint(quoteIfNeeded(f.value), valueColor))
	}

	builder.WriteByte('\n')
	return builder.String()
}

func (h *prettyHandler) paint(text, color string) string {
	if !h.colorize || color == "" || text == "" {
		return text
	}
	return color + text + ansiReset
}

// ----- field 抽取与归类 -----

type field struct {
	key   string
	value string
}

type accessLog struct {
	method   string
	status   int
	duration string
	bytes    string
	path     string
}

func pickScope(fields []field) (string, []field) {
	service, component := "", ""
	rest := make([]field, 0, len(fields))
	for _, f := range fields {
		switch f.key {
		case "service":
			service = f.value
		case "component":
			component = f.value
		default:
			rest = append(rest, f)
		}
	}
	switch {
	case service != "" && component != "":
		return service + "/" + component, rest
	case service != "":
		return service, rest
	default:
		return component, rest
	}
}

// pickAccess 识别 method/status/path 都齐的 HTTP access log，折叠成 accessLog。
func pickAccess(fields []field) (*accessLog, []field) {
	var method, path, durationMs, bytesWritten, remoteIP string
	var status int
	hasMethod, hasStatus, hasPath := false, false, false
	rest := make([]field, 0, len(fields))
	for _, f := range fields {
		switch f.key {
		case "method":
			method = f.value
			hasMethod = true
		case "status":
			if value, err := strconv.Atoi(f.value); err == nil {
				status = value
				hasStatus = true
			} else {
				rest = append(rest, f)
			}
		case "path":
			path = f.value
			hasPath = true
		case "duration_ms":
			durationMs = f.value
		case "bytes":
			bytesWritten = f.value
		case "remote_ip":
			remoteIP = f.value
		default:
			rest = append(rest, f)
		}
	}
	if !(hasMethod && hasStatus && hasPath) {
		// 不是 access log，把抽出来的字段补回去。
		if method != "" {
			rest = append(rest, field{key: "method", value: method})
		}
		if path != "" {
			rest = append(rest, field{key: "path", value: path})
		}
		if durationMs != "" {
			rest = append(rest, field{key: "duration_ms", value: durationMs})
		}
		if bytesWritten != "" {
			rest = append(rest, field{key: "bytes", value: bytesWritten})
		}
		if remoteIP != "" {
			rest = append(rest, field{key: "remote_ip", value: remoteIP})
		}
		return nil, rest
	}
	if remoteIP != "" && remoteIP != "127.0.0.1" && remoteIP != "::1" {
		rest = append(rest, field{key: "ip", value: remoteIP})
	}
	return &accessLog{
		method:   method,
		status:   status,
		duration: durationMs + "ms",
		bytes:    formatBytes(bytesWritten),
		path:     path,
	}, rest
}

func pickRequestID(fields []field) (string, []field) {
	rest := make([]field, 0, len(fields))
	requestID := ""
	for _, f := range fields {
		if f.key == "request_id" {
			requestID = f.value
			continue
		}
		rest = append(rest, f)
	}
	if len(requestID) > 8 {
		requestID = requestID[:8]
	}
	return requestID, rest
}

// ----- 颜色策略 -----

func formatLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DEBUG  "
	case level < slog.LevelWarn:
		return "INFO   "
	case level < slog.LevelError:
		return "WARNING"
	default:
		return "ERROR  "
	}
}

func colorForLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return ansiCyan
	case level < slog.LevelWarn:
		return ansiMagenta
	case level < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed + ansiBold
	}
}

func colorForMessage(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return ansiBrightBlack
	case level < slog.LevelWarn:
		return ansiGreen
	case level < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed
	}
}

func colorForMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet:
		return ansiGreen
	case http.MethodPost:
		return ansiBlue
	case http.MethodPut, http.MethodPatch:
		return ansiYellow
	case http.MethodDelete:
		return ansiRed
	default:
		return ansiMagenta
	}
}

func colorForStatus(status int) string {
	switch {
	case status >= 500:
		return ansiRed + ansiBold
	case status >= 400:
		return ansiYellow
	case status >= 300:
		return ansiCyan
	case status >= 200:
		return ansiGreen
	default:
		return ansiBrightBlack
	}
}

// ----- 字节展示 -----

func formatBytes(raw string) string {
	if raw == "" {
		return "0B"
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return raw
	}
	switch {
	case value >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(value)/float64(1<<20))
	case value >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(value)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", value)
	}
}

// ----- attr 展开 -----

func appendAttrs(target []field, attrs []slog.Attr, groups []string) []field {
	for _, attr := range attrs {
		target = appendAttr(target, attr, groups)
	}
	return target
}

func appendAttr(target []field, attr slog.Attr, groups []string) []field {
	if attr.Equal(slog.Attr{}) {
		return target
	}
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if key := strings.TrimSpace(attr.Key); key != "" {
			nextGroups = append(append([]string{}, groups...), key)
		}
		for _, nested := range attr.Value.Group() {
			target = appendAttr(target, nested, nextGroups)
		}
		return target
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(append(append([]string{}, groups...), key), ".")
	}
	value := stringifyValue(attr.Value)
	if key == "" {
		return target
	}
	return append(target, field{key: key, value: value})
}

func stringifyValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
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
	default:
		return fmt.Sprint(value.Any())
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
