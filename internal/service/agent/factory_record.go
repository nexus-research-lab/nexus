package agent

import (
	"encoding/json"
	"strconv"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

const (
	defaultMainAgentAvatar = "nexus"
	// 与 web/src/lib/avatar.ts 的 Agent 图标资源范围保持一致。
	agentAvatarIconStart = 1
	agentAvatarIconEnd   = 53
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
	options := defaultAgentOptions(isMain)
	if request.Options != nil {
		options = mergeOptions(options, *request.Options)
	}
	options.SkillIDs, options.DisabledSkillIDs = runtimecommand.BindManagedSemanticSkills(
		options.SkillIDs,
		options.DisabledSkillIDs,
	)

	return agentrepo.CreateRecord{
		AgentID:              agentID,
		OwnerUserID:          ownerUserID,
		Slug:                 BuildWorkspaceDirName(agentID),
		Name:                 normalizedName,
		WorkspacePath:        workspacePath,
		Status:               status,
		IsMain:               isMain,
		Avatar:               resolveAgentAvatar(request.Avatar, agentID, isMain),
		Description:          request.Description,
		VibeTagsJSON:         mustJSONString(request.VibeTags),
		DisplayName:          normalizedName,
		Headline:             "",
		ProfileMarkdown:      "",
		RuntimeID:            buildStableID("runtime", agentID),
		ProfileID:            buildStableID("profile", agentID),
		Provider:             options.Provider,
		Model:                options.Model,
		PermissionMode:       options.PermissionMode,
		AllowedToolsJSON:     mustJSONString(options.AllowedTools),
		DisallowedToolsJSON:  mustJSONString(options.DisallowedTools),
		MCPServersJSON:       mustJSONString(options.MCPServers),
		ConnectorIDsJSON:     mustJSONString(options.ConnectorIDs),
		SkillIDsJSON:         mustJSONString(options.SkillIDs),
		DisabledSkillIDsJSON: mustJSONString(options.DisabledSkillIDs),
		MaxTurns:             options.MaxTurns,
		MaxThinkingTokens:    options.MaxThinkingTokens,
		SettingSourcesJSON:   mustJSONString(options.SettingSources),
		RuntimeVersion:       1,
	}
}

// resolveAgentAvatar 为创建和读取路径提供同一套头像兜底规则。
func resolveAgentAvatar(avatar string, agentID string, isMain bool) string {
	if normalized := strings.TrimSpace(avatar); normalized != "" {
		return normalized
	}
	if isMain {
		return defaultMainAgentAvatar
	}
	return stableAgentAvatar(agentID)
}

// stableAgentAvatar 用 Agent ID 生成稳定的“随机”头像，避免每次读取都换身份。
func stableAgentAvatar(agentID string) string {
	seed := strings.TrimSpace(agentID)
	if seed == "" {
		seed = defaultMainAgentAvatar
	}
	var hash uint32
	for _, character := range seed {
		hash = hash*31 + uint32(character)
	}
	rangeSize := uint32(agentAvatarIconEnd - agentAvatarIconStart + 1)
	return strconv.Itoa(agentAvatarIconStart + int(hash%rangeSize))
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
		ResolveWorkspacePath(cfg, ownerUserID, agentID),
		"active",
		true,
	)
}

func defaultMainAgentOptions() protocol.Options {
	return defaultAgentOptions(true)
}

func defaultAgentOptions(isMain bool) protocol.Options {
	skillIDs := []string{"imagegen", "visualize", "automation"}
	skillIDs, _ = runtimecommand.BindManagedSemanticSkills(skillIDs, nil)
	skillIDs = append(skillIDs, "nexus-configuration")
	if isMain {
		skillIDs = append(skillIDs, "nexus-manager")
	}
	return protocol.Options{
		AllowedTools:     []string{},
		ConnectorIDs:     []string{},
		PermissionMode:   protocol.DefaultAgentPermissionMode,
		SkillIDs:         skillIDs,
		DisabledSkillIDs: []string{},
		SettingSources:   []string{"project"},
	}
}

func pointer(value protocol.Options) *protocol.Options {
	return &value
}

func mergeOptions(base protocol.Options, incoming protocol.Options) protocol.Options {
	result := base
	// 当前 Web 主流程会显式提交 provider/model 字段；
	// 这里按完整快照语义处理，空字符串表示“跟随默认模型”。
	result.Provider = strings.TrimSpace(incoming.Provider)
	result.Model = strings.TrimSpace(incoming.Model)
	if incoming.PermissionMode != "" {
		result.PermissionMode = string(runtimepermission.NormalizeMode(sdkpermission.Mode(incoming.PermissionMode)))
	}
	result.PermissionMode = string(runtimepermission.NormalizeMode(sdkpermission.Mode(result.PermissionMode)))
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
	if incoming.ConnectorIDs != nil {
		result.ConnectorIDs = incoming.ConnectorIDs
	}
	if incoming.SkillIDs != nil {
		result.SkillIDs = incoming.SkillIDs
	}
	if incoming.DisabledSkillIDs != nil {
		result.DisabledSkillIDs = incoming.DisabledSkillIDs
	}
	if incoming.SettingSources != nil {
		result.SettingSources = incoming.SettingSources
	}
	return result
}

func mustJSONString(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(payload)
}
