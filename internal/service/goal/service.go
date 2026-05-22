package goal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Service 负责 Goal 状态机、审计事件和后续运行时决策。
type Service struct {
	config    config.Config
	repo      Repository
	nowFn     func() time.Time
	idFactory func(string) string
}

// NewService 创建 Goal 服务。
func NewService(cfg config.Config, repo Repository) *Service {
	return &Service{
		config:    cfg,
		repo:      repo,
		nowFn:     func() time.Time { return time.Now().UTC() },
		idFactory: newID,
	}
}

// Create 创建当前 Goal。
func (s *Service) Create(ctx context.Context, request protocol.CreateGoalRequest) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	sessionKey, objective, err := validateCreateRequest(request)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return nil, ErrGoalConflict
	}

	now := s.nowFn()
	tokenBudget := normalizeBudget(request.TokenBudget, s.config.GoalDefaultTokenBudget)
	item := protocol.Goal{
		ID:          s.idFactory("goal"),
		SessionKey:  sessionKey,
		Objective:   objective,
		Status:      protocol.GoalStatusActive,
		TokenBudget: tokenBudget,
		Version:     1,
		CreatedBy:   strings.TrimSpace(request.CreatedBy),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    cloneMap(request.Metadata),
	}
	created, err := s.repo.CreateGoal(ctx, item)
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *created, "created", protocol.GoalUpdateSourceUser, "", map[string]any{"objective": created.Objective}); err != nil {
		return nil, err
	}
	return created, nil
}

// Current 返回 session 当前 Goal。
func (s *Service) Current(ctx context.Context, sessionKey string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return item, nil
}

// Update 更新当前 Goal 文本、预算或 metadata。
func (s *Service) Update(ctx context.Context, goalID string, request protocol.UpdateGoalRequest) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	changed := false
	if request.Objective != nil {
		objective := strings.TrimSpace(*request.Objective)
		if objective == "" {
			return nil, ErrGoalInvalidInput
		}
		item.Objective = objective
		changed = true
	}
	if request.TokenBudget != nil {
		item.TokenBudget = normalizeBudget(request.TokenBudget, 0)
		changed = true
	}
	if request.Metadata != nil {
		item.Metadata = cloneMap(request.Metadata)
		changed = true
	}
	if !changed {
		return item, nil
	}
	updated, err := s.persistTransition(ctx, *item, item.Status, protocol.GoalUpdateSourceUser, "updated", "", nil)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Pause 暂停 active Goal。
func (s *Service) Pause(ctx context.Context, goalID string) (*protocol.Goal, error) {
	return s.changeStatus(ctx, goalID, protocol.GoalStatusPaused, protocol.GoalUpdateSourceUser, "paused", "", nil)
}

// Resume 恢复 paused/blocked Goal。
func (s *Service) Resume(ctx context.Context, goalID string) (*protocol.Goal, error) {
	return s.changeStatus(ctx, goalID, protocol.GoalStatusActive, protocol.GoalUpdateSourceUser, "resumed", "", nil)
}

// Clear 清除当前 Goal。
func (s *Service) Clear(ctx context.Context, goalID string) (*protocol.Goal, error) {
	return s.changeStatus(ctx, goalID, protocol.GoalStatusCleared, protocol.GoalUpdateSourceUser, "cleared", "", nil)
}

func (s *Service) changeStatus(
	ctx context.Context,
	goalID string,
	status protocol.GoalStatus,
	source protocol.GoalUpdateSource,
	eventType string,
	roundID string,
	payload map[string]any,
) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	return s.persistTransition(ctx, *item, status, source, eventType, roundID, payload)
}

func (s *Service) loadMutableGoal(ctx context.Context, goalID string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	if !protocol.IsCurrentGoalStatus(item.Status) {
		return nil, ErrGoalInvalidState
	}
	return item, nil
}

func (s *Service) persistTransition(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	source protocol.GoalUpdateSource,
	eventType string,
	roundID string,
	payload map[string]any,
) (*protocol.Goal, error) {
	status = protocol.NormalizeGoalStatus(status)
	if !canTransition(source, item.Status, status) {
		return nil, ErrGoalInvalidState
	}
	expectedVersion := item.Version
	now := s.nowFn()
	item.Status = status
	item.Version++
	item.UpdatedAt = now
	switch status {
	case protocol.GoalStatusComplete:
		item.CompletedAt = &now
	case protocol.GoalStatusBlocked:
		item.BlockedAt = &now
	case protocol.GoalStatusCleared:
		item.ClearedAt = &now
	}
	updated, err := s.repo.UpdateGoal(ctx, item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *updated, eventType, source, roundID, payload); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) appendEvent(ctx context.Context, item protocol.Goal, eventType string, source protocol.GoalUpdateSource, roundID string, payload map[string]any) error {
	event := protocol.GoalEvent{
		ID:         s.idFactory("goal_event"),
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  eventType,
		Source:     source,
		RoundID:    strings.TrimSpace(roundID),
		Payload:    cloneMap(payload),
		CreatedAt:  s.nowFn(),
	}
	return s.repo.AppendEvent(ctx, event)
}

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
	objective := strings.TrimSpace(request.Objective)
	if objective == "" {
		return "", "", ErrGoalInvalidInput
	}
	return sessionKey, objective, nil
}

func normalizeBudget(input *int64, fallback int64) *int64 {
	if input != nil {
		if *input <= 0 {
			return nil
		}
		value := *input
		return &value
	}
	if fallback <= 0 {
		return nil
	}
	return &fallback
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func newID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}
