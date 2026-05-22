package feishudocx

// BitableTarget 是从 URL 或 token 里解析出的多维表格目标。
type BitableTarget struct {
	AppToken string `json:"app_token"`
	TableID  string `json:"table_id,omitempty"`
	Raw      string `json:"raw"`
}

// BitableTableListResult 表示多维表格下的数据表列表。
type BitableTableListResult struct {
	AppToken      string           `json:"app_token"`
	Tables        []map[string]any `json:"tables"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}

// BitableFieldListResult 表示多维表格字段列表。
type BitableFieldListResult struct {
	AppToken      string           `json:"app_token"`
	TableID       string           `json:"table_id"`
	Fields        []map[string]any `json:"fields"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}

// BitableRecordListResult 表示多维表格记录列表。
type BitableRecordListResult struct {
	AppToken      string           `json:"app_token"`
	TableID       string           `json:"table_id"`
	Records       []map[string]any `json:"records"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}
