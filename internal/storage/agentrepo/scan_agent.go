package agentrepo

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
)

// Scanner 表示 sql.Row 和 sql.Rows 都满足的最小扫描接口。
type Scanner interface {
	Scan(dest ...any) error
}

// ScanAgent 把 Agent/Profile/Runtime 联表结果解码成协议模型。
func ScanAgent(scanner Scanner) (protocol.Agent, error) {
	var (
		item                protocol.Agent
		vibeTagsJSON        string
		allowedToolsJSON    string
		disallowedToolsJSON string
		mcpServersJSON      string
		settingSourcesJSON  string
		maxTurns            sql.NullInt64
		maxThinkingTokens   sql.NullInt64
		createdAt           time.Time
	)

	err := scanner.Scan(
		&item.AgentID,
		&item.Name,
		&item.OwnerUserID,
		&item.WorkspacePath,
		&item.Status,
		&item.IsMain,
		&item.Avatar,
		&item.Description,
		&vibeTagsJSON,
		&item.DisplayName,
		&item.Headline,
		&item.ProfileMarkdown,
		&createdAt,
		&item.Options.Provider,
		&item.Options.PermissionMode,
		&allowedToolsJSON,
		&disallowedToolsJSON,
		&mcpServersJSON,
		&maxTurns,
		&maxThinkingTokens,
		&settingSourcesJSON,
	)
	if err != nil {
		return protocol.Agent{}, err
	}

	item.CreatedAt = createdAt
	item.VibeTags = decodeStringSlice(vibeTagsJSON)
	item.Options.AllowedTools = jsoncodec.ParseStringSlice(allowedToolsJSON)
	item.Options.DisallowedTools = jsoncodec.ParseStringSlice(disallowedToolsJSON)
	item.Options.MCPServers = jsoncodec.ParseMap(mcpServersJSON)
	item.Options.SettingSources = jsoncodec.ParseStringSlice(settingSourcesJSON)
	if maxTurns.Valid {
		value := int(maxTurns.Int64)
		item.Options.MaxTurns = &value
	}
	if maxThinkingTokens.Valid {
		value := int(maxThinkingTokens.Int64)
		item.Options.MaxThinkingTokens = &value
	}
	return item, nil
}

func decodeStringSlice(raw string) []string {
	result := make([]string, 0)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return []string{}
	}
	return result
}
