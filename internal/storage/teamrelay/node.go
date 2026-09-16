package teamrelay

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// NodeGrant 保存本机授权意图；机器凭据必须在进入仓储前加密，浏览器 Cookie 只存哈希。
type NodeGrant struct {
	Scope, OwnerUserID, NodeID, State     string
	Name, CookieHash, CredentialEncrypted string
	AgentIDs                              []string
	RemoteURL                             string
	ExecutionEnabled                      bool
}

func (r *Repository) NodeGrants(ctx context.Context) ([]NodeGrant, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT scope, owner_user_id FROM team_node_grants WHERE state='authorized'`)
	if err != nil {
		return nil, err
	}
	var keys [][2]string
	for rows.Next() {
		var key [2]string
		if err = rows.Scan(&key[0], &key[1]); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []NodeGrant{}
	for _, key := range keys {
		item, err := r.NodeGrant(ctx, key[0], key[1])
		if err != nil {
			return nil, err
		}
		if item != nil && item.State == "authorized" {
			result = append(result, *item)
		}
	}
	return result, nil
}

var ErrNodeConflict = errors.New("节点授权状态已变化")

func (r *Repository) NodeGrant(ctx context.Context, scope, owner string) (*NodeGrant, error) {
	var record NodeGrant
	var data string
	err := r.db.QueryRowContext(ctx, `SELECT node_id, state, data_json FROM team_node_grants WHERE scope = `+r.dialect.Bind(1)+` AND owner_user_id = `+r.dialect.Bind(2), scope, owner).Scan(&record.NodeID, &record.State, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload NodeGrant
	if err = json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, err
	}
	payload.Scope, payload.OwnerUserID, payload.NodeID, payload.State = scope, owner, record.NodeID, record.State
	return &payload, nil
}

// PrepareNodeGrant 在远端注册之前提交凭据；只有已撤销的旧身份允许被新意图替换。
func (r *Repository) PrepareNodeGrant(ctx context.Context, record NodeGrant) (*NodeGrant, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO team_node_grants(scope, owner_user_id, node_id, state, data_json)
		VALUES (`+r.dialect.BindList(5)+`) ON CONFLICT(scope) DO UPDATE SET node_id = excluded.node_id, state = excluded.state, data_json = excluded.data_json
		WHERE team_node_grants.state = 'revoked' AND team_node_grants.owner_user_id = excluded.owner_user_id`, record.Scope, record.OwnerUserID, record.NodeID, "pending", string(data))
	if err != nil {
		return nil, err
	}
	return r.NodeGrant(ctx, record.Scope, record.OwnerUserID)
}

func (r *Repository) SetNodeState(ctx context.Context, record NodeGrant, from, to string) error {
	// 撤销终态清掉加密凭据；保留精确 Node 身份以隔离迟到响应。
	if to == "revoked" {
		record.CredentialEncrypted = ""
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE team_node_grants SET state = `+r.dialect.Bind(1)+`, data_json = `+r.dialect.Bind(2)+`
		WHERE scope = `+r.dialect.Bind(3)+` AND owner_user_id = `+r.dialect.Bind(4)+` AND node_id = `+r.dialect.Bind(5)+` AND state = `+r.dialect.Bind(6), to, string(data), record.Scope, record.OwnerUserID, record.NodeID, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return ErrNodeConflict
	}
	if err != nil {
		return err
	}
	if to == "revoking" || to == "revoked" {
		// 与 ready→running 共用授权行锁，只释放未启动或原生 round 已结束的任务。
		if _, err = tx.ExecContext(ctx, `UPDATE team_node_jobs SET state='failed' WHERE node_id=`+r.dialect.Bind(1)+` AND owner_user_id=`+r.dialect.Bind(2)+` AND state IN ('claiming','ready','draining')`, record.NodeID, record.OwnerUserID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
