// INPUT: 读模型数据库、scope 及其当前 generation。
// OUTPUT: 每个 scope 一份的尾部增量状态（尾部原始行与调用方自定义的状态 JSON）以及已冻结前缀的身份键集合。
// POS: DM 与 Room 增量刷新共用的持久化协议；状态与分组在同一事务内发布，进程重启后可直接续接。
package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 单条语句写入或查询的身份键数量，低于 SQLite 默认的绑定参数上限。
const historyReadTailKeyBatch = 300

// 身份键的种类。前三种是规范化阶段会跨行关联的历史行身份：compact 与投递回执按 message_id，
// 未完成轮次、result 配对与分组按 round/group key，引导去重按 source_round_id；
// 后两种是 transcript 条目的父链身份与工具调用 id，决定主链选择与并行条目归属。
const (
	historyTailKeyMessage = iota + 1
	historyTailKeyGroup
	historyTailKeyGuidance
	historyTailKeyTranscriptLink
	historyTailKeyTranscriptTool
)

type historyTailKey struct {
	Kind int
	Key  string
}

// historyTailKeys 是一组身份键。两组原始数据的身份集合互不相交时，它们的投影与规范化结果互不影响。
type historyTailKeys map[historyTailKey]struct{}

func (k historyTailKeys) add(kind int, key string) {
	if key != "" {
		k[historyTailKey{Kind: kind, Key: key}] = struct{}{}
	}
}

func (k historyTailKeys) addRow(row protocol.Message) {
	k.add(historyTailKeyMessage, stringFromAny(row["message_id"]))
	k.add(historyTailKeyGroup, historyPageGroupKey(row, false))
	if stringFromAny(row[overlayKindField]) == overlayKindExternalDelivery {
		k.add(historyTailKeyMessage, stringFromAny(row["assistant_message_id"]))
	}
	if stringFromAny(row["role"]) == "user" {
		k.add(historyTailKeyGuidance, stringFromAny(row["source_round_id"]))
	}
	if metadata, ok := row["metadata"].(map[string]any); ok {
		k.add(historyTailKeyGuidance, stringFromAny(metadata["source_round_id"]))
	}
}

func (k historyTailKeys) addTranscriptEntry(entry map[string]any) {
	k.add(historyTailKeyTranscriptLink, stringFromAny(entry["uuid"]))
	k.add(historyTailKeyTranscriptLink, stringFromAny(entry["parentUuid"]))
	k.add(historyTailKeyTranscriptLink, stringFromAny(entry["logicalParentUuid"]))
	k.addTranscriptTools(entry)
}

func (k historyTailKeys) addTranscriptTools(entry map[string]any) {
	for _, id := range transcriptToolUseIDs(entry) {
		k.add(historyTailKeyTranscriptTool, id)
	}
	for _, id := range transcriptToolResultIDs(entry) {
		k.add(historyTailKeyTranscriptTool, id)
	}
	_, toolUseID := transcriptSubagentAttachment(entry)
	k.add(historyTailKeyTranscriptTool, toolUseID)
}

func (k historyTailKeys) intersects(other historyTailKeys) bool {
	if len(k) > len(other) {
		k, other = other, k
	}
	for key := range k {
		if _, ok := other[key]; ok {
			return true
		}
	}
	return false
}

