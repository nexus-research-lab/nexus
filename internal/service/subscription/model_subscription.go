package subscription

const (
	PlanFree  = "free"
	PlanAdmin = "admin"

	PlanStatusActive   = "active"
	PlanStatusArchived = "archived"
)

type Plan struct {
	PlanKey           string `json:"plan_key"`
	DisplayName       string `json:"display_name"`
	Status            string `json:"status"`
	MonthlyTokenLimit *int64 `json:"monthly_token_limit"`
	Notes             string `json:"notes"`
	SortOrder         int    `json:"sort_order"`
}

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
	Notes             string   `json:"notes"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type Overview struct {
	Plans       []Plan    `json:"plans"`
	Accounts    []Account `json:"accounts"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`
	UpdatedAt   string    `json:"updated_at"`
}

type UpdateUserSubscriptionInput struct {
	OwnerUserID string `json:"owner_user_id"`
	PlanKey     string `json:"plan_key"`
}

type UpsertPlanInput struct {
	PlanKey           string `json:"plan_key"`
	DisplayName       string `json:"display_name"`
	Status            string `json:"status"`
	MonthlyTokenLimit *int64 `json:"monthly_token_limit"`
	Notes             string `json:"notes"`
	SortOrder         int    `json:"sort_order"`
}
