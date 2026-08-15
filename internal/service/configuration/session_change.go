// INPUT: 已动态鉴权的 owner-main/agent-self 与 owner-confined workspace Session 服务。
// OUTPUT: 不含 SDK session_id/options 的目录快照、目标版本与写后可核验投影。
// POS: nexuscfg Sessions 域的数据最小化边界；Room conversation 始终归 rooms 域。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type sessionConfigurationView struct {
	SessionKey           string    `json:"session_key"`
	AgentID              string    `json:"agent_id"`
	ChannelType          string    `json:"channel_type"`
	ChatType             string    `json:"chat_type"`
	Title                string    `json:"title"`
	CreatedAt            time.Time `json:"created_at"`
	ConfigurationVersion int64     `json:"configuration_version"`
}

type sessionTitleChangeResult struct {
	SessionKey           string `json:"session_key"`
	Title                string `json:"title"`
	ConfigurationVersion int64  `json:"configuration_version"`
}

func safeSessionConfigurationView(item protocol.Session) sessionConfigurationView {
	return sessionConfigurationView{
		SessionKey:           strings.TrimSpace(item.SessionKey),
		AgentID:              strings.TrimSpace(item.AgentID),
		ChannelType:          strings.TrimSpace(item.ChannelType),
		ChatType:             strings.TrimSpace(item.ChatType),
		Title:                item.Title,
		CreatedAt:            item.CreatedAt,
		ConfigurationVersion: item.ConfigurationVersion,
	}
}

func safeSessionTitleChangeResult(item protocol.Session) sessionTitleChangeResult {
	return sessionTitleChangeResult{
		SessionKey:           strings.TrimSpace(item.SessionKey),
		Title:                item.Title,
		ConfigurationVersion: item.ConfigurationVersion,
	}
}

func (s *Service) sessionDomainValues(
	ctx context.Context,
	actor *resolvedActor,
	target string,
) (any, []Check, int64, ScopeRef, error) {
	scope := ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}
	if s.sessions == nil {
		err := errors.New("Sessions 配置服务未装配")
		return nil, []Check{errorCheck(DomainSessions, "session_directory_readable", err)}, 0, scope, err
	}
	target = strings.TrimSpace(target)
	if actor.Authority == AuthorityAgentSelf {
		target = strings.TrimSpace(actor.SessionKey)
	}
	if target != "" {
		item, err := s.sessions.GetMutableSession(ctx, target)
		if err != nil {
			return nil, []Check{errorCheck(DomainSessions, "session_target_readable", err)}, 0, scope, err
		}
		if item == nil {
			return nil, nil, 0, scope, sessionsvc.ErrSessionNotFound
		}
		scope = ScopeRef{Kind: ScopeKindAgent, ID: strings.TrimSpace(item.AgentID)}
		view := safeSessionConfigurationView(*item)
		return view, []Check{okCheck(
			DomainSessions,
			"session_target_readable",
			fmt.Sprintf(
				"已核对 Agent %s 的 workspace session 与 configuration_version=%d；SDK session_id、resume 与 options 不对配置模型暴露",
				view.AgentID,
				view.ConfigurationVersion,
			),
		)}, view.ConfigurationVersion, scope, nil
	}

	items, err := s.sessions.ListMutableSessions(ctx)
	if err != nil {
		return nil, []Check{errorCheck(DomainSessions, "session_directory_readable", err)}, 0, scope, err
	}
	views := make([]sessionConfigurationView, 0, len(items))
	for _, item := range items {
		views = append(views, safeSessionConfigurationView(item))
	}
	return views, []Check{okCheck(
		DomainSessions,
		"session_directory_readable",
		fmt.Sprintf("已核对 %d 个 owner workspace Agent session；Room conversation 已排除", len(views)),
	)}, 0, scope, nil
}
