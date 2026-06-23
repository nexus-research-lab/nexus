package feishudocx

import "strings"

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
