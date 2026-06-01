package goalobjective

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

const (
	rewriteRequestTimeout = 30 * time.Second
	rewriteAttemptTimeout = 15 * time.Second
	rewriteMaxTokens      = 512
	rewriteMaxRunes       = 4000
	rewriteSystemPrompt   = `你是 Goal 目标整理器。
把用户提供的目标草稿改写为一个适合当前会话长程执行的 Goal objective。
要求：
1. 保留用户真实意图、最终状态、范围限制、验收条件、命名文件/分支/仓库/问题编号、交付物和重要上下文。
2. 保留会影响成功判定的用户偏好、约束和禁止事项；不要把目标改写成当前进度总结。
3. 用清晰、具体、可执行、可验证的表述描述完整目标，便于后续逐项审计是否完成。
4. 不要缩小、放宽、替换或重新定义成功范围；不要改成更容易、更安全、更小或更像待办标题的目标。
5. 不添加用户没有要求的 token 预算、工具、截止时间、技术方案、假设、验收条件或额外范围。
6. 使用用户主要使用的语言；如果原文包含中英文专有名词或路径，原样保留。
7. 不输出解释、标题、列表前缀、引号或 Markdown 代码块。
8. 只返回改写后的 objective 文本。`
)

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type Request struct {
	OwnerUserID string
	Provider    string
	Model       string
	Objective   string
}

type Service struct {
	providers providerResolver
	prefs     preferencesService
	llmClient *llm.Client
}

func NewService(providers providerResolver, prefs preferencesService) *Service {
	return &Service{
		providers: providers,
		prefs:     prefs,
		llmClient: llm.NewClient(http.DefaultClient),
	}
}

func (s *Service) Rewrite(ctx context.Context, request Request) (string, error) {
	objective := strings.TrimSpace(request.Objective)
	if objective == "" {
		return "", errors.New("goal objective is empty")
	}
	if s == nil || s.providers == nil {
		return "", errors.New("goal objective rewriter is not configured")
	}
	config, err := s.resolveLLMConfig(ctx, request)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, rewriteRequestTimeout)
	defer cancel()
	attemptCtx, attemptCancel := context.WithTimeout(ctx, rewriteAttemptTimeout)
	defer attemptCancel()
	text, err := s.llmClient.GenerateText(attemptCtx, llm.GenerateTextRequest{
		Config: config,
		System: rewriteSystemPrompt,
		Messages: []llm.Message{{
			Role:    "user",
			Content: truncateObjective(objective, rewriteMaxRunes),
		}},
		MaxTokens:   rewriteMaxTokens,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	rewritten := sanitizeObjective(text)
	if rewritten == "" {
		return "", errors.New("goal objective rewrite returned empty result")
	}
	return rewritten, nil
}

func (s *Service) RewriteGoalObjective(ctx context.Context, ownerUserID string, objective string) (string, error) {
	return s.Rewrite(ctx, Request{
		OwnerUserID: ownerUserID,
		Objective:   objective,
	})
}

func (s *Service) resolveLLMConfig(ctx context.Context, request Request) (*clientopts.RuntimeConfig, error) {
	if s.prefs != nil {
		ownerUserID := strings.TrimSpace(request.OwnerUserID)
		if ownerUserID != "" {
			prefs, err := s.prefs.Get(ctx, ownerUserID)
			if err != nil {
				return nil, err
			}
			selection := prefs.DefaultBackgroundModelSelection
			if strings.TrimSpace(selection.Provider) != "" && strings.TrimSpace(selection.Model) != "" {
				return s.providers.ResolveLLMConfig(ctx, selection.Provider, selection.Model)
			}
		}
	}
	return s.providers.ResolveLLMConfig(ctx, request.Provider, request.Model)
}

func sanitizeObjective(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "`")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'“”‘’")
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > rewriteMaxRunes {
		return truncateObjective(value, rewriteMaxRunes)
	}
	return value
}

func truncateObjective(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return strings.TrimSpace(value)
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes]))
}
