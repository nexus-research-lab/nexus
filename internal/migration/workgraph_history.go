// INPUT: Version 160 backlog, canonical Session transcripts and exact persisted runtime identities.
// OUTPUT: Retryable insert-only backfill, with completion committed atomically with recovered facts.
// POS: WorkGraph historical compatibility owner; normal view reads never visit transcripts.
package migration

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type workGraphHistorySource func(context.Context, string, string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error)

func RunWorkGraphHistoryBackfill(ctx context.Context, cfg config.Config, db *sql.DB, sessions *sessionsvc.Service, logger *slog.Logger) error {
	return backfillWorkGraphHistory(ctx, orchestrationstore.NewRepository(cfg, db), func(ctx context.Context, owner, session string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
		return readWorkGraphHistory(ctx, sessions, owner, session)
	}, logger)
}

type workGraphHistoryRepository interface {
	ListRuntimeHistoryBackfills(context.Context, protocol.Execution, int) ([]protocol.Execution, error)
	RuntimeGraphForMigration(context.Context, protocol.Execution) (protocol.ExecutionRuntimeGraph, error)
	CompleteRuntimeHistoryBackfill(context.Context, protocol.Execution, protocol.ExecutionRuntimeGraph) error
}

func backfillWorkGraphHistory(ctx context.Context, repository workGraphHistoryRepository, source workGraphHistorySource, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	var failures []error
	after := protocol.Execution{}
	var loadedOwner, loadedSession string
	var tasks []orchestrationsvc.RuntimeGraphSubagentTaskHistory
	var runs []orchestrationsvc.RuntimeGraphSubagentToolHistory
	var sourceErr error
	for {
		batch, err := repository.ListRuntimeHistoryBackfills(ctx, after, 32)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		if len(batch) == 0 {
			return errors.Join(failures...)
		}
		for _, execution := range batch {
			if err := ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			after = execution
			ownerContext := authctx.WithPrincipal(ctx, &authctx.Principal{UserID: execution.OwnerUserID, Role: authctx.RoleOwner})
			graph, err := repository.RuntimeGraphForMigration(ownerContext, execution)
			if err == nil && orchestrationsvc.RuntimeGraphNeedsHistoryBackfill(graph) {
				if loadedOwner != execution.OwnerUserID || loadedSession != execution.SessionKey {
					loadedOwner, loadedSession = execution.OwnerUserID, execution.SessionKey
					tasks, runs, sourceErr = source(ownerContext, execution.OwnerUserID, execution.SessionKey)
				}
				err = sourceErr
				if err == nil {
					graph = orchestrationsvc.RecoverRuntimeGraphHistory(execution.OwnerUserID, execution.SessionKey, graph, tasks, runs)
				}
			}
			if err == nil {
				for i := range graph.Nodes {
					graph.Nodes[i] = orchestrationsvc.PrepareRuntimeGraphRead(graph.Nodes[i])
				}
				err = repository.CompleteRuntimeHistoryBackfill(ctx, execution, graph)
			}
			if err != nil {
				failures = append(failures, err)
				logger.Warn("WorkGraph history remains pending", "execution_id", execution.ID, "err", err)
			}
		}
	}
}
