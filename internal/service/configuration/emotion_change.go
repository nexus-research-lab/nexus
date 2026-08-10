// INPUT: 动态重验后的 Agent/Room actor 与 runtime 固化的 session/conversation。
// OUTPUT: 不可由模型指定的 emotion context ID、严格输入和不含绝对路径的状态投影。
// POS: Emotion 配置域的作用域与隐私边界。
package configuration

import (
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
