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
		Source: "user preferences JSON + encrypted/isolated credential file", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("update", "合并更新偏好；WebSearch 变更会同步到活跃 nxs runtime", false, "immediate"),
		},
	},
	{
		Name: DomainProviders, Description: "LLM、图片 Provider、模型卡、默认模型与连通状态",
		Source: "database", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("create", "创建私有 Provider", true, "next_round"),
			op("update", "更新私有 Provider", true, "next_round"),
			op("delete", "删除 Provider；被 Agent 使用时需显式 force", true, "next_round"),
			op("fetch_models", "从 Provider 刷新模型卡", true, "immediate"),
			op("update_model", "更新模型卡与能力覆盖", true, "next_round"),
			op("set_default_model", "设置 Provider 默认模型", true, "next_round"),
			op("test_provider", "执行 Provider 连通性测试并记录结果", true, "immediate"),
			op("test_model", "执行模型最小请求并记录结果", true, "immediate"),
		},
	},
	{
		Name: DomainAgents, Description: "Agent 身份、模型、权限、工具、MCP server 与 Skill 选择",
		Source: "database + derived workspace settings", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("create", "创建 Agent 与独立 workspace", true, "next_round"),
			op("update", "主智能体更新任意自有 Agent；权限模式立即同步，其他 runtime 配置下一轮重载", true, "mixed"),
			op("delete", "删除非主 Agent 及其关联数据", true, "immediate"),
			op("update_self_profile", "普通 Agent 只在自己的 DM 更新公开身份资料", true, "ui_immediate_runtime_next_round"),
			op("update_self_runtime", "普通 Agent 在 owner 边界内选择模型并调整运行上限", true, "next_round"),
		},
	},
	{
		Name: DomainEmotion, Description: "当前 Agent 自有的基础情绪与可信 DM/Room 上下文情绪",
		Source: "versioned .agents/emotion.json", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("set_base", "在当前 Agent 私有 DM 中更新基础情绪；fatigue 保持 runtime 只读", false, "next_round"),
			op("set_context", "更新当前可信 DM 或 Room conversation 的上下文情绪", false, "next_round"),
			op("clear_context", "清除当前可信 DM 或 Room conversation 的上下文情绪", false, "next_round"),
		},
	},
	{
		Name: DomainChannels, Description: "IM Channel、账号、路由 Agent 与配对授权",
		Source: "database with encrypted credentials", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("upsert", "创建或更新 Channel 并热重载", true, "immediate"),
			op("delete_config", "删除 Channel 配置及账号", true, "immediate"),
			op("delete_account", "删除 Channel 账号", true, "immediate"),
			op("create_pairing", "创建 IM 配对授权", true, "next_ingress"),
			op("update_pairing", "更新配对 Agent、名称或状态", true, "next_ingress"),
			op("delete_pairing", "删除配对授权", true, "next_ingress"),
		},
	},
	{
		Name: DomainConnectors, Description: "外部连接器连接状态、直接凭据与用户 OAuth 应用",
		Source: "database with encrypted credentials", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("connect", "使用显式凭据连接非 OAuth Connector", true, "next_session"),
			op("disconnect", "断开 Connector 并清除连接凭据", true, "next_session"),
			op("save_oauth_client", "保存用户 OAuth Client ID/Secret", true, "next_session"),
			op("delete_oauth_client", "删除 OAuth 应用并断开依赖连接", true, "next_session"),
		},
	},
	{
		Name: DomainSkills, Description: "外部 Skill 来源、更新状态与 Agent 安装选择",
		Source: "database + user skill library", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("search_external", "搜索已启用的远端 Skill 来源；不修改目录", false, "immediate"),
			op("preview_external", "读取远端 Skill 详情预览；不修改目录", false, "immediate"),
			op("create_private_source", "新增并验证 owner 私有 Skill 来源；Bearer Token 只通过 Settings 或人工 CLI secret slot 输入", true, "immediate"),
			op("import_git", "从 HTTPS Git 仓库的受控子目录导入 Skill", true, "next_round"),
			op("import_url", "从受校验 HTTPS URL 导入 SKILL.md 或 zip", true, "next_round"),
			op("import_skills_sh", "从 skills.sh 标识导入 Skill", true, "next_round"),
			op("import_private", "用可信 source_id 和 skill_id 从私有来源导入 Skill", true, "next_round"),
			op("update_source", "更新系统来源开关，或私有来源名称、开关与认证", true, "immediate"),
			op("delete_private_source", "删除 owner 私有 Skill 来源；已导入 Skill 保留", true, "immediate"),
			op("install", "为 Agent 安装 Skill", true, "next_round"),
			op("uninstall", "从 Agent 非破坏性停用 Skill", true, "next_round"),
			op("install_self", "普通 Agent 为自己安装 owner 可见目录中的 Skill", true, "next_round"),
			op("uninstall_self", "普通 Agent 从自己非破坏性停用 Skill", true, "next_round"),
			op("delete", "删除用户导入 Skill 并移除所有 Agent 引用", true, "next_round"),
			op("check_updates", "检查已导入 Skill 更新；只刷新健康元数据", false, "immediate"),
			op("update_single", "以目录版本 CAS 更新一个远端 Skill", true, "next_round"),
			op("update_all", "从一个目录版本开始串行更新全部远端 Skill；部分完成必须 reconcile", true, "next_round"),
		},
	},
	{
		Name: DomainHost, Description: "主机启动参数、环境策略与桌面状态根",
		Source: "deployment environment + native desktop host", ManagedBy: "deployment/native desktop control plane", Mutable: false,
	},
	{
		Name: DomainAutomation, Description: "定时任务、后台执行、交付与运行历史",
		Source: "database + scheduler runtime", ManagedBy: "automation Skill + nexus.command", Mutable: true,
	},
	{
		Name: DomainSessions, Description: "Owner workspace 中的 Agent 对话目录与安全生命周期",
		Source: "versioned Agent workspace session meta + runtime manager", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("update_title", "更新普通 Agent session 标题；Room conversation 标题仍由 rooms 域管理", true, "ui_immediate"),
			op("delete", "先安全关闭目标 runtime，再以版本 CAS 删除普通 Agent session 与 transcript", true, "immediate"),
		},
	},
	{
		Name: DomainRooms, Description: "Room 共享资料、成员、群主与协作策略",
		Source: "database + room runtime", ManagedBy: "nexuscfg", Mutable: true,
		Operations: []OperationDefinition{
			op("create", "创建 Room、初始 conversation、成员与群主", true, "ui_immediate_runtime_next_input"),
			op("update_profile", "更新 Room 名称、描述与头像", true, "ui_immediate_runtime_next_round"),
			op("set_collaboration_policy", "更新 Room Skill、群主自动接管和私聊协作策略", true, "security_immediate_runtime_next_round"),
			op("add_member", "向当前 Room 添加一个普通 Agent", true, "immediate"),
			op("remove_member", "立即撤销成员权限并中断其活跃 Room 任务", true, "immediate"),
			op("set_member_participation", "暂停或恢复成员参与；暂停会立即封锁调度和旧输出", true, "immediate"),
			op("transfer_host", "把群主转让给当前 Room 的现有 Agent 成员", true, "authority_immediate_routing_next_input"),
			op("create_conversation", "在 Room 中创建新的 draft conversation", true, "ui_immediate_runtime_next_input"),
			op("update_conversation", "更新 Room 内指定 conversation 标题", true, "ui_immediate_runtime_next_round"),
			op("delete_conversation", "删除指定 conversation 并关闭运行态、清理资料与 Goal", true, "immediate"),
			op("delete", "删除 Room、全部对话、运行态、共享资料与关联 Goal", true, "immediate"),
		},
	},
	{
		Name: DomainWorkspaces, Description: "Agent workspace 文件与持久化协作资料",
		Source: "workspace filesystem", ManagedBy: "nexusctl / native runtime file tools", Mutable: true,
	},
	{
		Name: DomainGoals, Description: "持久 Goal objective、生命周期、继续执行与使用状态",
		Source: "database + goal runtime", ManagedBy: "goal-manager Skill + nexus.command", Mutable: true,
	},
	{
		Name: DomainExecutions, Description: "Execution、Plan、WorkGraph、Assignment、验收与恢复状态",
		Source: "database + execution runtime", ManagedBy: "execution-orchestrator Skill + nexus.command", Mutable: true,
	},
}

