package logx

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
)

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

func formatDuration(raw string) string {
	if raw == "" {
		return "-"
	}
	ms, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw + "ms"
	}
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return raw + "ms"
}

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
			nextGroups = slices.Concat(groups, []string{key})
		}
		for _, nested := range attr.Value.Group() {
			target = appendAttr(target, nested, nextGroups)
		}
		return target
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(slices.Concat(groups, []string{key}), ".")
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

func formatSDKContext(debugLog *sdkDebugLog) string {
	if debugLog == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if sessionKey := shortSessionKey(debugLog.sessionKey); sessionKey != "" {
		parts = append(parts, "s="+padCompactValue(sessionKey, 12))
	}
	if agentID := shortIdentifier(debugLog.agentID); agentID != "" {
		parts = append(parts, "a="+padCompactValue(agentID, 12))
	}
	if roundID := shortIdentifier(debugLog.roundID); roundID != "" {
		parts = append(parts, "r="+padCompactValue(roundID, 12))
	}
	return strings.Join(parts, " ")
}

func shortSessionKey(sessionKey string) string {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" {
		return ""
	}
	if index := strings.LastIndex(normalized, ":"); index >= 0 {
		normalized = normalized[index+1:]
	}
	return shortIdentifier(normalized)
}

func shortIdentifier(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) <= 12 {
		return normalized
	}
	return string(runes[:12])
}

func padCompactValue(value string, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}
