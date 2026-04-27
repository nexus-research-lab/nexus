package agent

import (
	"encoding/json"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

// BuildCreateRecord 构建落库记录。
func BuildCreateRecord(
	cfg config.Config,
	request protocol.CreateRequest,
	ownerUserID string,
	normalizedName string,
	agentID string,
	workspacePath string,
	status string,
	isMain bool,
) agentrepo.CreateRecord {
	options := protocol.Options{}
	if isMain {
		options = defaultMainAgentOptions()
	}
	if request.Options != nil {
		options = mergeOptions(options, *request.Options)
	}

	return agentrepo.CreateRecord{
		AgentID:             agentID,
		OwnerUserID:         ownerUserID,
		Slug:                BuildWorkspaceDirName(normalizedName),
		Name:                normalizedName,
		WorkspacePath:       workspacePath,
		Status:              status,
		IsMain:              isMain,
		Avatar:              request.Avatar,
		Description:         request.Description,
		VibeTagsJSON:        mustJSONString(request.VibeTags, "[]"),
		DisplayName:         normalizedName,
		Headline:            "",
		ProfileMarkdown:     "",
		RuntimeID:           buildStableID("runtime", agentID),
		ProfileID:           buildStableID("profile", agentID),
		Provider:            options.Provider,
		PermissionMode:      options.PermissionMode,
		AllowedToolsJSON:    mustJSONString(options.AllowedTools, "[]"),
		DisallowedToolsJSON: mustJSONString(options.DisallowedTools, "[]"),
		MCPServersJSON:      mustJSONString(options.MCPServers, "{}"),
		MaxTurns:            options.MaxTurns,
		MaxThinkingTokens:   options.MaxThinkingTokens,
		SettingSourcesJSON:  mustJSONString(options.SettingSources, "[]"),
		RuntimeVersion:      1,
	}
}

// BuildDefaultMainAgentRecord 构建主智能体默认记录。
func BuildDefaultMainAgentRecord(cfg config.Config, ownerUserID string) agentrepo.CreateRecord {
	name := cfg.DefaultAgentID
	agentID := cfg.DefaultAgentID
	if strings.TrimSpace(ownerUserID) != systemOwnerUserID {
		agentID = buildStableID("main_agent", ownerUserID)
	}
	return BuildCreateRecord(
		cfg,
		protocol.CreateRequest{Name: name, Options: pointer(defaultMainAgentOptions())},
		ownerUserID,
		name,
		agentID,
		ResolveWorkspacePath(cfg, ownerUserID, name),
		"active",
		true,
	)
}

func defaultMainAgentOptions() protocol.Options {
	return protocol.Options{
		AllowedTools:   []string{"AskUserQuestion", "Bash", "Edit", "Glob", "Grep", "LS", "Read", "Skill", "TodoWrite", "WebFetch", "WebSearch", "Write"},
		PermissionMode: "default",
		SettingSources: []string{"project"},
	}
}

func pointer(value protocol.Options) *protocol.Options {
	return &value
}

func mergeOptions(base protocol.Options, incoming protocol.Options) protocol.Options {
	result := base
	// 当前 Web 主流程会显式提交 provider 字段；
	// 这里按完整快照语义处理，空字符串表示“跟随默认 Provider”。
	result.Provider = strings.TrimSpace(incoming.Provider)
	if incoming.PermissionMode != "" {
		result.PermissionMode = incoming.PermissionMode
	}
	if incoming.AllowedTools != nil {
		result.AllowedTools = incoming.AllowedTools
	}
	if incoming.DisallowedTools != nil {
		result.DisallowedTools = incoming.DisallowedTools
	}
	if incoming.MaxTurns != nil {
		result.MaxTurns = incoming.MaxTurns
	}
	if incoming.MaxThinkingTokens != nil {
		result.MaxThinkingTokens = incoming.MaxThinkingTokens
	}
	if incoming.MCPServers != nil {
		result.MCPServers = incoming.MCPServers
	}
	if incoming.SettingSources != nil {
		result.SettingSources = incoming.SettingSources
	}
	return result
}

func mustJSONString(value any, fallback string) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(payload)
}
