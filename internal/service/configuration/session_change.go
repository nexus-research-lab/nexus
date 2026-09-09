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

func validateSessionsChange(request ChangeRequest) error {
	switch request.Operation {
	case "update_title":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&sessionTitleInput{})
	case "delete":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&struct{}{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeSessionsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "update_title":
		if s.sessions == nil {
			return nil, errors.New("Sessions 配置服务未装配")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Session 标题更新缺少 configuration_version；请重新 plan")
		}
		var input sessionTitleInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		updated, err := s.sessions.UpdateSessionTitleAtVersion(
			ctx,
			request.Target,
			input.Title,
			stateVersion,
		)
		if err != nil {
			return nil, err
		}
		if updated == nil {
			return nil, errors.New("Session 标题更新未返回可核验结果")
		}
		return safeSessionTitleChangeResult(*updated), nil
	case "delete":
		if s.sessions == nil {
			return nil, errors.New("Sessions 配置服务未装配")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Session 删除缺少 configuration_version；请重新 plan")
		}
		err := s.sessions.DeleteSessionAtVersion(ctx, request.Target, stateVersion)
		return map[string]any{
			"session_key": request.Target,
			"deleted":     err == nil || sessionsvc.SessionDeletionCommitted(err),
		}, err
	default:
		return nil, unsupportedChange(request)
	}
}

func (s *Service) verifySessionTitleChange(
	ctx context.Context,
	request ChangeRequest,
) (Check, error) {
	var input sessionTitleInput
	if err := strictDecodeJSON(request.Input, &input); err != nil {
		return Check{}, err
	}
	expectedTitle := strings.TrimSpace(input.Title)
	if expectedTitle == "" {
		expectedTitle = "New Chat"
	}
	item, err := s.sessions.GetMutableSession(ctx, request.Target)
	if err != nil {
		return Check{}, fmt.Errorf("重新读取 Session: %w", err)
	}
	actualTitle := ""
	if item != nil {
		actualTitle = strings.TrimSpace(item.Title)
	}
	if item == nil || actualTitle != expectedTitle {
		return Check{}, fmt.Errorf(
			"Session 标题写后不一致：expected=%q actual=%q",
			expectedTitle,
			actualTitle,
		)
	}
	return Check{
		Code: "session_title_verified", Status: "ok",
		Message: "已从 owner workspace 重新读取 Session，并核对标题与计划一致",
		Domain:  DomainSessions, Target: request.Target, Verified: true,
	}, nil
}

type sessionTitleInput struct {
	Title string `json:"title"`
}

func (s *Service) verifyDeletedSessions(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	if s.sessions == nil {
		return errors.New("Sessions 配置服务未装配")
	}
	if _, err := s.sessions.GetMutableSession(ctx, request.Target); err == nil {
		return fmt.Errorf("Session %s 删除后仍存在", request.Target)
	} else if !errors.Is(err, sessionsvc.ErrSessionNotFound) {
		return fmt.Errorf("核对已删除 Session: %w", err)
	}

	return nil
}
