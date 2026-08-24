// INPUT: owner、canonical Slash 候选与可选 exact preview_id。
// OUTPUT: 名称是否未被固定命令或其他命名 WorkGraph 占用。
// POS: UI 预检与保存调度共用的唯一命名冲突判定；数据库唯一索引仍负责最终并发栅栏。
package workgraphworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CheckSlashNameAvailability 判断名称是否可用于新 Draft，或仍属于该 Draft 已保存的命名图。
func (s *Service) CheckSlashNameAvailability(
	ctx context.Context,
	ownerUserID string,
	slashName string,
	previewID string,
) (*protocol.WorkGraphWorkflowSlashNameAvailability, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	slashName = normalizeSlashName(slashName)
	previewID = strings.TrimSpace(previewID)
	if ownerUserID == "" || !workflowSlashNamePattern.MatchString(slashName) {
		return nil, fmt.Errorf("%w: slash_name is invalid", ErrInvalidInput)
	}
	if s == nil || s.repository == nil {
		return nil, errors.New("workgraph workflow service is unavailable")
	}
	result := &protocol.WorkGraphWorkflowSlashNameAvailability{SlashName: slashName}
	if _, reserved := reservedWorkflowSlashNames[slashName]; reserved {
		return result, nil
	}
	existing, err := s.repository.GetBySlashName(ctx, ownerUserID, slashName)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		result.Available = true
		return result, nil
	}
	savedWorkflowID, err := s.savedWorkflowIDForPreview(ctx, ownerUserID, previewID)
	if err != nil {
		return nil, err
	}
	result.Available = existing.ID == savedWorkflowID
	return result, nil
}

func (s *Service) savedWorkflowIDForPreview(
	ctx context.Context,
	ownerUserID string,
	previewID string,
) (string, error) {
	if previewID == "" {
		return "", nil
	}
	draft, err := s.loadDraftByID(ctx, ownerUserID, previewID)
	if err != nil {
		return "", err
	}
	if draft != nil {
		return strings.TrimSpace(draft.SavedWorkflowID), nil
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	record, ok := s.previews[previewCacheKey(ownerUserID, previewID)]
	if !ok || record.ownerUserID != ownerUserID {
		return "", nil
	}
	return strings.TrimSpace(record.savedWorkflowID), nil
}
