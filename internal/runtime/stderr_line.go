package runtime

import (
	"cmp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// NormalizeRuntimeStderrLine 归一化 runtime 子进程 stderr 单行内容。
func NormalizeRuntimeStderrLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || utf8.ValidString(trimmed) {
		return trimmed
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().String(trimmed)
	if err != nil {
		return trimmed
	}
	decoded = strings.TrimSpace(decoded)
	return cmp.Or(decoded, trimmed)
}
