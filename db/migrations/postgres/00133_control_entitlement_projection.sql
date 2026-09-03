-- +goose Up
CREATE TABLE owner_entitlements (
    owner_user_id VARCHAR(128) NOT NULL PRIMARY KEY,
    plan_key VARCHAR(64) NOT NULL,
    plan_name VARCHAR(128) NOT NULL,
    monthly_token_limit BIGINT,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    projected_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT ck_owner_entitlements_monthly_token_limit
        CHECK (monthly_token_limit IS NULL OR monthly_token_limit >= 0),
    FOREIGN KEY (owner_user_id)
        REFERENCES owner_profiles (owner_user_id) ON DELETE CASCADE
);

CREATE TABLE control_projection_state (
    singleton_id INTEGER NOT NULL PRIMARY KEY CHECK (singleton_id = 1),
    identity_invalidation_cursor BIGINT NOT NULL DEFAULT 0
        CHECK (identity_invalidation_cursor >= 0)
);
INSERT INTO control_projection_state (singleton_id, identity_invalidation_cursor)
VALUES (1, 0);

INSERT INTO owner_entitlements (
    owner_user_id, plan_key, plan_name, monthly_token_limit, updated_at, projected_at
)
SELECT p.owner_user_id,
       COALESCE(us.plan_key, 'free'),
       sp.display_name,
       sp.monthly_token_limit,
       COALESCE(us.updated_at, sp.updated_at),
       CURRENT_TIMESTAMP
FROM owner_profiles p
JOIN local_owner_bindings b ON b.local_owner_key = p.owner_user_id
LEFT JOIN user_subscriptions us ON us.owner_user_id = p.owner_user_id
JOIN subscription_plans sp ON sp.plan_key = COALESCE(us.plan_key, 'free');

-- +goose Down
DROP TABLE control_projection_state;
DROP TABLE owner_entitlements;
