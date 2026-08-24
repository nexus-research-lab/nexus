// INPUT: 用户已经看过并确认保存的 exact preview_id、来源 scope 与 coordinator Agent。
// OUTPUT: 在独立隐藏 DM 中运行、且不续写来源 transcript 的内部 Agent 保存 round 调度回执。
// POS: 草图确认与 execution-orchestrator Skill + Nexus CLI 持久化之间的隔离后台调度边界。
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
	OwnerUserID      string
	AgentID          string
	SourceSessionKey string
	PreviewID        string
	Prompt           string
}

// SaveRoundDispatcher 由宿主把 exact preview 保存请求路由到独立隐藏 DM Agent round。
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
	preview, sourceAgentID, alreadyScheduled, err := s.claimPreviewForSave(
		ctx,
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
	availability, err := s.CheckSlashNameAvailability(
		ctx,
		ownerUserID,
		preview.SlashName,
		preview.PreviewID,
	)
	if err != nil {
		s.releasePreviewSaveClaim(ctx, ownerUserID, preview.PreviewID)
		return nil, err
	}
	if !availability.Available {
		s.releasePreviewSaveClaim(ctx, ownerUserID, preview.PreviewID)
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	dispatchRequest := SaveRoundRequest{
		OwnerUserID:      ownerUserID,
		AgentID:          sourceAgentID,
		SourceSessionKey: preview.SourceSessionKey,
		PreviewID:        preview.PreviewID,
		Prompt:           renderBackgroundSavePrompt(preview),
	}
	if err = s.saveDispatcher.DispatchWorkGraphSave(ctx, dispatchRequest); err != nil {
		s.releasePreviewSaveClaim(ctx, ownerUserID, preview.PreviewID)
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

语言要求：除命令、Skill 名称和标识符外，所有思考摘要、过程状态、工具调用说明和最终回复都必须使用简体中文，禁止输出英文叙述。

请使用 execution-orchestrator Skill 和受管的 Nexus 命令行工具，原样保存用户已经确认的精确 WorkGraph 草图。

草图标识 preview_id：%s
保存后的命令：/%s

请读取 distill_workgraph 的最新操作契约，只提交 preview_id。不要重新读取源图、重新选择节点、重做抽象或改写草图。任务完成后直接结束内部任务轮次；如果运行时要求输出过程或结束文本，也只能使用简体中文。`,
		preview.PreviewID,
		preview.SlashName,
	)
}
