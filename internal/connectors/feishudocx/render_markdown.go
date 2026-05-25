package feishudocx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	blockTypePage       = 1
	blockTypeText       = 2
	blockTypeHeading1   = 3
	blockTypeHeading9   = 11
	blockTypeBullet     = 12
	blockTypeOrdered    = 13
	blockTypeCode       = 14
	blockTypeQuote      = 15
	blockTypeTodo       = 17
	blockTypeDivider    = 22
	blockTypeFile       = 23
	blockTypeImage      = 27
	blockTypeSheet      = 30
	blockTypeTable      = 31
	blockTypeTableCell  = 32
	blockTypeBitable    = 33
	blockTypeQuoteGroup = 34
	blockTypeBoard      = 43
)

type markdownRenderer struct {
	blocks      map[string]Block
	order       []string
	title       string
	withBlockID bool
	unsupported map[string]int
	visited     map[string]bool
}

func newMarkdownRenderer(blocks []Block, title string, withBlockID bool) *markdownRenderer {
	renderer := &markdownRenderer{
		blocks:      map[string]Block{},
		title:       title,
		withBlockID: withBlockID,
		unsupported: map[string]int{},
		visited:     map[string]bool{},
	}
	for index, block := range blocks {
		id := blockID(block)
		if id == "" {
			id = fmt.Sprintf("__index_%d", index)
		}
		renderer.blocks[id] = block
		renderer.order = append(renderer.order, id)
	}
	return renderer
}

