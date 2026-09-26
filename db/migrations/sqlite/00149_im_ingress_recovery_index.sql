-- +goose Up
CREATE INDEX idx_im_ingress_recovery ON im_ingress_messages(owner_user_id, channel_type, account_id, req_id)
WHERE status = 'processing' AND dispatch_phase = 'dispatching';

-- +goose Down
DROP INDEX idx_im_ingress_recovery;
