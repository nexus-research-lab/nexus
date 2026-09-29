-- +goose Up
-- 历史 processing 的副作用未知；不能迁移成可重新执行的 prepared。
ALTER TABLE im_ingress_messages ADD COLUMN dispatch_phase TEXT NOT NULL DEFAULT 'dispatching' CHECK (dispatch_phase IN ('prepared', 'dispatching'));

ALTER TABLE im_ingress_messages ADD COLUMN payload_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE im_ingress_messages DROP COLUMN payload_hash;
ALTER TABLE im_ingress_messages DROP COLUMN dispatch_phase;
