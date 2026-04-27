package connectors

// ConnectionSnapshot 是 Agent MCP 调用 connector REST API 所需的连接快照。
type ConnectionSnapshot struct {
	ConnectorID string            `json:"connector_id"`
	AuthType    string            `json:"auth_type"`
	APIBaseURL  string            `json:"api_base_url"`
	AccessToken string            `json:"-"`
	ShopDomain  string            `json:"shop_domain,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}
