// INPUT: 长会话逐轮追加、进程重启、round index 缩短，以及库内 round 分块被其他写入者改写。
// OUTPUT: round index 只写入内容变化的分块，落盘结果与整体重写一致；备忘只在 round_revision 未变时生效。
// POS: 读模型 round index 分块落盘的回归。
package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// trackHistoryReadModelRoundWrites 让测试库记录每次 round 分块插入或改写的块号。
func trackHistoryReadModelRoundWrites(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE test_round_writes(chunk INTEGER NOT NULL);
		CREATE TRIGGER test_round_inserts AFTER INSERT ON history_read_round_chunks
		BEGIN INSERT INTO test_round_writes(chunk) VALUES (NEW.chunk); END;
		CREATE TRIGGER test_round_updates AFTER UPDATE ON history_read_round_chunks
		BEGIN INSERT INTO test_round_writes(chunk) VALUES (NEW.chunk); END;`); err != nil {
		t.Fatal(err)
	}
}

// takeHistoryReadModelRoundWrites 取走上次调用以来被写入的块号。
func takeHistoryReadModelRoundWrites(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `DELETE FROM test_round_writes RETURNING chunk`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	written := []int64{}
	for rows.Next() {
		var chunk int64
		if err = rows.Scan(&chunk); err != nil {
			t.Fatal(err)
		}
		written = append(written, chunk)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(written)
	return written
}

func TestAgentHistoryTailRefreshWritesOnlyChangedRoundChunks(t *testing.T) {
	// 起点离第二块写满还差一轮：依次覆盖写满旧块、开新块、重启后续写。
	const rounds = 2*historyReadModelRoundChunkSize - 1
	f := newLongSessionFixture(t, rounds, 64)
	f.readFirstPage(t)
	database := func() *sql.DB {
		db, err := f.history.readModel.database(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	trackHistoryReadModelRoundWrites(t, database())

	refresh := func(want ...int64) {
		t.Helper()
		before := f.history.cache.tailRefreshes.Load()
		f.readFirstPage(t)
		if f.history.cache.tailRefreshes.Load() == before {
			t.Fatal("refresh did not take the tail path")
		}
		if got := takeHistoryReadModelRoundWrites(t, database()); !slices.Equal(got, want) {
			t.Fatalf("round chunks written = %v, want %v", got, want)
		}
	}
	f.appendRound(t)
	refresh(1)
	f.appendRound(t)
	refresh(2)
	// 重启后备忘丢失：回读库内各块比对，仍然只写变化的块。
	simulateHistoryProcessRestart(t, f)
	f.appendRound(t)
	refresh(2)
	f.appendRound(t)
	refresh(2)

	scope := f.history.historyPageAccess(f.workspacePath, f.session).Scope
	got := dumpHistoryReadModelRounds(t, database(), scope)
	if len(got) != 3 {
		t.Fatalf("round chunks = %d, want 3", len(got))
	}
	if err := f.history.readModel.deleteScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	f.history = NewAgentHistoryStore(filepath.Dir(f.workspacePath))
	f.readFirstPage(t)
	if want := dumpHistoryReadModelRounds(t, database(), scope); !reflect.DeepEqual(got, want) {
		t.Fatalf("incremental round chunks diverged from full rewrite\ngot  %v\nwant %v", got, want)
	}
}

func TestHistoryReadModelSyncRoundIndex(t *testing.T) {
	ctx := context.Background()
	model := &historyReadModel{path: filepath.Join(t.TempDir(), historyReadModelFileName)}
	db, err := model.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	trackHistoryReadModelRoundWrites(t, db)
	const scope = "scope"
	const size = historyReadModelRoundChunkSize
	// items 生成 count 个 round，retitled 中的位置换一个标题。
	items := func(count int, retitled ...int) []protocol.SessionRoundIndexItem {
		result := make([]protocol.SessionRoundIndexItem, count)
		for index := range result {
			result[index] = protocol.SessionRoundIndexItem{RoundID: fmt.Sprintf("round-%d", index), Title: "title", Timestamp: int64(index + 1)}
		}
		for _, index := range retitled {
			result[index].Title = "retitled"
		}
		return result
	}
	revision := int64(0)
	// sync 模拟一次发布：同步 round 分块、换修订号并更新备忘，然后核对库内内容与写入的块号。
	sync := func(published *historyReadModelScope, next []protocol.SessionRoundIndexItem, want ...int64) *historyReadModelScope {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		chunks, err := model.syncRoundIndex(ctx, tx, scope, published, next)
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		revision++
		model.memo.storeRounds(scope, revision, chunks)
		if got := takeHistoryReadModelRoundWrites(t, db); !slices.Equal(got, want) {
			t.Fatalf("round chunks written = %v, want %v", got, want)
		}
		stored, err := readHistoryReadModelRoundIndex(ctx, db, scope, len(next))
		if err != nil || !reflect.DeepEqual(stored, next) {
			t.Fatalf("stored round index diverged (%v): %d items, want %d", err, len(stored), len(next))
		}
		return &historyReadModelScope{RoundCount: len(next), RoundRevision: revision}
	}

	published := sync(nil, items(2*size+10), 0, 1, 2)
	published = sync(published, items(2*size+11, size+3), 1, 2)
	published = sync(published, items(size, size-1), 0)
	published = sync(published, items(0))
	published = sync(published, items(size+1), 0, 1)

	// 其他写入者改写了分块并换了修订号：备忘作废，回读库内各块后仍只写变化的块。
	foreign, err := json.Marshal(items(size, 5))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE history_read_round_chunks SET payload = ? WHERE scope = ? AND chunk = 0`, foreign, scope); err != nil {
		t.Fatal(err)
	}
	takeHistoryReadModelRoundWrites(t, db)
	published = sync(&historyReadModelScope{RoundCount: published.RoundCount, RoundRevision: -1}, items(size+2), 0, 1)

	// 库内分块与 scope 行的 round 数不符时无法确认已有内容，整体重写。
	sync(&historyReadModelScope{RoundCount: published.RoundCount + 1, RoundRevision: -1}, items(size+2), 0, 1)
}

func TestHistoryReadModelRoundChunkHashCoversEveryField(t *testing.T) {
	hash := func(item protocol.SessionRoundIndexItem) uint64 {
		return historyReadModelRoundChunkHash([]protocol.SessionRoundIndexItem{item})
	}
	base := hash(protocol.SessionRoundIndexItem{})
	if base == historyReadModelRoundChunkHash(nil) {
		t.Fatal("an empty chunk hashes like a zero-value round")
	}
	itemType := reflect.TypeFor[protocol.SessionRoundIndexItem]()
	duration := int64(0)
	for index := range itemType.NumField() {
		var item protocol.SessionRoundIndexItem
		switch value := reflect.ValueOf(&item).Elem().Field(index).Addr().Interface().(type) {
		case *string:
			*value = "x"
		case *int64:
			*value = 1
		case *bool:
			*value = true
		case **int64:
			*value = &duration
		case *[]string:
			*value = []string{""}
		default:
			t.Fatalf("field %s has a type the round hash does not cover", itemType.Field(index).Name)
		}
		if hash(item) == base {
			t.Fatalf("field %s does not change the round hash", itemType.Field(index).Name)
		}
	}
}
