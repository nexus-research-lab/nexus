// INPUT: exact owner/Session、completed WorkGraph、Draft preview_id 与完整 revision。
// OUTPUT: 会话内统一的来源图/Draft/命名图目录、提取、查询、修改、版本选择和保存边界。
// POS: execution-orchestrator Skill 与 UI 共用的 WorkGraph authoring 业务入口。
package workgraphworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type executionHistoryViewer interface {
	ListHistoryViews(context.Context, string, string, int) ([]protocol.ExecutionView, error)
}

// PreviewSavedWorkflow 为能力目录中的命名图恢复或创建同源 Draft。
func (s *Service) PreviewSavedWorkflow(
	ctx context.Context,
	ownerUserID string,
	workflowID string,
	outputLanguage string,
) (*protocol.WorkGraphWorkflowPreview, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	workflowID = strings.TrimSpace(workflowID)
	outputLanguage = normalizeWorkflowOutputLanguage(outputLanguage)
	if ownerUserID == "" || workflowID == "" || outputLanguage == "" {
		return nil, fmt.Errorf("%w: saved WorkGraph preview scope is incomplete", ErrInvalidInput)
	}
	drafts, ok := s.repository.(DraftRepository)
	if !ok {
		return nil, errorsUnavailableDraftPersistence()
	}
	if existing, err := drafts.GetDraftBySavedWorkflowID(ctx, ownerUserID, workflowID); err != nil {
		return nil, err
	} else if existing != nil {
		if err = s.renewDraftLease(ctx, drafts, existing); err != nil {
			return nil, err
		}
		s.hydrateDraft(*existing)
		preview := cloneWorkflowPreview(existing.Preview)
		return &preview, nil
	}
	workflow, err := s.repository.GetByID(ctx, ownerUserID, workflowID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, ErrNotFound
	}
	if existing, lookupErr := drafts.GetDraftBySource(
		ctx, ownerUserID, workflow.SourceSessionKey, workflow.SourceExecutionID,
	); lookupErr != nil {
		return nil, lookupErr
	} else if existing != nil {
		if err = s.renewDraftLease(ctx, drafts, existing); err != nil {
			return nil, err
		}
		if existing.SavedWorkflowID != "" && existing.SavedWorkflowID != workflow.ID {
			return nil, fmt.Errorf("%w: source Draft belongs to another named WorkGraph", ErrInvalidInput)
		}
		if err = drafts.SetDraftSaveState(
			ctx, ownerUserID, existing.PreviewID, false, workflow.ID,
			existing.SelectedRevision, s.now().UTC(),
		); err != nil {
			return nil, err
		}
		existing.SavedWorkflowID = workflow.ID
		existing.SavedRevision = existing.SelectedRevision
		existing.SaveScheduled = false
		s.hydrateDraft(*existing)
		preview := cloneWorkflowPreview(existing.Preview)
		return &preview, nil
	}
	if s.agents == nil {
		return nil, errorsUnavailableDraftPersistence()
	}
	mainAgent, err := s.agents.GetDefaultAgent(ctx)
	if err != nil {
		return nil, err
	}
	if mainAgent == nil || strings.TrimSpace(mainAgent.OwnerUserID) != ownerUserID || strings.TrimSpace(mainAgent.AgentID) == "" {
		return nil, fmt.Errorf("%w: Nexus main Agent is unavailable", ErrInvalidInput)
	}
	now := s.now().UTC()
	preview := protocol.WorkGraphWorkflowPreview{
		PreviewID: newPreviewID(), SlashName: workflow.SlashName,
		Title: workflow.Title, Description: workflow.Description,
		SourceExecutionID: workflow.SourceExecutionID, SourceSessionKey: workflow.SourceSessionKey,
		Objective: workflow.Objective, CompletionCriteria: append([]string(nil), workflow.CompletionCriteria...),
		Nodes: cloneWorkflowNodes(workflow.Nodes), Dependencies: append([]protocol.WorkGraphWorkflowDependency(nil), workflow.Dependencies...),
		ExpiresAt: now.Add(workflowPreviewTTL),
	}
	created, err := drafts.CreateDraft(ctx, protocol.WorkGraphWorkflowDraft{
		PreviewID: preview.PreviewID, OwnerUserID: ownerUserID,
		SourceExecutionID: preview.SourceExecutionID, SourceSessionKey: preview.SourceSessionKey,
		SourceAgentID: strings.TrimSpace(mainAgent.AgentID), OutputLanguage: outputLanguage,
		HeadRevision: 1, SelectedRevision: 1, Preview: cloneWorkflowPreview(preview),
		Versions:        []protocol.WorkGraphWorkflowPreviewVersion{{Revision: 1, Preview: cloneWorkflowPreview(preview), CreatedAt: now}},
		SavedWorkflowID: workflow.ID, SavedRevision: 1,
		ExpiresAt: preview.ExpiresAt, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	s.hydrateDraft(*created)
	result := cloneWorkflowPreview(created.Preview)
	return &result, nil
}

// InspectLibrary 返回当前 Session 的 completed source、Draft 及 owner 命名图。
func (s *Service) InspectLibrary(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
) (*protocol.WorkGraphWorkflowLibrary, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sourceSessionKey = strings.TrimSpace(sourceSessionKey)
	if ownerUserID == "" || sourceSessionKey == "" {
		return nil, fmt.Errorf("%w: WorkGraph library scope is incomplete", ErrInvalidInput)
	}
	result := &protocol.WorkGraphWorkflowLibrary{}
	if history, ok := s.executions.(executionHistoryViewer); ok {
		views, err := history.ListHistoryViews(ctx, ownerUserID, sourceSessionKey, 40)
		if err != nil {
			return nil, err
		}
		for _, view := range views {
			if view.Status != protocol.ExecutionStatusCompleted {
				continue
			}
			result.Sources = append(result.Sources, protocol.WorkGraphWorkflowSourceSummary{
				ExecutionID: view.ID, Status: view.Status, Objective: view.Objective,
				NodeCount: len(view.WorkItems), UpdatedAt: view.UpdatedAt,
			})
		}
	}
	drafts, err := s.ListDrafts(ctx, ownerUserID, sourceSessionKey)
	if err != nil {
		return nil, err
	}
	result.Drafts = drafts
	result.Workflows, err = s.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetDraft 返回 exact source Session 中当前选中版本和完整不可变历史。
func (s *Service) GetDraft(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	previewID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := s.loadDraftByID(ctx, strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID))
	if err != nil {
		return nil, err
	}
	if draft == nil || draft.SourceSessionKey != strings.TrimSpace(sourceSessionKey) {
		return nil, ErrNotFound
	}
	return draft, nil
}

// ReviseDraftPreview 让普通对话中的 Skill 与隐藏编辑 Session 共用完整草图 CAS。
func (s *Service) ReviseDraftPreview(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	request protocol.ReviseWorkGraphWorkflowDraftRequest,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := s.GetDraft(ctx, ownerUserID, sourceSessionKey, request.PreviewID)
	if err != nil {
		return nil, err
	}
	if request.Revision != draft.HeadRevision {
		return nil, fmt.Errorf("%w: Draft head revision changed", ErrInvalidInput)
	}
	next, err := normalizeAndValidateEditorPreview(draft.Preview, request.ReviseWorkGraphWorkflowPreviewRequest)
	if err != nil {
		return nil, err
	}
	if _, reserved := reservedWorkflowSlashNames[next.SlashName]; reserved {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
	}
	existing, err := s.repository.GetBySlashName(ctx, strings.TrimSpace(ownerUserID), next.SlashName)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != draft.SavedWorkflowID {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
	}
	repository, ok := s.repository.(DraftRepository)
	if !ok {
		return nil, errorsUnavailableDraftPersistence()
	}
	now := s.now().UTC()
	updated, err := repository.AppendDraftVersion(
		ctx, strings.TrimSpace(ownerUserID), draft.PreviewID, draft.HeadRevision,
		next, now, now.Add(workflowPreviewTTL),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: Draft head revision changed", ErrInvalidInput)
	}
	s.hydrateDraft(*updated)
	return cloneWorkflowDraft(updated), nil
}

// SelectDraftRevision 选择旧版本作为当前偏好基线；下一次模型修改仍以 head revision 做 CAS。
func (s *Service) SelectDraftRevision(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	previewID string,
	headRevision int64,
	selectedRevision int64,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := s.GetDraft(ctx, ownerUserID, sourceSessionKey, previewID)
	if err != nil {
		return nil, err
	}
	if draft.HeadRevision != headRevision || selectedRevision <= 0 {
		return nil, fmt.Errorf("%w: Draft head revision changed", ErrInvalidInput)
	}
	repository, ok := s.repository.(DraftRepository)
	if !ok {
		return nil, errorsUnavailableDraftPersistence()
	}
	updated, err := repository.SelectDraftVersion(
		ctx, strings.TrimSpace(ownerUserID), draft.PreviewID,
		headRevision, selectedRevision, s.now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: selected Draft version is unavailable", ErrInvalidInput)
	}
	s.hydrateDraft(*updated)
	return cloneWorkflowDraft(updated), nil
}

func errorsUnavailableDraftPersistence() error {
	return fmt.Errorf("%w: WorkGraph Draft persistence is unavailable", ErrInvalidInput)
}
