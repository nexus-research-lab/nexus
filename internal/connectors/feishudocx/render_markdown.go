package feishudocx

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
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
