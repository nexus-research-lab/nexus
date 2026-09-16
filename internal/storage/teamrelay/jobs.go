// INPUT: 精确 Node、Agent、claim、Delivery 和本机执行事实。
// OUTPUT: 执行前 inbox CAS、完整输出 outbox 与不自动重跑的未知运行记录。
// POS: 机器消费者的持久副作用边界，SQL 方言复用公共 storage。
package teamrelay

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

type NodeJob struct {
	ID, NodeID, OwnerUserID, LocalAgentID, AgentID, Scope, State string
	RoomID, ConversationID, RoundID                              string
	Delivery                                                     *relaycontract.Delivery
	CandidateID, CandidateText                                   string
	Sequence, OutputBytes                                        int
	Failed                                                       bool
	CandidateSent                                                bool
}

type NodeOutput struct {
	Sequence int
	ID       string
	Input    relaycontract.DeliveryOutput
}

func (r *Repository) NodeJob(ctx context.Context, owner, id string) (*NodeJob, error) {
	var data, state string
	err := r.db.QueryRowContext(ctx, `SELECT state,data_json FROM team_node_jobs WHERE owner_user_id=`+r.dialect.Bind(1)+` AND id=`+r.dialect.Bind(2), owner, id).Scan(&state, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var item NodeJob
	if err = json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}
	item.State = state
	return &item, nil
}

func (r *Repository) ActiveNodeJob(ctx context.Context, owner, agent string) (*NodeJob, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM team_node_jobs WHERE owner_user_id=`+r.dialect.Bind(1)+` AND local_agent_id=`+r.dialect.Bind(2)+` AND state IN ('claiming','ready','running','draining','review_required')`, owner, agent).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.NodeJob(ctx, owner, id)
}

func (r *Repository) PrepareNodeJob(ctx context.Context, item NodeJob) (*NodeJob, error) {
	item.State = "claiming"
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO team_node_jobs(id,node_id,owner_user_id,local_agent_id,state,data_json,scope) VALUES (`+r.dialect.BindList(7)+`) ON CONFLICT DO NOTHING`, item.ID, item.NodeID, item.OwnerUserID, item.LocalAgentID, item.State, string(data), item.Scope)
	if err != nil {
		return nil, err
	}
	return r.ActiveNodeJob(ctx, item.OwnerUserID, item.LocalAgentID)
}

// SaveNodeJob 和可选输出同事务提交。旧状态不能覆盖新终态或接管另一台机器的执行。
func (r *Repository) SaveNodeJob(ctx context.Context, item NodeJob, from string, output *relaycontract.DeliveryOutput) error {
	if item.State == "completed" {
		item.CandidateID, item.CandidateText = "", ""
	}
	if output != nil {
		item.Sequence++
		for _, block := range output.Content.Blocks {
			item.OutputBytes += len(block.Text)
		}
		if item.OutputBytes > 1<<20 {
			return errors.New("单次投递完整输出超过 1 MiB")
		}
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if from == "ready" && item.State == "running" {
		locked, err := tx.ExecContext(ctx, `UPDATE team_node_grants SET state=state WHERE node_id=`+r.dialect.Bind(1)+` AND owner_user_id=`+r.dialect.Bind(2)+` AND state='authorized'`, item.NodeID, item.OwnerUserID)
		if err != nil {
			return err
		}
		count, err := locked.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrNodeConflict
		}
		var raw string
		if err = tx.QueryRowContext(ctx, `SELECT data_json FROM team_node_grants WHERE node_id=`+r.dialect.Bind(1), item.NodeID).Scan(&raw); err != nil {
			return err
		}
		var grant NodeGrant
		if err = json.Unmarshal([]byte(raw), &grant); err != nil {
			return err
		}
		if !grant.ExecutionEnabled || grant.Scope != item.Scope {
			return ErrNodeConflict
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE team_node_jobs SET state=`+r.dialect.Bind(1)+`,data_json=`+r.dialect.Bind(2)+` WHERE id=`+r.dialect.Bind(3)+` AND owner_user_id=`+r.dialect.Bind(4)+` AND node_id=`+r.dialect.Bind(5)+` AND state=`+r.dialect.Bind(6), item.State, string(data), item.ID, item.OwnerUserID, item.NodeID, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNodeConflict
	}
	if item.Delivery != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE team_node_jobs SET source_room_id=`+r.dialect.Bind(1)+`,source_message_id=`+r.dialect.Bind(2)+`,delivery_id=`+r.dialect.Bind(3)+` WHERE id=`+r.dialect.Bind(4), item.Delivery.RoomID, item.Delivery.MessageID, item.Delivery.ID, item.ID); err != nil {
			return err
		}
	}
	if output != nil {
		encoded, err := json.Marshal(output)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO team_node_outputs(job_id,sequence,output_id,data_json) VALUES (`+r.dialect.BindList(4)+`)`, item.ID, item.Sequence, item.ID+"_"+strconv.Itoa(item.Sequence), string(encoded))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) NextNodeOutput(ctx context.Context, jobID string) (*NodeOutput, error) {
	var item NodeOutput
	var data string
	err := r.db.QueryRowContext(ctx, `SELECT sequence,output_id,data_json FROM team_node_outputs WHERE job_id=`+r.dialect.Bind(1)+` AND sent=FALSE ORDER BY sequence LIMIT 1`, jobID).Scan(&item.Sequence, &item.ID, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(data), &item.Input); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) AckNodeOutput(ctx context.Context, jobID string, sequence int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE team_node_outputs SET sent=TRUE,data_json='' WHERE job_id=`+r.dialect.Bind(1)+` AND sequence=`+r.dialect.Bind(2), jobID, sequence)
	return err
}

func (r *Repository) NodeJobs(ctx context.Context, owner, scope string) ([]NodeJob, error) {
	return r.readNodeJobs(ctx, `SELECT state,data_json FROM team_node_jobs WHERE owner_user_id=`+r.dialect.Bind(1)+` AND scope=`+r.dialect.Bind(2)+` ORDER BY CASE WHEN state IN ('completed','failed') THEN 1 ELSE 0 END,created_at DESC,id DESC LIMIT 100`, scope, owner, scope)
}

// NodeMessageJobs 通过精确消息索引读取旧执行，不扩大常规授权面板的任务窗口。
func (r *Repository) NodeMessageJobs(ctx context.Context, owner, scope, room string, messages []string, jobID string) ([]NodeJob, error) {
	args := []any{owner, scope, room, jobID}
	match := `id=` + r.dialect.Bind(4)
	for _, id := range messages {
		args = append(args, id)
		match += ` OR source_message_id=` + r.dialect.Bind(len(args))
		args = append(args, id)
		match += ` OR delivery_id=` + r.dialect.Bind(len(args))
	}
	return r.readNodeJobs(ctx, `SELECT state,data_json FROM team_node_jobs WHERE owner_user_id=`+r.dialect.Bind(1)+` AND scope=`+r.dialect.Bind(2)+` AND source_room_id=`+r.dialect.Bind(3)+` AND (`+match+`) ORDER BY created_at,id`, scope, args...)
}

func (r *Repository) readNodeJobs(ctx context.Context, query, scope string, args ...any) ([]NodeJob, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []NodeJob{}
	for rows.Next() {
		var state, data string
		if err = rows.Scan(&state, &data); err != nil {
			return nil, err
		}
		var item NodeJob
		if err = json.Unmarshal([]byte(data), &item); err != nil {
			return nil, err
		}
		if item.Scope == scope {
			item.State = state
			result = append(result, item)
		}
	}
	return result, rows.Err()
}
