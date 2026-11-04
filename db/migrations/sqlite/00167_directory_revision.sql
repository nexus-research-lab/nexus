-- +goose Up
-- Committed scalar revisions let the server observe writes from standalone nexusctl processes.
-- agent_revision also advances for Agent/Profile display changes, so clients reload the Agent list only when needed.
CREATE TABLE directory_revision (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL,
    agent_revision BIGINT NOT NULL
);
INSERT INTO directory_revision (id, revision, agent_revision) VALUES (1, 0, 0);

-- +goose StatementBegin
CREATE TRIGGER directory_revision_rooms_insert AFTER INSERT ON rooms
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_rooms_update AFTER UPDATE OF name, description, avatar, host_agent_id, host_auto_reply_enabled, configuration_version ON rooms
WHEN OLD.name IS NOT NEW.name OR OLD.description IS NOT NEW.description OR OLD.avatar IS NOT NEW.avatar OR OLD.host_agent_id IS NOT NEW.host_agent_id OR OLD.host_auto_reply_enabled IS NOT NEW.host_auto_reply_enabled OR OLD.configuration_version IS NOT NEW.configuration_version
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_rooms_delete AFTER DELETE ON rooms
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_members_insert AFTER INSERT ON members
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_members_update AFTER UPDATE ON members
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_members_delete AFTER DELETE ON members
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_conversations_insert AFTER INSERT ON conversations
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_conversations_update AFTER UPDATE OF title, is_draft ON conversations
WHEN OLD.title IS NOT NEW.title OR OLD.is_draft IS NOT NEW.is_draft
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_conversations_delete AFTER DELETE ON conversations
BEGIN
    UPDATE directory_revision SET revision = revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_agents_insert AFTER INSERT ON agents
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_agents_update AFTER UPDATE OF name, description, avatar, status ON agents
WHEN OLD.name IS NOT NEW.name OR OLD.description IS NOT NEW.description OR OLD.avatar IS NOT NEW.avatar OR OLD.status IS NOT NEW.status
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_agents_delete AFTER DELETE ON agents
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_profiles_insert AFTER INSERT ON profiles
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_profiles_update AFTER UPDATE OF display_name, avatar_url ON profiles
WHEN OLD.display_name IS NOT NEW.display_name OR OLD.avatar_url IS NOT NEW.avatar_url
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER directory_revision_profiles_delete AFTER DELETE ON profiles
BEGIN
    UPDATE directory_revision SET revision = revision + 1, agent_revision = agent_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER directory_revision_rooms_insert;
DROP TRIGGER directory_revision_rooms_update;
DROP TRIGGER directory_revision_rooms_delete;
DROP TRIGGER directory_revision_members_insert;
DROP TRIGGER directory_revision_members_update;
DROP TRIGGER directory_revision_members_delete;
DROP TRIGGER directory_revision_conversations_insert;
DROP TRIGGER directory_revision_conversations_update;
DROP TRIGGER directory_revision_conversations_delete;
DROP TRIGGER directory_revision_agents_insert;
DROP TRIGGER directory_revision_agents_update;
DROP TRIGGER directory_revision_agents_delete;
DROP TRIGGER directory_revision_profiles_insert;
DROP TRIGGER directory_revision_profiles_update;
DROP TRIGGER directory_revision_profiles_delete;
DROP TABLE directory_revision;
