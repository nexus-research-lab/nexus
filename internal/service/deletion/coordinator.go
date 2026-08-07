// INPUT: owner、删除种类、稳定目标与完成清理所需的完整 payload。
// OUTPUT: 可跨失败和重启恢复的删除任务，以及幂等完成/失败状态更新。
// POS: 业务主记录与数据库外 runtime/文件产物之间的持久事务桥。
package deletion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Kind 表示具有独立恢复语义的删除操作。
type Kind string

const (
	KindSession       Kind = "session"
	KindRoom          Kind = "room"
	KindConversation  Kind = "conversation"
	KindRoomMember    Kind = "room_member"
	KindAgent         Kind = "agent"
	KindScheduledTask Kind = "scheduled_task"
	KindSkill         Kind = "skill"
)

// Job 是删除恢复器持有的完整持久任务。
type Job struct {
	ID          string
	OwnerUserID string
	Kind        Kind
	TargetID    string
	Payload     json.RawMessage
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Coordinator 持久化跨介质删除任务。
type Coordinator struct {
	db      *sql.DB
	dialect storage.SQLDialect
	goals   sessionGoalCleaner
	tasks   sessionTaskCleaner
}

// NewCoordinator 创建共享删除协调器。
func NewCoordinator(cfg config.Config, db *sql.DB) *Coordinator {
	return &Coordinator{
		db:      db,
		dialect: storage.NewSQLDialect(cfg.DatabaseDriver),
	}
}

// SetGoalCleaner 注入 Session 作用域 Goal 清理器。
func (c *Coordinator) SetGoalCleaner(cleaner sessionGoalCleaner) {
	if c != nil {
		c.goals = cleaner
	}
}

// SetTaskCleaner 注入绑定到 Session 的定时任务清理器。
func (c *Coordinator) SetTaskCleaner(cleaner sessionTaskCleaner) {
	if c != nil {
		c.tasks = cleaner
	}
}

// Ensure 先登记删除任务；同目标已有任务时保留最初的完整 payload。
func (c *Coordinator) Ensure(
	ctx context.Context,
	ownerUserID string,
	kind Kind,
	targetID string,
	payload any,
) (Job, error) {
	if c == nil || c.db == nil {
		return Job{}, errors.New("删除协调器未初始化")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	targetID = strings.TrimSpace(targetID)
	if ownerUserID == "" || targetID == "" || !kind.Valid() {
		return Job{}, errors.New("删除任务作用域不完整")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Job{}, fmt.Errorf("编码删除任务: %w", err)
	}
	jobID := stableJobID(ownerUserID, kind, targetID)
	query := c.dialect.InsertIgnoreInto("deletion_jobs") + ` (
    job_id, owner_user_id, kind, target_id, payload_json
) VALUES (` + c.dialect.BindList(4) + `, ` + c.dialect.JSONValue(5) + `)` + c.dialect.InsertIgnoreSuffix()
	if _, err = c.db.ExecContext(
		ctx,
		query,
		jobID,
		ownerUserID,
		string(kind),
		targetID,
		string(encoded),
	); err != nil {
		return Job{}, fmt.Errorf("登记删除任务: %w", err)
	}
	job, err := c.Load(ctx, ownerUserID, kind, targetID)
	if err != nil {
		return Job{}, err
	}
	if job == nil {
		return Job{}, errors.New("删除任务登记后不可见")
	}
	return *job, nil
}

// Load 读取指定目标尚未完成的删除任务。
func (c *Coordinator) Load(
	ctx context.Context,
	ownerUserID string,
	kind Kind,
	targetID string,
) (*Job, error) {
	if c == nil || c.db == nil || !kind.Valid() {
		return nil, nil
	}
	row := c.db.QueryRowContext(ctx, `
SELECT job_id, owner_user_id, kind, target_id, `+c.dialect.JSONText("payload_json")+`,
       attempts, COALESCE(last_error, ''), created_at, updated_at
FROM deletion_jobs
WHERE owner_user_id = `+c.dialect.Bind(1)+`
  AND kind = `+c.dialect.Bind(2)+`
  AND target_id = `+c.dialect.Bind(3)+`
LIMIT 1`,
		strings.TrimSpace(ownerUserID),
		string(kind),
		strings.TrimSpace(targetID),
	)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取删除任务: %w", err)
	}
	return &job, nil
}

// ListPending 返回指定种类的全部待恢复任务。
func (c *Coordinator) ListPending(ctx context.Context, kinds ...Kind) ([]Job, error) {
	if c == nil || c.db == nil {
		return nil, nil
	}
	args := make([]any, 0, len(kinds))
	query := `
SELECT job_id, owner_user_id, kind, target_id, ` + c.dialect.JSONText("payload_json") + `,
       attempts, COALESCE(last_error, ''), created_at, updated_at
FROM deletion_jobs`
	if len(kinds) > 0 {
		binds := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			if !kind.Valid() {
				continue
			}
			args = append(args, string(kind))
			binds = append(binds, c.dialect.Bind(len(args)))
		}
		if len(binds) == 0 {
			return nil, nil
		}
		query += " WHERE kind IN (" + strings.Join(binds, ",") + ")"
	}
	query += " ORDER BY updated_at ASC, job_id ASC"
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("列出删除任务: %w", err)
	}
	defer rows.Close()
	result := make([]Job, 0)
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, job)
	}
	return result, rows.Err()
}

