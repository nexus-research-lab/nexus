package memory

import (
	"errors"
	"path/filepath"
	"strings"
)

// SearchMatch 表示检索结果。
type SearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Slice 表示文件片段。
type Slice struct {
	Path     string `json:"path"`
	FromLine int    `json:"from_line"`
	ToLine   int    `json:"to_line"`
	Content  string `json:"content"`
}

// Repository 负责管理 workspace 记忆文件。
type Repository struct {
	workspacePath string
	parser        Parser
}

var (
	rootMemoryFiles  = []string{"MEMORY.md", "SOUL.md", "TOOLS.md", "AGENTS.md", "RUNBOOK.md"}
	errEntryNotFound = errors.New("条目未找到")
)

// NewRepository 创建记忆仓储。
func NewRepository(workspacePath string) *Repository {
	return &Repository{
		workspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
		parser:        Parser{},
	}
}
