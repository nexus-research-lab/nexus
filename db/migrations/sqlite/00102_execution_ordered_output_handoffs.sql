-- +goose Up

-- Output-scope overlap is valid when the owners are ordered by an all-hard
-- dependency path. That DAG-aware invariant is enforced before WritePlan;
-- the legacy per-Plan unique index cannot distinguish an ordered handoff from
-- concurrent ownership and therefore rejects valid Plans at INSERT time.
DROP INDEX IF EXISTS uq_execution_plan_exclusive_output_claim;

-- +goose Down

-- A Plan may now durably contain multiple ordered exclusive claims for the
-- same key, so restoring the legacy unique index is not data-safe.
SELECT 1;
