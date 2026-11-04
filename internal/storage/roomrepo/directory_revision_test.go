// INPUT: 两个独立 SQLite 连接上的 Room 事务、成员变更及迁移回滚。
// OUTPUT: 另一连接仅观察已提交目录变化，普通消息活动不增加目录结构版本。
// POS: standalone CLI 与宿主共享数据库的目录通知回归。
package roomrepo

import (
	"context"
	"github.com/pressly/goose/v3"
	"testing"
	"time"
)

func TestDirectoryRevisionAcrossConnections(t *testing.T) {
	writer, _, writeRepo, reader := newConcurrentRoomRepositories(t)
	ctx := context.Background()
	read := func() int64 {
		t.Helper()
		revision, err := reader.DirectoryRevision(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return revision
	}
	before := read()
	seedRoomRepositoryAgent(t, writer, "owner", "one")
	seedRoomRepositoryAgent(t, writer, "owner", "two")
	createRepositoryRoom(t, writeRepo, "owner", "room", "one", "one", "two")
	created := read()
	if created <= before {
		t.Fatal("creation did not invalidate directory")
	}
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE rooms SET name = 'pending' WHERE id = 'room'`); err != nil {
		t.Fatal(err)
	}
	if read() != created {
		t.Fatal("uncommitted change became visible")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if read() != created {
		t.Fatal("rollback changed directory revision")
	}
	for _, query := range []string{
		`UPDATE rooms SET name = 'renamed' WHERE id = 'room'`,
		`UPDATE conversations SET title = 'renamed conversation' WHERE room_id = 'room'`,
		`DELETE FROM members WHERE room_id = 'room' AND member_agent_id = 'two'`,
	} {
		previous := read()
		if _, err := writer.Exec(query); err != nil {
			t.Fatal(err)
		}
		if read() <= previous {
			t.Fatalf("missing invalidation for %s", query)
		}
	}
	var conversationID string
	if err := writer.QueryRow(`SELECT id FROM conversations WHERE room_id = 'room'`).Scan(&conversationID); err != nil {
		t.Fatal(err)
	}
	previous := read()
	if err := writeRepo.TouchConversationActivity(ctx, conversationID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if read() != previous {
		t.Fatal("message activity caused directory reload")
	}
	if _, err := writer.Exec(`DELETE FROM rooms WHERE id = 'room'`); err != nil {
		t.Fatal(err)
	}
	if read() <= previous {
		t.Fatal("room deletion did not invalidate directory")
	}
	// 版本迁移可完整回滚并重新应用，旧数据库升级保留原有资源。
	if err := goose.Down(writer, roomrepoMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(writer, roomrepoMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if read() != 0 {
		t.Fatal("reapplied revision baseline is not zero")
	}
}
