-- +goose Up
CREATE TABLE subscription_plans (
  plan_key VARCHAR(64) NOT NULL PRIMARY KEY,
  display_name VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  monthly_token_limit INTEGER,
  notes TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 100,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  CONSTRAINT ck_subscription_plans_status CHECK (status IN ('active', 'archived')),
  CONSTRAINT ck_subscription_plans_monthly_token_limit CHECK (monthly_token_limit IS NULL OR monthly_token_limit >= 0)
);

CREATE TABLE user_subscriptions (
  owner_user_id VARCHAR(64) NOT NULL PRIMARY KEY,
  plan_key VARCHAR(64) NOT NULL DEFAULT 'free',
  period_start DATETIME,
  period_end DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  CONSTRAINT fk_user_subscriptions_plan FOREIGN KEY (plan_key) REFERENCES subscription_plans(plan_key)
);

CREATE INDEX idx_user_subscriptions_plan ON user_subscriptions(plan_key);

INSERT INTO subscription_plans (
  plan_key,
  display_name,
  status,
  monthly_token_limit,
  notes,
  sort_order,
  created_at,
  updated_at
) VALUES
  ('free', 'Free', 'active', 200000, '开放注册前默认免费额度', 10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('admin', 'Admin', 'active', NULL, '管理员与内部测试无限额度', 90, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- +goose Down
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS subscription_plans;
