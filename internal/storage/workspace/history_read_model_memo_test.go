// INPUT: 长会话逐轮追加、库内分组被外部改写，以及备忘与库内签名不一致。
// OUTPUT: 前缀复用与备忘快路径落盘的读模型与完整重写逐字段一致，未变化分组不被重写。
// POS: DM 读模型增量落盘的回归。
package workspace

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type historyReadModelDump struct {
	Rows []string
}

func dumpHistoryReadModelScope(t *testing.T, f *longSessionFixture) historyReadModelDump {
	t.Helper()
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil || !ok {
		t.Fatalf("scope missing: ok=%v err=%v", ok, err)
	}
	dump := historyReadModelDump{}
	rows, err := db.QueryContext(context.Background(),
		`SELECT sequence, cursor_round_id, cursor_timestamp, group_key,
			COALESCE(first_non_synthetic_timestamp, -1), synthetic_interrupts_json, payload_digest, generation
		 FROM history_read_groups WHERE scope = ? ORDER BY sequence`, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var sequence, cursorTimestamp, first int64
		var cursorRoundID, groupKey, synthetic, digest, generation string
		if err = rows.Scan(&sequence, &cursorRoundID, &cursorTimestamp, &groupKey, &first, &synthetic, &digest, &generation); err != nil {
			t.Fatal(err)
		}
		if generation != scope.Generation {
			t.Fatalf("stale generation row survived: %s != %s", generation, scope.Generation)
		}
		dump.Rows = append(dump.Rows, fmt.Sprint(sequence, cursorRoundID, cursorTimestamp, groupKey, first, synthetic, digest))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	var keys int
	if err = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM history_read_message_keys WHERE scope = ? AND generation = ?`, access.Scope, scope.Generation).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if len(dump.Rows) != scope.GroupCount || keys == 0 {
		t.Fatalf("scope rows=%d group_count=%d keys=%d", len(dump.Rows), scope.GroupCount, keys)
	}
	return dump
}

// trackHistoryReadModelGroupWrites 在测试库上记录每次分组插入的 sequence。
func trackHistoryReadModelGroupWrites(t *testing.T, f *longSessionFixture) func() []int64 {
	t.Helper()
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(), `
		CREATE TABLE test_group_writes(sequence INTEGER NOT NULL);
		CREATE TRIGGER test_group_writes_trigger AFTER INSERT ON history_read_groups
		BEGIN INSERT INTO test_group_writes(sequence) VALUES (NEW.sequence); END;`); err != nil {
		t.Fatal(err)
	}
	return func() []int64 {
		t.Helper()
		rows, err := db.QueryContext(context.Background(), `DELETE FROM test_group_writes RETURNING sequence`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		written := []int64{}
		for rows.Next() {
			var sequence int64
			if err = rows.Scan(&sequence); err != nil {
				t.Fatal(err)
			}
			written = append(written, sequence)
		}
		slices.Sort(written)
		return written
	}
}

// rewriteHistoryReadModelFromScratch 丢弃库内 scope 与备忘后重建，得到完整写入的参照结果。
func rewriteHistoryReadModelFromScratch(t *testing.T, f *longSessionFixture) historyReadModelDump {
	t.Helper()
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	if err := access.ReadModel.deleteScope(context.Background(), access.Scope); err != nil {
		t.Fatal(err)
	}
	f.readFirstPage(t)
	return dumpHistoryReadModelScope(t, f)
}

func TestHistoryReadModelPersistReusesUnchangedGroups(t *testing.T) {
	f := newLongSessionFixture(t, 12, 64)
	f.readFirstPage(t)
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	previous := dumpHistoryReadModelScope(t, f)
	writes := trackHistoryReadModelGroupWrites(t, f)
	// 本测试覆盖全量重建的落盘备忘；丢掉已发布的尾轮状态，避免刷新走 DM 增量路径。
	forceFullRebuild := func() { dropAgentHistoryTailState(t, f) }
	for round := 0; round < 4; round++ {
		f.appendRound(t)
		forceFullRebuild()
		f.readFirstPage(t)
		current := dumpHistoryReadModelScope(t, f)
		// 追加一轮只新增末尾分组，之前的分组行原地保留。
		if written := writes(); len(current.Rows) != len(previous.Rows)+1 ||
			!reflect.DeepEqual(written, []int64{int64(len(previous.Rows))}) ||
			!reflect.DeepEqual(current.Rows[:len(previous.Rows)], previous.Rows) {
			t.Fatalf("round %d rewrote unchanged groups: wrote %v", round, written)
		}
		if access.ReadModel.memo.load(access.Scope, readHistoryReadModelGenerationForTest(t, f)) == nil {
			t.Fatalf("round %d did not record a persist memo", round)
		}
		previous = current
	}

	// 库内分组被外部改写后，签名与备忘不再一致，必须重新计算并修复该分组。
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(),
		`UPDATE history_read_groups SET payload_digest = 'corrupt' WHERE scope = ? AND sequence = 3`, access.Scope); err != nil {
		t.Fatal(err)
	}
	f.appendRound(t)
	forceFullRebuild()
	f.readFirstPage(t)
	repaired := dumpHistoryReadModelScope(t, f)
	if strings.Contains(strings.Join(repaired.Rows, "\n"), "corrupt") {
		t.Fatal("corrupted group survived the memo fast path")
	}
	if written := writes(); len(written) == 0 || written[0] != 3 {
		t.Fatalf("repair must start at the corrupted group, wrote %v", written)
	}

	// 与丢弃一切后完整写入的结果逐字段一致。
	if want := rewriteHistoryReadModelFromScratch(t, f); !reflect.DeepEqual(repaired.Rows, want.Rows) {
		t.Fatalf("incremental persist diverged from full rewrite\ngot  %v\nwant %v", repaired.Rows, want.Rows)
	}
}

func readHistoryReadModelGenerationForTest(t *testing.T, f *longSessionFixture) string {
	t.Helper()
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil || !ok {
		t.Fatalf("scope missing: ok=%v err=%v", ok, err)
	}
	return scope.Generation
}

func TestHistoryReadModelGroupHashDistinguishesContentAndTypes(t *testing.T) {
	group := func(items ...map[string]any) historyPageIndexedGroup {
		rows := make([]map[string]any, 0, len(items))
		rows = append(rows, items...)
		converted := make([]historyPageIndexedGroup, 1)
		for _, item := range rows {
			converted[0].Items = append(converted[0].Items, item)
		}
		converted[0].CursorRoundID = "r"
		return converted[0]
	}
	base := group(map[string]any{"a": "1", "b": []map[string]any{{"x": int64(1)}}, "c": true})
	same := group(map[string]any{"c": true, "b": []map[string]any{{"x": int64(1)}}, "a": "1"})
	hash := func(g historyPageIndexedGroup, sequence int) uint64 {
		return historyReadModelGroupHash("scope", "gen", sequence, g)
	}
	if hash(base, 0) != hash(same, 0) {
		t.Fatal("map order changed the hash")
	}
	variants := []historyPageIndexedGroup{
		group(map[string]any{"a": 1, "b": []map[string]any{{"x": int64(1)}}, "c": true}),
		group(map[string]any{"a": "1", "b": []map[string]any{{"x": 1}}, "c": true}),
		group(map[string]any{"a": "1", "b": []any{map[string]any{"x": int64(1)}}, "c": true}),
		group(map[string]any{"a": "1", "b": []map[string]any{{"x": int64(2)}}, "c": true}),
		group(map[string]any{"a": "1", "b": []map[string]any{{"x": int64(1)}}, "c": false}),
		group(map[string]any{"a": "1", "b": []map[string]any{{"x": int64(1)}}}),
		group(map[string]any{"a": "1"}, map[string]any{"b": []map[string]any{{"x": int64(1)}}, "c": true}),
	}
	for i, variant := range variants {
		if hash(variant, 0) == hash(base, 0) {
			t.Fatalf("variant %d collided with base", i)
		}
	}
	if hash(base, 1) == hash(base, 0) {
		t.Fatal("sequence not part of the hash")
	}
}

func TestHistoryReadModelMemoSurvivesRepeatedRebuilds(t *testing.T) {
	var store historyReadModelMemoStore
	chunks := []historyReadModelRoundChunkMemo{{Hash: 1, Bytes: 2}}
	// 每次全量重建都是先 forget 再写入；重复重建不能挤掉同一 scope 刚写入的备忘。
	for revision := range int64(2 * maxHistoryReadModelMemoScopes) {
		store.forget("scope")
		store.store("scope", "generation", []historyReadModelGroupMemo{{Hash: 3}})
		store.storeRounds("scope", revision, chunks)
		if store.load("scope", "generation") == nil {
			t.Fatalf("第 %d 次重建后分组备忘丢失", revision)
		}
		if _, ok := store.loadRounds("scope", revision); !ok {
			t.Fatalf("第 %d 次重建后 round 分块备忘丢失", revision)
		}
	}
	for index := range maxHistoryReadModelMemoScopes {
		store.storeRounds(fmt.Sprintf("other-%d", index), 0, chunks)
	}
	if _, ok := store.loadRounds("scope", 0); ok {
		t.Fatal("最旧的 scope 应被淘汰")
	}
	if _, ok := store.loadRounds("other-0", 0); !ok {
		t.Fatal("较新的 scope 不应被淘汰")
	}
}
