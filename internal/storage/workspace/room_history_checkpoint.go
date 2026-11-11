// INPUT: Room 公开引用、成员 transcript/overlay 快照和私有 session 身份。
// OUTPUT: 经过来源与摘要校验的逐成员公开消息投影，可在下次启动继续缺失成员的回填。
// POS: 宿主派生库中的 Room 重建检查点；只有 ledger 点名的消息能够进入本表。
package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 只对要驻留的公开投影计重；未公开的历史文件仍可流式跨过任意多个批次。
type roomRefBudget int64

func (b *roomRefBudget) replace(oldBytes, newBytes int) error {
	remaining := int64(*b) + int64(oldBytes) - int64(newBytes)
	if remaining < 0 {
		return errHistoryPageIndexResourceLimit
	}
	*b = roomRefBudget(remaining)
	return nil
}

type roomRefCheckpoint struct {
	owner        string
	conversation string
	member       string
	stamp        string
}

func (c roomRefCheckpoint) scope() string { return roomHistoryScope(c.owner, c.conversation) }

type roomRefSelection struct {
	ref roomTranscriptRef
	ids map[string]bool
}

type roomRefSelections map[string]*roomRefSelection

func (s roomRefSelections) add(ref roomTranscriptRef, ids ...string) {
	key := ref.cacheKey()
	member := s[key]
	if member == nil {
		member = &roomRefSelection{ref: ref, ids: make(map[string]bool)}
		s[key] = member
	}
	for _, id := range ids {
		member.ids[id] = true
	}
}

func (s *RoomHistoryStore) selectRoomRefs(owner string, rows []map[string]any) roomRefSelections {
	members := roomRefSelections{}
	checked := roomWorkspaceChecks{}
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindTranscriptRef {
			continue
		}
		ref, ok := roomTranscriptRefFromRow(row)
		if !ok {
			continue
		}
		if checked.allows(s.paths, owner, ref.WorkspacePath) {
			members.add(ref, ref.MessageID)
		}
	}
	return members
}

// 成员按固定顺序回填，某个成员写入检查点后即能独立复用，后续成员或最终 generation 的失败不会撤销它。
func (s *RoomHistoryStore) prepareRoomRefCheckpoints(ctx context.Context, owner, conversation string, members roomRefSelections, sources []historyPageSourceSnapshot, cache roomTranscriptIndexCache, budget roomRefBudget) error {
	if len(members) == 0 {
		return nil
	}
	started := time.Now()
	var reused, recovered int
	db, dbErr := s.readModel.database(ctx)
	for _, key := range slices.Sorted(maps.Keys(members)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		member := members[key]

		var pair []historyPageSourceSnapshot
		for _, source := range sources {
			if member.ref.dependsOn(source) {
				pair = append(pair, source)
			}
		}
		if len(pair) != 2 {
			return errHistoryPageIndexInvalid
		}
		point := roomRefCheckpoint{owner: owner, conversation: conversation, member: historyPageScope(key), stamp: historyPageSourceDigest(pair)}
		items := make(map[string]protocol.Message)
		missing := member.ids
		if dbErr == nil {
			available := budget
			saved, remaining, readErr := point.read(ctx, db, member.ids, &available)
			if readErr == nil {
				items, missing, budget = saved, remaining, available
			} else if errors.Is(readErr, errHistoryPageIndexResourceLimit) {
				return readErr
			} else if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		reused += len(member.ids) - len(missing)
		cache[key] = items
		if len(missing) == 0 {
			continue
		}
		fresh, projectErr := s.agentHistory.ForOwner(owner).projectRoomTranscriptRefs(ctx, member.ref, missing, &budget)
		if err := ctx.Err(); err != nil {
			return err
		}
		// 只有读取前后的选择和指纹一致，才能解释本次重放的结果或错误。
		_, changed, err := s.snapshotRoomHistoryDependencies(ctx, owner, pair)
		if errors.Is(err, errHistoryPageIndexInvalid) || len(changed) > 0 {
			return errHistoryPageSourcesChanged
		}
		if err != nil {
			return err
		}
		if projectErr != nil {
			return projectErr
		}
		maps.Copy(items, fresh)
		recovered += len(missing)
		if dbErr != nil {
			continue
		}
		if err = s.writeRoomRefCheckpoints(ctx, db, point, missing, fresh); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logx.FromContext(ctx).Warn("Room 历史成员检查点未保存", "conversation_id", conversation, "err", err)
		}
	}
	logx.FromContext(ctx).Info("Room 历史成员投影完成", "conversation_id", conversation, "members", len(members), "cached_refs", reused, "rebuilt_refs", recovered, "duration_ms", time.Since(started).Milliseconds())
	return nil
}

