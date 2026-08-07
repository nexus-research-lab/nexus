package protocol

import "strings"

const (
	// OptionRuntimeKind 表示创建/续用 SDK session 时使用的 runtime 类型。
	OptionRuntimeKind = "runtime_kind"
	// OptionRuntimeProvider 表示创建/续用 SDK session 时使用的 provider key。
	OptionRuntimeProvider = "runtime_provider"
	// OptionRuntimeModel 表示创建/续用 SDK session 时使用的模型。
	OptionRuntimeModel = "runtime_model"
	// OptionSessionProvider 表示当前 Nexus Session 显式覆盖的 provider。
	OptionSessionProvider = "session_provider"
	// OptionSessionModel 表示当前 Nexus Session 显式覆盖的模型。
	OptionSessionModel = "session_model"
	// OptionSessionPermissionMode 表示当前 Nexus Session 显式覆盖的权限模式。
	OptionSessionPermissionMode = "session_permission_mode"
)

// SessionRuntimeSettings 表示当前 Nexus Session 的运行时覆盖。
//
// Provider 与 Model 必须同时为空或同时有值；空值表示继续继承 Agent / 用户默认值。
// Room 中模型归目标 Agent Session，权限由服务端同步到同一 Conversation 的全部主 Session。
type SessionRuntimeSettings struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	PermissionMode string `json:"permission_mode"`
}

// SessionRuntimeSettingsFromOptions 从 Session options 读取规范化覆盖。
func SessionRuntimeSettingsFromOptions(options map[string]any) SessionRuntimeSettings {
	if len(options) == 0 {
		return SessionRuntimeSettings{}
	}
	return SessionRuntimeSettings{
		Provider:       sessionOptionString(options[OptionSessionProvider]),
		Model:          sessionOptionString(options[OptionSessionModel]),
		PermissionMode: sessionOptionString(options[OptionSessionPermissionMode]),
	}
}

// WithSessionRuntimeSettings 返回应用覆盖后的 options 副本。
//
// 空覆盖会删除对应 key，避免把“继承默认值”误持久化为另一份默认配置。
func WithSessionRuntimeSettings(
	options map[string]any,
	settings SessionRuntimeSettings,
) map[string]any {
	result := make(map[string]any, len(options)+3)
	for key, value := range options {
		result[key] = value
	}
	setSessionOption(result, OptionSessionProvider, settings.Provider)
	setSessionOption(result, OptionSessionModel, settings.Model)
	setSessionOption(result, OptionSessionPermissionMode, settings.PermissionMode)
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
