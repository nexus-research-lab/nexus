// INPUT: 用户确认的 exact owner/source Draft 与可选命令名、标题、描述。
// OUTPUT: 同事务持久化的草图版本和命名工作图；成功后立即失效 Slash 目录。
// POS: 已生成草图的确定性保存边界，不调用模型、不启动后台 round。
package workgraphworkflow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workflowstore "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
)

// ConfirmSave 直接保存用户已确认的草图；表单只能修改元信息，图结构来自 durable Draft。
func (s *Service) ConfirmSave(ctx context.Context, ownerUserID string, request protocol.ConfirmWorkGraphWorkflowSaveRequest) (*protocol.WorkGraphWorkflowSaveReceipt, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.PreviewID == "" {
		return nil, ErrInvalidInput
	}
	if s == nil || s.repository == nil {
		return nil, errorsUnavailableDraftPersistence()
	}
	draft, err := s.GetDraft(ctx, ownerUserID, request.SourceSessionKey, request.PreviewID)
	if err != nil {
		return nil, err
	}
	if request.HeadRevision <= 0 || request.HeadRevision > draft.HeadRevision ||
		request.SelectedRevision <= 0 || request.SelectedRevision > request.HeadRevision {
		return nil, ErrRevisionConflict
	}
	preview := cloneWorkflowPreview(draft.Preview)
	fresh := request.HeadRevision == draft.HeadRevision && request.SelectedRevision == draft.SelectedRevision
	if !fresh {
		found := false
		for _, version := range draft.Versions {
			if version.Revision == request.SelectedRevision {
				preview, found = cloneWorkflowPreview(version.Preview), true
				break
			}
		}
		if !found {
			return nil, ErrRevisionConflict
		}
	}
	if name := normalizeSlashName(request.SlashName); name != "" {
		preview.SlashName = name
	}
	if title := strings.TrimSpace(request.Title); title != "" {
		preview.Title = title
	}
	if description := strings.TrimSpace(request.Description); description != "" {
		preview.Description = description
	}
	if err = validateWorkflowMetadata(preview.SlashName, preview.Title, preview.Description); err != nil {
		return nil, err
	}
	if !fresh {
		if draft.SavedWorkflowID != "" {
			saved, readErr := s.repository.GetByID(ctx, ownerUserID, draft.SavedWorkflowID)
			if readErr != nil {
				return nil, readErr
			}
			// A lost response may replay an already committed exact snapshot, but must never
			// advance the current Draft or overwrite a newer edit.
			if saved != nil && workflowMatchesPreview(*saved, preview) {
				return &protocol.WorkGraphWorkflowSaveReceipt{PreviewID: preview.PreviewID, Status: "saved", Workflow: saved}, nil
			}
		}
		return nil, ErrRevisionConflict
	}
	availability, err := s.CheckSlashNameAvailability(ctx, ownerUserID, preview.SlashName, preview.PreviewID)
	if err != nil {
		return nil, err
	}
	if !availability.Available {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	if draft.SavedWorkflowID == "" {
		workflows, listErr := s.repository.List(ctx, ownerUserID)
		if listErr != nil {
			return nil, listErr
		}
		if len(workflows) >= maxWorkflowCount {
			return nil, fmt.Errorf("%w: at most %d workflows are allowed", ErrInvalidInput, maxWorkflowCount)
		}
	}
	saved, err := s.persistDraft(ctx, *draft, preview, "")
	if err != nil {
		return nil, err
	}
	return &protocol.WorkGraphWorkflowSaveReceipt{PreviewID: preview.PreviewID, Status: "saved", Workflow: saved}, nil
}

// GetSaveState 只核对当前草稿与已生效命令，不重放保存，也不按旧 saved_revision 猜成功。
func (s *Service) GetSaveState(ctx context.Context, owner, sourceSessionKey, previewID string) (*protocol.WorkGraphWorkflowSaveState, error) {
	draft, err := s.GetDraft(ctx, owner, sourceSessionKey, previewID)
	if err != nil {
		return nil, err
	}
	state := &protocol.WorkGraphWorkflowSaveState{Preview: draft.Preview, Status: "unsaved"}
	if draft.SavedWorkflowID == "" {
		return state, nil
	}
	state.Workflow, err = s.repository.GetByID(ctx, owner, draft.SavedWorkflowID)
	if err != nil || state.Workflow == nil {
		return state, err
	}
	for _, version := range draft.Versions {
		if workflowMatchesPreview(*state.Workflow, version.Preview) {
			state.SavedRevision = version.Revision
		}
	}
	if workflowMatchesPreview(*state.Workflow, draft.Preview) {
		state.Status = "saved"
		state.SavedRevision = draft.SelectedRevision
	}
	return state, nil
}

func (s *Service) persistDraft(ctx context.Context, draft protocol.WorkGraphWorkflowDraft, preview protocol.WorkGraphWorkflowPreview, workflowID string) (*protocol.WorkGraphWorkflow, error) {
	repository, ok := s.repository.(DraftRepository)
	if !ok {
		return nil, errorsUnavailableDraftPersistence()
	}
	if draft.SavedWorkflowID != "" {
		workflowID = draft.SavedWorkflowID
	}
	if workflowID == "" {
		workflowID = workflowIDForCommand(draft.OwnerUserID, "preview:"+draft.PreviewID)
	}
	target, err := s.repository.GetByID(ctx, draft.OwnerUserID, workflowID)
	if err != nil {
		return nil, err
	}
	if target == nil && draft.SavedWorkflowID != "" {
		return nil, ErrNotFound
	}
	now := s.now().UTC()
	workflow := protocol.WorkGraphWorkflow{
		ID: workflowID, OwnerUserID: draft.OwnerUserID,
		SlashName: preview.SlashName, Title: preview.Title, Description: preview.Description,
		SourceExecutionID: preview.SourceExecutionID, SourceSessionKey: preview.SourceSessionKey,
		Objective: preview.Objective, CompletionCriteria: slices.Clone(preview.CompletionCriteria),
		Nodes: cloneWorkflowNodes(preview.Nodes), Dependencies: slices.Clone(preview.Dependencies),
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	expectedVersion := int64(0)
	changed := true
	if target != nil {
		expectedVersion = target.Version
		workflow.Version = expectedVersion + 1
		workflow.CreatedAt = target.CreatedAt
		if workflowMatchesPreview(*target, preview) {
			workflow = *target
			changed = false
		}
	}
	saved, err := repository.SaveDraft(ctx, draft, preview, workflow, expectedVersion, now)
	if err != nil {
		if errors.Is(err, workflowstore.ErrRevisionConflict) {
			return nil, fmt.Errorf("%w: confirmed Draft changed", ErrRevisionConflict)
		}
		if duplicateWorkflowError(err) {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
		}
		return nil, err
	}
	if preview.SlashName != draft.Preview.SlashName || preview.Title != draft.Preview.Title || preview.Description != draft.Preview.Description {
		draft.HeadRevision++
		draft.SelectedRevision = draft.HeadRevision
		draft.Versions = append(draft.Versions, protocol.WorkGraphWorkflowPreviewVersion{
			Revision: draft.HeadRevision, Preview: cloneWorkflowPreview(preview), CreatedAt: now,
		})
	}
	draft.Preview = cloneWorkflowPreview(preview)
	draft.SavedWorkflowID, draft.SavedRevision, draft.SaveScheduled = saved.ID, draft.SelectedRevision, false
	draft.UpdatedAt = now
	s.hydrateDraft(draft)
	if changed && s.onChanged != nil {
		s.onChanged(ctx, draft.OwnerUserID)
	}
	return saved, nil
}
