package feishudocx

import "strings"

func (r *markdownRenderer) inlineText(block Block, field string) string {
	raw, _ := block[field].(map[string]any)
	if raw == nil {
		return ""
	}
	elements := textElements(raw["elements"])
	parts := make([]string, 0, len(elements))
	for _, element := range elements {
		if element == nil {
			continue
		}
		parts = append(parts, renderTextElement(element))
	}
	return strings.Join(parts, "")
}

func textElements(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if element, ok := item.(map[string]any); ok {
				result = append(result, element)
			}
		}
		return result
	default:
		return nil
	}
}

func renderTextElement(element map[string]any) string {
	if textRun, _ := element["text_run"].(map[string]any); textRun != nil {
		content, _ := textRun["content"].(string)
		style, _ := textRun["text_element_style"].(map[string]any)
		return applyTextStyle(content, style)
	}
	if mention, _ := element["mention_user"].(map[string]any); mention != nil {
		return "@" + firstNonEmpty(stringField(mention, "name"), stringField(mention, "user_id"))
	}
	if mention, _ := element["mention_doc"].(map[string]any); mention != nil {
		title := firstNonEmpty(stringField(mention, "title"), "文档")
		if urlValue := stringField(mention, "url"); urlValue != "" {
			return "[" + title + "](" + urlValue + ")"
		}
		return title
	}
	if equation, _ := element["equation"].(map[string]any); equation != nil {
		return "$" + stringField(equation, "content") + "$"
	}
	if preview, _ := element["link_preview"].(map[string]any); preview != nil {
		return firstNonEmpty(stringField(preview, "url"), stringField(preview, "title"))
	}
	return ""
}

func applyTextStyle(content string, style map[string]any) string {
	if content == "" || style == nil {
		return content
	}
	if link, _ := style["link"].(map[string]any); link != nil {
		if urlValue := stringField(link, "url"); urlValue != "" {
			content = "[" + content + "](" + urlValue + ")"
		}
	}
	if boolField(style, "inline_code") {
		content = "`" + content + "`"
	}
	if boolField(style, "bold") {
		content = "**" + content + "**"
	}
	if boolField(style, "italic") {
		content = "*" + content + "*"
	}
	if boolField(style, "strikethrough") {
		content = "~~" + content + "~~"
	}
	return content
}
