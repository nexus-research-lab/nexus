-- +goose Up
CREATE INDEX IF NOT EXISTS idx_rooms_owner_updated ON rooms (owner_user_id, updated_at DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rooms_owner_type ON rooms (owner_user_id, room_type);
CREATE INDEX IF NOT EXISTS idx_agents_owner_status_main_created ON agents (owner_user_id, status, is_main DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_agents_owner_status_lower_name ON agents (owner_user_id, status, LOWER(name));
CREATE INDEX IF NOT EXISTS idx_runtimes_provider_agent ON runtimes ((COALESCE(NULLIF(TRIM(provider), ''), '')), agent_id);
CREATE INDEX IF NOT EXISTS idx_connector_connections_state ON connector_connections (state);
CREATE INDEX IF NOT EXISTS idx_automation_delivery_routes_agent_updated ON automation_delivery_routes (agent_id, updated_at DESC, route_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_created ON automation_cron_jobs (created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_agent_created ON automation_cron_jobs (agent_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_enabled_agent ON automation_cron_jobs (enabled, agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_heartbeat_states_enabled_agent ON automation_heartbeat_states (enabled, agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_system_events_status_created ON automation_system_events (status, created_at ASC, event_id ASC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_system_events_status_created;
DROP INDEX IF EXISTS idx_automation_heartbeat_states_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_created;
DROP INDEX IF EXISTS idx_automation_delivery_routes_agent_updated;
DROP INDEX IF EXISTS idx_connector_connections_state;
DROP INDEX IF EXISTS idx_runtimes_provider_agent;
DROP INDEX IF EXISTS idx_agents_owner_status_lower_name;
DROP INDEX IF EXISTS idx_agents_owner_status_main_created;
DROP INDEX IF EXISTS idx_rooms_owner_type;
DROP INDEX IF EXISTS idx_rooms_owner_updated;
