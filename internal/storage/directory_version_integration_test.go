// INPUT: Real SQLite and optional isolated PostgreSQL databases with two independent connection pools.
// OUTPUT: Committed-only directory versions, Agent sub-version separation and PostgreSQL commit-time advancement.
// POS: Cross-driver coverage for the directory revision protocol used by the cross-process watcher.
package storage_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

func TestDirectoryVersionIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			writer := openIntegrationDatabase(t, cfg)
			readerDB := openIntegrationDatabase(t, cfg)
			checkDirectoryVersion(t, driver, writer, readerDB)
		})
	}
}

func checkDirectoryVersion(t *testing.T, driver string, writer, readerDB *sql.DB) {
	ctx := t.Context()
	repo := roomrepo.NewSQLRepository(driver, writer)
	reader := roomrepo.NewSQLRepository(driver, readerDB)
	read := func() roomrepo.DirectoryVersion {
		t.Helper()
		version, err := reader.DirectoryVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return version
	}
	for _, id := range []string{"directory-a", "directory-b"} {
		if _, err := repo.CreateRoom(ctx, roomrepo.CreateRoomBundle{
			Room:         protocol.RoomRecord{ID: id, OwnerUserID: "owner", RoomType: protocol.RoomTypeGroup, Name: id, SkillNames: []string{}},
			Conversation: protocol.ConversationRecord{ID: id + "-main", RoomID: id, ConversationType: protocol.ConversationTypeMain, Title: "main"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	before := read()
	if before.Directory == 0 || before.Agent != 0 {
		t.Fatalf("room creation version = %+v", before)
	}

	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE rooms SET name = 'first' WHERE id = 'directory-a'`,
		`UPDATE rooms SET name = 'second' WHERE id = 'directory-a'`,
		`UPDATE conversations SET title = 'renamed' WHERE id = 'directory-a-main'`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if driver == "postgres" {
		// 提交时才推进单行版本：并发目录事务不因未提交事务持有版本行锁而阻塞。
		blockedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := writer.ExecContext(blockedCtx, `UPDATE rooms SET name = 'concurrent' WHERE id = 'directory-b'`)
		cancel()
		if err != nil {
			t.Fatalf("concurrent directory write blocked on the revision row: %v", err)
		}
	}
	if read().Directory > before.Directory+1 {
		t.Fatal("uncommitted change became visible")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	committed := read()
	if committed.Directory <= before.Directory || committed.Agent != before.Agent {
		t.Fatalf("committed room change version = %+v, before %+v", committed, before)
	}
	if driver == "postgres" && committed.Directory != before.Directory+2 {
		// 并发写一次、批量事务一次：PostgreSQL 每个事务只推进一次。
		t.Fatalf("transactions advanced %d times", committed.Directory-before.Directory)
	}

	rollback, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rollback.ExecContext(ctx, `UPDATE rooms SET name = 'pending' WHERE id = 'directory-a'`); err != nil {
		t.Fatal(err)
	}
	if err := rollback.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.ExecContext(ctx, `UPDATE rooms SET configuration_version = configuration_version WHERE id = 'directory-a'`); err != nil {
		t.Fatal(err)
	}
	if read() != committed {
		t.Fatal("rollback or lock-only write advanced the directory version")
	}

	if _, err := writer.ExecContext(ctx, `
INSERT INTO agents (id, owner_user_id, slug, name, description, definition, status, workspace_path, is_main, vibe_tags)
VALUES ('directory-agent', 'owner', 'directory-agent', 'Agent', '', '', 'active', '/tmp/directory-agent', FALSE, '[]')`); err != nil {
		t.Fatal(err)
	}
	agentChanged := read()
	if agentChanged.Agent <= committed.Agent || agentChanged.Directory <= committed.Directory {
		t.Fatalf("agent change version = %+v, before %+v", agentChanged, committed)
	}
}
