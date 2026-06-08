package feishudocx

// SearchResult 表示飞书云文档全文搜索结果。
type SearchResult struct {
	Query     string         `json:"query"`
	Offset    int            `json:"offset"`
	Count     int            `json:"count"`
	DocsTypes []string       `json:"docs_types,omitempty"`
	OwnerIDs  []string       `json:"owner_ids,omitempty"`
	ChatIDs   []string       `json:"chat_ids,omitempty"`
	Data      map[string]any `json:"data"`
}
