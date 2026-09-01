// INPUT: 共享 SQLite 文件上的多连接 Room host/成员并发 mutation。
// OUTPUT: 单语句权限快照一致性与“至少保留一个 Agent”并发不变量证明。
// POS: Room-first 跨方言锁协议的 SQLite 行为回归测试。
package roomrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

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

func TestRoomAuthorizationSnapshotRemainsConsistentDuringHostTransfers(t *testing.T) {
	dbA, _, repositoryA, repositoryB := newConcurrentRoomRepositories(t)
	const (
		ownerID = "owner-authorization-snapshot"
		roomID  = "room-authorization-snapshot"
		agentA  = "agent-authorization-a"
		agentB  = "agent-authorization-b"
	)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentA)
	seedRoomRepositoryAgent(t, dbA, ownerID, agentB)
	createRepositoryRoom(t, repositoryA, ownerID, roomID, agentA, agentA, agentB)

	writerDone := make(chan error, 1)
	go func() {
		for index := 0; index < 30; index++ {
			nextHost := agentB
			if index%2 == 1 {
				nextHost = agentA
			}
			if _, err := repositoryA.UpdateRoom(
				context.Background(),
				ownerID,
				roomID,
				UpdateRoomPatch{HostAgentID: &nextHost},
			); err != nil {
				writerDone <- err
				return
			}
		}
		writerDone <- nil
	}()

	for {
		snapshot, err := repositoryB.GetRoomAuthorizationSnapshot(
			context.Background(),
			ownerID,
			roomID,
			agentA,
		)
		if err != nil {
			t.Fatalf("读取并发权限快照失败: %v", err)
		}
		assertConsistentAuthorizationSnapshot(t, snapshot, agentA, agentB)
		select {
		case err = <-writerDone:
			if err != nil {
				t.Fatalf("并发转移 host 失败: %v", err)
			}
			finalSnapshot, finalErr := repositoryB.GetRoomAuthorizationSnapshot(
				context.Background(),
				ownerID,
				roomID,
				agentA,
			)
			if finalErr != nil {
				t.Fatalf("读取最终权限快照失败: %v", finalErr)
			}
			assertConsistentAuthorizationSnapshot(t, finalSnapshot, agentA, agentB)
			return
		default:
		}
	}
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

func assertConsistentAuthorizationSnapshot(
	t *testing.T,
	snapshot *protocol.RoomAuthorizationSnapshot,
	agentA string,
	agentB string,
) {
	t.Helper()

	if snapshot == nil {
		t.Fatal("Room authorization snapshot = nil")
	}
	if !snapshot.AgentIsMember || snapshot.AgentID != agentA {
		t.Fatalf("权限快照成员事实错误: %+v", snapshot)
	}
	if snapshot.ConfigurationVersion != snapshot.AuthorityEpoch {
		t.Fatalf("权限快照出现 torn version/epoch: %+v", snapshot)
	}
	expectedHost := agentA
	if snapshot.ConfigurationVersion%2 == 0 {
		expectedHost = agentB
	}
	if snapshot.HostAgentID != expectedHost {
		t.Fatalf("权限快照出现 torn host/version: %+v, want host=%s", snapshot, expectedHost)
	}
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
