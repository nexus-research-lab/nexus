// =====================================================
// @File   ：policy.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package agent

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var nameAllowedPattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9 _-]+$`)

const (
	nameMinLength = 2
	nameMaxLength = 40
)

// NormalizeName 标准化 Agent 名称。
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

// BuildWorkspaceDirName 生成安全目录名。
func BuildWorkspaceDirName(agentName string) string {
	normalized := strings.ReplaceAll(NormalizeName(agentName), " ", "_")
	var builder strings.Builder
	lastUnderscore := false
	for _, value := range normalized {
		switch {
		case unicode.IsLetter(value), unicode.IsDigit(value), value == '_', value == '-':
			builder.WriteRune(value)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteRune('_')
			}
			lastUnderscore = true
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "agent"
	}
	return result
}

// WorkspaceBasePath 返回 workspace 根目录。
func WorkspaceBasePath(cfg config.Config) string {
	if strings.TrimSpace(cfg.WorkspacePath) != "" {
		return expandHome(cfg.WorkspacePath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nexus/workspace"
	}
	return filepath.Join(home, ".nexus", "workspace")
}

// ResolveWorkspacePath 计算 Agent workspace 路径。
func ResolveWorkspacePath(cfg config.Config, agentName string) string {
	return filepath.Join(WorkspaceBasePath(cfg), BuildWorkspaceDirName(agentName))
}

// BuildCreateRecord 构建落库记录。
func BuildCreateRecord(cfg config.Config, request CreateRequest, normalizedName string, agentID string, workspacePath string, status string) CreateRecord {
	options := Options{}
	if agentID == cfg.DefaultAgentID {
		options = defaultMainAgentOptions(cfg)
	}
	if request.Options != nil {
		options = mergeOptions(options, *request.Options)
	}

	return CreateRecord{
		AgentID:             agentID,
		Slug:                BuildWorkspaceDirName(normalizedName),
		Name:                normalizedName,
		WorkspacePath:       workspacePath,
		Status:              status,
		Avatar:              request.Avatar,
		Description:         request.Description,
		VibeTagsJSON:        mustJSONString(request.VibeTags, "[]"),
		DisplayName:         normalizedName,
		Headline:            "",
		ProfileMarkdown:     "",
		RuntimeID:           buildStableID("runtime", agentID),
		ProfileID:           buildStableID("profile", agentID),
		Provider:            options.Provider,
		Model:               options.Model,
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
func BuildDefaultMainAgentRecord(cfg config.Config) CreateRecord {
	name := cfg.DefaultAgentID
	return BuildCreateRecord(
		cfg,
		CreateRequest{Name: name, Options: pointer(defaultMainAgentOptions(cfg))},
		name,
		cfg.DefaultAgentID,
		filepath.Join(WorkspaceBasePath(cfg), cfg.DefaultAgentID),
		"active",
	)
}

// ValidateName 校验名称格式。
func ValidateName(name string) string {
	normalized := NormalizeName(name)
	switch {
	case normalized == "":
		return "名称不能为空"
	case len([]rune(normalized)) < nameMinLength:
		return fmt.Sprintf("名称至少 %d 个字符", nameMinLength)
	case len([]rune(normalized)) > nameMaxLength:
		return fmt.Sprintf("名称不能超过 %d 个字符", nameMaxLength)
	case !nameAllowedPattern.MatchString(normalized):
		return "仅支持中文、英文、数字、空格、下划线和连字符"
	default:
		return ""
	}
}

// NewAgentID 生成新的 agent_id。
func NewAgentID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())[:12]
}

func defaultMainAgentOptions(cfg config.Config) Options {
	options := Options{
		AllowedTools:   []string{"AskUserQuestion", "Bash", "Edit", "Glob", "Grep", "LS", "Read", "Skill", "TodoWrite", "WebFetch", "WebSearch", "Write"},
		PermissionMode: "default",
		SettingSources: []string{"project"},
	}
	if cfg.MainAgentModel != "" {
		options.Model = cfg.MainAgentModel
	}
	return options
}

func pointer(value Options) *Options {
	return &value
}

func mergeOptions(base Options, incoming Options) Options {
	result := base
	if incoming.Model != "" {
		result.Model = incoming.Model
	}
	if incoming.Provider != "" {
		result.Provider = incoming.Provider
	}
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

func buildStableID(prefix string, raw string) string {
	digest := sha1.Sum([]byte(raw))
	return prefix + "_" + hex.EncodeToString(digest[:])[:20]
}

func mustJSONString(value any, fallback string) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(payload)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
