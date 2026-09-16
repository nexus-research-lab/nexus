// INPUT: Immutable IM intents and exact owner/delivery/reply identities.
// OUTPUT: Durable origin queries, CAS send claims and replay-safe reply states.
// POS: Host database boundary for the IM scenario; no Room ledger dependency.
package imdelivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

var ErrConflict = errors.New("IM delivery intent conflict")
var ErrUnavailable = errors.New("IM delivery record unavailable")

type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}
func (r *Repository) b(i int) string { return r.dialect.Bind(i) }

func (r *Repository) SaveDelivery(ctx context.Context, d Delivery) (Delivery, error) {
	if d.ID == "" || d.OwnerUserID == "" || d.TargetSessionKey == "" {
		return Delivery{}, ErrUnavailable
	}
	d.ReturnRevoked = false
	d.State = ""
	d.ReceiptJSON = ""
	d.CreatedAt = 0
	raw, err := json.Marshal(d)
	if err != nil {
		return Delivery{}, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO im_deliveries(owner_user_id,delivery_id,target_session_key,source_session_key,intent_json,created_at,pairing_id) VALUES (`+r.dialect.BindList(7)+`) ON CONFLICT(owner_user_id,delivery_id) DO NOTHING`, d.OwnerUserID, d.ID, d.TargetSessionKey, d.Source.SessionKey, string(raw), time.Now().UnixMilli(), d.PairingID)
	if err != nil {
		return Delivery{}, err
	}
	saved, err := r.GetDelivery(ctx, d.OwnerUserID, d.ID)
	if err != nil {
		return Delivery{}, err
	}
	compare := saved
	compare.ReturnRevoked = false
	compare.State = ""
	compare.ReceiptJSON = ""
	compare.CreatedAt = 0
	check, _ := json.Marshal(compare)
	if string(check) != string(raw) {
		return Delivery{}, ErrConflict
	}
	return saved, nil
}
func (r *Repository) GetDelivery(ctx context.Context, owner, id string) (Delivery, error) {
	row := r.db.QueryRowContext(ctx, `SELECT intent_json,send_state,receipt_json,created_at,return_revoked FROM im_deliveries WHERE owner_user_id=`+r.b(1)+` AND delivery_id=`+r.b(2), owner, id)
	return scanDelivery(row)
}

type scanner interface{ Scan(...any) error }

func scanDelivery(row scanner) (Delivery, error) {
	var d Delivery
	var raw string
	if err := row.Scan(&raw, &d.State, &d.ReceiptJSON, &d.CreatedAt, &d.ReturnRevoked); err != nil {
		return d, err
	}
	state, receipt, created, revoked := d.State, d.ReceiptJSON, d.CreatedAt, d.ReturnRevoked
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return d, err
	}
	d.ReturnRevoked = revoked
	d.State = state
	d.ReceiptJSON = receipt
	d.CreatedAt = created
	return d, nil
}

// ClaimSend commits unknown BEFORE any physical send. Ordinary retries never
// replay unknown effects. Automation alone owns its separate attempt authority.
func (r *Repository) ClaimSend(ctx context.Context, owner, id string, automationAttempt bool) (bool, error) {
	allowed := ` AND send_state IN ('pending','not_sent')`
	if automationAttempt {
		allowed = ""
	}
	result, err := r.db.ExecContext(ctx, `UPDATE im_deliveries SET send_state='unknown' WHERE owner_user_id=`+r.b(1)+` AND delivery_id=`+r.b(2)+` AND return_revoked=0`+allowed, owner, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (r *Repository) FinishSend(ctx context.Context, owner, id, state, receipt string) error {
	if state != "sent" && state != "not_sent" && state != "unknown" {
		return ErrConflict
	}
	_, err := r.db.ExecContext(ctx, `UPDATE im_deliveries SET send_state=`+r.b(1)+`,receipt_json=`+r.b(2)+` WHERE owner_user_id=`+r.b(3)+` AND delivery_id=`+r.b(4)+` AND send_state='unknown'`, state, receipt, owner, id)
	return err
}
func (r *Repository) List(ctx context.Context, owner, session, query string, offset, limit int) ([]Delivery, error) {
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return nil, ErrConflict
	}
	pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(query)) + "%"
	rows, err := r.db.QueryContext(ctx, `SELECT intent_json,send_state,receipt_json,created_at,return_revoked FROM im_deliveries WHERE owner_user_id=`+r.b(1)+` AND target_session_key=`+r.b(2)+` AND LOWER(intent_json) LIKE `+r.b(3)+` ESCAPE '!' ORDER BY created_at DESC,delivery_id DESC LIMIT `+r.b(4)+` OFFSET `+r.b(5), owner, session, pattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Delivery{}
	for rows.Next() {
		d, e := scanDelivery(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (r *Repository) SaveReply(ctx context.Context, reply Reply) (Reply, error) {
	if reply.OwnerUserID == "" || reply.ID == "" || reply.DeliveryID == "" || reply.Input.ID == "" {
		return Reply{}, ErrUnavailable
	}
	reply.State = ""
	reply.CreatedAt = 0
	raw, err := json.Marshal(reply)
	if err != nil {
		return Reply{}, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO im_delivery_replies(owner_user_id,reply_id,delivery_id,request_message_id,intent_json,created_at) VALUES (`+r.dialect.BindList(6)+`) ON CONFLICT(owner_user_id,delivery_id,request_message_id) DO NOTHING`, reply.OwnerUserID, reply.ID, reply.DeliveryID, reply.Input.ID, string(raw), time.Now().UnixMilli())
	if err != nil {
		return Reply{}, err
	}
	saved, err := r.GetReply(ctx, reply.OwnerUserID, reply.ID)
	if err != nil {
		return Reply{}, err
	}
	compare := saved
	compare.State = ""
	compare.CreatedAt = 0
	check, _ := json.Marshal(compare)
	if string(check) != string(raw) {
		return Reply{}, ErrConflict
	}
	return saved, nil
}
func (r *Repository) GetReply(ctx context.Context, owner, id string) (Reply, error) {
	return scanReply(r.db.QueryRowContext(ctx, `SELECT intent_json,admission_state,created_at FROM im_delivery_replies WHERE owner_user_id=`+r.b(1)+` AND reply_id=`+r.b(2), owner, id))
}
func scanReply(row scanner) (Reply, error) {
	var reply Reply
	var raw, state string
	var created int64
	if err := row.Scan(&raw, &state, &created); err != nil {
		return reply, err
	}
	if err := json.Unmarshal([]byte(raw), &reply); err != nil {
		return reply, err
	}
	reply.State = state
	reply.CreatedAt = created
	return reply, nil
}
func (r *Repository) TransitionReply(ctx context.Context, owner, id, from, to string) (bool, error) {
	if from == "" || to == "" {
		return false, ErrConflict
	}
	result, err := r.db.ExecContext(ctx, `UPDATE im_delivery_replies SET admission_state=`+r.b(1)+` WHERE owner_user_id=`+r.b(2)+` AND reply_id=`+r.b(3)+` AND admission_state=`+r.b(4), to, owner, id, from)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (r *Repository) Replies(ctx context.Context, owner, delivery string) ([]Reply, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT intent_json,admission_state,created_at FROM im_delivery_replies WHERE owner_user_id=`+r.b(1)+` AND delivery_id=`+r.b(2)+` ORDER BY created_at DESC LIMIT 50`, owner, delivery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reply{}
	for rows.Next() {
		v, e := scanReply(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// PendingAfter reads a keyset page for startup repair, without external sends.
func (r *Repository) PendingAfter(ctx context.Context, owner, id string) ([]Reply, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT intent_json,admission_state,created_at FROM im_delivery_replies WHERE admission_state IN ('pending','accepted') AND (owner_user_id > `+r.b(1)+` OR (owner_user_id = `+r.b(2)+` AND reply_id > `+r.b(3)+`)) ORDER BY owner_user_id,reply_id LIMIT 1000`, owner, owner, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reply{}
	for rows.Next() {
		v, e := scanReply(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) VerifyReply(ctx context.Context, owner, id, session, agent, content string) (Reply, error) {
	reply, err := r.GetReply(ctx, owner, id)
	if err != nil {
		return Reply{}, err
	}
	d, err := r.GetDelivery(ctx, owner, reply.DeliveryID)
	if err != nil {
		return Reply{}, err
	}
	if d.Source.SessionKey != session || d.Source.AgentID != agent || reply.ForwardedContent != content {
		return Reply{}, fmt.Errorf("%w: queued feedback identity", ErrConflict)
	}
	return reply, nil
}
