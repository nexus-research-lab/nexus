// INPUT: CC Switch 本地配置目录、当前 runtime 与用户选择。
// OUTPUT: 不含凭据的同步预览，以及同步后的 Provider/模型摘要。
// POS: Provider 服务的 CC Switch 对外协议模型，不承载本地文件解析。
package provider

const (
	ccSwitchAppClaude = "claude"
	ccSwitchAppCodex  = "codex"

	ccSwitchStatusReady       = "ready"
	ccSwitchStatusIncomplete  = "incomplete"
	ccSwitchStatusUnsupported = "unsupported"
)

// CCSwitchPreviewInput 描述 CC Switch 本地配置预览参数。
type CCSwitchPreviewInput struct {
	ConfigDir   string `json:"config_dir,omitempty"`
	RuntimeKind string `json:"-"`
}

// CCSwitchSyncInput 描述一次 CC Switch 同步请求。
type CCSwitchSyncInput struct {
	ConfigDir   string   `json:"config_dir,omitempty"`
	SourceKeys  []string `json:"source_keys"`
	SetDefault  bool     `json:"set_default,omitempty"`
	RuntimeKind string   `json:"-"`
}

// CCSwitchModelPreview 描述可安全展示的 CC Switch 模型。
type CCSwitchModelPreview struct {
	ModelID       string   `json:"model_id"`
	DisplayName   string   `json:"display_name"`
	ContextWindow *int     `json:"context_window,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
}

// CCSwitchProviderPreview 描述一个待同步的 CC Switch Provider。
type CCSwitchProviderPreview struct {
	SourceKey               string                 `json:"source_key"`
	AppType                 string                 `json:"app_type"`
	Name                    string                 `json:"name"`
	Provider                string                 `json:"provider"`
	APIFormat               string                 `json:"api_format,omitempty"`
	BaseURL                 string                 `json:"base_url,omitempty"`
	Current                 bool                   `json:"current"`
	Existing                bool                   `json:"existing"`
	CanSync                 bool                   `json:"can_sync"`
	CurrentRuntimeSupported bool                   `json:"current_runtime_supported"`
	Status                  string                 `json:"status"`
	Reason                  string                 `json:"reason,omitempty"`
	RuntimeSupport          []string               `json:"runtime_support,omitempty"`
	DefaultModel            string                 `json:"default_model,omitempty"`
	Models                  []CCSwitchModelPreview `json:"models"`
}

// CCSwitchPreview 描述 CC Switch 本地配置的可同步状态。
type CCSwitchPreview struct {
	Detected          bool                      `json:"detected"`
	ConfigDir         string                    `json:"config_dir"`
	DatabasePath      string                    `json:"database_path"`
	SchemaVersion     int                       `json:"schema_version,omitempty"`
	ProviderCount     int                       `json:"provider_count"`
	ReadyCount        int                       `json:"ready_count"`
	ModelCount        int                       `json:"model_count"`
	NeedsDefault      bool                      `json:"needs_default"`
	RecommendedSource string                    `json:"recommended_source,omitempty"`
	Providers         []CCSwitchProviderPreview `json:"providers"`
}

// CCSwitchSyncResult 描述 CC Switch 同步结果。
type CCSwitchSyncResult struct {
	Created          int             `json:"created"`
	Updated          int             `json:"updated"`
	ProviderCount    int             `json:"provider_count"`
	ModelCount       int             `json:"model_count"`
	DefaultSelection *ModelSelection `json:"default_selection,omitempty"`
}

type ccSwitchCandidate struct {
	preview   CCSwitchProviderPreview
	authToken string
}

type ccSwitchSource struct {
	configDir     string
	databasePath  string
	schemaVersion int
	candidates    []ccSwitchCandidate
}
