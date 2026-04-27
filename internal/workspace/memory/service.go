package memory

import (
	"fmt"
	"strings"
	"time"
)

// LogResult 表示记录日记后的结果。
type LogResult struct {
	Path           string         `json:"path"`
	EntryID        string         `json:"entry_id"`
	Entry          string         `json:"entry"`
	Status         string         `json:"status"`
	Count          int            `json:"count"`
	RelatedEntries []ReviewItem   `json:"related_entries"`
	Promoted       *PromoteResult `json:"promoted,omitempty"`
}

// PromoteResult 表示长期提升结果。
type PromoteResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Service 负责检索、记录、提升和流转工作区记忆。
type Service struct {
	repository *Repository
	factory    Factory
	matcher    SimilarityMatcher
}

var promotionTargets = map[string]struct {
	Filename string
	Section  string
}{
	"memory": {Filename: "MEMORY.md", Section: "长期记忆"},
	"soul":   {Filename: "SOUL.md", Section: "行为准则"},
	"tools":  {Filename: "TOOLS.md", Section: "工具备忘"},
	"agents": {Filename: "AGENTS.md", Section: "执行规则"},
}

// NewService 创建记忆服务。
func NewService(workspacePath string) *Service {
	return &Service{
		repository: NewRepository(workspacePath),
		factory:    Factory{},
		matcher:    SimilarityMatcher{},
	}
}

// Search 执行关键词检索。
func (s *Service) Search(query string, limit int) ([]SearchMatch, error) {
	return s.repository.Search(query, limit)
}

// Get 读取文件片段。
func (s *Service) Get(relativePath string, fromLine int, lines int) (*Slice, error) {
	return s.repository.ReadSlice(relativePath, fromLine, lines)
}

// ReviewRecentEntries 返回近期日记摘要。
func (s *Service) ReviewRecentEntries(days int, limit int) ([]ReviewItem, error) {
	entries, err := s.repository.ListRecentEntries(days, limit)
	if err != nil {
		return nil, err
	}
	items := make([]ReviewItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.ReviewItem())
	}
	return items, nil
}

// BuildReviewMarkdown 构造注入 prompt 的近期摘要。
func (s *Service) BuildReviewMarkdown(days int, limit int, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 1200
	}
	items, err := s.ReviewRecentEntries(days, limit)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(items))
	total := 0
	for _, item := range items {
		line := fmt.Sprintf("- `%s`: %s (状态=%s, 次数=%d)", item.Path, strings.TrimPrefix(item.Headline, "### "), item.Status, item.Count)
		total += len(line)
		if total > maxChars {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// Log 记录今日日记。
func (s *Service) Log(kind string, title string, category string, fields []Field, promoteTarget string) (*LogResult, error) {
	preview, err := s.factory.Create(kind, title, category, fields, nil, zeroTime())
	if err != nil {
		return nil, err
	}
	candidates, err := s.repository.ListRecentEntries(90, 200)
	if err != nil {
		return nil, err
	}
	relatedEntries := s.matcher.FindRelated(preview, candidates, 5)
	entry, err := s.factory.Create(kind, title, category, fields, relatedEntries, zeroTime())
	if err != nil {
		return nil, err
	}
	path, err := s.repository.AppendEntry(entry)
	if err != nil {
		return nil, err
	}

	var promoted *PromoteResult
	target := strings.TrimSpace(promoteTarget)
	if target == "" {
		target = s.factory.InferAutoPromotionTarget(entry)
	}
	if target != "" {
		promotion, promoteErr := s.Promote(target, buildPromotionContent(entry), entry.Title, entry.ID)
		if promoteErr != nil {
			return nil, promoteErr
		}
		entry.SetStatus("promoted")
		entry.SetField("提升目标", target)
		promoted = promotion
	}

	items := make([]ReviewItem, 0, len(relatedEntries))
	for _, item := range relatedEntries {
		items = append(items, item.ReviewItem())
	}
	return &LogResult{
		Path:           path,
		EntryID:        entry.ID,
		Entry:          entry.Markdown(),
		Status:         entry.Status(),
		Count:          entry.Count(),
		RelatedEntries: items,
		Promoted:       promoted,
	}, nil
}

// Promote 把经验提升到长期文件。
func (s *Service) Promote(target string, content string, title string, entryID string) (*PromoteResult, error) {
	config, ok := promotionTargets[strings.ToLower(strings.TrimSpace(target))]
	if !ok {
		return nil, fmt.Errorf("不支持的提升目标: %s", target)
	}
	bullet := strings.TrimSpace(content)
	if strings.TrimSpace(title) != "" {
		bullet = fmt.Sprintf("- %s：%s", strings.TrimSpace(title), strings.TrimSpace(content))
	} else {
		bullet = "- " + bullet
	}
	path, err := s.repository.AppendToMemorySection(config.Filename, config.Section, bullet)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(entryID) != "" {
		if _, err = s.repository.UpdateEntry(entryID, func(entry *Entry) {
			entry.SetStatus("promoted")
			entry.SetField("提升目标", strings.ToLower(strings.TrimSpace(target)))
		}); err != nil {
			return nil, err
		}
	}
	return &PromoteResult{Path: path, Content: bullet}, nil
}

// ResolveEntry 把条目标记为已解决。
func (s *Service) ResolveEntry(entryID string, note string) (*ReviewItem, error) {
	entry, err := s.repository.UpdateEntry(entryID, func(item *Entry) {
		item.SetStatus("resolved")
		item.SetField("已解决", strings.TrimSpace(note))
	})
	if err != nil {
		return nil, err
	}
	result := entry.ReviewItem()
	return &result, nil
}

// SetEntryStatus 更新条目状态。
func (s *Service) SetEntryStatus(entryID string, status string, note string) (*ReviewItem, error) {
	entry, err := s.repository.UpdateEntry(entryID, func(item *Entry) {
		item.SetStatus(strings.TrimSpace(status))
		if strings.TrimSpace(note) != "" {
			item.SetField("状态说明", strings.TrimSpace(note))
		}
	})
	if err != nil {
		return nil, err
	}
	result := entry.ReviewItem()
	return &result, nil
}

func buildPromotionContent(entry *Entry) string {
	for _, key := range []string{"提升内容", "行动", "经验", "修复", "详情"} {
		value := strings.TrimSpace(entry.FieldValue(key))
		if value != "" {
			return value
		}
	}
	return entry.Title
}

func zeroTime() time.Time {
	return time.Time{}
}
