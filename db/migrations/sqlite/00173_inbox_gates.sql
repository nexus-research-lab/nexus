-- +goose Up
-- 常驻对话 inbox 的暂停闸门：用户停止即暂停宿主恢复输入，用户再开口即恢复；revision 在每次停止或恢复时递增。
CREATE TABLE agent_inbox_gates (
    inbox_key VARCHAR(512) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    paused BOOLEAN NOT NULL,
    revision BIGINT NOT NULL,
    updated_at DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS agent_inbox_gates;
