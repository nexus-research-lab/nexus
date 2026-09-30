// INPUT: 已授权配置快照的 domain、scope、target、state version 与完整值。
// OUTPUT: 宿主持久密钥绑定的版本化 HMAC revision，不暴露低熵秘密。
// POS: 配置读取/计划/写后/恢复的统一 revision；批准摘要仍由进程临时密钥绑定。
package configuration

import (
	"context"
	"fmt"
	"strings"

	configurationstore "github.com/nexus-research-lab/nexus/internal/storage/configuration"
)

const durableRevisionPrefix = "hmac-sha256:v2:"

func (s *Service) snapshotRevision(
	ctx context.Context,
	domain string,
	scope ScopeRef,
	target string,
	stateVersion int64,
	values any,
) (string, error) {
	key, err := configurationstore.NewRevisionKeyStore(s.cfg, s.db).Key(ctx)
	if err != nil {
		return "", fmt.Errorf("读取配置 revision 密钥: %w", err)
	}
	digest, err := integrityRevisionFor(map[string]any{
		"version": 2, "domain": domain, "scope": scope,
		"target": strings.TrimSpace(target), "state_version": stateVersion, "values": values,
	}, key)
	if err != nil {
		return "", err
	}
	return durableRevisionPrefix + strings.TrimPrefix(digest, "hmac-sha256:"), nil
}
