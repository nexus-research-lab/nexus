package logx

import (
	"log/slog"
	"net/http"
	"strings"
)

// ANSI 配色
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

func colorForSDKSummary(summary string) string {
	normalized := strings.TrimSpace(summary)
	switch {
	case strings.HasPrefix(normalized, "stream "):
		return ansiCyan
	case strings.HasPrefix(normalized, "assistant "):
		return ansiGreen
	case strings.HasPrefix(normalized, "result "):
		return ansiBlue
	case strings.HasPrefix(normalized, "system "):
		return ansiMagenta
	case strings.HasPrefix(normalized, "tool_progress"):
		return ansiYellow
	default:
		return ansiWhite
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
