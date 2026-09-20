package roomrepo

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestReplyPreviewOneRowIsolationAndInvalidation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	run := func(query string) {
		t.Helper()
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	run(`PRAGMA foreign_keys = ON`)
	run(`CREATE TABLE rooms (id TEXT PRIMARY KEY, owner_user_id TEXT, room_type TEXT);
	 CREATE TABLE conversations (id TEXT PRIMARY KEY, room_id TEXT REFERENCES rooms(id) ON DELETE CASCADE);
	 INSERT INTO rooms VALUES ('dm','a','dm'),('group','a','room'),('other','b','dm');
	 INSERT INTO conversations VALUES ('one','dm'),('two','dm'),('public','group'),('foreign','other');`)
	migration, err := os.ReadFile(filepath.Join(roomRepositoryMigrationDir(t, "sqlite"), "00145_room_reply_previews.sql"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(migration), "-- +goose Down")
	run(parts[0])
	run(parts[1])
	run(parts[0])
	r := NewSQLRepository("sqlite", db)
	ctx := context.Background()
	oldTime := time.Now().Add(-time.Minute).UnixMilli()
	key := protocol.BuildAgentSessionKey("agent", "web", "dm", "two", "")
	write := func(owner, conv, key, body string, public, complete bool, timestamp int64) {
		t.Helper()
		if err := r.RecordReplyPreview(ctx, owner, conv, key, public, protocol.Message{"role": "assistant", "message_id": body, "content": body, "is_complete": complete, "timestamp": timestamp}); err != nil {
			t.Fatal(err)
		}
	}
	assertPreview := func(owner, room, want string) {
		t.Helper()
		values, err := r.ListRoomReplyPreviews(ctx, owner)
		if err != nil || values[room] != want {
			t.Fatalf("previews=%v, err=%v, want %q", values, err, want)
		}
	}
	write("a", "one", "first", "旧回复", false, true, oldTime)
	write("a", "two", key, "新回复", false, true, oldTime+2)
	write("a", "one", "first", "乱序旧回复", false, true, oldTime+1)
	write("a", "two", key, "流式未完成", false, false, oldTime+3)
	assertPreview("a", "dm", "新回复")
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM room_reply_previews`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows=%d, err=%v", count, err)
	}
	write("b", "two", key, "跨账号", false, true, oldTime+4)
	assertPreview("b", "dm", "")
	write("a", "public", "private", "成员私有内容", false, true, oldTime+5)
	assertPreview("a", "group", "")
	write("a", "public", protocol.BuildRoomSharedSessionKey("public"), "公区回复", true, true, oldTime+6)
	assertPreview("a", "group", "公区回复")
	// 编辑非当前摘要来源时保留当前正文，但仍拒绝该 Room 编辑前的延迟回复。
	firstKey := protocol.BuildAgentSessionKey("agent", "web", "dm", "one", "")
	if err := r.InvalidateReplyPreview(ctx, "a", firstKey); err != nil {
		t.Fatal(err)
	}
	write("a", "one", firstKey, "旧会话延迟回复", false, true, oldTime+10)
	assertPreview("a", "dm", "新回复")
	if err := r.InvalidateReplyPreview(ctx, "a", key); err != nil {
		t.Fatal(err)
	}
	write("a", "two", key, "编辑前延迟到达", false, true, oldTime+9)
	assertPreview("a", "dm", "")
	write("a", "two", key, "编辑后新回复", false, true, time.Now().Add(time.Second).UnixMilli())
	assertPreview("a", "dm", "编辑后新回复")
	run(`DELETE FROM conversations WHERE id = 'two'`)
	write("a", "two", key, "删除后延迟到达", false, true, time.Now().Add(2*time.Second).UnixMilli())
	assertPreview("a", "dm", "")
	run(`DELETE FROM rooms WHERE id = 'group'`)
	assertPreview("a", "group", "")
	// 尚未生成摘要的会话也需要失效栅栏，不能让旧消息成为第一条摘要。
	firstKey = protocol.BuildAgentSessionKey("agent", "web", "dm", "one", "")
	if err := r.InvalidateReplyPreview(ctx, "a", firstKey); err != nil {
		t.Fatal(err)
	}
	write("a", "one", firstKey, "首次延迟写入", false, true, oldTime)
	assertPreview("a", "dm", "")
}
