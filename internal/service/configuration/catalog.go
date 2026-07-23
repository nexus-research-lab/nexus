// INPUT: 配置域名称与操作名称。
// OUTPUT: 稳定的配置能力目录、确认门槛与运行时生效语义。
// POS: configuration 控制面的能力真相源。
package configuration

import (
	"fmt"
	"slices"
	"strings"
)

var domainCatalog = []DomainDefinition{
	{
		Name: DomainPreferences, Description: "用户级聊天、runtime、WebSearch 与默认 Agent 偏好",
		Source: "user preferences JSON + encrypted/isolated credential file", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("update", "合并更新偏好；WebSearch 变更会同步到活跃 nxs runtime", false, "immediate"),
		},
	},
	{
		Name: DomainProviders, Description: "LLM、图片 Provider、模型卡、默认模型与连通状态",
		Source: "database", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("create", "创建私有 Provider", false, "next_round"),
			op("update", "更新私有 Provider", false, "next_round"),
			op("delete", "删除 Provider；被 Agent 使用时需显式 force", true, "next_round"),
			op("fetch_models", "从 Provider 刷新模型卡", false, "immediate"),
			op("update_model", "更新模型卡与能力覆盖", false, "next_round"),
			op("set_default_model", "设置 Provider 默认模型", false, "next_round"),
			op("test_provider", "执行 Provider 连通性测试并记录结果", false, "immediate"),
			op("test_model", "执行模型最小请求并记录结果", false, "immediate"),
		},
	},
	{
		Name: DomainAgents, Description: "Agent 身份、模型、权限、工具、MCP server 与 Skill 选择",
		Source: "database + derived workspace settings", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("create", "创建 Agent 与独立 workspace", false, "next_session"),
			op("update", "更新 Agent 与 runtime 配置", false, "next_session"),
			op("delete", "删除非主 Agent 及其关联数据", true, "immediate"),
		},
	},
	{
		Name: DomainChannels, Description: "IM Channel、账号、路由 Agent 与配对授权",
		Source: "database with encrypted credentials", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("upsert", "创建或更新 Channel 并热重载", false, "immediate"),
			op("delete_config", "删除 Channel 配置及账号", true, "immediate"),
			op("delete_account", "删除 Channel 账号", true, "immediate"),
			op("create_pairing", "创建 IM 配对授权", false, "immediate"),
			op("update_pairing", "更新配对 Agent、名称或状态", false, "immediate"),
			op("delete_pairing", "删除配对授权", true, "immediate"),
		},
	},
	{
		Name: DomainConnectors, Description: "外部连接器连接状态、直接凭据与用户 OAuth 应用",
		Source: "database with encrypted credentials", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("connect", "使用显式凭据连接非 OAuth Connector", false, "next_session"),
			op("disconnect", "断开 Connector 并清除连接凭据", true, "next_session"),
			op("save_oauth_client", "保存用户 OAuth Client ID/Secret", false, "next_session"),
			op("delete_oauth_client", "删除 OAuth 应用并断开依赖连接", true, "next_session"),
		},
	},
	{
		Name: DomainSkills, Description: "外部 Skill 来源、更新状态与 Agent 安装选择",
		Source: "database + user skill library", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("update_source", "启用或禁用外部 Skill 来源", false, "immediate"),
			op("install", "为 Agent 安装 Skill", false, "next_session"),
			op("uninstall", "从 Agent 卸载 Skill", false, "next_session"),
			op("delete", "删除用户导入 Skill 并移除所有 Agent 引用", true, "next_session"),
			op("check_updates", "检查已导入 Skill 更新", false, "immediate"),
			op("update_imported", "更新全部已导入 Skill", false, "next_session"),
		},
	},
	{
		Name: DomainHost, Description: "主机启动参数、环境策略与桌面可持久化 runtime settings",
		Source: "environment + runtime-settings.json", ManagedBy: "nexus_config", Mutable: true,
		Operations: []OperationDefinition{
			op("update_runtime_settings", "更新桌面 workspace_path；服务重启后生效", true, "restart_required"),
		},
	},
	{
		Name: DomainAutomation, Description: "定时任务、Heartbeat、交付与运行历史",
		Source: "database + scheduler runtime", ManagedBy: "nexus_automation", Mutable: true,
	},
	{
		Name: DomainRooms, Description: "Room 结构、成员、会话与协作运行态",
		Source: "database + room runtime", ManagedBy: "nexus_room / nexus-manager", Mutable: true,
	},
	{
		Name: DomainWorkspaces, Description: "Agent workspace 文件与持久化协作资料",
		Source: "workspace filesystem", ManagedBy: "nexus-manager", Mutable: true,
	},
	{
		Name: DomainGoals, Description: "Goal、Workflow、Plan、继续执行与使用状态",
		Source: "database + goal runtime", ManagedBy: "nexus_goal", Mutable: true,
	},
}

