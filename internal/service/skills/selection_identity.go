// INPUT: Skill catalog 来源元数据与 Agent 选择作用域。
// OUTPUT: 对话计划可回传并在 apply 时重验的稳定、无路径明文 source_identity。
// POS: Skills catalog 来源记录到非破坏性 Agent 开关协议的身份绑定层。
package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func selectionInfo(record catalogRecord) Info {
	info := record.Detail.Info
	info.TargetScope = targetScopeForInfo(info)
	info.SourceIdentity = sourceIdentityForRecord(record)
	return info
}

func targetScopeForInfo(info Info) AgentSkillTargetScope {
	if info.Scope == scopeRoom {
		return ""
	}
	if info.SourceType == sourceTypeWorkspace || info.StorageScope == storageScopeAgent {
		return AgentSkillTargetWorkspace
	}
	return AgentSkillTargetGlobalLibrary
}

func sourceIdentityForRecord(record catalogRecord) string {
	info := record.Detail.Info
	targetScope := targetScopeForInfo(info)
	if targetScope == "" {
		return ""
	}
	payload := strings.Join([]string{
		string(targetScope),
		strings.TrimSpace(info.Name),
		strings.TrimSpace(info.SourceType),
		strings.TrimSpace(info.SourceKind),
		strings.TrimSpace(info.SourceRef),
		strings.TrimSpace(info.StorageScope),
		strings.TrimSpace(info.OriginKind),
		strings.TrimSpace(info.Version),
		strings.TrimSpace(info.Scope),
		strings.TrimSpace(info.Title),
		strings.TrimSpace(info.Description),
		strings.Join(info.Tags, "\x00"),
		strings.TrimSpace(record.Detail.ReadmeMarkdown),
	}, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return "skill-source:" + hex.EncodeToString(sum[:])
}