func (r *markdownRenderer) Render(documentID string) string {
	rootID := r.rootID(documentID)
	if rootID == "" {
		return ""
	}
	lines := r.renderChildren(rootID, 0)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (r *markdownRenderer) UnsupportedBlocks() map[string]int {
	if len(r.unsupported) == 0 {
		return nil
	}
	result := map[string]int{}
	for key, value := range r.unsupported {
		result[key] = value
	}
	return result
}

func (r *markdownRenderer) rootID(documentID string) string {
	if _, ok := r.blocks[documentID]; ok {
		return documentID
	}
	for _, id := range r.order {
		if blockType(r.blocks[id]) == blockTypePage {
			return id
		}
	}
	if len(r.order) == 0 {
		return ""
	}
	return r.order[0]
}

func (r *markdownRenderer) renderChildren(parentID string, depth int) []string {
	parent := r.blocks[parentID]
	children := blockChildren(parent)
	if len(children) == 0 && blockType(parent) == blockTypePage {
		for _, id := range r.order {
			if id != parentID {
				children = append(children, id)
			}
		}
	}
	var lines []string
	for _, childID := range children {
		rendered := r.renderBlock(childID, depth)
		if rendered == "" {
			continue
		}
		lines = append(lines, rendered)
	}
	return lines
}

func (r *markdownRenderer) renderBlock(id string, depth int) string {
	if r.visited[id] {
		return ""
	}
	block, ok := r.blocks[id]
	if !ok {
		return ""
	}
	r.visited[id] = true
	content := r.renderSelf(block, depth)
	if !selfConsumesChildren(blockType(block)) {
		childLines := r.renderChildren(id, childDepth(block, depth))
		if len(childLines) > 0 {
			if content != "" {
				content += "\n" + strings.Join(childLines, "\n")
			} else {
				content = strings.Join(childLines, "\n")
			}
		}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if r.withBlockID {
		return fmt.Sprintf("<!-- feishu-docx:block_id=%s -->\n%s\n<!-- /feishu-docx:block_id -->", id, content)
	}
	return content
}

func (r *markdownRenderer) renderSelf(block Block, depth int) string {
	switch t := blockType(block); {
	case t == blockTypeText:
		return r.inlineText(block, "text")
	case t >= blockTypeHeading1 && t <= blockTypeHeading9:
		level := t - blockTypeHeading1 + 1
		return strings.Repeat("#", level) + " " + r.inlineText(block, "heading"+strconv.Itoa(level))
	case t == blockTypeBullet:
		return indent(depth) + "- " + r.inlineText(block, "bullet")
	case t == blockTypeOrdered:
		return indent(depth) + "1. " + r.inlineText(block, "ordered")
	case t == blockTypeTodo:
		return indent(depth) + "- [ ] " + r.inlineText(block, "todo")
	case t == blockTypeCode:
		return "```\n" + strings.TrimRight(r.inlineText(block, "code"), "\n") + "\n```"
	case t == blockTypeQuote:
		return quoteMarkdown(r.inlineText(block, "quote"))
	case t == blockTypeDivider:
		return "---"
	case t == blockTypeImage:
		return mediaPlaceholder("image", block)
	case t == blockTypeFile:
		return mediaPlaceholder("file", block)
	case t == blockTypeSheet:
		return mediaPlaceholder("sheet", block)
	case t == blockTypeBitable:
		return mediaPlaceholder("bitable", block)
	case t == blockTypeBoard:
		return mediaPlaceholder("board", block)
	case t == blockTypeTable:
		return r.renderTable(block)
	case t == blockTypeTableCell || t == blockTypePage || t == blockTypeQuoteGroup:
		return ""
	default:
		if len(blockChildren(block)) == 0 {
			r.unsupported[strconv.Itoa(t)]++
		}
		return ""
	}
}

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

func (r *markdownRenderer) renderTable(block Block) string {
	cellIDs := blockChildren(block)
	if len(cellIDs) == 0 {
		if table, _ := block["table"].(map[string]any); table != nil {
			cellIDs = stringSlice(table["cells"])
		}
	}
	rows, columns := tableSize(block, len(cellIDs))
	if rows == 0 || columns == 0 {
		r.unsupported["table"]++
		return ""
	}
	values := make([]string, rows*columns)
	for index := range values {
		if index >= len(cellIDs) {
			continue
		}
		values[index] = escapeTableCell(r.renderCell(cellIDs[index]))
	}
	var lines []string
	for row := 0; row < rows; row++ {
		var cells []string
		for column := 0; column < columns; column++ {
			cells = append(cells, firstNonEmpty(values[row*columns+column], " "))
		}
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
		if row == 0 {
			lines = append(lines, "| "+strings.Join(repeatString("---", columns), " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

func (r *markdownRenderer) renderCell(cellID string) string {
	cell, ok := r.blocks[cellID]
	if !ok {
		return ""
	}
	var parts []string
	for _, childID := range blockChildren(cell) {
		block := r.blocks[childID]
		if block == nil {
			continue
		}
		if textField := textFieldForBlockType(blockType(block)); textField != "" {
			parts = append(parts, r.inlineText(block, textField))
		}
	}
	return strings.Join(parts, "<br>")
}

func ParseDocumentTarget(raw string) (DocumentTarget, error) {
	value := strings.TrimSpace(raw)
	target := DocumentTarget{Raw: value}
	if value == "" {
		return target, fmt.Errorf("飞书文档 URL 或 document_id 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.Contains(value, "/") {
			return target, fmt.Errorf("飞书文档 URL 格式不正确")
		}
		target.DocumentID = value
		target.SourceType = "docx"
		return target, nil
	}
	segments := splitPath(parsed.Path)
	for index, segment := range segments {
		if index+1 >= len(segments) {
			continue
		}
		switch segment {
		case "docx":
			target.DocumentID = segments[index+1]
			target.SourceType = "docx"
			return target, nil
		case "wiki":
			target.WikiToken = segments[index+1]
			target.SourceType = "wiki"
			return target, nil
		}
	}
	return target, fmt.Errorf("暂只支持飞书 docx/wiki 链接")
}

func blockID(block Block) string {
	if block == nil {
		return ""
	}
	value, _ := block["block_id"].(string)
	return strings.TrimSpace(value)
}

func blockType(block Block) int {
	if block == nil {
		return 0
	}
	switch value := block["block_type"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case jsonNumber:
		parsed, _ := strconv.Atoi(value.String())
		return parsed
	default:
		return 0
	}
}

type jsonNumber interface {
	String() string
}

func blockChildren(block Block) []string {
	if block == nil {
		return nil
	}
	return stringSlice(block["children"])
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok && value != "" {
				result = append(result, value)
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

func textFieldForBlockType(blockType int) string {
	switch {
	case blockType == blockTypeText:
		return "text"
	case blockType >= blockTypeHeading1 && blockType <= blockTypeHeading9:
		return "heading" + strconv.Itoa(blockType-blockTypeHeading1+1)
	case blockType == blockTypeBullet:
		return "bullet"
	case blockType == blockTypeOrdered:
		return "ordered"
	case blockType == blockTypeTodo:
		return "todo"
	case blockType == blockTypeCode:
		return "code"
	case blockType == blockTypeQuote:
		return "quote"
	default:
		return ""
	}
}

func tableSize(block Block, cellCount int) (int, int) {
	table, _ := block["table"].(map[string]any)
	property, _ := table["property"].(map[string]any)
	rows := intField(property, "row_size")
	columns := intField(property, "column_size")
	if rows > 0 && columns > 0 {
		return rows, columns
	}
	if cellCount == 0 {
		return 0, 0
	}
	return 1, cellCount
}

func childDepth(block Block, depth int) int {
	switch blockType(block) {
	case blockTypeBullet, blockTypeOrdered, blockTypeTodo:
		return depth + 1
	default:
		return depth
	}
}

func selfConsumesChildren(blockType int) bool {
	return blockType == blockTypeTable
}

func mediaPlaceholder(kind string, block Block) string {
	var token string
	if raw, _ := block[kind].(map[string]any); raw != nil {
		token = firstNonEmpty(stringField(raw, "token"), stringField(raw, "file_token"))
	}
	if token == "" {
		return "<!-- feishu-docx:" + kind + " -->"
	}
	return "<!-- feishu-docx:" + kind + " token=" + token + " -->"
}

func quoteMarkdown(value string) string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		lines = append(lines, "> "+line)
	}
	return strings.Join(lines, "\n")
}

func escapeTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func splitPath(path string) []string {
	raw := strings.Split(path, "/")
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func indent(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

func repeatString(value string, count int) []string {
	if count <= 0 {
		return nil
	}
	result := make([]string, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func stringField(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func boolField(raw map[string]any, key string) bool {
	value, _ := raw[key].(bool)
	return value
}

func intField(raw map[string]any, key string) int {
	if raw == nil {
		return 0
	}
	switch value := raw[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
