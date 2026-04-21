package config

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// LoadDotEnv 从指定文件加载 .env 到进程环境变量。
// 不传文件名时默认查找 ./.env 和 ../.env。
// 已存在的环境变量不会被覆盖。
func LoadDotEnv(filenames ...string) error {
	if len(filenames) == 0 {
		filenames = []string{".env", "../.env"}
	}
	for _, name := range filenames {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		envMap, err := parseEnvBytes(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		for key, value := range envMap {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
		return nil
	}
	return nil
}

// parseEnvBytes 解析 .env 文件内容，返回 key-value map。
func parseEnvBytes(src []byte) (map[string]string, error) {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	out := make(map[string]string)
	cutset := src
	for {
		cutset = findStatementStart(cutset)
		if cutset == nil {
			break
		}
		key, left, err := locateKeyName(cutset)
		if err != nil {
			return nil, err
		}
		value, left, err := extractVarValue(left, out)
		if err != nil {
			return nil, err
		}
		out[key] = value
		cutset = left
	}
	return out, nil
}

// findStatementStart 跳过空白和注释行，定位下一条语句起始位置。
func findStatementStart(src []byte) []byte {
	pos := bytes.IndexFunc(src, func(r rune) bool {
		return !unicode.IsSpace(r)
	})
	if pos == -1 {
		return nil
	}
	src = src[pos:]
	if src[0] == '#' {
		newline := bytes.IndexByte(src, '\n')
		if newline == -1 {
			return nil
		}
		return findStatementStart(src[newline+1:])
	}
	return src
}

// locateKeyName 解析变量名，支持 export 前缀。
func locateKeyName(src []byte) (key string, cutset []byte, err error) {
	src = bytes.TrimLeftFunc(src, unicode.IsSpace)

	// 跳过 export 前缀
	if bytes.HasPrefix(src, []byte("export ")) {
		src = bytes.TrimLeft(src[7:], " \t")
	}

	for i, char := range src {
		r := rune(char)
		switch {
		case char == '=' || char == ':':
			key = string(src[:i])
			cutset = bytes.TrimLeftFunc(src[i+1:], unicode.IsSpace)
			key = strings.TrimRightFunc(key, unicode.IsSpace)
			if key == "" {
				return "", nil, fmt.Errorf("empty key before '='")
			}
			return key, cutset, nil
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.':
			continue
		case unicode.IsSpace(r):
			continue
		default:
			return "", nil, fmt.Errorf("unexpected character %q in variable name near %q", char, src)
		}
	}
	return "", nil, fmt.Errorf("no '=' found in line")
}

// extractVarValue 解析变量值，支持引号、转义、变量展开。
func extractVarValue(src []byte, vars map[string]string) (string, []byte, error) {
	if len(src) == 0 {
		return "", nil, nil
	}

	// 带引号的值
	if src[0] == '\'' || src[0] == '"' {
		quote := src[0]
		for i := 1; i < len(src); i++ {
			if src[i] == byte(quote) && src[i-1] != '\\' {
				raw := string(src[1:i])
				if quote == '"' {
					raw = expandEscapes(raw)
					raw = expandVariables(raw, vars)
				}
				rest := src[i+1:]
				if idx := bytes.IndexByte(rest, '\n'); idx >= 0 {
					rest = rest[idx+1:]
				} else {
					rest = nil
				}
				return raw, rest, nil
			}
		}
		end := bytes.IndexByte(src, '\n')
		if end == -1 {
			end = len(src)
		}
		return "", nil, fmt.Errorf("unterminated quoted value: %s", src[:end])
	}

	// 无引号值：读到行尾，去掉内联注释
	lineEnd := bytes.IndexByte(src, '\n')
	if lineEnd == -1 {
		lineEnd = len(src)
	}
	line := string(src[:lineEnd])

	// 处理行内注释: `value # comment`
	if idx := strings.Index(line, " #"); idx > 0 {
		line = line[:idx]
	}

	value := strings.TrimRight(line, " \t")
	value = expandVariables(value, vars)

	var rest []byte
	if lineEnd < len(src) {
		rest = src[lineEnd+1:]
	}
	return value, rest, nil
}

var (
	escapeRegex        = regexp.MustCompile(`\\.`)
	unescapeCharsRegex = regexp.MustCompile(`\\([^$])`)
)

// expandEscapes 处理双引号内的转义序列。
func expandEscapes(str string) string {
	out := escapeRegex.ReplaceAllStringFunc(str, func(match string) string {
		c := strings.TrimPrefix(match, `\`)
		switch c {
		case "n":
			return "\n"
		case "r":
			return "\r"
		case "t":
			return "\t"
		default:
			return match
		}
	})
	return unescapeCharsRegex.ReplaceAllString(out, "$1")
}

var expandVarRegex = regexp.MustCompile(`(\\)?\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandVariables 展开 $VAR 和 ${VAR} 形式的变量引用。
func expandVariables(v string, localVars map[string]string) string {
	// 先展开 ${VAR} 形式
	v = expandVarRegex.ReplaceAllStringFunc(v, func(s string) string {
		submatch := expandVarRegex.FindStringSubmatch(s)
		if submatch[1] == "\\" {
			return s[1:]
		}
		name := submatch[2]
		if val, ok := localVars[name]; ok {
			return val
		}
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return ""
	})

	// 再展开 $VAR 形式（仅限大写字母、数字、下划线）
	simpleVarRegex := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	v = simpleVarRegex.ReplaceAllStringFunc(v, func(s string) string {
		name := s[1:]
		if val, ok := localVars[name]; ok {
			return val
		}
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return s
	})
	return v
}
