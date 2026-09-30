-- +goose Up
ALTER TABLE sandbox_process_launches ADD COLUMN launch_order SMALLINT NOT NULL DEFAULT 4 CHECK (launch_order BETWEEN 1 AND 4);
ALTER TABLE sandbox_process_launches DROP CONSTRAINT sandbox_process_launches_pkey;
ALTER TABLE sandbox_process_launches ADD PRIMARY KEY (owner_user_id,session_key,generation,launch_order);
DROP INDEX idx_sandbox_process_latest;
CREATE INDEX idx_sandbox_process_latest ON sandbox_process_launches(owner_user_id,session_key,generation DESC,launch_order DESC);
-- +goose Down
-- Duplicate generations reject this transaction rather than discarding original evidence.
ALTER TABLE sandbox_process_launches DROP CONSTRAINT sandbox_process_launches_pkey;
ALTER TABLE sandbox_process_launches ADD PRIMARY KEY (owner_user_id,session_key,generation);
DROP INDEX idx_sandbox_process_latest;
CREATE INDEX idx_sandbox_process_latest ON sandbox_process_launches(owner_user_id,session_key,generation DESC);
ALTER TABLE sandbox_process_launches DROP COLUMN launch_order;
