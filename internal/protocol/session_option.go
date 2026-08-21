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
	// 该脱敏指纹只用于判断旧物理 session 是否需要 fork，不是 provider cache key。
	OptionRuntimeToolSurfaceFingerprint = "runtime_tool_surface_fingerprint"
	// OptionRuntimeSegmentedTranscript 表示一次 Nexus Session 跨多个非复制 SDK transcript 续写。
	OptionRuntimeSegmentedTranscript = "runtime_segmented_transcript"
	// OptionRuntimeForkSourceSessionID 表示尚待首次用户输入物化的 fork source。
	OptionRuntimeForkSourceSessionID = "runtime_fork_source_session_id"
	// OptionRuntimeForkMessageID 表示尚待首次用户输入物化的 fork transcript 边界。
	OptionRuntimeForkMessageID = "runtime_fork_message_id"
	// OptionRuntimeForkAtTranscriptTail 表示 fork 边界已经是 source transcript 尾部，runtime 可直接复制完整 source。
	OptionRuntimeForkAtTranscriptTail = "runtime_fork_at_transcript_tail"
	// OptionRuntimeRetainedTranscriptSessionIDs 表示不进入读模型、但由该 Room Session 延迟回收的 transcript。
	OptionRuntimeRetainedTranscriptSessionIDs = "runtime_retained_transcript_session_ids"
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
	// OptionSessionHiddenFromDirectory 表示宿主创建、只由精确业务入口读取的短期 Session。
	OptionSessionHiddenFromDirectory = "session_hidden_from_directory"
	// OptionSessionPurpose 表示宿主签发的短期 Session 用途；普通用户输入不能改写。
	OptionSessionPurpose = "session_purpose"
	// OptionSessionDisplayAfterUnixMilli 让 fork Session 继承模型上下文时，只在嵌入视图展示分支后的消息。
	OptionSessionDisplayAfterUnixMilli = "session_display_after_unix_milli"
)

const SessionPurposeWorkGraphEditor = "workgraph_editor"

// ScopedSessionRuntimePolicy 是宿主为精确业务 Session 签发的系统提示与完整工具覆盖。
type ScopedSessionRuntimePolicy struct {
	SystemPrompt      string
	ToolPolicy        RuntimeToolPolicy
	AllowedSkillNames []string
	DisableSkills     bool
	DisableConnectors bool
}

// SessionIsHiddenFromDirectory 判断 Session 是否只属于精确业务入口。
func SessionIsHiddenFromDirectory(session Session) bool {
	hidden, _ := session.Options[OptionSessionHiddenFromDirectory].(bool)
	return hidden
}

// SessionPurpose 返回宿主签发的 Session 用途。
func SessionPurpose(session Session) string {
	purpose, _ := session.Options[OptionSessionPurpose].(string)
	return strings.TrimSpace(purpose)
}

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

// SessionConnectorSelection 是 Session Connector 覆盖的可比较快照。
// Inherit 区分“继承 Agent 默认值”和“显式选择空集合”。
type SessionConnectorSelection struct {
	Inherit      bool
	ConnectorIDs []string
}

// SessionConnectorSelectionFromOptions 生成顺序无关、去重的 Connector 选择快照。
func SessionConnectorSelectionFromOptions(options map[string]any) SessionConnectorSelection {
	settings := SessionRuntimeSettingsFromOptions(options)
	if settings.ConnectorIDs == nil {
		return SessionConnectorSelection{Inherit: true}
	}
	seen := make(map[string]struct{}, len(*settings.ConnectorIDs))
	connectorIDs := make([]string, 0, len(*settings.ConnectorIDs))
	for _, connectorID := range *settings.ConnectorIDs {
		connectorID = strings.TrimSpace(connectorID)
		if connectorID == "" {
			continue
		}
		if _, exists := seen[connectorID]; exists {
			continue
		}
		seen[connectorID] = struct{}{}
		connectorIDs = append(connectorIDs, connectorID)
	}
	slices.Sort(connectorIDs)
	return SessionConnectorSelection{ConnectorIDs: connectorIDs}
}

// Equal 把 Connector 集合作为无序选择比较，同时保留继承语义。
func (s SessionConnectorSelection) Equal(other SessionConnectorSelection) bool {
	return s.Inherit == other.Inherit && slices.Equal(s.ConnectorIDs, other.ConnectorIDs)
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