func op(name, description string, confirm bool, effect string) OperationDefinition {
	return OperationDefinition{
		Name: name, Description: description, RequiresConfirmation: confirm, RuntimeEffect: effect,
	}
}

// Definitions 返回配置域目录副本。
func Definitions() []DomainDefinition {
	definitions := slices.Clone(domainCatalog)
	for index := range definitions {
		definitions[index] = hydrateDefinition(definitions[index])
	}
	return definitions
}

func definitionFor(domain string) (DomainDefinition, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, definition := range domainCatalog {
		if definition.Name == domain {
			return hydrateDefinition(definition), nil
		}
	}
	return DomainDefinition{}, fmt.Errorf("未知配置域 %q；可用域: %s", domain, strings.Join(domainNames(), ", "))
}

func hydrateDefinition(definition DomainDefinition) DomainDefinition {
	operations := slices.Clone(definition.Operations)
	for index := range operations {
		target, shape, required := operationContract(definition.Name, operations[index].Name)
		operations[index].TargetDescription = target
		operations[index].InputShape = shape
		operations[index].RequiredInputFields = required
	}
	definition.Operations = operations
	return definition
}

func operationContract(domain, operation string) (string, any, []string) {
	switch domain + "." + operation {
	case DomainPreferences + ".update":
		return "", map[string]any{
			"chat_default_delivery_policy":       "queue|interrupt|reject",
			"agent_runtime_kind":                 "string",
			"agent_sdk_diagnostics_enabled":      "boolean",
			"runtime_settings":                   "object keyed by runtime kind",
			"web_search":                         "WebSearchSettings object",
			"web_search_api_key":                 "secret string; never repeat",
			"default_agent_options":              agentOptionsShape(),
			"default_image_model_selection":      modelSelectionShape(),
			"default_vision_model_selection":     modelSelectionShape(),
			"default_background_model_selection": modelSelectionShape(),
		}, nil
	case DomainProviders + ".create":
		return "", map[string]any{
			"provider": "unique provider key", "provider_kind": "llm|image_generation",
			"preset_key": "preset key or custom", "api_format": "provider API format",
			"display_name": "string", "auth_token": "secret string; never repeat",
			"base_url": "URL", "models_path": "string", "enabled": "boolean",
		}, []string{"provider", "auth_token"}
	case DomainProviders + ".update":
		return "provider key", map[string]any{
			"provider_kind": "llm|image_generation", "preset_key": "string",
			"api_format": "string", "display_name": "string",
			"auth_token": "optional replacement secret", "base_url": "URL",
			"models_path": "string", "enabled": "boolean",
		}, nil
	case DomainProviders + ".delete":
		return "provider key", map[string]any{"force": "boolean; reassign referenced Agents when true"}, nil
	case DomainProviders + ".fetch_models", DomainProviders + ".test_provider":
		return "provider key", map[string]any{}, nil
	case DomainProviders + ".update_model":
		return "provider key", map[string]any{
			"model_id": "string",
			"input": map[string]any{
				"enabled": "boolean", "is_default": "boolean",
				"capabilities_override": "object with optional vision/image_output/tool_calling/reasoning/embedding booleans",
				"context_window":        "optional integer", "max_output_tokens": "optional integer",
				"provider_options": "object",
			},
		}, []string{"model_id", "input"}
	case DomainProviders + ".set_default_model", DomainProviders + ".test_model":
		return "provider key", map[string]any{"model_id": "string"}, []string{"model_id"}
	case DomainAgents + ".create":
		return "", map[string]any{
			"name": "string", "options": agentOptionsShape(), "avatar": "string",
			"description": "string", "vibe_tags": "string[]",
		}, []string{"name"}
	case DomainAgents + ".update":
		return "agent_id", map[string]any{
			"name": "optional string", "options": agentOptionsShape(), "avatar": "optional string",
			"description": "optional string", "vibe_tags": "string[]",
		}, nil
	case DomainAgents + ".delete":
		return "non-main agent_id", map[string]any{}, nil
	case DomainChannels + ".upsert":
		return "channel_type", map[string]any{
			"agent_id": "routing agent_id", "config": "public string map",
			"credentials": "secret string map; never repeat",
		}, []string{"agent_id"}
	case DomainChannels + ".delete_config":
		return "channel_type", map[string]any{}, nil
	case DomainChannels + ".delete_account":
		return "channel_type", map[string]any{"account_id": "string"}, []string{"account_id"}
	case DomainChannels + ".create_pairing":
		return "", map[string]any{
			"channel_type": "string", "account_id": "optional string",
			"chat_type": "dm|group", "external_ref": "external user/group id",
			"thread_id": "optional string", "external_name": "optional string",
			"agent_id": "routing agent_id", "status": "pending|active|disabled|rejected",
			"source": "manual|ingress|wechat_qr",
		}, []string{"channel_type", "chat_type", "external_ref", "agent_id"}
	case DomainChannels + ".update_pairing":
		return "pairing_id", map[string]any{
			"agent_id": "optional string", "status": "optional pending|active|disabled|rejected",
			"external_name": "optional string",
		}, nil
	case DomainChannels + ".delete_pairing":
		return "pairing_id", map[string]any{}, nil
	case DomainConnectors + ".connect":
		return "connector_id", map[string]any{
			"credentials": "secret string map matching connector auth_type; never repeat",
		}, []string{"credentials"}
	case DomainConnectors + ".disconnect", DomainConnectors + ".delete_oauth_client":
		return "connector_id", map[string]any{}, nil
	case DomainConnectors + ".save_oauth_client":
		return "connector_id", map[string]any{
			"client_id": "string", "client_secret": "secret string; never repeat",
		}, []string{"client_id", "client_secret"}
	case DomainSkills + ".update_source":
		return "source_id from inspect", map[string]any{"enabled": "boolean"}, []string{"enabled"}
	case DomainSkills + ".install", DomainSkills + ".uninstall":
		return "skill name", map[string]any{"agent_id": "string"}, []string{"agent_id"}
	case DomainSkills + ".delete":
		return "imported skill name", map[string]any{}, nil
	case DomainSkills + ".check_updates", DomainSkills + ".update_imported":
		return "", map[string]any{}, nil
	case DomainHost + ".update_runtime_settings":
		return "", map[string]any{"workspace_path": "absolute or home-relative directory path; empty restores deployment default"}, nil
	default:
		return "", nil, nil
	}
}

