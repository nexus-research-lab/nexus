-- +goose Up
-- Committed scalar revisions let the server observe writes from standalone nexusctl processes.
-- agent_revision also advances for Agent/Profile display changes, so clients reload the Agent list only when needed.
CREATE TABLE directory_revision (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL,
    agent_revision BIGINT NOT NULL
);
INSERT INTO directory_revision (id, revision, agent_revision) VALUES (1, 0, 0);

-- Deferred constraint triggers advance the singleton row once per transaction at commit time,
-- so directory writes do not hold its row lock for the whole transaction or take it before other row locks.
-- +goose StatementBegin
CREATE FUNCTION advance_directory_revision() RETURNS trigger AS $$
BEGIN
    IF COALESCE(current_setting('nexus.directory_revision_advanced', true), '') <> 'on' THEN
        UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
        PERFORM set_config('nexus.directory_revision_advanced', 'on', true);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION advance_agent_directory_revision() RETURNS trigger AS $$
BEGIN
    IF COALESCE(current_setting('nexus.agent_directory_revision_advanced', true), '') <> 'on' THEN
        UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
        PERFORM set_config('nexus.agent_directory_revision_advanced', 'on', true);
        PERFORM set_config('nexus.directory_revision_advanced', 'on', true);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER directory_revision_rooms AFTER INSERT OR DELETE ON rooms
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_rooms_update AFTER UPDATE OF name, description, avatar, host_agent_id, host_auto_reply_enabled, configuration_version ON rooms
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.name IS DISTINCT FROM NEW.name OR OLD.description IS DISTINCT FROM NEW.description OR OLD.avatar IS DISTINCT FROM NEW.avatar OR OLD.host_agent_id IS DISTINCT FROM NEW.host_agent_id OR OLD.host_auto_reply_enabled IS DISTINCT FROM NEW.host_auto_reply_enabled OR OLD.configuration_version IS DISTINCT FROM NEW.configuration_version)
EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_members AFTER INSERT OR DELETE ON members
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_members_update AFTER UPDATE ON members
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD IS DISTINCT FROM NEW)
EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_conversations AFTER INSERT OR DELETE ON conversations
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_conversations_update AFTER UPDATE OF title, is_draft ON conversations
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.title IS DISTINCT FROM NEW.title OR OLD.is_draft IS DISTINCT FROM NEW.is_draft)
EXECUTE FUNCTION advance_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_agents AFTER INSERT OR DELETE ON agents
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION advance_agent_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_agents_update AFTER UPDATE OF name, description, avatar, status ON agents
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.name IS DISTINCT FROM NEW.name OR OLD.description IS DISTINCT FROM NEW.description OR OLD.avatar IS DISTINCT FROM NEW.avatar OR OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION advance_agent_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_profiles AFTER INSERT OR DELETE ON profiles
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION advance_agent_directory_revision();

CREATE CONSTRAINT TRIGGER directory_revision_profiles_update AFTER UPDATE OF display_name, avatar_url ON profiles
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.display_name IS DISTINCT FROM NEW.display_name OR OLD.avatar_url IS DISTINCT FROM NEW.avatar_url)
EXECUTE FUNCTION advance_agent_directory_revision();

-- +goose Down
DROP TRIGGER directory_revision_rooms ON rooms;
DROP TRIGGER directory_revision_rooms_update ON rooms;
DROP TRIGGER directory_revision_members ON members;
DROP TRIGGER directory_revision_members_update ON members;
DROP TRIGGER directory_revision_conversations ON conversations;
DROP TRIGGER directory_revision_conversations_update ON conversations;
DROP TRIGGER directory_revision_agents ON agents;
DROP TRIGGER directory_revision_agents_update ON agents;
DROP TRIGGER directory_revision_profiles ON profiles;
DROP TRIGGER directory_revision_profiles_update ON profiles;
DROP FUNCTION advance_agent_directory_revision();
DROP FUNCTION advance_directory_revision();
DROP TABLE directory_revision;
