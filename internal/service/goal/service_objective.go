package goal

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type objectiveRewriter interface {
	RewriteGoalObjective(context.Context, string, string) (string, error)
}

// SetObjectiveRewriter 注入可选的 Goal objective 改写器，用于把 UI 草稿整理成可执行目标。
func (s *Service) SetObjectiveRewriter(rewriter objectiveRewriter) {
	s.rewriter = rewriter
}

func (s *Service) rewriteCreateObjective(ctx context.Context, request protocol.CreateGoalRequest, objective string) (string, map[string]any) {
	metadata := cloneMap(request.Metadata)
	if skipObjectiveRewriteForCreatedBy(request.CreatedBy) {
		return objective, metadata
	}
	rewritten := s.rewriteObjectiveBestEffort(ctx, request.OwnerUserID, objective)
	if rewritten == "" || rewritten == objective {
		return objective, metadata
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["source_objective"] = objective
	metadata["objective_normalized"] = true
	return rewritten, metadata
}

func (s *Service) rewriteUpdateObjective(ctx context.Context, request protocol.UpdateGoalRequest, objective string, payload map[string]any) (string, map[string]any) {
	rewritten := s.rewriteObjectiveBestEffort(ctx, request.OwnerUserID, objective)
	if rewritten == "" || rewritten == objective {
		return objective, payload
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["source_objective"] = objective
	payload["objective_normalized"] = true
	return rewritten, payload
}

func (s *Service) rewriteObjectiveBestEffort(ctx context.Context, ownerUserID string, objective string) string {
	if s == nil || s.rewriter == nil {
		return ""
	}
	rewritten, err := s.rewriter.RewriteGoalObjective(ctx, strings.TrimSpace(ownerUserID), objective)
	if err != nil {
		return ""
	}
	normalized, err := normalizeObjective(rewritten)
	if err != nil {
		return ""
	}
	return normalized
}

func skipObjectiveRewriteForCreatedBy(createdBy string) bool {
	return strings.TrimSpace(createdBy) == "model"
}