// DecodePayload 将任务 payload 解码为领域删除清单。
func DecodePayload(job Job, target any) error {
	if len(job.Payload) == 0 {
		return errors.New("删除任务 payload 为空")
	}
	if err := json.Unmarshal(job.Payload, target); err != nil {
		return fmt.Errorf("解码删除任务 %s: %w", job.ID, err)
	}
	return nil
}

// Fail 保留任务并记录本次失败，供下一次请求或启动恢复器继续。
func (c *Coordinator) Fail(ctx context.Context, job Job, failure error) error {
	if c == nil || c.db == nil || strings.TrimSpace(job.ID) == "" {
		return failure
	}
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	_, err := c.db.ExecContext(ctx, `
UPDATE deletion_jobs
SET attempts = attempts + 1,
    last_error = `+c.dialect.Bind(1)+`,
    updated_at = `+c.dialect.CurrentTimestamp()+`
WHERE job_id = `+c.dialect.Bind(2),
		message,
		job.ID,
	)
	return errors.Join(failure, err)
}

// Complete 在全部介质清理完成后移除持久任务。
func (c *Coordinator) Complete(ctx context.Context, job Job) error {
	if c == nil || c.db == nil || strings.TrimSpace(job.ID) == "" {
		return nil
	}
	_, err := c.db.ExecContext(
		ctx,
		"DELETE FROM deletion_jobs WHERE job_id = "+c.dialect.Bind(1),
		job.ID,
	)
	return err
}

// Valid 判断是否是受支持的删除种类。
func (k Kind) Valid() bool {
	switch k {
	case KindSession, KindRoom, KindConversation, KindRoomMember, KindAgent, KindScheduledTask, KindSkill:
		return true
	default:
		return false
	}
}

type jobScanner interface {
	Scan(...any) error
}

func scanJob(scanner jobScanner) (Job, error) {
	var job Job
	var kind string
	var payload string
	err := scanner.Scan(
		&job.ID,
		&job.OwnerUserID,
		&kind,
		&job.TargetID,
		&payload,
		&job.Attempts,
		&job.LastError,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	job.Kind = Kind(kind)
	job.Payload = json.RawMessage(payload)
	return job, err
}

func stableJobID(ownerUserID string, kind Kind, targetID string) string {
	digest := sha256.Sum256([]byte(ownerUserID + "\x00" + string(kind) + "\x00" + targetID))
	return "delete_" + hex.EncodeToString(digest[:16])
}
