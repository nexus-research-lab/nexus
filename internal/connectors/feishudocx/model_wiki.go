package feishudocx

// WikiSpace 表示飞书知识库空间元数据。
type WikiSpace struct {
	SpaceID     string `json:"space_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	SpaceType   string `json:"space_type,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
}

// WikiSpaceListResult 表示知识库空间分页列表。
type WikiSpaceListResult struct {
	Items     []WikiSpace `json:"items"`
	PageToken string      `json:"page_token,omitempty"`
	HasMore   bool        `json:"has_more"`
}

// WikiNode 表示 Wiki 节点解析后的对象元数据。
type WikiNode struct {
	SpaceID         string `json:"space_id,omitempty"`
	NodeToken       string `json:"node_token,omitempty"`
	ObjToken        string `json:"obj_token,omitempty"`
	ObjType         string `json:"obj_type,omitempty"`
	ParentNodeToken string `json:"parent_node_token,omitempty"`
	NodeType        string `json:"node_type,omitempty"`
	OriginNodeToken string `json:"origin_node_token,omitempty"`
	OriginSpaceID   string `json:"origin_space_id,omitempty"`
	HasChild        bool   `json:"has_child,omitempty"`
	Title           string `json:"title,omitempty"`
	ObjCreateTime   string `json:"obj_create_time,omitempty"`
	ObjEditTime     string `json:"obj_edit_time,omitempty"`
	NodeCreateTime  string `json:"node_create_time,omitempty"`
	Creator         string `json:"creator,omitempty"`
	Owner           string `json:"owner,omitempty"`
	NodeURL         string `json:"node_url,omitempty"`
	DocumentURL     string `json:"document_url,omitempty"`
}

// WikiNodeListResult 表示知识库节点分页列表。
type WikiNodeListResult struct {
	Items     []WikiNode `json:"items"`
	PageToken string     `json:"page_token,omitempty"`
	HasMore   bool       `json:"has_more"`
}
