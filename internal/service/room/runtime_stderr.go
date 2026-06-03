package room

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func normalizeRuntimeStderrLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || utf8.ValidString(trimmed) {
		return trimmed
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().String(trimmed)
	if err != nil {
		return trimmed
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		return trimmed
	}
	return decoded
}
