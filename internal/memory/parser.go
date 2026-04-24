package memory

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	headingPattern = regexp.MustCompile(`^### (?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}) - \[(?P<kind>[A-Z]+)\] (?P<body>.+)$`)
	fieldPattern   = regexp.MustCompile(`^\*\s+\*\*(?P<key>[^*]+)\*\*: ?(?P<value>.*)$`)
)

// Parser 负责把 markdown 日记解析为结构化条目。
type Parser struct{}

// Parse 解析单个 markdown 文件。
func (Parser) Parse(content string, path string) ([]*Entry, error) {
	lines := strings.Split(content, "\n")
	result := make([]*Entry, 0, 8)
	current := make([]string, 0, 8)

	for _, line := range lines {
		if headingPattern.MatchString(line) {
			if len(current) > 0 {
				entry, err := parseEntry(current, path)
				if err != nil {
					return nil, err
				}
				result = append(result, entry)
			}
			current = []string{line}
			continue
		}
		if len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		entry, err := parseEntry(current, path)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func parseEntry(lines []string, path string) (*Entry, error) {
	match := headingPattern.FindStringSubmatch(lines[0])
	if len(match) == 0 {
		return nil, fmt.Errorf("日记标题格式不正确")
	}
	indexes := namedGroups(headingPattern)
	createdAt, err := time.ParseInLocation("2006-01-02 15:04", match[indexes["timestamp"]], time.Local)
	if err != nil {
		return nil, err
	}
	kind := match[indexes["kind"]]
	category, title := splitHeadingBody(kind, match[indexes["body"]])

	fields := make([]Field, 0, len(lines))
	entryID := ""
	for _, line := range lines[1:] {
		fieldMatch := fieldPattern.FindStringSubmatch(line)
		if len(fieldMatch) == 0 {
			continue
		}
		fieldIndexes := namedGroups(fieldPattern)
		key := strings.TrimSpace(fieldMatch[fieldIndexes["key"]])
		value := strings.TrimSpace(fieldMatch[fieldIndexes["value"]])
		if key == "ID" {
			entryID = value
			continue
		}
		fields = append(fields, Field{Key: key, Value: value})
	}
	if entryID == "" {
		entryID = deriveEntryID(kind, createdAt, title)
	}
	return &Entry{
		ID:        entryID,
		CreatedAt: createdAt,
		Kind:      kind,
		Title:     title,
		Category:  category,
		Fields:    fields,
		Path:      path,
	}, nil
}

func splitHeadingBody(kind string, body string) (string, string) {
	if kind == "LRN" && strings.Contains(body, ": ") {
		parts := strings.SplitN(body, ": ", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", strings.TrimSpace(body)
}

func deriveEntryID(kind string, createdAt time.Time, title string) string {
	digest := md5.Sum([]byte(title))
	return fmt.Sprintf("%s-%s-%s", kind, createdAt.Format("20060102-1504"), hex.EncodeToString(digest[:])[:8])
}

func namedGroups(pattern *regexp.Regexp) map[string]int {
	result := make(map[string]int)
	for index, name := range pattern.SubexpNames() {
		if name != "" {
			result[name] = index
		}
	}
	return result
}