func agentOptionsShape() map[string]any {
	return map[string]any{
		"provider": "string", "model": "string", "permission_mode": "default|acceptEdits|bypassPermissions|plan",
		"allowed_tools": "string[]", "disallowed_tools": "string[]",
		"max_turns": "optional integer", "max_thinking_tokens": "optional integer",
		"mcp_servers": "object; secrets remain legacy storage and are always redacted on reads",
		"skill_ids":   "string[]", "setting_sources": "string[]",
	}
}

func modelSelectionShape() map[string]any {
	return map[string]any{"provider": "string", "model": "string"}
}

func operationFor(definition DomainDefinition, operation string) (OperationDefinition, error) {
	operation = strings.ToLower(strings.TrimSpace(operation))
	for _, candidate := range definition.Operations {
		if candidate.Name == operation {
			return candidate, nil
		}
	}
	if definition.ManagedBy != "nexus_config" {
		return OperationDefinition{}, fmt.Errorf("%s 由 %s 管理，请使用对应对话工具", definition.Name, definition.ManagedBy)
	}
	return OperationDefinition{}, fmt.Errorf("配置域 %s 不支持操作 %q", definition.Name, operation)
}

func domainNames() []string {
	names := make([]string, 0, len(domainCatalog))
	for _, definition := range domainCatalog {
		names = append(names, definition.Name)
	}
	return names
}
