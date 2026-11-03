// INPUT: Synthetic user data in an immutable v0.2.5 SQLite schema snapshot.
// OUTPUT: Production startup migrations preserve Room/Goal data and can safely run again.
// POS: Previous-release upgrade regression; never opens the installed app's state.
package main

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/goal"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

func TestPreviousReleaseDatabaseUpgrade(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_APP_ROOT", root)
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
	compressed, err := os.Open("testdata/v0.2.5.db.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader, err := gzip.NewReader(compressed)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: filepath.Join(t.TempDir(), "upgrade.db")}
	if err = os.WriteFile(cfg.DatabaseURL, data, 0600); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = runMigrations(cfg, logx.NewDiscardLogger()); err != nil {
			t.Fatalf("startup migration %d: %v", attempt, err)
		}
		db, err := storage.OpenDB(cfg)
		if err != nil {
			t.Fatal(err)
		}
		room, readErr := roomrepo.NewSQLRepository("sqlite", db).GetRoom(t.Context(), "upgrade-owner", "upgrade-room")
		if readErr != nil || room == nil || room.Room.Name != "升级前的用户房间" || room.Room.ConfigurationVersion != 1 {
			t.Fatalf("Room after upgrade: %+v, %v", room, readErr)
		}
		item, readErr := goal.NewRepository(cfg, db).GetGoal(t.Context(), "upgrade-goal")
		if readErr != nil || item == nil || item.Objective != "保留用户目标" || item.Status != protocol.GoalStatusPaused || item.Version != 3 {
			t.Fatalf("Goal after upgrade: %+v, %v", item, readErr)
		}
		var conversationTitle string
		if err = db.QueryRow("SELECT title FROM conversations WHERE id = ?", "upgrade-conversation").Scan(&conversationTitle); err != nil || conversationTitle != "保留此对话" {
			t.Fatalf("conversation after upgrade: %q, %v", conversationTitle, err)
		}
		rows, err := db.Query("PRAGMA foreign_key_check")
		if err != nil {
			t.Fatal(err)
		}
		if rows.Next() {
			t.Fatal("upgrade left broken foreign keys")
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
