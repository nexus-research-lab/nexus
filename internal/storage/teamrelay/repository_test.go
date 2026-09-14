package teamrelay

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRepositoryProjectsSnapshotDifferenceAndCommitIdempotently(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "team-relay.db")
	migrationDB, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(migrationDB, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if err = migrationDB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`INSERT INTO owner_profiles (
owner_user_id, username, display_name, role, status, created_at, updated_at
) VALUES
('owner-1', 'alice', 'Alice', 'member', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('owner-2', 'bob', 'Bob', 'member', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}

	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()
	room := relaycontract.RoomView{
		Room: relaycontract.Room{ID: "room-1", TeamID: "team-1"},
		Conversation: relaycontract.Conversation{
			ID: "conversation-1", RoomID: "room-1", SyncStreamID: "stream-1", StreamEpoch: "epoch-1",
		},
	}
	if err = repository.ProjectRoom(ctx, "owner-1", "deployment-1", room); err != nil {
		t.Fatal(err)
	}
	first := testMessage("message-1", 1, "first")
	if err = repository.ProjectSnapshot(ctx, "owner-1", relaycontract.Snapshot{
		StreamID: "stream-1", StreamEpoch: "epoch-1", SnapshotSeq: 1, Messages: []relaycontract.Message{first},
	}); err != nil {
		t.Fatal(err)
	}
	if err = repository.ProjectSnapshot(ctx, "owner-1", relaycontract.Snapshot{
		StreamID: "stream-1", StreamEpoch: "epoch-1", SnapshotSeq: 1, Messages: []relaycontract.Message{first},
	}); err != nil {
		t.Fatalf("duplicate snapshot: %v", err)
	}
	second := testMessage("message-2", 2, "second")
	if err = repository.ProjectDifference(ctx, "owner-1", relaycontract.Difference{
		StreamID: "stream-1", StreamEpoch: "epoch-1",
		Events: []relaycontract.SyncEvent{{EventSeq: 2, Type: "message.created", Message: second}},
	}); err != nil {
		t.Fatal(err)
	}
	third := testMessage("message-3", 3, "third")
	if err = repository.ProjectCommit(ctx, "owner-1", relaycontract.MessageCommit{
		Message: third, StreamID: "stream-1", StreamEpoch: "epoch-1", EventSeq: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if err = repository.ProjectRoom(ctx, "owner-2", "deployment-1", room); err != nil {
		t.Fatal(err)
	}
	sharedSnapshot := relaycontract.Snapshot{
		StreamID: "stream-1", StreamEpoch: "epoch-1", SnapshotSeq: 3,
		Messages: []relaycontract.Message{first, second, third},
	}
	var waitGroup sync.WaitGroup
	projectionErrors := make(chan error, 2)
	for _, ownerUserID := range []string{"owner-1", "owner-2"} {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			projectionErrors <- repository.ProjectSnapshot(ctx, ownerUserID, sharedSnapshot)
		}()
	}
	waitGroup.Wait()
	close(projectionErrors)
	for projectionErr := range projectionErrors {
		if projectionErr != nil {
			t.Fatal(projectionErr)
		}
	}
	if err = repository.ProjectDifference(ctx, "owner-1", relaycontract.Difference{
		StreamID: "stream-1", StreamEpoch: "epoch-1",
		Events: []relaycontract.SyncEvent{{EventSeq: 5, Type: "message.created", Message: testMessage("message-5", 5, "gap")}},
	}); err == nil {
		t.Fatal("gap difference must fail")
	}

	var count int
	var relaySeq, nextRoomSeq int64
	if err = db.QueryRow(`SELECT COUNT(*) FROM team_relay_messages`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT cursor.relay_seq, conversation.next_room_seq
FROM team_relay_owner_cursors AS cursor
JOIN team_relay_conversations AS conversation
  ON conversation.deployment_id = cursor.deployment_id
 AND conversation.conversation_id = cursor.conversation_id
WHERE cursor.owner_user_id = 'owner-1' AND cursor.conversation_id = 'conversation-1'`).Scan(
		&relaySeq, &nextRoomSeq,
	); err != nil {
		t.Fatal(err)
	}
	if count != 3 || relaySeq != 3 || nextRoomSeq != 4 {
		t.Fatalf("projection count=%d relay_seq=%d next_room_seq=%d", count, relaySeq, nextRoomSeq)
	}
	if err = db.QueryRow(`SELECT relay_seq FROM team_relay_owner_cursors
WHERE owner_user_id = 'owner-2' AND conversation_id = 'conversation-1'`).Scan(&relaySeq); err != nil {
		t.Fatal(err)
	}
	if relaySeq != 3 {
		t.Fatalf("second owner relay_seq=%d", relaySeq)
	}
	room.Conversation.StreamEpoch = "epoch-2"
	if err = repository.ProjectRoom(ctx, "owner-1", "deployment-1", room); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM team_relay_messages`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT SUM(cursor.relay_seq), conversation.next_room_seq
FROM team_relay_owner_cursors AS cursor
JOIN team_relay_conversations AS conversation
  ON conversation.deployment_id = cursor.deployment_id
 AND conversation.conversation_id = cursor.conversation_id
WHERE cursor.conversation_id = 'conversation-1'
GROUP BY conversation.next_room_seq`).Scan(&relaySeq, &nextRoomSeq); err != nil {
		t.Fatal(err)
	}
	if count != 0 || relaySeq != 0 || nextRoomSeq != 1 {
		t.Fatalf("epoch reset count=%d relay_seq=%d next_room_seq=%d", count, relaySeq, nextRoomSeq)
	}
}

