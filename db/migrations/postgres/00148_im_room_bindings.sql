-- +goose Up
-- IM 传输会话与执行目标分离；目标切换不改写原会话历史。
ALTER TABLE im_pairings ADD COLUMN target_room_id TEXT NOT NULL DEFAULT '';
ALTER TABLE im_pairings ADD COLUMN target_conversation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE im_pairings ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE im_ingress_messages ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 0;
CREATE TABLE im_room_inputs (
    owner_user_id TEXT NOT NULL,
    root_round_id TEXT NOT NULL,
    pairing_id TEXT NOT NULL,
    binding_version BIGINT NOT NULL,
    agent_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    target_json TEXT NOT NULL,
    content TEXT NOT NULL,
    PRIMARY KEY (owner_user_id, root_round_id)
);
CREATE TABLE im_room_replies (
    owner_user_id TEXT NOT NULL,
    root_round_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    state TEXT NOT NULL,
    PRIMARY KEY (owner_user_id, root_round_id, message_id)
);
-- +goose Down
ALTER TABLE im_ingress_messages DROP COLUMN binding_version;
DROP TABLE im_room_replies;
DROP TABLE im_room_inputs;
ALTER TABLE im_pairings DROP COLUMN binding_version;
ALTER TABLE im_pairings DROP COLUMN target_conversation_id;
ALTER TABLE im_pairings DROP COLUMN target_room_id;
