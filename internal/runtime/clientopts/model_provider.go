package clientopts

// RuntimeConfig 表示运行时使用的 Provider 解析结果。
type RuntimeConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
	APIFormat   string
	// UseMaxCompletionTokens 让 Chat Completions 使用现代 token 上限字段。
	UseMaxCompletionTokens bool
	Reasoning              bool
	Vision                 bool
	ContextWindow          int
	MaxOutputTokens        int
}
