// INPUT: Goal 工具结果、状态与 actual/budget usage。
// OUTPUT: Codex 兼容的 Goal 投影，以及只在完成收据中出现的宿主结算字段与 result-first 交付指引。
// POS: Goal command 操作的稳定输出边界。
package operation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func structuredResult(_ string, content map[string]any) runtimecommand.Result {
	text := "{}"
	if payload, err := json.MarshalIndent(goalCommandTextPayloadFrom(content), "", "  "); err == nil {
		text = string(payload)
	}
	return runtimecommand.Result{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: content,
	}
}

func appliedResult(summary string, content map[string]any) runtimecommand.Result {
	content = maps.Clone(content)
	content["outcome"] = string(protocol.MutationResultApplied)
	return structuredResult(summary, content)
}

type goalCommandTextPayload struct {
	Goal               any `json:"goal"`
	RemainingTokens    any `json:"remainingTokens"`
	ObjectiveAlignment any `json:"objectiveAlignment,omitempty"`
	NextAction         any `json:"nextAction,omitempty"`
	// CompletionBudgetReport 保持 Codex 兼容的模型可见完成指引。
	CompletionBudgetReport any `json:"completionBudgetReport"`
}

type goalTextValue struct {
	ThreadID           any `json:"threadId"`
	Objective          any `json:"objective"`
	ObjectiveRevision  any `json:"objectiveRevision"`
	CompletionCriteria any `json:"completionCriteria"`
	Status             any `json:"status"`
	TokenBudget        any `json:"tokenBudget"`
	TokensUsed         any `json:"tokensUsed"`
	TimeUsedSeconds    any `json:"timeUsedSeconds"`
	CreatedAt          any `json:"createdAt"`
	UpdatedAt          any `json:"updatedAt"`
	Blocker            any `json:"blocker,omitempty"`
}

func goalCommandTextPayloadFrom(content map[string]any) goalCommandTextPayload {
	return goalCommandTextPayload{
		Goal:                   goalTextValueFromAny(content["goal"]),
		RemainingTokens:        content["remainingTokens"],
		ObjectiveAlignment:     content["objectiveAlignment"],
		NextAction:             content["nextAction"],
		CompletionBudgetReport: content["completionBudgetReport"],
	}
}

func goalTextValueFromAny(value any) any {
	goal, ok := value.(map[string]any)
	if !ok || goal == nil {
		return nil
	}
	return goalTextValue{
		ThreadID:           goal["threadId"],
		Objective:          goal["objective"],
		ObjectiveRevision:  goal["objectiveRevision"],
		CompletionCriteria: goal["completionCriteria"],
		Status:             goal["status"],
		TokenBudget:        goal["tokenBudget"],
		TokensUsed:         goal["tokensUsed"],
		TimeUsedSeconds:    goal["timeUsedSeconds"],
		CreatedAt:          goal["createdAt"],
		UpdatedAt:          goal["updatedAt"],
		Blocker:            goal["blocker"],
	}
}

func errorResult(err error) runtimecommand.Result {
	text := "goal command failed"
	if err != nil {
		text = err.Error()
	}
	return errorResultText(text)
}

func errorResultText(text string) runtimecommand.Result {
	return runtimecommand.Result{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: map[string]any{
			"outcome": "rejected",
			"message": text,
		},
		IsError: true,
	}
}

func errorResultWithNextAction(err error, nextAction map[string]any) runtimecommand.Result {
	text := "goal command failed"
	if err != nil {
		text = err.Error()
	}
	return runtimecommand.Result{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: map[string]any{
			"outcome":    "rejected",
			"message":    text,
			"nextAction": nextAction,
		},
		IsError: true,
	}
}

func planModeGoalMutationResult(operationName string) runtimecommand.Result {
	return errorResultText(
		operationName + " is validation-only in Plan Mode and did not change Goal, Execution, Plan, or cancellation state; leave Plan Mode and retry to persist it",
	)
}

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode input: expected one JSON object")
	}
	return nil
}

func goalPayload(item *protocol.Goal) map[string]any {
	return goalPayloadWithOptions(item, goalPayloadOptions{})
}

func goalCompletionPayload(item *protocol.Goal) map[string]any {
	return goalPayloadWithOptions(item, goalPayloadOptions{completionBudgetReport: true})
}

func goalMutationPayload(item *protocol.Goal) map[string]any {
	payload := goalPayload(item)
	payload["outcome"] = protocol.MutationResultApplied
	return payload
}

