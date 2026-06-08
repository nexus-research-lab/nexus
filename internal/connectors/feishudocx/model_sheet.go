package feishudocx

// SheetTarget 是从 URL 或 token 里解析出的电子表格目标。
type SheetTarget struct {
	SpreadsheetToken string `json:"spreadsheet_token"`
	SheetID          string `json:"sheet_id,omitempty"`
	Raw              string `json:"raw"`
}

// SheetListResult 表示电子表格内工作表列表。
type SheetListResult struct {
	SpreadsheetToken string           `json:"spreadsheet_token"`
	Sheets           []map[string]any `json:"sheets"`
}

// SheetValuesResult 表示指定范围的单元格取值。
type SheetValuesResult struct {
	SpreadsheetToken string  `json:"spreadsheet_token"`
	Range            string  `json:"range"`
	Revision         int     `json:"revision,omitempty"`
	Values           [][]any `json:"values"`
}

// SheetFindResult 表示电子表格内容查找结果。
type SheetFindResult struct {
	SpreadsheetToken string         `json:"spreadsheet_token"`
	SheetID          string         `json:"sheet_id"`
	Query            string         `json:"query"`
	Range            string         `json:"range,omitempty"`
	FindResult       map[string]any `json:"find_result"`
}