func TestMigrationDeduplicatesExistingOwnerProjection(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "team-relay-migration.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../db/migrations/sqlite", 135); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO owner_profiles (
owner_user_id, username, display_name, role, status, created_at, updated_at
) VALUES
('owner-1', 'alice', 'Alice', 'member', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('owner-2', 'bob', 'Bob', 'member', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO team_relay_conversations (
owner_user_id, deployment_id, team_id, room_id, conversation_id, stream_id,
stream_epoch, relay_seq, next_room_seq
) VALUES
('owner-1', 'deployment-1', 'team-1', 'room-1', 'conversation-1', 'stream-1', 'epoch-1', 1, 2),
('owner-2', 'deployment-1', 'team-1', 'room-1', 'conversation-1', 'stream-1', 'epoch-1', 1, 2);
INSERT INTO team_relay_messages (
owner_user_id, conversation_id, message_id, message_seq, room_seq, author_type,
author_user_id, author_username, author_display_name, client_message_id, content_json, created_at
) VALUES
('owner-1', 'conversation-1', 'message-1', 1, 1, 'user', 'user-1', 'alice', 'Alice', 'client-1', '{"version":"v1","blocks":[]}', CURRENT_TIMESTAMP),
('owner-2', 'conversation-1', 'message-1', 1, 1, 'user', 'user-1', 'alice', 'Alice', 'client-1', '{"version":"v1","blocks":[]}', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../db/migrations/sqlite", 136); err != nil {
		t.Fatal(err)
	}

	var messageCount, cursorCount int
	var roomSeq, nextRoomSeq int64
	if err = db.QueryRow(`SELECT COUNT(*), MIN(room_seq) FROM team_relay_messages`).Scan(
		&messageCount, &roomSeq,
	); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM team_relay_owner_cursors`).Scan(&cursorCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT next_room_seq FROM team_relay_conversations`).Scan(&nextRoomSeq); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 || cursorCount != 2 || roomSeq != 1 || nextRoomSeq != 2 {
		t.Fatalf("messages=%d cursors=%d room_seq=%d next_room_seq=%d",
			messageCount, cursorCount, roomSeq, nextRoomSeq)
	}
	if err = goose.DownTo(db, "../../../db/migrations/sqlite", 135); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM team_relay_messages`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 2 {
		t.Fatalf("downgraded owner messages=%d", messageCount)
	}
}

func testMessage(id string, sequence int64, text string) relaycontract.Message {
	return relaycontract.Message{
		ID: id, ConversationID: "conversation-1", MessageSeq: sequence,
		AuthorType: relaycontract.AuthorTypeUser, AuthorUserID: "user-1", AuthorUsername: "alice",
		AuthorDisplayName: "Alice", ClientMessageID: "client-" + id,
		Content: relaycontract.MessageContent{Version: relaycontract.ContentVersionV1, Blocks: []relaycontract.ContentBlock{{
			Type: relaycontract.BlockTypeMarkdown, Text: text,
		}}},
		CreatedAt: time.Date(2026, 9, 8, 12, 0, int(sequence), 0, time.UTC),
	}
}
