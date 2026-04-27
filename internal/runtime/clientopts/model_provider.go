package clientopts

// RuntimeConfig 表示运行时使用的 Provider 解析结果。
type RuntimeConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
}
