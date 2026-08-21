// INPUT: 用户已经看过并确认保存的 exact preview_id 与 owner/session scope。
// OUTPUT: 不进入聊天时间线的内部 Agent 保存 round 调度回执。
// POS: 草图确认与 execution-orchestrator Skill + Nexus CLI 持久化之间的后台调度边界。
package workgraphworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const workGraphSavePurpose = "workgraph_distillation"

// SaveRoundRequest 是宿主内部 round 的最小可信输入；不包含草图节点或运行事实。
type SaveRoundRequest struct {
	OwnerUserID string
	SessionKey  string
	PreviewID   string
	Prompt      string
}

// SaveRoundDispatcher 由宿主把 exact preview 保存请求路由到 DM 或 Room 的隐藏 Agent round。
type SaveRoundDispatcher interface {
	DispatchWorkGraphSave(context.Context, SaveRoundRequest) error
}

// SetSaveRoundDispatcher 注入宿主内部 round 调度器；UI 不可直接调用持久化 service。
func (s *Service) SetSaveRoundDispatcher(dispatcher SaveRoundDispatcher) {
	if s != nil {
		s.saveDispatcher = dispatcher
	}
}

// ScheduleSave 校验 exact preview 后启动隐藏后台 round；真正落库仍由该 round 内的 CLI 完成。
func (s *Service) ScheduleSave(
	ctx context.Context,
	ownerUserID string,
	request protocol.ScheduleWorkGraphWorkflowSaveRequest,
) (*protocol.WorkGraphWorkflowSaveReceipt, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	request.SlashName = normalizeSlashName(request.SlashName)
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.PreviewID == "" {
		return nil, fmt.Errorf("%w: source session and preview_id are required", ErrInvalidInput)
	}
	if s == nil || s.repository == nil || s.saveDispatcher == nil {
		return nil, errors.New("workgraph background save dispatcher is unavailable")
	}
	preview, alreadyScheduled, err := s.claimPreviewForSave(
		ownerUserID,
		request.SourceSessionKey,
		request.PreviewID,
		request.SlashName,
		request.Title,
		request.Description,
	)
	if err != nil {
		return nil, err
	}
	if alreadyScheduled {
		return scheduledSaveReceipt(preview.PreviewID), nil
	}
	existing, err := s.repository.GetBySlashName(ctx, ownerUserID, preview.SlashName)
	if err != nil {
		s.releasePreviewSaveClaim(ownerUserID, preview.PreviewID)
		return nil, err
	}
	if existing != nil {
		s.releasePreviewSaveClaim(ownerUserID, preview.PreviewID)
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	dispatchRequest := SaveRoundRequest{
		OwnerUserID: ownerUserID,
		SessionKey:  preview.SourceSessionKey,
		PreviewID:   preview.PreviewID,
		Prompt:      renderBackgroundSavePrompt(preview),
	}
	if err = s.saveDispatcher.DispatchWorkGraphSave(ctx, dispatchRequest); err != nil {
		s.releasePreviewSaveClaim(ownerUserID, preview.PreviewID)
		return nil, err
	}
	return scheduledSaveReceipt(preview.PreviewID), nil
}

func scheduledSaveReceipt(previewID string) *protocol.WorkGraphWorkflowSaveReceipt {
	return &protocol.WorkGraphWorkflowSaveReceipt{
		PreviewID: strings.TrimSpace(previewID),
		Status:    "scheduled",
	}
}

func renderBackgroundSavePrompt(preview protocol.WorkGraphWorkflowPreview) string {
	return fmt.Sprintf(`这是用户在 WorkGraph 草图确认界面发起的内部后台保存任务，不要输出面向用户的对话说明。

请使用 execution-orchestrator Skill 和受管 Nexus CLI，原样保存用户已经确认的 exact WorkGraph 草图。

草图 preview_id：%s
保存后命令：/%s

请读取 distill_workgraph 的 fresh contract，只提交 preview_id。不要重新读取源图、重新选择节点、重做抽象或改写草图。`,
		preview.PreviewID,
		preview.SlashName,
	)
}