func op(name, description string, confirm bool, effect string) OperationDefinition {
	return OperationDefinition{
		Name: name, Description: description, RequiresConfirmation: confirm, RuntimeEffect: effect,
	}
}

func secretSlotShape() map[string]string {
	return map[string]string{
		"$secret": "8-64 character opaque slot ID; the human enters its value only in the Nexus approval card",
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
			"web_search_api_key":                 secretSlotShape(),
			"default_agent_options":              agentOptionsShape(),
			"default_image_model_selection":      modelSelectionShape(),
			"default_vision_model_selection":     modelSelectionShape(),
			"default_background_model_selection": modelSelectionShape(),
		}, nil
	case DomainProviders + ".create":
		return "", map[string]any{
			"provider": "unique provider key", "provider_kind": "llm|image_generation",
			"preset_key": "preset key or custom", "api_format": "provider API format",
			"display_name": "string", "auth_token": secretSlotShape(),
			"base_url": "URL", "models_path": "string", "enabled": "boolean",
		}, []string{"provider", "auth_token"}
	case DomainProviders + ".update":
		return "provider key", map[string]any{
			"provider_kind": "llm|image_generation", "preset_key": "string",
			"api_format": "string", "display_name": "string",
			"auth_token": secretSlotShape(), "base_url": "URL",
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
				"provider_options": "mixed JSON object; keep structural string/number/boolean/array/object values literal; token/client_secret/api_key/password leaves and every headers/env/credentials value use {$secret: opaque_slot_id}",
			},
		}, []string{"model_id", "input"}
	case DomainProviders + ".set_default_model", DomainProviders + ".test_model":
		return "provider key", map[string]any{"model_id": "string"}, []string{"model_id"}
	case DomainAgents + ".create":
		return "", map[string]any{
			"name": "string", "options": agentOptionsShape(), "avatar": "string",
			"description": "string", "profile_template": "optional initial identity markdown",
			"vibe_tags": "string[]",
		}, []string{"name"}
	case DomainAgents + ".update":
		return "agent_id", map[string]any{
			"name": "optional string", "options": agentOptionsShape(), "avatar": "optional string",
			"description": "optional string", "vibe_tags": "string[]",
		}, nil
	case DomainAgents + ".update_self_profile":
		return "omitted or current agent_id only", map[string]any{
			"name": "optional string", "avatar": "optional string",
			"description": "optional string", "vibe_tags": "optional string[]",
		}, nil
	case DomainAgents + ".update_self_runtime":
		return "omitted or current agent_id only", map[string]any{
			"provider": "enabled provider key", "model": "enabled model id",
			"max_turns": "optional integer", "max_thinking_tokens": "optional integer",
		}, nil
	case DomainAgents + ".delete":
		return "non-main agent_id", map[string]any{}, nil
	case DomainEmotion + ".set_base":
		return "omitted; always current agent", map[string]any{
			"mood": "string", "energy": "integer 0-10",
			"valence": "integer 0-10", "description": "string",
		}, []string{"mood", "energy", "valence", "description"}
	case DomainEmotion + ".set_context":
		return "omitted; current DM/Room context is fixed by runtime", map[string]any{
			"mood": "string", "valence": "integer 0-10", "trigger": "string",
		}, []string{"mood", "valence", "trigger"}
	case DomainEmotion + ".clear_context":
		return "omitted; current DM/Room context is fixed by runtime", map[string]any{}, nil
	case DomainChannels + ".upsert":
		return "channel_type", map[string]any{
			"agent_id": "routing agent_id", "config": "public string map",
			"credentials": "map whose values are {$secret: opaque_slot_id}",
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
			"credentials": "map matching connector auth_type; every value is {$secret: opaque_slot_id}",
		}, []string{"credentials"}
	case DomainConnectors + ".disconnect", DomainConnectors + ".delete_oauth_client":
		return "connector_id", map[string]any{}, nil
	case DomainConnectors + ".save_oauth_client":
		return "connector_id", map[string]any{
			"client_id": "string", "client_secret": secretSlotShape(),
		}, []string{"client_id", "client_secret"}
	case DomainSkills + ".update_source":
		return "source_id from inspect", map[string]any{
			"name":      "private source only; optional non-empty string",
			"enabled":   "optional boolean",
			"auth_type": "private source only; optional none|bearer",
			"token":     secretSlotShape(),
		}, nil
	case DomainSkills + ".create_private_source":
		return "omitted; Nexus derives source_id from the validated URL", map[string]any{
			"name":      "non-empty display name",
			"url":       "private registry HTTPS URL",
			"auth_type": "none|bearer",
			"token":     secretSlotShape(),
		}, []string{"name", "url", "auth_type"}
	case DomainSkills + ".delete_private_source":
		return "deletable private source_id from inspect", map[string]any{}, nil
	case DomainSkills + ".import_private":
		return "private source_id from inspect", map[string]any{
			"skill_id": "exact opaque skill_id returned by private source search",
		}, []string{"skill_id"}
	case DomainSkills + ".search_external":
		return "", map[string]any{
			"query":          "non-empty search query",
			"include_readme": "optional boolean",
		}, []string{"query"}
	case DomainSkills + ".preview_external":
		return "", map[string]any{"detail_url": "HTTPS detail URL returned by search"}, []string{"detail_url"}
	case DomainSkills + ".import_git":
		return "", map[string]any{
			"repository_url": "HTTPS Git repository URL",
			"branch":         "optional branch",
			"skill_path":     "optional relative subdirectory",
		}, []string{"repository_url"}
	case DomainSkills + ".import_url":
		return "", map[string]any{"source_url": "validated HTTPS SKILL.md or zip URL"}, []string{"source_url"}
	case DomainSkills + ".import_skills_sh":
		return "", map[string]any{
			"package_spec": "skills.sh package/repository identifier",
			"skill_slug":   "skill slug",
		}, []string{"package_spec", "skill_slug"}
	case DomainSkills + ".install", DomainSkills + ".uninstall":
		return "skill name", map[string]any{
				"agent_id":        "string",
				"target_scope":    "global_library|agent_workspace",
				"source_identity": "exact source_identity returned by inspect",
			},
			[]string{"agent_id", "target_scope", "source_identity"}
	case DomainSkills + ".install_self", DomainSkills + ".uninstall_self":
		return "skill name", map[string]any{
				"target_scope":    "global_library|agent_workspace",
				"source_identity": "exact source_identity returned by inspect",
			},
			[]string{"target_scope", "source_identity"}
	case DomainSkills + ".delete", DomainSkills + ".update_single":
		return "imported skill name", map[string]any{}, nil
	case DomainSkills + ".check_updates", DomainSkills + ".update_all":
		return "", map[string]any{}, nil
	case DomainSessions + ".update_title":
		return "owner-main: exact owner-scoped Agent session_key; agent-self: omitted/current trusted DM only",
			map[string]any{"title": "string; blank resets to New Chat"},
			[]string{"title"}
	case DomainSessions + ".delete":
		return "owner-main only: exact owner-scoped Agent session_key; current executing session cannot delete itself",
			map[string]any{},
			nil
	case DomainRooms + ".create":
		return "omitted; generated by Nexus", map[string]any{
			"agent_ids": "non-empty agent_id[]", "name": "optional string",
			"description": "optional string", "title": "optional initial conversation title",
			"avatar": "optional string", "skill_names": "optional skill name[]",
			"host_agent_id":            "optional member agent_id",
			"host_auto_reply_enabled":  "boolean",
			"private_messages_enabled": "boolean",
		}, []string{"agent_ids"}
	case DomainRooms + ".update_profile":
		return "owner-main: room_id; Room host: omitted/current room only", map[string]any{
			"name": "optional string", "description": "optional string",
			"avatar": "optional string",
		}, nil
	case DomainRooms + ".set_collaboration_policy":
		return "owner-main: room_id; Room host: omitted/current room only", map[string]any{
			"skill_names":              "optional string[]",
			"host_auto_reply_enabled":  "optional boolean",
			"private_messages_enabled": "optional boolean",
		}, nil
	case DomainRooms + ".add_member", DomainRooms + ".remove_member", DomainRooms + ".transfer_host":
		return "owner-main: room_id; Room host: omitted/current room only",
			map[string]any{"agent_id": "existing owner-scoped Agent id"}, []string{"agent_id"}
	case DomainRooms + ".set_member_participation":
		return "owner-main: room_id; Room host: omitted/current room only", map[string]any{
			"agent_id": "current Room member agent_id",
			"paused":   "boolean",
		}, []string{"agent_id", "paused"}
	case DomainRooms + ".create_conversation":
		return "owner-main: room_id; Room host: omitted/current room only",
			map[string]any{"title": "optional string"}, nil
	case DomainRooms + ".update_conversation":
		return "owner-main: room_id; Room host: omitted/current room only", map[string]any{
			"conversation_id": "conversation in target Room", "title": "non-empty string",
		}, []string{"conversation_id", "title"}
	case DomainRooms + ".delete_conversation":
		return "owner-main: room_id; Room host: omitted/current room only",
			map[string]any{"conversation_id": "conversation in target Room"},
			[]string{"conversation_id"}
	case DomainRooms + ".delete":
		return "owner-main only: exact room_id", map[string]any{}, nil
	default:
		return "", nil, nil
	}
}