func (c roomRefCheckpoint) read(ctx context.Context, db *sql.DB, wanted map[string]bool, budget *roomRefBudget) (map[string]protocol.Message, map[string]bool, error) {
	items := make(map[string]protocol.Message)
	missing := maps.Clone(wanted)
	ids, err := json.Marshal(slices.Sorted(maps.Keys(wanted)))
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT message_id,payload,digest FROM history_room_refs
	WHERE scope=? AND member=? AND stamp=? AND message_id IN (SELECT value FROM json_each(?))`, c.scope(), c.member, c.stamp, string(ids))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var payload, digest []byte
		if err = rows.Scan(&id, &payload, &digest); err != nil {
			return nil, nil, err
		}
		sum := sha256.Sum256(payload)
		if !slices.Equal(sum[:], digest) {
			continue
		}
		var value protocol.Message
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			continue
		}
		if value != nil {
			if stringFromAny(value["message_id"]) != id {
				continue
			}
			if err = budget.replace(0, len(payload)); err != nil {
				return nil, nil, err
			}
			items[id] = value
		}
		delete(missing, id)
	}
	return items, missing, rows.Err()
}

func (s *RoomHistoryStore) writeRoomRefCheckpoints(ctx context.Context, db *sql.DB, point roomRefCheckpoint, missing map[string]bool, items map[string]protocol.Message) error {
	const targetBytes = 1 << 20
	const maxItems = 256
	batch := make([]roomRefRecord, 0, maxItems)
	size := 0
	for _, id := range slices.Sorted(maps.Keys(missing)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		payload, err := json.Marshal(items[id])
		if err != nil {
			return err
		}
		batch = append(batch, roomRefRecord{id: id, payload: payload})
		size += len(payload)
		if size >= targetBytes || len(batch) == maxItems {
			if err = s.commitRoomRefBatch(ctx, db, point, batch); err != nil {
				return err
			}
			clear(batch)
			batch = batch[:0]
			size = 0
		}
	}
	if len(batch) == 0 {
		return nil
	}
	return s.commitRoomRefBatch(ctx, db, point, batch)
}

type roomRefRecord struct {
	id      string
	payload []byte
}

func (s *RoomHistoryStore) commitRoomRefBatch(ctx context.Context, db *sql.DB, point roomRefCheckpoint, batch []roomRefRecord) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query, err := tx.PrepareContext(ctx, `INSERT INTO history_room_refs(scope,member,stamp,message_id,payload,digest,updated_at_ms) VALUES(?,?,?,?,?,?,?)
	ON CONFLICT(scope,member,message_id) DO UPDATE SET stamp=excluded.stamp,payload=excluded.payload,digest=excluded.digest,updated_at_ms=excluded.updated_at_ms`)
	if err != nil {
		return err
	}
	defer query.Close()
	scope := point.scope()
	now := time.Now().UnixMilli()
	for _, record := range batch {
		sum := sha256.Sum256(record.payload)
		if _, err = query.ExecContext(ctx, scope, point.member, point.stamp, record.id, record.payload, sum[:], now); err != nil {
			return err
		}
	}
	// 删除方与本事务竞争同一写锁；删除后不留下迟到的派生消息。
	root, err := s.openRoomHistoryPageIndexRoot(point.owner, point.conversation, false)
	if err != nil {
		return err
	}
	root.Close()
	return tx.Commit()
}
