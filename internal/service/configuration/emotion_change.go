// INPUT: 动态重验后的 Agent/Room actor 与 runtime 固化的 session/conversation。
// OUTPUT: 不可由模型指定的 emotion context ID、严格输入和不含绝对路径的状态投影。
// POS: Emotion 配置域的作用域与隐私边界。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

type emotionBaseInput struct {
	Mood        string `json:"mood"`
	Energy      int    `json:"energy"`
	Valence     int    `json:"valence"`
	Description string `json:"description"`
}

type emotionContextInput struct {
	Mood    string `json:"mood"`
	Valence int    `json:"valence"`
	Trigger string `json:"trigger"`
}

func trustedEmotionContextID(actor *resolvedActor) (string, error) {
	if actor == nil {
		return "", errors.New("Emotion 配置缺少可信 actor")
	}
	switch actor.ContextKind {
	case ContextKindAgent:
		sessionKey := strings.TrimSpace(actor.SessionKey)
		if sessionKey == "" {
			return "", errors.New("Emotion 私有 DM 缺少可信 session")
		}
		return "dm:" + sessionKey, nil
	case ContextKindRoom:
		conversationID := strings.TrimSpace(actor.ConversationID)
		if conversationID == "" {
			return "", errors.New("Emotion Room 上下文缺少可信 conversation")
		}
		return "room:" + conversationID, nil
	default:
		return "", fmt.Errorf("Emotion 不支持上下文 %q", actor.ContextKind)
	}
}

func safeEmotionView(view agentsvc.RuntimeEmotionView) agentsvc.RuntimeEmotionView {
	view = agentsvc.SafeRuntimeEmotionView(view)
	// Emotion 使用独立 version 做 CAS。时间戳会在首次读取尚未持久化的
	// 默认状态时随 now 变化，不能进入配置 revision，否则 plan 会自行失效。
	view.Base.UpdatedAt = time.Time{}
	view.Fatigue.UpdatedAt = time.Time{}
	if view.Context != nil {
		view.Context.UpdatedAt = time.Time{}
	}
	return view
}

func validateEmotionScore(field string, value int) error {
	if value < 0 || value > 10 {
		return fmt.Errorf("%s 必须在 0 到 10 之间", field)
	}
	return nil
}

func validateEmotionText(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s 不能为空", field)
	}
	return nil
}

func validateEmotionChange(request ChangeRequest) error {
	switch request.Operation {
	case "set_base":
		var input emotionBaseInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if err := validateEmotionText("mood", input.Mood); err != nil {
			return err
		}
		if err := validateEmotionScore("energy", input.Energy); err != nil {
			return err
		}
		if err := validateEmotionScore("valence", input.Valence); err != nil {
			return err
		}
		return validateEmotionText("description", input.Description)
	case "set_context":
		var input emotionContextInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if err := validateEmotionText("mood", input.Mood); err != nil {
			return err
		}
		if err := validateEmotionScore("valence", input.Valence); err != nil {
			return err
		}
		return validateEmotionText("trigger", input.Trigger)
	case "clear_context":
		return request.decodeInput(&struct{}{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeEmotionChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "set_base":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		var input emotionBaseInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		view, err := s.agents.SetAgentRuntimeEmotionBaseAtVersion(
			ctx,
			actor.AgentID,
			agentsvc.RuntimeEmotionBaseUpdate{
				Mood: input.Mood, Energy: input.Energy,
				Valence: input.Valence, Description: input.Description,
			},
			stateVersion,
		)
		return safeEmotionView(view), err
	case "set_context":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		contextID, err := trustedEmotionContextID(actor)
		if err != nil {
			return nil, err
		}
		var input emotionContextInput
		if err = request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		view, err := s.agents.SetAgentRuntimeEmotionContextAtVersion(
			ctx,
			actor.AgentID,
			agentsvc.RuntimeEmotionContextUpdate{
				ContextID: contextID, Mood: input.Mood,
				Valence: input.Valence, Trigger: input.Trigger,
			},
			stateVersion,
		)
		return safeEmotionView(view), err
	case "clear_context":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		contextID, err := trustedEmotionContextID(actor)
		if err != nil {
			return nil, err
		}
		view, err := s.agents.ClearAgentRuntimeEmotionContextAtVersion(
			ctx,
			actor.AgentID,
			contextID,
			stateVersion,
		)
		return safeEmotionView(view), err
	default:
		return nil, unsupportedChange(request)
	}
}

func (s *Service) readEmotionConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	contextID, contextErr := trustedEmotionContextID(actor)
	if contextErr != nil {
		return nil, nil, 0, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, contextErr
	}
	view, err := s.agents.GetAgentRuntimeEmotionView(
		ctx,
		actor.AgentID,
		contextID,
		time.Now(),
	)
	if err != nil {
		return nil, []Check{errorCheck(DomainEmotion, "emotion_state_readable", err)}, 0,
			ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, err
	}
	return safeEmotionView(view),
		[]Check{okCheck(
			DomainEmotion,
			"emotion_state_readable",
			"已核对当前 Agent 的版本化情绪状态；fatigue 为 runtime 只读，绝对 workspace 路径不对模型暴露",
		)},
		view.Version,
		ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID},
		nil
}
