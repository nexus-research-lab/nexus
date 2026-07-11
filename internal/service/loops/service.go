package loops

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

//go:embed data/catalog.json
var catalogPayload []byte

// ErrLoopNotFound 表示请求的 loop 不存在。
var ErrLoopNotFound = errors.New("loop not found")

var (
	catalogOnce  sync.Once
	catalogItems []Loop
	catalogErr   error
)

var hiddenLoopSlugs = map[string]struct{}{
	"ci-failure-watcher":          {},
	"dependency-audit-weekly":     {},
	"deploy-verification-loop":    {},
	"guardrails-learning-loop":    {},
	"post-edit-test-guard":        {},
	"post-merge-regression-guard": {},
	"pr-babysitter":               {},
	"pr-watch-loop":               {},
	"pre-commit-guard":            {},
	"ralph-story-executor":        {},
	"reflexion-debug-loop":        {},
	"security-audit-weekly":       {},
	"spec-first-ship":             {},
}

// Service 提供内置 loop catalog 查询。
type Service struct {
	items  []Loop
	bySlug map[string]Loop
}

// NewService 创建内置 loop catalog 服务。
func NewService() *Service {
	items, err := loadCatalog()
	if err != nil {
		panic(fmt.Sprintf("加载 loop catalog 失败: %v", err))
	}
	visibleItems := visibleLoops(items)
	bySlug := make(map[string]Loop, len(visibleItems))
	for _, item := range visibleItems {
		bySlug[item.Slug] = item
	}
	return &Service{items: visibleItems, bySlug: bySlug}
}

// StaticCount 返回内置 catalog 数量，用于能力摘要。
func StaticCount() int {
	items, err := loadCatalog()
	if err != nil {
		return 0
	}
	return len(visibleLoops(items))
}

// ListLoops 返回按语言本地化后的 loop 列表。
func (s *Service) ListLoops(_ context.Context, locale string) []Loop {
	if s == nil {
		return nil
	}
	items := make([]Loop, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item.Localized(locale))
	}
	return items
}

// GetLoop 返回按语言本地化后的 loop 详情。
func (s *Service) GetLoop(_ context.Context, slug string, locale string) (Loop, error) {
	if s == nil {
		return Loop{}, ErrLoopNotFound
	}
	item, ok := s.bySlug[strings.TrimSpace(slug)]
	if !ok {
		return Loop{}, ErrLoopNotFound
	}
	return item.Localized(locale), nil
}

func loadCatalog() ([]Loop, error) {
	catalogOnce.Do(func() {
		if err := json.Unmarshal(catalogPayload, &catalogItems); err != nil {
			catalogErr = err
			return
		}
	})
	if catalogErr != nil {
		return nil, catalogErr
	}
	return catalogItems, nil
}

func visibleLoops(items []Loop) []Loop {
	visible := make([]Loop, 0, len(items))
	for _, item := range items {
		if _, hidden := hiddenLoopSlugs[item.Slug]; hidden || item.TriggerType != "manual" {
			continue
		}
		visible = append(visible, item)
	}
	return visible
}
