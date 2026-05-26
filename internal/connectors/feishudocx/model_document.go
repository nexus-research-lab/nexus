package feishudocx

import "time"

const (
	defaultAPIBaseURL = "https://open.feishu.cn"
	defaultDocBaseURL = "https://feishu.cn"
)

// Document 表示飞书 Docx 文档元数据。
type Document struct {
	DocumentID string `json:"document_id"`
	RevisionID int64  `json:"revision_id,omitempty"`
	Title      string `json:"title,omitempty"`
}

// Block 是飞书文档 Block 的开放 JSON 形态。
type Block map[string]any

// DocumentTarget 是从 URL 或 token 里解析出的文档目标。
type DocumentTarget struct {
	DocumentID string `json:"document_id,omitempty"`
	WikiToken  string `json:"wiki_token,omitempty"`
	SourceType string `json:"source_type"`
	Raw        string `json:"raw"`
}

// ConvertResult 表示 Markdown 转 Block 的结果。
type ConvertResult struct {
	FirstLevelBlockIDs []string `json:"first_level_block_ids"`
	Blocks             []Block  `json:"blocks"`
}

// AppendResult 表示写入 Block 的结果。
type AppendResult struct {
	Children           []Block          `json:"children,omitempty"`
	BlockIDRelations   []map[string]any `json:"block_id_relations,omitempty"`
	DocumentRevisionID int64            `json:"document_revision_id,omitempty"`
	CreatedBlocks      int              `json:"created_blocks"`
}

// ExportMarkdownResult 表示文档导出为 Markdown 的结果。
type ExportMarkdownResult struct {
	DocumentID        string         `json:"document_id"`
	Title             string         `json:"title,omitempty"`
	SourceType        string         `json:"source_type"`
	Markdown          string         `json:"markdown"`
	BlockCount        int            `json:"block_count"`
	UnsupportedBlocks map[string]int `json:"unsupported_blocks,omitempty"`
}

// CreateDocumentResult 表示创建文档的结果。
type CreateDocumentResult struct {
	DocumentID    string `json:"document_id"`
	URL           string `json:"url"`
	Title         string `json:"title,omitempty"`
	CreatedBlocks int    `json:"created_blocks,omitempty"`
}

// DriveListResult 表示云空间文件列表。
type DriveListResult struct {
	Files         []map[string]any `json:"files"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
}

// Clock 便于后续测试注入时间；当前用于保持包内时间依赖集中。
type Clock func() time.Time
