-- +goose Up
CREATE TABLE execution_dispatch_policies (
 policy_id TEXT PRIMARY KEY,
 execution_id TEXT NOT NULL REFERENCES executions(execution_id) ON DELETE CASCADE,
 plan_id TEXT NOT NULL REFERENCES execution_plan_revisions(plan_id) ON DELETE CASCADE,
 work_item_id TEXT NOT NULL REFERENCES execution_work_items(work_item_id) ON DELETE CASCADE,
 spec_id TEXT NOT NULL,
 intent_json TEXT NOT NULL,
 state TEXT NOT NULL CHECK (state IN ('pending', 'assigned', 'queued', 'cancelled')),
 reason TEXT NOT NULL DEFAULT '',
 assignment_id TEXT NOT NULL DEFAULT '',
 attempt_id TEXT NOT NULL DEFAULT '',
 next_attempt_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_execution_dispatch_policy_due ON execution_dispatch_policies(state, next_attempt_at, policy_id);
CREATE INDEX idx_execution_dispatch_policy_plan ON execution_dispatch_policies(plan_id, work_item_id);
CREATE UNIQUE INDEX idx_execution_dispatch_policy_current ON execution_dispatch_policies(plan_id, work_item_id) WHERE state IN ('pending', 'assigned');
-- +goose Down
DROP TABLE execution_dispatch_policies;
