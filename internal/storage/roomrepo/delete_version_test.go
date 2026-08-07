// INPUT: 已创建并被并发更新的 Room 与 stale/current configuration_version。
// OUTPUT: stale 删除不破坏聚合、current 删除提交、重复删除稳定返回不存在。
// POS: Room 删除 CAS 的仓储回归测试。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestDeleteRoomAtVersionRejectsStalePlanAndPreservesRoom(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "nexus.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, roomrepoMigrationDir(t)); err != nil {
		t.Fatal(err)
	}

	repository := NewSQLRepository("sqlite", db)
	const (
		ownerID = "owner-room-delete-cas"
		roomID  = "room-delete-cas"
	)
	created, err := repository.CreateRoom(t.Context(), CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID: roomID, OwnerUserID: ownerID, RoomType: protocol.RoomTypeGroup,
			Name: "Room delete CAS",
		},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-delete-cas", RoomID: roomID,
			ConversationType: protocol.ConversationTypeMain,
			Title:            "Room delete CAS",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	description := "newer configuration"
	updated, err := repository.UpdateRoom(
		t.Context(),
		ownerID,
		roomID,
		UpdateRoomPatch{Description: &description},
	)
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := repository.DeleteRoomAtVersion(
		t.Context(),
		ownerID,
		roomID,
		created.Room.ConfigurationVersion,
	)
	if deleted || !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale delete deleted=%v err=%v", deleted, err)
	}
	preserved, err := repository.GetRoom(t.Context(), ownerID, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if preserved == nil ||
		preserved.Room.ConfigurationVersion != updated.Room.ConfigurationVersion ||
		preserved.Room.Description != description {
		t.Fatalf("stale delete damaged Room: %+v", preserved)
	}

	deleted, err = repository.DeleteRoomAtVersion(
		context.Background(),
		ownerID,
		roomID,
		updated.Room.ConfigurationVersion,
	)
	if err != nil || !deleted {
		t.Fatalf("current delete deleted=%v err=%v", deleted, err)
	}
	if remaining, getErr := repository.GetRoom(t.Context(), ownerID, roomID); getErr != nil || remaining != nil {
		t.Fatalf("deleted Room remains: room=%+v err=%v", remaining, getErr)
	}
	deleted, err = repository.DeleteRoomAtVersion(
		t.Context(),
		ownerID,
		roomID,
		updated.Room.ConfigurationVersion,
	)
	if err != nil || deleted {
		t.Fatalf("repeated repository delete deleted=%v err=%v", deleted, err)
	}
}
