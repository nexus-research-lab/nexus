-- +goose Up
-- =====================================================
-- @File   ：00002_go_schema_alignment.sql
-- @Date   ：2026/04/16 23:58:00
-- @Author ：leemysw
-- 2026/04/16 23:58:00   Create
-- =====================================================

DROP INDEX IF EXISTS idx_auth_sessions_revoked_at;
DROP INDEX IF EXISTS idx_auth_sessions_expires_at;
DROP INDEX IF EXISTS idx_auth_sessions_user;
DROP INDEX IF EXISTS uq_auth_sessions_token_hash;
DROP TABLE IF EXISTS auth_sessions;

CREATE TABLE users (
    user_id VARCHAR(64) NOT NULL PRIMARY KEY,
    username VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    last_login_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_users_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled'))
);
CREATE UNIQUE INDEX uq_users_username ON users (username);

CREATE TABLE auth_password_credentials (
    credential_id VARCHAR(64) NOT NULL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    password_hash TEXT NOT NULL,
    password_algo VARCHAR(32) NOT NULL,
    password_updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_auth_password_credentials_algo CHECK (password_algo IN ('argon2id')),
    CONSTRAINT uq_auth_password_credentials_user UNIQUE (user_id),
    FOREIGN KEY(user_id) REFERENCES users (user_id) ON DELETE CASCADE
);

CREATE TABLE auth_sessions (
    session_id VARCHAR(64) NOT NULL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    session_token_hash VARCHAR(64) NOT NULL,
    auth_method VARCHAR(32) NOT NULL,
    expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    last_seen_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    client_ip VARCHAR(255),
    user_agent TEXT,
    revoked_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_auth_sessions_method CHECK (auth_method IN ('password')),
    FOREIGN KEY(user_id) REFERENCES users (user_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_auth_sessions_token_hash ON auth_sessions (session_token_hash);
CREATE INDEX idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);
CREATE INDEX idx_auth_sessions_revoked_at ON auth_sessions (revoked_at);

-- +goose Down
-- =====================================================
-- @File   ：00002_go_schema_alignment.sql
-- @Date   ：2026/04/16 23:58:00
-- @Author ：leemysw
-- 2026/04/16 23:58:00   Create
-- =====================================================

DROP INDEX IF EXISTS idx_auth_sessions_revoked_at;
DROP INDEX IF EXISTS idx_auth_sessions_expires_at;
DROP INDEX IF EXISTS idx_auth_sessions_user;
DROP INDEX IF EXISTS uq_auth_sessions_token_hash;
DROP TABLE IF EXISTS auth_sessions;

DROP TABLE IF EXISTS auth_password_credentials;

DROP INDEX IF EXISTS uq_users_username;
DROP TABLE IF EXISTS users;

CREATE TABLE auth_sessions (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_token_hash VARCHAR(64) NOT NULL,
    username VARCHAR(128) NOT NULL,
    expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);
CREATE INDEX idx_auth_sessions_username ON auth_sessions (username);
CREATE UNIQUE INDEX uq_auth_sessions_token_hash ON auth_sessions (session_token_hash);
