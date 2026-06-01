package goal

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

//go:embed templates/objective_updated.md
var objectiveUpdatedPromptTemplate string

//go:embed templates/budget_limit.md
var budgetLimitPromptTemplate string

type guidanceDispatcher interface {
	QueueGuidanceInput(context.Context, string, string, string) ([]string, error)
}

type contextualGuidanceDispatcher interface {
	QueueContextualGuidanceInput(context.Context, string, string, string, string) ([]string, error)
}

type budgetSteeringSuppressionKey struct{}

// SetGuidanceDispatcher 注入运行时引导队列，用于把 Goal steering 送入正在执行的 round。
func (s *Service) SetGuidanceDispatcher(dispatcher guidanceDispatcher) {
	s.guidance = dispatcher
}

func withBudgetLimitSteeringSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetSteeringSuppressionKey{}, true)
}

func budgetLimitSteeringSuppressed(ctx context.Context) bool {
	value, _ := ctx.Value(budgetSteeringSuppressionKey{}).(bool)
	return value
}

func (s *Service) queueGoalSteering(ctx context.Context, item protocol.Goal, event protocol.GoalEvent) {
	if s.guidance == nil {
		return
	}
	prompt := ""
	switch {
	case event.EventType == "updated" && eventPayloadBool(event.Payload, "objective_updated") && protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusActive:
		prompt = buildObjectiveUpdatedPrompt(item)
	case event.EventType == "budget_limited":
		if budgetLimitSteeringSuppressed(ctx) {
			return
		}
		prompt = buildBudgetLimitPrompt(item)
	}
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if dispatcher, ok := s.guidance.(contextualGuidanceDispatcher); ok {
		_, _ = dispatcher.QueueContextualGuidanceInput(ctx, item.SessionKey, event.ID, "goal", prompt)
		return
	}
	_, _ = s.guidance.QueueGuidanceInput(ctx, item.SessionKey, event.ID, prompt)
}

func eventPayloadBool(payload map[string]any, key string) bool {
	value, ok := payload[key]
	if !ok {
		return false
	}
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func buildObjectiveUpdatedPrompt(item protocol.Goal) string {
	tokenBudget, remainingTokens := goalBudgetStrings(item)
	return renderGoalPromptTemplate(objectiveUpdatedPromptTemplate, map[string]string{
		"objective":        escapeGoalPromptText(strings.TrimSpace(item.Objective)),
		"tokens_used":      fmt.Sprintf("%d", item.Usage.Total()),
		"token_budget":     tokenBudget,
		"remaining_tokens": remainingTokens,
	})
}

func buildBudgetLimitPrompt(item protocol.Goal) string {
	tokenBudget, _ := goalBudgetStrings(item)
	return renderGoalPromptTemplate(budgetLimitPromptTemplate, map[string]string{
		"objective":         escapeGoalPromptText(strings.TrimSpace(item.Objective)),
		"time_used_seconds": fmt.Sprintf("%d", item.TimeUsedSeconds),
		"tokens_used":       fmt.Sprintf("%d", item.Usage.Total()),
		"token_budget":      tokenBudget,
	})
}

func goalBudgetStrings(item protocol.Goal) (string, string) {
	tokenBudget := "none"
	if item.TokenBudget != nil {
		tokenBudget = fmt.Sprintf("%d", *item.TokenBudget)
	}
	remainingTokens := "unbounded"
	if remaining := item.RemainingTokens(); remaining != nil {
		remainingTokens = fmt.Sprintf("%d", *remaining)
	}
	return tokenBudget, remainingTokens
}
