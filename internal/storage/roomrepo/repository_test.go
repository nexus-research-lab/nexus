// repository_test.go 覆盖 Room 仓储方言、迁移、并发、目录修订、回复预览、删除与版本测试。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// goose.SetDialect writes an unsynchronized package-level global in goose v3,
// which data-races when t.Parallel() tests run migrations concurrently.
// Set it once per test package instead of in every test helper.
var (
	gooseDialectOnce sync.Once
	gooseDialectErr  error
)

func ensureGooseSQLiteDialect(t *testing.T) {
	t.Helper()
	gooseDialectOnce.Do(func() {
		gooseDialectErr = goose.SetDialect("sqlite3")
	})
	if gooseDialectErr != nil {
		t.Fatalf("设置 goose 方言失败: %v", gooseDialectErr)
	}
}

func roomRepositoryMigrationDir(t *testing.T, dialect string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", dialect)
}

func TestConcurrentUnversionedRoomMemberRemovalRetainsOneAgent(t *testing.T) {
	dbA, _, repositoryA, repositoryB := newConcurrentRoomRepositories(t)
	const (
		ownerID = "owner-concurrent-removal"
		roomID  = "room-concurrent-removal"
		agentA  = "agent-concurrent-a"
		agentB  = "agent-concurrent-b"
	)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentA)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentB)
	createRepositoryRoom(t, repositoryA, ownerID, roomID, agentA, agentA, agentB)

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := repositoryA.RemoveRoomMember(context.Background(), ownerID, roomID, agentA)
		results <- err
	}()
	go func() {
		<-start
		_, err := repositoryB.RemoveRoomMember(context.Background(), ownerID, roomID, agentB)
		results <- err
	}()
	close(start)

	successes := 0
	retainedInvariant := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case strings.Contains(err.Error(), "至少保留一个 agent"):
			retainedInvariant++
		default:
			t.Fatalf("并发移除返回非预期错误: %v", err)
		}
	}
	if successes != 1 || retainedInvariant != 1 {
		t.Fatalf("并发移除结果 success=%d retained=%d, want 1/1", successes, retainedInvariant)
	}

	roomValue, err := repositoryA.GetRoom(context.Background(), ownerID, roomID)
	if err != nil {
		t.Fatalf("读取并发移除后的 Room 失败: %v", err)
	}
	agentMembers := 0
	for _, member := range roomValue.Members {
		if member.MemberType == protocol.MemberTypeAgent {
			agentMembers++
		}
	}
	if agentMembers != 1 {
		t.Fatalf("并发移除后 agent 成员数 = %d, want 1", agentMembers)
	}
	assertStoredRoomVersions(t, roomValue.Room, 2, 2)
}

func TestConcurrentHostTransferAndMemberRemovalCannotPersistDanglingHost(t *testing.T) {
	dbA, _, repositoryA, repositoryB := newConcurrentRoomRepositories(t)
	const (
		ownerID = "owner-host-removal"
		roomID  = "room-host-removal"
		agentA  = "agent-host-removal-a"
		agentB  = "agent-host-removal-b"
	)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentA)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentB)
	createRepositoryRoom(t, repositoryA, ownerID, roomID, agentA, agentA, agentB)

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		nextHost := agentB
		_, err := repositoryA.UpdateRoom(
			context.Background(),
			ownerID,
			roomID,
			UpdateRoomPatch{HostAgentID: &nextHost},
		)
		results <- err
	}()
	go func() {
		<-start
		_, err := repositoryB.RemoveRoomMember(context.Background(), ownerID, roomID, agentB)
		results <- err
	}()
	close(start)

	for range 2 {
		err := <-results
		if err != nil && !strings.Contains(err.Error(), "群主必须是当前 Room") {
			t.Fatalf("host 转移/成员移除返回非预期错误: %v", err)
		}
	}
	roomValue, err := repositoryA.GetRoom(context.Background(), ownerID, roomID)
	if err != nil {
		t.Fatalf("读取并发 host/member 结果失败: %v", err)
	}
	if roomValue.Room.HostAgentID == "" {
		return
	}
	for _, member := range roomValue.Members {
		if member.MemberType == protocol.MemberTypeAgent &&
			member.MemberAgentID == roomValue.Room.HostAgentID {
			return
		}
	}
	t.Fatalf("并发 mutation 留下悬空 host: %+v members=%+v", roomValue.Room, roomValue.Members)
}

