package feishudocx

import (
	"fmt"
	"maps"
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
	return maps.Clone(r.unsupported)
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
	t := blockType(block)
	if t >= blockTypeHeading1 && t <= blockTypeHeading9 {
		return r.renderHeading(block, t)
	}
	renderer := markdownBlockRenderers[t]
	if renderer != nil {
		return renderer(r, block, depth)
	}
	r.recordUnsupported(block, t)
	return ""
}

type markdownBlockRenderer func(*markdownRenderer, Block, int) string

var markdownBlockRenderers = map[int]markdownBlockRenderer{
	blockTypeText:       renderTextBlock,
	blockTypeBullet:     renderBulletBlock,
	blockTypeOrdered:    renderOrderedBlock,
	blockTypeTodo:       renderTodoBlock,
	blockTypeCode:       renderCodeBlock,
	blockTypeQuote:      renderQuoteBlock,
	blockTypeDivider:    renderDividerBlock,
	blockTypeImage:      renderImageBlock,
	blockTypeFile:       renderFileBlock,
	blockTypeSheet:      renderSheetBlock,
	blockTypeBitable:    renderBitableBlock,
	blockTypeBoard:      renderBoardBlock,
	blockTypeTable:      renderTableBlock,
	blockTypeTableCell:  renderEmptyBlock,
	blockTypePage:       renderEmptyBlock,
	blockTypeQuoteGroup: renderEmptyBlock,
}

func (r *markdownRenderer) renderHeading(block Block, blockTypeValue int) string {
	level := blockTypeValue - blockTypeHeading1 + 1
	return strings.Repeat("#", level) + " " + r.inlineText(block, "heading"+strconv.Itoa(level))
}

func (r *markdownRenderer) recordUnsupported(block Block, blockTypeValue int) {
	if len(blockChildren(block)) == 0 {
		r.unsupported[strconv.Itoa(blockTypeValue)]++
	}
}

func renderTextBlock(renderer *markdownRenderer, block Block, _ int) string {
	return renderer.inlineText(block, "text")
}

func renderBulletBlock(renderer *markdownRenderer, block Block, depth int) string {
	return indent(depth) + "- " + renderer.inlineText(block, "bullet")
}

func renderOrderedBlock(renderer *markdownRenderer, block Block, depth int) string {
	return indent(depth) + "1. " + renderer.inlineText(block, "ordered")
}

func renderTodoBlock(renderer *markdownRenderer, block Block, depth int) string {
	return indent(depth) + "- [ ] " + renderer.inlineText(block, "todo")
}

func renderCodeBlock(renderer *markdownRenderer, block Block, _ int) string {
	return "```\n" + strings.TrimRight(renderer.inlineText(block, "code"), "\n") + "\n```"
}

func renderQuoteBlock(renderer *markdownRenderer, block Block, _ int) string {
	return quoteMarkdown(renderer.inlineText(block, "quote"))
}

func renderDividerBlock(_ *markdownRenderer, _ Block, _ int) string {
	return "---"
}

func renderImageBlock(_ *markdownRenderer, block Block, _ int) string {
	return mediaPlaceholder("image", block)
}

func renderFileBlock(_ *markdownRenderer, block Block, _ int) string {
	return mediaPlaceholder("file", block)
}

func renderSheetBlock(_ *markdownRenderer, block Block, _ int) string {
	return mediaPlaceholder("sheet", block)
}

func renderBitableBlock(_ *markdownRenderer, block Block, _ int) string {
	return mediaPlaceholder("bitable", block)
}

func renderBoardBlock(_ *markdownRenderer, block Block, _ int) string {
	return mediaPlaceholder("board", block)
}

func renderTableBlock(renderer *markdownRenderer, block Block, _ int) string {
	return renderer.renderTable(block)
}

func renderEmptyBlock(_ *markdownRenderer, _ Block, _ int) string {
	return ""
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

func escapeTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}
