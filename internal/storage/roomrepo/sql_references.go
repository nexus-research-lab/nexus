// INPUT: owner-scoped Room conversation identity and its canonical Room/member session keys.
// OUTPUT: whether any typed persistent record still references that conversation.
// POS: historical empty-conversation pruning safety boundary across SQLite/PostgreSQL schemas.
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type conversationReferenceQuerySpec struct {
	source                string
	from                  string
	ownerColumn           string
	sessionKeyColumns     []string
	conversationIDColumns []string
}

type conversationReferenceQuery struct {
	source string
	sql    string
	args   []any
}

// HasConversationReferences reports whether a conversation is still named by
// any typed persistent business record outside its own sessions/messages.
//
// Every query is owner-scoped when the referenced table carries owner data.
// Goal rows predate owner columns, so those checks rely on exact canonical
// session keys after first validating the conversation's owner/Room scope.
func (r *SQLRepository) HasConversationReferences(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	conversationID string,
	sessionKeys []string,
) (bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
	sessionKeys = normalizeConversationReferenceKeys(sessionKeys)
	if ownerUserID == "" || roomID == "" || conversationID == "" || len(sessionKeys) == 0 {
		return false, errors.New("conversation reference scope is incomplete")
	}

	inScope, err := r.hasConversationReferenceRow(
		ctx,
		`SELECT 1
FROM conversations AS conversation
JOIN rooms AS room ON room.id = conversation.room_id
WHERE room.owner_user_id = `+r.dialect.Bind(1)+`
  AND room.id = `+r.dialect.Bind(2)+`
  AND conversation.id = `+r.dialect.Bind(3)+`
LIMIT 1`,
		ownerUserID,
		roomID,
		conversationID,
	)
	if err != nil {
		return false, fmt.Errorf("validate conversation reference scope: %w", err)
	}
	if !inScope {
		return false, errors.New("conversation reference scope not found")
	}

	for _, check := range r.buildConversationReferenceQueries(
		ownerUserID,
		conversationID,
		sessionKeys,
	) {
		exists, queryErr := r.hasConversationReferenceRow(ctx, check.sql, check.args...)
		if queryErr != nil {
			return false, fmt.Errorf("probe %s references: %w", check.source, queryErr)
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func (r *SQLRepository) buildConversationReferenceQueries(
	ownerUserID string,
	conversationID string,
	sessionKeys []string,
) []conversationReferenceQuery {
	specs := []conversationReferenceQuerySpec{
		{
			source:      "automation_scheduled_tasks",
			from:        "automation_scheduled_tasks AS reference",
			ownerColumn: "reference.owner_user_id",
			sessionKeyColumns: []string{
				"reference.bound_session_key",
				"reference.named_session_key",
				"reference.source_session_key",
			},
			conversationIDColumns: []string{"reference.source_context_id"},
		},
		{
			source:            "automation_task_runs",
			from:              "automation_task_runs AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.session_key"},
		},
		{
			source: "automation_delivery_routes",
			from: `automation_delivery_routes AS reference
JOIN agents AS reference_agent ON reference_agent.id = reference.agent_id`,
			ownerColumn:       "reference_agent.owner_user_id",
			sessionKeyColumns: []string{"reference.session_key"},
		},
		{
			source:            "im_ingress_messages",
			from:              "im_ingress_messages AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.session_key"},
		},
		{
			source:                "token_usage_records",
			from:                  "token_usage_records AS reference",
			ownerColumn:           "reference.owner_user_id",
			sessionKeyColumns:     []string{"reference.session_key"},
			conversationIDColumns: []string{"reference.conversation_id"},
		},
		{
			source:            "session_goals",
			from:              "session_goals AS reference",
			sessionKeyColumns: []string{"reference.session_key"},
		},
		{
			source:            "goal_events",
			from:              "goal_events AS reference",
			sessionKeyColumns: []string{"reference.session_key"},
		},
		{
			source:            "goal_usage_source_checkpoints",
			from:              "goal_usage_source_checkpoints AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.runtime_session_key"},
		},
		{
			source:            "goal_usage_scope_bindings",
			from:              "goal_usage_scope_bindings AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.goal_session_key"},
		},
		{
			source:            "goal_usage_source_pending",
			from:              "goal_usage_source_pending AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.runtime_session_key", "reference.goal_session_key"},
		},
		{
			source:            "goal_usage_source_evidence",
			from:              "goal_usage_source_evidence AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.runtime_session_key", "reference.goal_session_key"},
		},
		{
			source:            "goal_usage_parent_ledger",
			from:              "goal_usage_parent_ledger AS reference",
			ownerColumn:       "reference.owner_user_id",
			sessionKeyColumns: []string{"reference.goal_session_key"},
		},
		{
			source: "rounds",
			from: `rounds AS reference
JOIN sessions AS reference_session ON reference_session.id = reference.session_id
JOIN conversations AS reference_conversation ON reference_conversation.id = reference_session.conversation_id
JOIN rooms AS reference_room ON reference_room.id = reference_conversation.room_id`,
			ownerColumn:           "reference_room.owner_user_id",
			conversationIDColumns: []string{"reference_conversation.id"},
		},
	}

	result := make([]conversationReferenceQuery, 0, len(specs))
	for _, spec := range specs {
		result = append(result, r.buildConversationReferenceQuery(
			spec,
			ownerUserID,
			conversationID,
			sessionKeys,
		))
	}
	return result
}

func (r *SQLRepository) buildConversationReferenceQuery(
	spec conversationReferenceQuerySpec,
	ownerUserID string,
	conversationID string,
	sessionKeys []string,
) conversationReferenceQuery {
	args := make([]any, 0, 1+len(spec.sessionKeyColumns)*len(sessionKeys)+len(spec.conversationIDColumns))
	predicates := make([]string, 0, 2)
	nextBind := 1
	if spec.ownerColumn != "" {
		predicates = append(predicates, spec.ownerColumn+" = "+r.dialect.Bind(nextBind))
		args = append(args, ownerUserID)
		nextBind++
	}

	references := make([]string, 0, len(spec.sessionKeyColumns)+len(spec.conversationIDColumns))
	for _, column := range spec.sessionKeyColumns {
		binds := make([]string, 0, len(sessionKeys))
		for _, sessionKey := range sessionKeys {
			binds = append(binds, r.dialect.Bind(nextBind))
			args = append(args, sessionKey)
			nextBind++
		}
		references = append(references, column+" IN ("+strings.Join(binds, ",")+")")
	}
	for _, column := range spec.conversationIDColumns {
		references = append(references, column+" = "+r.dialect.Bind(nextBind))
		args = append(args, conversationID)
		nextBind++
	}
	predicates = append(predicates, "("+strings.Join(references, " OR ")+")")

	return conversationReferenceQuery{
		source: spec.source,
		sql: "SELECT 1\nFROM " + spec.from + "\nWHERE " +
			strings.Join(predicates, "\n  AND ") + "\nLIMIT 1",
		args: args,
	}
}

func (r *SQLRepository) hasConversationReferenceRow(
	ctx context.Context,
	query string,
	args ...any,
) (bool, error) {
	var marker int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func normalizeConversationReferenceKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}
