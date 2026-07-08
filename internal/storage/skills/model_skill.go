package skills

import "time"

// SourceEntity 表示 skill 来源配置。
type SourceEntity struct {
	OwnerUserID   string
	SourceID      string
	Name          string
	Kind          string
	URL           string
	Trust         string
	Enabled       bool
	SortOrder     int
	LastCheckedAt *time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ImportedSkillEntity 表示已导入 skill 的数据库状态。
type ImportedSkillEntity struct {
	OwnerUserID     string
	SkillName       string
	Title           string
	Description     string
	Scope           string
	TagsJSON        string
	CategoryKey     string
	CategoryName    string
	Recommendation  string
	Version         string
	SourceID        string
	SourceKind      string
	SourceRef       string
	SourceName      string
	SourceTrust     string
	ImportMode      string
	GitURL          string
	GitBranch       string
	GitPath         string
	GitCommit       string
	RawURL          string
	DetailURL       string
	ContentHash     string
	UpdateAvailable bool
	LastImportedAt  *time.Time
	LastCheckedAt   *time.Time
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
