package logx

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func (h *prettyHandler) format(
	logTime time.Time,
	level slog.Level,
	scope string,
	message string,
	access *accessLog,
	requestID string,
	sdkDebug *sdkDebugLog,
	fields []field,
) string {
	builder := &strings.Builder{}

	// [ TIME ]
	builder.WriteString(h.paint("[ ", ansiWhite))
	builder.WriteString(h.paint(logTime.Format("15:04:05.000"), ansiBrightBlack))
	builder.WriteString(h.paint(" ]", ansiWhite))
	builder.WriteByte(' ')

	// LEVEL（默认紫色；按级别区分时再覆盖）
	builder.WriteString(h.paint(formatLevel(level), colorForLevel(level)))
	builder.WriteByte(' ')

	// | scope -
	builder.WriteString(h.paint("| ", ansiWhite))
	if scope == "" {
		scope = "-"
	}
	const scopeWidth = 10
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
		builder.WriteString(h.paint(fmt.Sprintf("%4d", access.status), colorForStatus(access.status)))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(fmt.Sprintf("%6s", access.duration), ansiBrightBlack))
		builder.WriteByte(' ')
		builder.WriteString(h.paint(fmt.Sprintf("%7s", access.bytes), ansiBrightBlack))
		builder.WriteByte(' ')

		if requestID != "" {
			builder.WriteString(h.paint("rid=", ansiBrightBlack))
			builder.WriteString(h.paint(requestID, ansiBrightBlack))
			builder.WriteByte(' ')
		}

		builder.WriteString(h.paint(access.path, ansiWhite))
		builder.WriteByte(' ')
	} else {
		builder.WriteString(h.paint(strings.TrimSpace(message), colorForMessage(level)))
		compactContext := ""
		if sdkDebug != nil {
			compactContext = formatSDKContext(sdkDebug)
			if compactContext != "" {
				builder.WriteByte(' ')
				builder.WriteString(h.paint(compactContext, ansiBrightBlack))
			}
		}
		if sdkDebug != nil && strings.TrimSpace(sdkDebug.summary) != "" {
			if strings.TrimSpace(message) != "" || compactContext != "" {
				builder.WriteByte(' ')
			}
			builder.WriteString(h.paint(strings.TrimSpace(sdkDebug.summary), colorForSDKSummary(sdkDebug.summary)))
		}
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
