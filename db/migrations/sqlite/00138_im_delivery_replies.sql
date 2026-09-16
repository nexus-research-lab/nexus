-- +goose Up
CREATE TABLE im_deliveries (
 owner_user_id TEXT NOT NULL,
 delivery_id TEXT NOT NULL,
 target_session_key TEXT NOT NULL,
 pairing_id TEXT NOT NULL,
 return_revoked INTEGER NOT NULL DEFAULT 0,
 source_session_key TEXT NOT NULL,
 intent_json TEXT NOT NULL,
 send_state TEXT NOT NULL DEFAULT 'pending',
 receipt_json TEXT NOT NULL DEFAULT '',
 created_at BIGINT NOT NULL,
 PRIMARY KEY (owner_user_id, delivery_id)
);
CREATE INDEX idx_im_deliveries_target ON im_deliveries(owner_user_id, target_session_key, created_at, delivery_id);
CREATE INDEX idx_im_deliveries_pairing ON im_deliveries(owner_user_id,pairing_id);
CREATE INDEX idx_im_deliveries_source ON im_deliveries(owner_user_id, source_session_key);
CREATE TABLE im_delivery_replies (
 owner_user_id TEXT NOT NULL,
 reply_id TEXT NOT NULL,
 delivery_id TEXT NOT NULL,
 request_message_id TEXT NOT NULL,
 intent_json TEXT NOT NULL,
 admission_state TEXT NOT NULL DEFAULT 'pending',
 created_at BIGINT NOT NULL,
 PRIMARY KEY(owner_user_id, reply_id),
 UNIQUE(owner_user_id, delivery_id, request_message_id)
);
CREATE INDEX idx_im_replies_pending ON im_delivery_replies(admission_state, created_at);
ALTER TABLE im_ingress_messages ADD COLUMN delivery_input_json TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_im_ingress_round ON im_ingress_messages(owner_user_id, session_key, round_id);
-- +goose Down
DROP INDEX idx_im_ingress_round;
ALTER TABLE im_ingress_messages DROP COLUMN delivery_input_json;
DROP TABLE im_delivery_replies;
DROP TABLE im_deliveries;
