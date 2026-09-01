// INPUT: 当前 owner 与页面持有的创建 request_id。
// OUTPUT: 未受理、已提交或已删除的持久创建结果，不重放创建副作用。
// POS: Automation 创建后的权威对账入口；供刷新和响应丢失恢复使用。
package automation

import (
	"context"
	"errors"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetTaskCreateRequestStatus 按当前 owner 查询一次创建意图的持久结果。
func (s *Service) GetTaskCreateRequestStatus(
	ctx context.Context,
	requestID string,
) (*automationdomain.ScheduledTaskCreateRequestStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, errors.New("request_id is required")
	}
	if len(requestID) > 128 {
		return nil, errors.New("request_id must not exceed 128 characters")
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("scheduled task create request lookup requires an owner")
	}
	task, found, err := s.repository.GetScheduledTaskCreateRequestResult(ctx, ownerUserID, requestID)
	if err != nil {
		return nil, err
	}
	result := &automationdomain.ScheduledTaskCreateRequestStatus{
		RequestID: requestID,
		Status:    automationdomain.TaskCreateRequestStatusNotFound,
	}
	if !found {
		return result, nil
	}
	if task == nil {
		result.Status = automationdomain.TaskCreateRequestStatusGone
		return result, nil
	}
	projected := projectTaskPermissionPolicy(*task)
	state := s.ensureJobState(projected)
	enriched := s.scheduledTaskRuntimeSnapshot(projected, state)
	result.Status = automationdomain.TaskCreateRequestStatusCommitted
	result.Task = &enriched
	return result, nil
}
