package goal

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) ensureEnabled() error {
	if s == nil || s.repo == nil {
		return ErrGoalDisabled
	}
	if !s.config.GoalEnabled {
		return ErrGoalDisabled
	}
	return nil
}

func validateCreateRequest(request protocol.CreateGoalRequest) (string, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	objective, err := normalizeObjective(request.Objective)
	if err != nil {
		return "", "", err
	}
	return sessionKey, objective, nil
}

func createGoalEventSource(createdBy string) protocol.GoalUpdateSource {
	switch strings.TrimSpace(createdBy) {
	case "model":
		return protocol.GoalUpdateSourceModel
	case "system":
		return protocol.GoalUpdateSourceSystem
	case "external", "app_server":
		return protocol.GoalUpdateSourceExternal
	default:
		return protocol.GoalUpdateSourceUser
	}
}

func normalizeObjective(input string) (string, error) {
	objective := strings.TrimSpace(input)
	if objective == "" {
		return "", newGoalInvalidInputError(goalObjectiveEmptyMessage)
	}
	if utf8.RuneCountInString(objective) > maxGoalObjectiveRunes {
		return "", newGoalInvalidInputError(goalObjectiveTooLongMessage)
	}
	return objective, nil
}

func normalizeCreateBudget(input *int64) (*int64, error) {
	if input == nil {
		return nil, nil
	}
	if *input <= 0 {
		return nil, newGoalInvalidInputError(goalBudgetPositiveMessage)
	}
	value := *input
	return &value, nil
}

func normalizeUpdateBudget(input *int64) (*int64, error) {
	if input == nil {
		return nil, nil
	}
	if *input <= 0 {
		return nil, newGoalInvalidInputError(goalBudgetPositiveMessage)
	}
	value := *input
	return &value, nil
}

func goalTokenBudgetEqual(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneMap(input map[string]any) map[string]any {
	return maps.Clone(input)
}

func renderGoalPromptTemplate(template string, values map[string]string) string {
	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{{ "+key+" }}", value)
	}
	return strings.NewReplacer(replacements...).Replace(template)
}

func newID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}
