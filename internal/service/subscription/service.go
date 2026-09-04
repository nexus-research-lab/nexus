package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	storagesubscription "github.com/nexus-research-lab/nexus/internal/storage/subscription"
)

type Account struct {
	OwnerUserID       string   `json:"owner_user_id"`
	Username          string   `json:"username"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	UserStatus        string   `json:"user_status"`
	PlanKey           string   `json:"plan_key"`
	PlanName          string   `json:"plan_name"`
	MonthlyTokenLimit *int64   `json:"monthly_token_limit"`
	UsedTokens        int64    `json:"used_tokens"`
	UsedPercent       *float64 `json:"used_percent"`
	SessionCount      int64    `json:"session_count"`
	MessageCount      int64    `json:"message_count"`
	PeriodStart       string   `json:"period_start"`
	PeriodEnd         string   `json:"period_end"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type UsageAccount struct {
	ControlUserID string `json:"control_user_id"`
	UsedTokens    int64  `json:"used_tokens"`
	SessionCount  int64  `json:"session_count"`
	MessageCount  int64  `json:"message_count"`
}

type UsageOverview struct {
	Accounts    []UsageAccount `json:"accounts"`
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	UpdatedAt   string         `json:"updated_at"`
}

var (
	ErrEntitlementUnavailable = errors.New("Control entitlement projection unavailable")
	ErrQuotaExceeded          = errors.New("subscription token quota exceeded")
)

// QuotaExceededError 保留诊断用量，同时只向客户端暴露可行动的提示。
type QuotaExceededError struct {
	UsedTokens  int64
	LimitTokens int64
}

func (e QuotaExceededError) Error() string {
	return fmt.Sprintf("%s: used %d of %d monthly tokens", ErrQuotaExceeded, e.UsedTokens, e.LimitTokens)
}

func (e QuotaExceededError) Unwrap() error { return ErrQuotaExceeded }

func (e QuotaExceededError) ClientMessage() string {
	return "当前账号本月的订阅额度已全部用尽，暂时无法发起新的 Agent 请求。这是账号级月度额度，不是单条回复的输出长度限制。请升级套餐，或等待下个计费周期重置后再继续使用。"
}

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

// UsageOverview 只返回 Nexus 自有的本地用量事实，套餐和成员由 Control 组合。
func (s *Service) UsageOverview(ctx context.Context) (UsageOverview, error) {
	now := s.now().UTC()
	periodStart, periodEnd := currentMonthlyPeriod(now)
	records, err := s.repository.ListUsage(ctx, periodStart, periodEnd)
	if err != nil {
		return UsageOverview{}, err
	}
	accounts := make([]UsageAccount, 0, len(records))
	for _, record := range records {
		accounts = append(accounts, UsageAccount{
			ControlUserID: record.ControlUserID,
			UsedTokens:    record.UsedTokens,
			SessionCount:  record.SessionCount,
			MessageCount:  record.MessageCount,
		})
	}
	return UsageOverview{
		Accounts:    accounts,
		PeriodStart: formatTime(periodStart),
		PeriodEnd:   formatTime(periodEnd),
		UpdatedAt:   formatTime(now),
	}, nil
}

func (s *Service) CurrentAccount(ctx context.Context, ownerUserID string) (*Account, error) {
	normalizedOwnerUserID := strings.TrimSpace(ownerUserID)
	if normalizedOwnerUserID == authctx.SystemUserID {
		return nil, nil
	}
	now := s.now().UTC()
	periodStart, periodEnd := currentMonthlyPeriod(now)
	account, err := s.repository.GetAccount(ctx, normalizedOwnerUserID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrEntitlementUnavailable
	}
	result := mapAccount(*account, periodStart, periodEnd)
	return &result, nil
}

// EnsureQuotaAvailable 在账号达到 Control 投影的月度 token 额度后阻止新 runtime 请求。
func (s *Service) EnsureQuotaAvailable(ctx context.Context, ownerUserID string) error {
	account, err := s.CurrentAccount(ctx, ownerUserID)
	if err != nil || account == nil || account.MonthlyTokenLimit == nil {
		return err
	}
	if account.UsedTokens >= *account.MonthlyTokenLimit {
		return QuotaExceededError{
			UsedTokens:  account.UsedTokens,
			LimitTokens: *account.MonthlyTokenLimit,
		}
	}
	return nil
}

func mapAccount(
	entity storagesubscription.AccountEntity,
	periodStart time.Time,
	periodEnd time.Time,
) Account {
	var usedPercent *float64
	if entity.MonthlyTokenLimit != nil && *entity.MonthlyTokenLimit > 0 {
		percent := float64(entity.UsedTokens) / float64(*entity.MonthlyTokenLimit) * 100
		usedPercent = &percent
	}
	return Account{
		OwnerUserID:       entity.OwnerUserID,
		Username:          entity.Username,
		DisplayName:       entity.DisplayName,
		Role:              entity.Role,
		UserStatus:        entity.UserStatus,
		PlanKey:           entity.PlanKey,
		PlanName:          entity.PlanName,
		MonthlyTokenLimit: entity.MonthlyTokenLimit,
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
