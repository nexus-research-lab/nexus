package skills

import (
	"strings"
)

type frontmatterData struct {
	Name           string
	Title          string
	Description    string
	Scope          string
	Tags           []string
	Version        string
	CategoryKey    string
	CategoryName   string
	Recommendation string
	ReadmeMarkdown string
}

func parseSkillFrontmatter(content string, fallbackName string) frontmatterData {
	data := frontmatterData{
		Name:           strings.TrimSpace(fallbackName),
		Title:          strings.TrimSpace(fallbackName),
		Scope:          "any",
		Tags:           []string{},
		ReadmeMarkdown: content,
	}
	lines := strings.Split(extractFrontmatter(content), "\n")
	pendingKey := ""
	pendingList := make([]string, 0, 4)
	for index := 0; index < len(lines); index++ {
		rawLine := lines[index]
		line := strings.TrimRight(rawLine, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if pendingKey != "" && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			pendingList = append(pendingList, strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- ")), "\"'"))
			continue
		}
		if pendingKey != "" {
			assignFrontmatterValue(&data, pendingKey, pendingList)
			pendingKey = ""
			pendingList = pendingList[:0]
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if isFrontmatterBlockScalarMarker(value) {
			blockLines := make([]string, 0, 4)
			index++
			for index < len(lines) {
				blockLine := strings.TrimRight(lines[index], "\r")
				if isFrontmatterTopLevelKeyLine(blockLine) {
					index--
					break
				}
				blockLines = append(blockLines, blockLine)
				index++
			}
			assignFrontmatterValue(&data, key, parseFrontmatterBlockScalar(value, blockLines))
			continue
		}
		if value == "" {
			pendingKey = key
			pendingList = pendingList[:0]
			continue
		}
		assignFrontmatterValue(&data, key, parseFrontmatterScalar(value))
	}
	if pendingKey != "" {
		assignFrontmatterValue(&data, pendingKey, pendingList)
	}
	if data.Name == "" {
		data.Name = fallbackName
	}
	if data.Title == "" {
		data.Title = data.Name
	}
	if data.Scope == "" {
		data.Scope = "any"
	}
	return data
}

func extractFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := strings.TrimPrefix(content, "---")
	index := strings.Index(rest, "\n---")
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:index])
}

func parseFrontmatterScalar(value string) any {
	clean := strings.Trim(strings.TrimSpace(value), "\"'")
	if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
		rawItems := strings.Split(strings.Trim(clean, "[]"), ",")
		items := make([]string, 0, len(rawItems))
		for _, item := range rawItems {
			normalized := strings.Trim(strings.TrimSpace(item), "\"'")
			if normalized != "" {
				items = append(items, normalized)
			}
		}
		return items
	}
	return clean
}

func isFrontmatterBlockScalarMarker(value string) bool {
	switch strings.TrimSpace(value) {
	case "|", "|-", "|+", ">", ">-", ">+":
		return true
	default:
		return false
	}
}

func isFrontmatterTopLevelKeyLine(line string) bool {
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	parts := strings.SplitN(line, ":", 2)
	return len(parts) == 2 && strings.TrimSpace(parts[0]) != ""
}

func parseFrontmatterBlockScalar(marker string, lines []string) string {
	normalizedLines := trimFrontmatterBlockIndent(lines)
	if strings.HasPrefix(strings.TrimSpace(marker), ">") {
		return foldFrontmatterBlockScalar(normalizedLines)
	}
	return strings.TrimSpace(strings.Join(normalizedLines, "\n"))
}

// 中文注释：YAML block scalar 会整体缩进，这里只剥掉公共缩进，保留正文内部缩进。
func trimFrontmatterBlockIndent(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return []string{}
	}

	minIndent := -1
	for _, line := range lines[start:end] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := countFrontmatterIndent(line)
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	trimmed := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		if len(line) < minIndent {
			trimmed = append(trimmed, "")
			continue
		}
		trimmed = append(trimmed, line[minIndent:])
	}
	return trimmed
}

func countFrontmatterIndent(line string) int {
	count := 0
	for count < len(line) && (line[count] == ' ' || line[count] == '\t') {
		count++
	}
	return count
}

func foldFrontmatterBlockScalar(lines []string) string {
	paragraphs := make([]string, 0, 2)
	current := make([]string, 0, 4)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, strings.Join(current, " "))
				current = current[:0]
			}
			continue
		}
		current = append(current, strings.TrimSpace(line))
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, strings.Join(current, " "))
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func assignFrontmatterValue(target *frontmatterData, key string, value any) {
	switch strings.TrimSpace(key) {
	case "name":
		target.Name = toString(value)
	case "title":
		target.Title = toString(value)
	case "description":
		target.Description = toString(value)
	case "scope":
		target.Scope = toString(value)
	case "version":
		target.Version = toString(value)
	case "tags":
		target.Tags = toStringSlice(value)
	case "category_key":
		target.CategoryKey = toString(value)
	case "category_name":
		target.CategoryName = toString(value)
	case "recommendation":
		target.Recommendation = toString(value)
	}
}

func toString(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func toStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append(make([]string, 0, len(typed)), typed...)
	case string:
		if typed == "" {
			return []string{}
		}
		if strings.Contains(typed, ",") {
			rawItems := strings.Split(typed, ",")
			items := make([]string, 0, len(rawItems))
			for _, item := range rawItems {
				normalized := strings.Trim(strings.TrimSpace(item), "\"'[]")
				if normalized != "" {
					items = append(items, normalized)
				}
			}
			return items
		}
		return []string{strings.Trim(typed, "\"'[]")}
	default:
		return []string{}
	}
}
