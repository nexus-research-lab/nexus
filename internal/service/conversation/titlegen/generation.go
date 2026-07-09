package titlegen

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
)

const (
	titleAttemptTimeout = 20 * time.Second
	// 部分 provider（如 Kimi 始终推理模型）无法关闭思考，128 token 会被推理吃光、
	// 正文为空触发 max_tokens 截断；放宽到 1024 给标题正文留足空间。
	titleMaxTokens    = 1024
	titleMaxAttempts  = 2
	titleSystemPrompt = `你是会话标题生成器。
请根据用户的第一条消息生成一个简短标题。
要求：
1. 用自己的话概括核心意图，不要原样复述。
2. 中文控制在 2 到 12 个字；英文控制在 2 到 6 个单词。
3. 不要使用引号、句号、冒号、emoji。
4. 不要输出任何思考过程、解释或前缀，直接给出标题文本。`
)

var errEmptyGeneratedTitle = errors.New("标题生成返回空结果")

func (s *Service) generateAndApply(ctx context.Context, request Request) {
	sessionEligible := false
	if request.shouldCheckSessionTitle() {
		ok, err := s.canAutoUpdateSession(ctx, request.SessionKey, request.FallbackTitle)
		if err != nil {
			s.logger.Warn("检查 session 标题状态失败",
				"session_key", request.SessionKey,
				"err", err,
			)
		} else {
			sessionEligible = ok
		}
	}
	conversationEligible := false
	resolvedRoomID := strings.TrimSpace(request.ConversationRoomID)
	if request.shouldCheckConversationTitle() {
		ok, roomID, err := s.canAutoUpdateConversation(
			ctx,
			request.ConversationID,
			request.ConversationRoomID,
			request.FallbackTitle,
		)
		if err != nil {
			s.logger.Warn("检查 room 对话标题状态失败",
				"conversation_id", request.ConversationID,
				"room_id", request.ConversationRoomID,
				"err", err,
			)
		} else {
			conversationEligible = ok
			if roomID != "" {
				resolvedRoomID = roomID
			}
		}
	}
	if !sessionEligible && !conversationEligible {
		s.logger.Info("跳过标题生成：目标当前不可自动更新",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
			"session_title", request.SessionTitle,
			"conversation_title", request.ConversationTitle,
			"conversation_room_name", request.ConversationRoomName,
			"fallback_title", request.FallbackTitle,
			"session_eligible", sessionEligible,
			"conversation_eligible", conversationEligible,
		)
		return
	}

	title, err := s.generateTitle(ctx, request, request.Content)
	if err != nil {
		if errors.Is(err, errEmptyGeneratedTitle) {
			s.logger.Warn("生成会话标题返回空结果",
				"session_key", request.SessionKey,
				"conversation_id", request.ConversationID,
				"provider", strings.TrimSpace(request.Provider),
				"model", strings.TrimSpace(request.Model),
				"err", err,
			)
			return
		}
		s.logger.Warn("生成会话标题失败",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
			"provider", strings.TrimSpace(request.Provider),
			"model", strings.TrimSpace(request.Model),
			"err", err,
		)
		return
	}
	if title == "" {
		return
	}

	updated := false
	if sessionEligible {
		ok, err := s.applySessionTitle(ctx, request.SessionKey, title, request.FallbackTitle)
		if err != nil {
			s.logger.Warn("更新 session 标题失败",
				"session_key", request.SessionKey,
				"title", title,
				"err", err,
			)
		} else if ok {
			updated = true
			s.logger.Info("session 标题已生成",
				"session_key", request.SessionKey,
				"title", title,
			)
		}
	}
	if conversationEligible {
		ok, err := s.applyConversationTitle(
			ctx,
			request.ConversationID,
			resolvedRoomID,
			title,
			request.FallbackTitle,
		)
		if err != nil {
			s.logger.Warn("更新 room 对话标题失败",
				"conversation_id", request.ConversationID,
				"room_id", request.ConversationRoomID,
				"title", title,
				"err", err,
			)
		} else if ok {
			updated = true
			s.logger.Info("room 对话标题已生成",
				"conversation_id", request.ConversationID,
				"room_id", request.ConversationRoomID,
				"title", title,
			)
		}
	}
	if updated {
		request.ConversationRoomID = resolvedRoomID
		s.broadcastResync(ctx, request)
	}
}

func (s *Service) generateTitle(
	ctx context.Context,
	request Request,
	content string,
) (string, error) {
	runtimeConfig, err := s.resolveLLMConfig(ctx, request)
	if err != nil {
		return "", fmt.Errorf("解析标题模型配置失败 request_provider=%q request_model=%q: %w", strings.TrimSpace(request.Provider), strings.TrimSpace(request.Model), err)
	}
	llmRequest := llm.GenerateTextRequest{
		Config:           runtimeConfig,
		System:           titleSystemPrompt,
		Messages:         []llm.Message{{Role: "user", Content: truncatePromptContent(content, 400)}},
		MaxTokens:        titleMaxTokens,
		Temperature:      0,
		DisableReasoning: true,
	}

	var lastErr error
	attempts := 0
	for attempt := 1; attempt <= titleMaxAttempts; attempt++ {
		attempts = attempt
		attemptCtx, cancel := context.WithTimeout(ctx, titleAttemptTimeout)
		title, err := s.doGenerateTitle(attemptCtx, llmRequest)
		cancel()
		if err == nil {
			return title, nil
		}
		lastErr = err
		if !shouldRetryTitleRequest(err) || attempt == titleMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(600 * time.Millisecond):
		}
	}
	return "", fmt.Errorf(
		"标题模型请求失败 resolved_provider=%q resolved_model=%q api_format=%q base_url_set=%t attempts=%d: %w",
		strings.TrimSpace(runtimeConfig.Provider),
		strings.TrimSpace(runtimeConfig.Model),
		strings.TrimSpace(runtimeConfig.APIFormat),
		strings.TrimSpace(runtimeConfig.BaseURL) != "",
		attempts,
		lastErr,
	)
}

func (s *Service) resolveLLMConfig(
	ctx context.Context,
	request Request,
) (*clientopts.RuntimeConfig, error) {
	if s.prefs != nil {
		ownerUserID := strings.TrimSpace(request.OwnerUserID)
		if ownerUserID != "" {
			prefs, err := s.prefs.Get(ctx, ownerUserID)
			if err != nil {
				return nil, err
			}
			selection := prefs.DefaultBackgroundModelSelection
			selection.Provider = strings.TrimSpace(selection.Provider)
			selection.Model = strings.TrimSpace(selection.Model)
			if selection.Provider != "" && selection.Model != "" {
				return s.providers.ResolveLLMConfig(ctx, selection.Provider, selection.Model)
			}
		}
	}
	return s.providers.ResolveLLMConfig(ctx, request.Provider, request.Model)
}

func (s *Service) doGenerateTitle(
	ctx context.Context,
	request llm.GenerateTextRequest,
) (string, error) {
	text, err := s.llmClient.GenerateText(ctx, request)
	if err != nil {
		return "", err
	}
	title := sanitizeGeneratedTitle(text)
	if title == "" {
		return "", errEmptyGeneratedTitle
	}
	return title, nil
}
