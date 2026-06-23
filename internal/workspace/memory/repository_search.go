package memory

import (
	"os"
	"strings"
)

// Search 在记忆文件中做关键词检索，以条目块为单位匹配，支持跨字段搜索。
func (r *Repository) Search(query string, limit int) ([]SearchMatch, error) {
	terms := tokenizeQuery(query)
	if len(terms) == 0 {
		return nil, newClientError("query 不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	items := make([]SearchMatch, 0, limit)
	for _, path := range r.iterSearchFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		relPath := toRelative(r.workspacePath, path)
		for _, block := range splitIntoBlocks(string(content)) {
			if !containsAllTerms(strings.ToLower(block.content), terms) {
				continue
			}
			items = append(items, SearchMatch{
				Path:    relPath,
				Line:    block.startLine,
				Content: block.headline,
			})
			if len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

// searchBlock 表示一个可检索的文件块。
type searchBlock struct {
	startLine int
	headline  string
	content   string
}

// splitIntoBlocks 按 markdown 标题把文件内容分割成可检索的块。
// 标题前的非空行以单行为单位加入结果，供检索 MEMORY.md 等无条目结构的文件。
func splitIntoBlocks(content string) []searchBlock {
	lines := strings.Split(content, "\n")
	var blocks []searchBlock
	blockStart := -1
	var blockLines []string

	flush := func() {
		if blockStart < 0 || len(blockLines) == 0 {
			return
		}
		blocks = append(blocks, searchBlock{
			startLine: blockStart + 1,
			headline:  strings.TrimSpace(blockLines[0]),
			content:   strings.Join(blockLines, "\n"),
		})
	}

	for i, line := range lines {
		if isMarkdownHeading(line) {
			flush()
			blockStart = i
			blockLines = []string{line}
		} else if blockStart >= 0 {
			blockLines = append(blockLines, line)
		} else {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				blocks = append(blocks, searchBlock{startLine: i + 1, headline: trimmed, content: line})
			}
		}
	}
	flush()
	return blocks
}

func isMarkdownHeading(line string) bool {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i > 0 && i <= 6 && i < len(line) && line[i] == ' '
}

func tokenizeQuery(query string) []string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func containsAllTerms(value string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(value, term) {
			return false
		}
	}
	return true
}
