package agent

import (
	"context"
	"errors"
	"testing"
)

type deletionTestRepository struct {
	Repository
	deleteErr error
	deleted   int
}

func (r *deletionTestRepository) DeleteAgent(context.Context, string, string) error {
	r.deleted++
	return r.deleteErr
}

type deletionTestCoordinator struct {
	cleanupErr error
	beforeErr  error
}

func (c deletionTestCoordinator) CoordinateAgentDeletion(
	ctx context.Context,
	_ string,
	_ string,
	deletePersistent func(context.Context) error,
) error {
	if c.beforeErr != nil {
		return c.beforeErr
	}
	if err := deletePersistent(ctx); err != nil {
		return err
	}
	return c.cleanupErr
}

func TestDeleteAgentPersistenceMarksCommittedCleanupFailure(t *testing.T) {
	repository := &deletionTestRepository{}
	service := &Service{
		repository:          repository,
		deletionCoordinator: deletionTestCoordinator{cleanupErr: errors.New("runtime revoke failed")},
	}

	err := service.deleteAgentPersistence(context.Background(), "owner-a", "agent-a")
	if err == nil || !AgentDeletionCommitted(err) || repository.deleted != 1 {
		t.Fatalf("已提交删除的清理失败必须可识别: err=%v deleted=%d", err, repository.deleted)
	}
}

func TestDeleteAgentPersistenceDoesNotMarkPreDeleteFailureCommitted(t *testing.T) {
	repository := &deletionTestRepository{}
	service := &Service{
		repository:          repository,
		deletionCoordinator: deletionTestCoordinator{beforeErr: errors.New("impact snapshot failed")},
	}

	err := service.deleteAgentPersistence(context.Background(), "owner-a", "agent-a")
	if err == nil || AgentDeletionCommitted(err) || repository.deleted != 0 {
		t.Fatalf("删除前失败不得报告已提交: err=%v deleted=%d", err, repository.deleted)
	}
}
