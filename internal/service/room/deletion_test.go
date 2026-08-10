// INPUT: Room 服务的 stale/current 删除版本，以及可失败的 runtime/Goal 清理器。
// OUTPUT: 提交前冲突不清理、提交后失败标记 reconcile 且所有清理阶段均被尝试。
// POS: Room 删除数据库优先与外围清理错误分类回归测试。
package room_test

import (
	"errors"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"

	_ "modernc.org/sqlite"
)

func TestDeleteRoomAtVersionSeparatesCASFailureFromCommittedCleanupFailure(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	runtimeCloser := &fakeRoomRuntimeCloser{err: errors.New("runtime close failed")}
	goalCleaner := &fakeRoomGoalCleaner{conversationErr: errors.New("goal cleanup failed")}
	roomService.SetRuntimeManager(runtimeCloser)
	roomService.SetGoalCleaner(goalCleaner)
	roomService.SetSessionArtifactDeletionCoordinator(
		&fakeRoomSessionArtifactDeletionCoordinator{},
	)

	agentValue := createTestAgent(t, agentService, t.Context(), "Room delete worker")
	roomContext, err := roomService.CreateRoom(t.Context(), protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Room deletion ordering",
	})
	if err != nil {
		t.Fatal(err)
	}
	description := "newer state"
	updated, err := roomService.UpdateRoom(
		t.Context(),
		roomContext.Room.ID,
		protocol.UpdateRoomRequest{
			Description:                  &description,
			ExpectedConfigurationVersion: &roomContext.Room.ConfigurationVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	err = roomService.DeleteRoomAtVersion(
		t.Context(),
		roomContext.Room.ID,
		roomContext.Room.ConfigurationVersion,
	)
	if !errors.Is(err, roomrepo.ErrConfigurationVersionConflict) ||
		roomsvc.RoomDeletionCommitted(err) {
		t.Fatalf("stale delete error=%v", err)
	}
	if len(runtimeCloser.keys) != 0 || len(goalCleaner.conversationCalls) != 0 {
		t.Fatalf(
			"stale delete touched cleanup runtime=%v goals=%v",
			runtimeCloser.keys,
			goalCleaner.conversationCalls,
		)
	}
	if preserved, getErr := roomService.GetRoom(t.Context(), roomContext.Room.ID); getErr != nil ||
		preserved.Room.ConfigurationVersion != updated.Room.ConfigurationVersion {
		t.Fatalf("stale delete did not preserve Room: room=%+v err=%v", preserved, getErr)
	}

	err = roomService.DeleteRoomAtVersion(
		t.Context(),
		roomContext.Room.ID,
		updated.Room.ConfigurationVersion,
	)
	if err == nil || !roomsvc.RoomDeletionCommitted(err) {
		t.Fatalf("post-commit cleanup error=%v", err)
	}
	if len(runtimeCloser.keys) == 0 || len(goalCleaner.conversationCalls) != 1 {
		t.Fatalf(
			"all cleanup stages were not attempted runtime=%v goals=%v",
			runtimeCloser.keys,
			goalCleaner.conversationCalls,
		)
	}
	if _, getErr := roomService.GetRoom(t.Context(), roomContext.Room.ID); !errors.Is(getErr, roomsvc.ErrRoomNotFound) {
		t.Fatalf("committed Room deletion was not persisted: %v", getErr)
	}
}
