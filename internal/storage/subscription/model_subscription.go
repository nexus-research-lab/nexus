package subscription

import "time"

// PlanEntity 是订阅套餐的持久化形状。
type PlanEntity struct {
	PlanKey           string
	DisplayName       string
	Status            string
	MonthlyTokenLimit *int64
	Notes             string
	SortOrder         int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UserSubscriptionEntity 是单个用户订阅配置的持久化形状。
type UserSubscriptionEntity struct {
	OwnerUserID string
	PlanKey     string
	PeriodStart *time.Time
	PeriodEnd   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AccountEntity 是运营页需要展示的账号订阅与用量聚合。
type AccountEntity struct {
	OwnerUserID       string
	Username          string
	DisplayName       string
	Role              string
	UserStatus        string
	PlanKey           string
	PlanName          string
	MonthlyTokenLimit *int64
	UsedTokens        int64
	SessionCount      int64
	MessageCount      int64
	PeriodStart       *time.Time
	PeriodEnd         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