func newConcurrentRoomRepositories(
	t *testing.T,
) (*sql.DB, *sql.DB, *SQLRepository, *SQLRepository) {
	t.Helper()

	databaseURL := t.TempDir() + "/concurrent-room.db"
	dbA := openConcurrentRoomDB(t, databaseURL)
	ensureGooseSQLiteDialect(t)
	if err := goose.Up(dbA, roomrepoMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	dbB := openConcurrentRoomDB(t, databaseURL)
	return dbA, dbB, NewSQLRepository("sqlite", dbA), NewSQLRepository("sqlite", dbB)
}

func openConcurrentRoomDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开并发测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err = db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		t.Fatalf("设置 SQLite busy_timeout 失败: %v", err)
	}
	if _, err = db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		_ = db.Close()
		t.Fatalf("设置 SQLite WAL 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func seedRoomRepositoryAgent(t *testing.T, db *sql.DB, ownerID string, agentID string) {
	t.Helper()

	if _, err := db.Exec(`
INSERT INTO agents (
    id, owner_user_id, slug, name, description, definition, status,
    workspace_path, is_main, vibe_tags
) VALUES (?, ?, ?, ?, '', '', 'active', ?, 0, '[]')`,
		agentID,
		ownerID,
		agentID,
		agentID,
		t.TempDir(),
	); err != nil {
		t.Fatalf("创建并发测试 Agent %s 失败: %v", agentID, err)
	}
}

func createRepositoryRoom(
	t *testing.T,
	repository *SQLRepository,
	ownerID string,
	roomID string,
	hostAgentID string,
	agentIDs ...string,
) {
	t.Helper()

	members := []protocol.MemberRecord{{
		ID:           "member-user-" + roomID,
		RoomID:       roomID,
		MemberType:   protocol.MemberTypeUser,
		MemberUserID: ownerID,
	}}
	for index, agentID := range agentIDs {
		members = append(members, protocol.MemberRecord{
			ID:            fmt.Sprintf("member-agent-%d-%s", index, roomID),
			RoomID:        roomID,
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: agentID,
		})
	}
	if _, err := repository.CreateRoom(context.Background(), CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID:          roomID,
			OwnerUserID: ownerID,
			RoomType:    protocol.RoomTypeGroup,
			Name:        roomID,
			Description: "",
			SkillNames:  []string{},
			HostAgentID: hostAgentID,
		},
		Members: members,
		Conversation: protocol.ConversationRecord{
			ID:               "conversation-" + roomID,
			RoomID:           roomID,
			ConversationType: protocol.ConversationTypeMain,
			Title:            roomID,
		},
	}); err != nil {
		t.Fatalf("创建并发测试 Room 失败: %v", err)
	}
}

func TestDirectoryRevisionAcrossConnections(t *testing.T) {
	writer, _, writeRepo, reader := newConcurrentRoomRepositories(t)
	ctx := context.Background()
	readVersion := func() DirectoryVersion {
		t.Helper()
		version, err := reader.DirectoryVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return version
	}
	read := func() int64 { return readVersion().Directory }
	before := readVersion()
	seedRoomRepositoryAgent(t, writer, "owner", "one")
	seedRoomRepositoryAgent(t, writer, "owner", "two")
	seeded := readVersion()
	if seeded.Agent <= before.Agent || seeded.Directory <= before.Directory {
		t.Fatal("agent creation did not invalidate the agent directory")
	}
	createRepositoryRoom(t, writeRepo, "owner", "room", "one", "one", "two")
	created := read()
	if created <= seeded.Directory {
		t.Fatal("creation did not invalidate directory")
	}
	if readVersion().Agent != seeded.Agent {
		t.Fatal("room creation invalidated the agent directory")
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
	if err := goose.DownTo(writer, roomrepoMigrationDir(t), 166); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(writer, roomrepoMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if readVersion() != (DirectoryVersion{}) {
		t.Fatal("reapplied revision baseline is not zero")
	}
}

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

func TestPlanConversationDeletionRejectsFinalConversation(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
	}
	_, err := planConversationDeletion(conversations, "main")
	if err == nil || !strings.Contains(err.Error(), "至少保留") {
		t.Fatalf("删除最后一个对话应返回明确错误，实际: %v", err)
	}
}

func TestPlanConversationDeletionKeepsMissingTargetDistinct(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
		{ID: "topic", ConversationType: protocol.ConversationTypeTopic},
	}
	plan, err := planConversationDeletion(conversations, "missing")
	if err != nil {
		t.Fatalf("不存在的目标不应报错: %v", err)
	}
	if plan.targetFound {
		t.Fatalf("不存在的目标不应标记为找到: %+v", plan)
	}
}