func goalCompletionMutationPayload(item *protocol.Goal) map[string]any {
	payload := goalCompletionPayload(item)
	payload["outcome"] = protocol.MutationResultApplied
	return payload
}

type goalPayloadOptions struct {
	completionBudgetReport bool
}

func goalPayloadWithOptions(item *protocol.Goal, options goalPayloadOptions) map[string]any {
	payload := map[string]any{
		"goal":                   commandGoalValue(item),
		"remainingTokens":        nil,
		"completionBudgetReport": nil,
	}
	if item == nil {
		return payload
	}
	remainingTokens := item.RemainingTokens()
	payload["remainingTokens"] = int64PointerValue(remainingTokens)
	if nextAction := pendingObjectiveTransitionAction(*item); nextAction != nil {
		payload["nextAction"] = nextAction
	}
	if options.completionBudgetReport {
		if report := completionUsageCheckpointReport(item); report != "" {
			if item.ID != "" {
				payload["goalId"] = item.ID
			}
			payload["usageFinalized"] = false
			payload["completionUsageCheckpointReport"] = report
			payload["completionBudgetReport"] = report
		}
	}
	return payload
}

func pendingObjectiveTransitionAction(item protocol.Goal) map[string]any {
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(item)
	if !ok || transition.Phase == goalsvc.ObjectiveTransitionBound {
		return nil
	}
	if transition.Phase == goalsvc.ObjectiveTransitionPrepared {
		return map[string]any{
			"domain":          runtimecommand.DomainGoal,
			"operation":       "retarget_goal",
			"targetObjective": transition.TargetObjective,
			"reason":          "retry the prepared Goal objective transition so the old WorkGraph is fenced and the new objective revision is committed",
		}
	}
	return map[string]any{
		"domain":    runtimecommand.DomainExecution,
		"operation": "prepare_plan_execution",
		"reason":    "prepare the complete successor WorkGraph for the current Goal objective revision, then commit its sealed proposal",
	}
}

func commandGoalValue(item *protocol.Goal) any {
	if item == nil {
		return nil
	}
	goal := map[string]any{
		"threadId":              item.SessionKey,
		"objective":             item.Objective,
		"objectiveRevision":     item.ObjectiveRevision(),
		"completionCriteria":    commandGoalCompletionCriteria(*item),
		"status":                commandGoalStatus(item.Status),
		"tokenBudget":           int64PointerValue(item.TokenBudget),
		"tokensUsed":            item.Usage.BudgetTokens(),
		"budgetTokens":          item.Usage.BudgetTokens(),
		"actualTokens":          item.Usage.ActualTokens(),
		"actualTokensEstimated": item.Usage.ActualTokensAreEstimated(),
		"timeUsedSeconds":       item.TimeUsedSeconds,
		"createdAt":             item.CreatedAt.Unix(),
		"updatedAt":             item.UpdatedAt.Unix(),
	}
	if blocker, ok := protocol.GoalBlockerFromGoal(*item); ok {
		goal["blocker"] = map[string]any{
			"id":            blocker.ID,
			"reason":        blocker.Reason,
			"neededInput":   blocker.NeededInput,
			"sinceRevision": blocker.SinceObjectiveRevision,
		}
	}
	return goal
}

func commandGoalCompletionCriteria(item protocol.Goal) []string {
	value, ok := item.Metadata[protocol.GoalMetadataCompletionCriteria]
	if !ok {
		return []string{}
	}
	var values []string
	switch typed := value.(type) {
	case []string:
		values = typed
	case []any:
		values = make([]string, 0, len(typed))
		for _, entry := range typed {
			text, textOK := entry.(string)
			if !textOK {
				return []string{}
			}
			values = append(values, text)
		}
	default:
		return []string{}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func commandGoalStatus(status protocol.GoalStatus) string {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusUsageLimited:
		return "usageLimited"
	case protocol.GoalStatusBudgetLimited:
		return "budgetLimited"
	default:
		return string(protocol.NormalizeGoalStatus(status))
	}
}

func int64PointerValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func completionBudgetReport(item *protocol.Goal) string {
	return completionUsageCheckpointReport(item)
}

func completionUsageCheckpointReport(item *protocol.Goal) string {
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusComplete {
		return ""
	}
	return "Goal achieved. Use the next final response as the complete user-facing delivery. It must stand on its own and satisfy `goal.objective`: include the full requested content when content itself is the deliverable; for files or artifacts, provide exact links or paths; for implementation, research, or external-state work, present the key outcomes and relevant verification. Do not make Goal completion the headline or replace the result with a completion notice or brief summary; mention completion only secondarily if useful. Then stop and wait for user input."
}
