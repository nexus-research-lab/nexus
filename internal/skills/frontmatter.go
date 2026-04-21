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
	for _, rawLine := range lines {
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
