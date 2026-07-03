package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	storagesubscription "github.com/nexus-research-lab/nexus/internal/storage/subscription"
)

var ErrInvalidInput = errors.New("invalid subscription input")

var planKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Service struct {
	repository *storagesubscription.Repository
	now        func() time.Time
}

func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: storagesubscription.NewRepository(cfg, db),
		now:        time.Now,
	}
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	now := s.now().UTC()
	periodStart, periodEnd := currentMonthlyPeriod(now)

	plans, err := s.repository.ListPlans(ctx)
	if err != nil {
		return Overview{}, err
	}
	accounts, err := s.repository.ListAccounts(ctx, periodStart, periodEnd)
	if err != nil {
		return Overview{}, err
	}

	overview := Overview{
		Plans:       make([]Plan, 0, len(plans)),
		Accounts:    make([]Account, 0, len(accounts)),
		PeriodStart: formatTime(periodStart),
		PeriodEnd:   formatTime(periodEnd),
		UpdatedAt:   formatTime(now),
	}
	for _, plan := range plans {
		overview.Plans = append(overview.Plans, mapPlan(plan))
	}
	for _, account := range accounts {
		overview.Accounts = append(overview.Accounts, mapAccount(account, periodStart, periodEnd))
	}
	return overview, nil
}

func (s *Service) CurrentAccount(ctx context.Context, ownerUserID string) (*Account, error) {
	now := s.now().UTC()
	periodStart, periodEnd := currentMonthlyPeriod(now)
	account, err := s.repository.GetAccount(ctx, strings.TrimSpace(ownerUserID), periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, nil
	}
	result := mapAccount(*account, periodStart, periodEnd)
	return &result, nil
}

func (s *Service) UpdateUserSubscription(ctx context.Context, input UpdateUserSubscriptionInput) (Overview, error) {
	normalized, err := normalizeUpdateUserSubscriptionInput(input)
	if err != nil {
		return Overview{}, err
	}

	plan, err := s.repository.GetPlan(ctx, normalized.PlanKey)
	if err != nil {
		return Overview{}, err
	}
	if plan == nil {
		return Overview{}, fmt.Errorf("%w: unknown plan_key", ErrInvalidInput)
	}

	now := s.now().UTC()
	periodStart, periodEnd := currentMonthlyPeriod(now)
	entity := storagesubscription.UserSubscriptionEntity{
		OwnerUserID: normalized.OwnerUserID,
		PlanKey:     normalized.PlanKey,
		PeriodStart: &periodStart,
		PeriodEnd:   &periodEnd,
		UpdatedAt:   now,
	}
	if err := s.repository.UpsertUserSubscription(ctx, entity); err != nil {
		return Overview{}, err
	}
	return s.Overview(ctx)
}

func (s *Service) UpsertPlan(ctx context.Context, input UpsertPlanInput) (Overview, error) {
	normalized, err := normalizeUpsertPlanInput(input)
	if err != nil {
		return Overview{}, err
	}
	if err := s.repository.UpsertPlan(ctx, storagesubscription.PlanEntity{
		PlanKey:           normalized.PlanKey,
		DisplayName:       normalized.DisplayName,
		Status:            normalized.Status,
		MonthlyTokenLimit: normalized.MonthlyTokenLimit,
		Notes:             normalized.Notes,
		SortOrder:         normalized.SortOrder,
		UpdatedAt:         s.now().UTC(),
	}); err != nil {
		return Overview{}, err
	}
	return s.Overview(ctx)
}

func normalizeUpdateUserSubscriptionInput(input UpdateUserSubscriptionInput) (UpdateUserSubscriptionInput, error) {
	normalized := UpdateUserSubscriptionInput{
		OwnerUserID: strings.TrimSpace(input.OwnerUserID),
		PlanKey:     strings.TrimSpace(input.PlanKey),
	}
	if normalized.OwnerUserID == "" {
		return UpdateUserSubscriptionInput{}, fmt.Errorf("%w: owner_user_id is required", ErrInvalidInput)
	}
	if normalized.PlanKey == "" {
		normalized.PlanKey = PlanFree
	}
	return normalized, nil
}

func normalizeUpsertPlanInput(input UpsertPlanInput) (UpsertPlanInput, error) {
	normalized := UpsertPlanInput{
		PlanKey:           strings.TrimSpace(input.PlanKey),
		DisplayName:       strings.TrimSpace(input.DisplayName),
		Status:            strings.TrimSpace(input.Status),
		MonthlyTokenLimit: input.MonthlyTokenLimit,
		Notes:             strings.TrimSpace(input.Notes),
		SortOrder:         input.SortOrder,
	}
	if !planKeyPattern.MatchString(normalized.PlanKey) {
		return UpsertPlanInput{}, fmt.Errorf("%w: invalid plan_key", ErrInvalidInput)
	}
	if normalized.DisplayName == "" {
		return UpsertPlanInput{}, fmt.Errorf("%w: display_name is required", ErrInvalidInput)
	}
	if normalized.Status == "" {
		normalized.Status = PlanStatusActive
	}
	if normalized.Status != PlanStatusActive && normalized.Status != PlanStatusArchived {
		return UpsertPlanInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
	if normalized.MonthlyTokenLimit != nil && *normalized.MonthlyTokenLimit < 0 {
		return UpsertPlanInput{}, fmt.Errorf("%w: monthly_token_limit must be non-negative", ErrInvalidInput)
	}
	if normalized.SortOrder == 0 {
		normalized.SortOrder = 100
	}
	return normalized, nil
}

func mapPlan(entity storagesubscription.PlanEntity) Plan {
	return Plan{
		PlanKey:           entity.PlanKey,
		DisplayName:       entity.DisplayName,
		Status:            entity.Status,
		MonthlyTokenLimit: entity.MonthlyTokenLimit,
		Notes:             entity.Notes,
		SortOrder:         entity.SortOrder,
	}
}

func mapAccount(entity storagesubscription.AccountEntity, fallbackStart time.Time, fallbackEnd time.Time) Account {
	limit := entity.MonthlyTokenLimit

	var usedPercent *float64
	if limit != nil && *limit > 0 {
		percent := float64(entity.UsedTokens) / float64(*limit) * 100
		usedPercent = &percent
	}

	periodStart := fallbackStart
	if entity.PeriodStart != nil {
		periodStart = *entity.PeriodStart
	}
	periodEnd := fallbackEnd
	if entity.PeriodEnd != nil {
		periodEnd = *entity.PeriodEnd
	}

	return Account{
		OwnerUserID:       entity.OwnerUserID,
		Username:          entity.Username,
		DisplayName:       entity.DisplayName,
		Role:              entity.Role,
		UserStatus:        entity.UserStatus,
		PlanKey:           entity.PlanKey,
		PlanName:          entity.PlanName,
		MonthlyTokenLimit: limit,
		UsedTokens:        entity.UsedTokens,
		UsedPercent:       usedPercent,
		SessionCount:      entity.SessionCount,
		MessageCount:      entity.MessageCount,
		PeriodStart:       formatTime(periodStart),
		PeriodEnd:         formatTime(periodEnd),
		CreatedAt:         formatTime(entity.CreatedAt),
		UpdatedAt:         formatTime(entity.UpdatedAt),
	}
}

func currentMonthlyPeriod(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
