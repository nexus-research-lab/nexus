package provider

import "time"

// Entity 表示 provider 表的一行持久化记录。
type Entity struct {
	ID          string
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
	Enabled     bool
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