// persistHistoryReadTail 写入 scope 的尾部增量状态；state 由调用方定义，nil 表示下次只能完整重建。
func persistHistoryReadTail(ctx context.Context, tx *sql.Tx, scope, generation string, rows []protocol.Message, state any) error {
	payload, err := marshalHistoryPageJSONBounded(rows, historyReadModelMaxGroupBytes)
	if err != nil {
		return err
	}
	statePayload, err := marshalHistoryPageJSONBounded(state, historyReadModelMaxPageBytes)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO history_read_tail(scope,generation,rows_json,state_json,rows_digest) VALUES(?,?,?,?,?)
 ON CONFLICT(scope) DO UPDATE SET generation=excluded.generation, rows_json=excluded.rows_json, state_json=excluded.state_json, rows_digest=excluded.rows_digest`, scope, generation, payload, statePayload, historyReadTailDigest(payload, statePayload))
	return err
}

// readHistoryReadTail 读取 scope 在指定 generation 下的尾部增量状态并校验摘要。
func readHistoryReadTail(ctx context.Context, db historyReadModelQuerier, scope, generation string) (rows []byte, state []byte, err error) {
	var digest string
	err = db.QueryRowContext(ctx, `SELECT rows_json,state_json,rows_digest FROM history_read_tail WHERE scope=? AND generation=?`, scope, generation).Scan(&rows, &state, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("tail state missing: %w", errHistoryPageIndexInvalid)
	}
	if err != nil {
		return nil, nil, err
	}
	if historyReadTailDigest(rows, state) != digest {
		return nil, nil, fmt.Errorf("tail state corrupted: %w", errHistoryPageIndexInvalid)
	}
	return rows, state, nil
}

func historyReadTailDigest(rows []byte, state []byte) string {
	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%d:", len(rows))
	_, _ = hasher.Write(rows)
	_, _ = fmt.Fprintf(hasher, "|%d:", len(state))
	_, _ = hasher.Write(state)
	return hex.EncodeToString(hasher.Sum(nil))
}

// replaceHistoryReadTailKeys 用 keys 替换 scope 已冻结前缀的全部身份键。
func replaceHistoryReadTailKeys(ctx context.Context, tx *sql.Tx, scope string, keys historyTailKeys) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM history_read_tail_keys WHERE scope=?`, scope); err != nil {
		return err
	}
	return insertHistoryReadTailKeys(ctx, tx, scope, keys)
}

// insertHistoryReadTailKeys 把新冻结的身份键并入 scope 的集合，已有的键保持不变。
func insertHistoryReadTailKeys(ctx context.Context, tx *sql.Tx, scope string, keys historyTailKeys) error {
	return forEachHistoryTailKeyBatch(keys, func(kind int, batch []string) error {
		args := make([]any, 0, len(batch)*3)
		for _, key := range batch {
			args = append(args, scope, kind, key)
		}
		statement := `INSERT OR IGNORE INTO history_read_tail_keys(scope,kind,key) VALUES ` +
			strings.TrimSuffix(strings.Repeat("(?,?,?),", len(batch)), ",")
		_, err := tx.ExecContext(ctx, statement, args...)
		return err
	})
}

// historyReadTailKeysContain 判断 keys 中是否有任何一个已在 scope 的身份键集合里。
func historyReadTailKeysContain(ctx context.Context, db historyReadModelQuerier, scope string, keys historyTailKeys) (bool, error) {
	found := false
	err := forEachHistoryTailKeyBatch(keys, func(kind int, batch []string) error {
		if found {
			return nil
		}
		args := make([]any, 0, len(batch)+2)
		args = append(args, scope, kind)
		for _, key := range batch {
			args = append(args, key)
		}
		statement := `SELECT EXISTS(SELECT 1 FROM history_read_tail_keys WHERE scope=? AND kind=? AND key IN (` +
			strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + `))`
		return db.QueryRowContext(ctx, statement, args...).Scan(&found)
	})
	return found, err
}

// forEachHistoryTailKeyBatch 按种类分批遍历身份键，每批不超过 historyReadTailKeyBatch 个。
func forEachHistoryTailKeyBatch(keys historyTailKeys, visit func(kind int, batch []string) error) error {
	byKind := make(map[int][]string)
	for key := range keys {
		byKind[key.Kind] = append(byKind[key.Kind], key.Key)
	}
	for kind, items := range byKind {
		for batch := range slices.Chunk(items, historyReadTailKeyBatch) {
			if err := visit(kind, batch); err != nil {
				return err
			}
		}
	}
	return nil
}

// transcriptSubagentAttachment 返回子任务 structured_output 附件所属的 agent 与工具调用 id，
// 其他条目返回空串。
func transcriptSubagentAttachment(entry map[string]any) (agentID string, toolUseID string) {
	if stringFromAny(entry["type"]) != "attachment" {
		return "", ""
	}
	attachment, _ := entry["attachment"].(map[string]any)
	if stringFromAny(attachment["type"]) != "structured_output" {
		return "", ""
	}
	data, _ := attachment["data"].(map[string]any)
	return textutil.FirstNonEmpty(stringFromAny(data["agent_id"]), stringFromAny(data["agentId"])),
		textutil.FirstNonEmpty(stringFromAny(data["tool_use_id"]), stringFromAny(data["toolUseId"]))
}
