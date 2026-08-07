// INPUT: 启动时遗留的 Session deletion jobs。
// OUTPUT: 以原 owner 身份幂等重放 runtime、次级引用与文件产物清理。
// POS: Session 删除跨进程恢复入口。
package session

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
)

// ReconcilePendingDeletions 重放尚未完成的 Session 删除任务。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) error {
	if s == nil || s.deletion == nil {
		return nil
	}
	jobs, err := s.deletion.ListPending(ctx, deletionsvc.KindSession)
	if err != nil {
		return err
	}
	errList := make([]error, 0)
	for _, job := range jobs {
		var payload sessionDeletionPayload
		if decodeErr := deletionsvc.DecodePayload(job, &payload); decodeErr != nil {
			errList = append(errList, s.deletion.Fail(ctx, job, decodeErr))
			continue
		}
		ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: job.OwnerUserID,
			Role:   authctx.RoleOwner,
		})
		if applyErr := s.applySessionDeletion(ownerCtx, job, payload, false); applyErr != nil {
			errList = append(errList, applyErr)
		}
	}
	return errors.Join(errList...)
}
