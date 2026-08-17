package protocol

import (
	"slices"
	"strings"
)

const (
	// OptionRuntimeKind 表示创建/续用 SDK session 时使用的 runtime 类型。
	OptionRuntimeKind = "runtime_kind"
	// OptionRuntimeProvider 表示创建/续用 SDK session 时使用的 provider key。
	OptionRuntimeProvider = "runtime_provider"
	// OptionRuntimeModel 表示创建/续用 SDK session 时使用的模型。
	OptionRuntimeModel = "runtime_model"
	// OptionRuntimeToolSurfaceFingerprint 表示当前 SDK session 首次采用的模型可见工具面。
	// 该脱敏指纹只用于判断旧 K3 session 是否需要换代，不是 provider cache key。
	OptionRuntimeToolSurfaceFingerprint = "runtime_tool_surface_fingerprint"
	// OptionRuntimeSegmentedTranscript 表示一次 Nexus Session 跨多个非复制 SDK transcript 续写。
	OptionRuntimeSegmentedTranscript = "runtime_segmented_transcript"
	// OptionSessionProvider 表示当前 Nexus Session 显式覆盖的 provider。
	OptionSessionProvider = "session_provider"
	// OptionSessionModel 表示当前 Nexus Session 显式覆盖的模型。
	OptionSessionModel = "session_model"
	// OptionSessionPermissionMode 表示当前 Nexus Session 显式覆盖的权限模式。
	OptionSessionPermissionMode = "session_permission_mode"
	// OptionSessionConnectorIDs 表示当前 Nexus Session 显式挂载的 Connector。
	OptionSessionConnectorIDs = "session_connector_ids"
	// OptionSessionAdditionalDirectories 表示桌面会话显式挂载的本机目录。
	OptionSessionAdditionalDirectories = "session_additional_directories"
)

// SessionRuntimeSettings 表示当前 Nexus Session 的运行时覆盖。
//
// Provider 与 Model 必须同时为空或同时有值；空值表示继续继承 Agent / 用户默认值。
// Room 中模型归目标 Agent Session，权限由服务端同步到同一 Conversation 的全部主 Session。
type SessionRuntimeSettings struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	PermissionMode string `json:"permission_mode"`
	// ConnectorIDs 为 nil 时继承 Agent 默认值；空数组表示当前 Session 显式不挂载。
	ConnectorIDs *[]string `json:"connector_ids"`
}

// SessionLocalDirectories 表示当前 Session 额外挂载的本机目录。
type SessionLocalDirectories struct {
	Directories []string `json:"directories"`
}

// SessionRuntimeSettingsFromOptions 从 Session options 读取规范化覆盖。
func SessionRuntimeSettingsFromOptions(options map[string]any) SessionRuntimeSettings {
	if len(options) == 0 {
		return SessionRuntimeSettings{}
	}
	settings := SessionRuntimeSettings{
		Provider:       sessionOptionString(options[OptionSessionProvider]),
		Model:          sessionOptionString(options[OptionSessionModel]),
		PermissionMode: sessionOptionString(options[OptionSessionPermissionMode]),
	}
	if raw, exists := options[OptionSessionConnectorIDs]; exists {
		values := sessionOptionStringSlice(raw)
		settings.ConnectorIDs = &values
	}
	return settings
}

// WithSessionRuntimeSettings 返回应用覆盖后的 options 副本。
//
// 空覆盖会删除对应 key，避免把“继承默认值”误持久化为另一份默认配置。
func WithSessionRuntimeSettings(
	options map[string]any,
	settings SessionRuntimeSettings,
) map[string]any {
	result := make(map[string]any, len(options)+4)
	for key, value := range options {
		result[key] = value
	}
	setSessionOption(result, OptionSessionProvider, settings.Provider)
	setSessionOption(result, OptionSessionModel, settings.Model)
	setSessionOption(result, OptionSessionPermissionMode, settings.PermissionMode)
	if settings.ConnectorIDs == nil {
		delete(result, OptionSessionConnectorIDs)
	} else {
		result[OptionSessionConnectorIDs] = slices.Clone(*settings.ConnectorIDs)
	}
	return result
}

// EffectiveSessionConnectorIDs 返回 Session 覆盖或 Agent 默认 Connector 列表。
func EffectiveSessionConnectorIDs(
	agentConnectorIDs []string,
	sessionOptions map[string]any,
) []string {
	settings := SessionRuntimeSettingsFromOptions(sessionOptions)
	if settings.ConnectorIDs != nil {
		return slices.Clone(*settings.ConnectorIDs)
	}
	return slices.Clone(agentConnectorIDs)
}

// SessionAdditionalDirectoriesFromOptions 从 Session options 读取附加目录。
func SessionAdditionalDirectoriesFromOptions(options map[string]any) []string {
	return sessionOptionStringSlice(options[OptionSessionAdditionalDirectories])
}

// WithSessionAdditionalDirectories 返回应用附加目录后的 options 副本。
func WithSessionAdditionalDirectories(
	options map[string]any,
	directories []string,
) map[string]any {
	result := make(map[string]any, len(options)+1)
	for key, value := range options {
		result[key] = value
	}
	normalized := sessionOptionStringSlice(directories)
	if len(normalized) == 0 {
		delete(result, OptionSessionAdditionalDirectories)
	} else {
		result[OptionSessionAdditionalDirectories] = normalized
	}
	return result
}

func setSessionOption(options map[string]any, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		delete(options, key)
		return
	}
	options[key] = value
}

func sessionOptionString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func sessionOptionStringSlice(value any) []string {
	var values []string
	switch typed := value.(type) {
	case []string:
		values = typed
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			text, _ := item.(string)
			values = append(values, text)
		}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