func TestRoomConfigurationVersionsPersistAndAdvance(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "nexus.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ensureGooseSQLiteDialect(t)
	if err = goose.Up(db, roomrepoMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	repository := NewSQLRepository("sqlite", db)
	roomContext, err := repository.CreateRoom(context.Background(), CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID:          "room-version-test",
			OwnerUserID: "owner-version-test",
			RoomType:    protocol.RoomTypeGroup,
			Name:        "版本测试房间",
			Description: "初始描述",
			SkillNames:  []string{},
		},
		Conversation: protocol.ConversationRecord{
			ID:               "conversation-version-test",
			RoomID:           "room-version-test",
			ConversationType: protocol.ConversationTypeMain,
			Title:            "版本测试房间",
		},
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 1, 1)

	if _, err = db.Exec(`
INSERT INTO agents (
    id, owner_user_id, slug, name, description, definition, status,
    workspace_path, is_main, vibe_tags, business_tags
) VALUES (?, ?, ?, ?, '', '', 'active', ?, 0, '["沉稳"]', '["企业"]')`,
		"agent-version-test",
		"owner-version-test",
		"agent-version-test",
		"版本测试助手",
		t.TempDir(),
	); err != nil {
		t.Fatalf("创建 host 测试 Agent 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO members (id, room_id, member_type, member_agent_id)
VALUES (?, ?, 'agent', ?)`,
		"member-version-test",
		"room-version-test",
		"agent-version-test",
	); err != nil {
		t.Fatalf("创建 host 测试成员失败: %v", err)
	}

	description := "更新后的描述"
	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{Description: &description},
	)
	if err != nil {
		t.Fatalf("更新 room 共享配置失败: %v", err)
	}
	if len(roomContext.MemberAgents) != 1 ||
		len(roomContext.MemberAgents[0].BusinessTags) != 1 ||
		roomContext.MemberAgents[0].BusinessTags[0] != "企业" ||
		len(roomContext.MemberAgents[0].VibeTags) != 1 ||
		roomContext.MemberAgents[0].VibeTags[0] != "沉稳" {
		t.Fatalf("Room 成员投影应区分业务标签与风格标签: %+v", roomContext.MemberAgents)
	}
	assertStoredRoomVersions(t, roomContext.Room, 2, 1)

	hostAgentID := "agent-version-test"
	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{HostAgentID: &hostAgentID},
	)
	if err != nil {
		t.Fatalf("更新 room host 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 3, 2)

	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{HostAgentID: &hostAgentID},
	)
	if err != nil {
		t.Fatalf("重复写入 room host 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 4, 2)

	expectedVersion := roomContext.Room.ConfigurationVersion
	conversationContext, err := repository.CreateConversation(
		context.Background(),
		CreateConversationBundle{
			OwnerUserID: "owner-version-test",
			RoomID:      "room-version-test",
			Conversation: protocol.ConversationRecord{
				ID:               "conversation-version-topic",
				RoomID:           "room-version-test",
				ConversationType: protocol.ConversationTypeTopic,
				Title:            "版本化话题",
			},
			ExpectedConfigurationVersion: &expectedVersion,
		},
	)
	if err != nil {
		t.Fatalf("创建 conversation 失败: %v", err)
	}
	assertStoredRoomVersions(t, conversationContext.Room, 5, 2)

	if _, err = repository.UpdateConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		"过期计划不应写入",
		expectedVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale conversation update err = %v, want version conflict", err)
	}
	conversationContext, err = repository.UpdateConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		"新标题",
		5,
	)
	if err != nil {
		t.Fatalf("更新 conversation 失败: %v", err)
	}
	if conversationContext.Conversation.Title != "新标题" {
		t.Fatalf("conversation title = %q, want 新标题", conversationContext.Conversation.Title)
	}
	assertStoredRoomVersions(t, conversationContext.Room, 6, 2)

	fallbackContext, err := repository.DeleteConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		6,
	)
	if err != nil {
		t.Fatalf("删除 conversation 失败: %v", err)
	}
	if fallbackContext == nil ||
		fallbackContext.Conversation.ID != "conversation-version-test" {
		t.Fatalf("unexpected conversation fallback: %+v", fallbackContext)
	}
	assertStoredRoomVersions(t, fallbackContext.Room, 7, 2)
}

func assertStoredRoomVersions(t *testing.T, roomValue protocol.RoomRecord, configurationVersion int64, authorityEpoch int64) {
	t.Helper()

	if roomValue.ConfigurationVersion != configurationVersion || roomValue.AuthorityEpoch != authorityEpoch {
		t.Fatalf(
			"room versions = configuration:%d authority:%d, want configuration:%d authority:%d",
			roomValue.ConfigurationVersion,
			roomValue.AuthorityEpoch,
			configurationVersion,
			authorityEpoch,
		)
	}
}

func roomrepoMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