func agentOptionsShape() map[string]any {
	return map[string]any{
		"provider": "string", "model": "string", "permission_mode": "default|acceptEdits|bypassPermissions|plan",
		"allowed_tools": "string[]", "disallowed_tools": "string[]",
		"connector_ids": "string[]; connected marketplace Connectors enabled by default for the Agent",
		"max_turns":     "optional integer", "max_thinking_tokens": "optional integer",
		"mcp_servers": map[string]any{
			"<server_name>": map[string]any{
				"type":          "stdio|http|sse string",
				"command":       "stdio command string",
				"args":          "stdio string[]",
				"env":           "stdio object; every value is {$secret: opaque_slot_id}",
				"url":           "http/sse URL string without embedded credentials",
				"headers":       "http/sse object; every value is {$secret: opaque_slot_id}",
				"headersHelper": "optional path string",
				"oauth": map[string]any{
					"clientId": "public client ID string", "callbackPort": "integer",
					"authServerMetadataUrl": "optional HTTPS URL", "xaa": "optional boolean",
				},
			},
			"_read_projection": "the stored mcp_servers object remains whole-object redacted on inspect/history",
		},
		"setting_sources": "string[]; change Skill selection through the scoped skills operations",
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
	if definition.ManagedBy != "nexuscfg" {
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
