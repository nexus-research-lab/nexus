package channels

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestControlServiceCoordinatesAgentDeletionAndRevokesRuntime(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	seedAgentChannelImpact(t, db)

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	channel := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", channel); err != nil {
		t.Fatal(err)
	}
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)

	err := service.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(ctx context.Context) error {
			return deleteSeededAgentChannelImpact(ctx, db)
		},
	)
	if err != nil {
		t.Fatalf("Agent 删除协调失败: %v", err)
	}
	if router.GetForOwner("owner-a", ChannelTypeTelegram) != nil || channel.stops != 1 {
		t.Fatalf("Agent 删除后 runtime 未立即撤销: runtime=%T stops=%d",
			router.GetForOwner("owner-a", ChannelTypeTelegram),
			channel.stops,
		)
	}
	assertNoAgentChannelImpact(t, db)
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("Agent 级联删除应推进 Channel version: version=%d err=%v", version, err)
	}
}

func TestControlServiceAgentDeletionReportsRuntimeStopFailureAfterCommit(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	seedAgentChannelImpact(t, db)

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	channel := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     errors.New("runtime stop failed"),
	}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", channel); err != nil {
		t.Fatal(err)
	}
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)

	err := service.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(ctx context.Context) error {
			return deleteSeededAgentChannelImpact(ctx, db)
		},
	)
	if err == nil || !errors.Is(err, channel.stopErr) {
		t.Fatalf("持久删除成功但 runtime 停止失败必须返回 reconcile error: %v", err)
	}
	if router.GetForOwner("owner-a", ChannelTypeTelegram) != nil {
		t.Fatal("停止失败的 runtime 也必须从 Router 注销，避免继续接收新流量")
	}
	assertNoAgentChannelImpact(t, db)
}

func TestControlServiceSerializesAgentDeletionWithChannelMutation(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	seedAgentChannelImpact(t, db)
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)

	deleteEntered := make(chan struct{})
	deleteRelease := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- service.CoordinateAgentDeletion(
			context.Background(),
			"owner-a",
			"agent-a",
			func(ctx context.Context) error {
				close(deleteEntered)
				<-deleteRelease
				return deleteSeededAgentChannelImpact(ctx, db)
			},
		)
	}()
	<-deleteEntered

	upsertDone := make(chan error, 1)
	go func() {
		_, err := service.UpsertChannelConfig(
			context.Background(),
			"owner-a",
			ChannelTypeTelegram,
			UpsertChannelConfigRequest{
				AgentID:     "agent-b",
				Credentials: map[string]string{"bot_token": "new-token"},
			},
		)
		upsertDone <- err
	}()
	select {
	case err := <-upsertDone:
		t.Fatalf("Channel mutation 不得越过 Agent 删除协调: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(deleteRelease)
	if err := <-deleteDone; err != nil {
		t.Fatalf("Agent 删除协调失败: %v", err)
	}
	if err := <-upsertDone; err != nil {
		t.Fatalf("删除完成后的新 Channel 写入失败: %v", err)
	}
	row, err := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if err != nil || row == nil || row.AgentID != "agent-b" {
		t.Fatalf("串行化后的新配置不正确: row=%+v err=%v", row, err)
	}
}

func seedAgentChannelImpact(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO channel_control_versions (owner_user_id, version) VALUES ('owner-a', 1);
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'telegram', 'agent-a', 'configured', '{}');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id, status, config_json)
VALUES ('owner-a', 'telegram', 'account-a', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, agent_id, status, source
) VALUES ('pair-a', 'owner-a', 'telegram', 'dm', 'chat-a', 'agent-a', 'active', 'manual');`)
	if err != nil {
		t.Fatal(err)
	}
}

func deleteSeededAgentChannelImpact(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range []string{
		"UPDATE channel_control_versions SET version = version + 1 WHERE owner_user_id = 'owner-a'",
		"DELETE FROM im_pairings WHERE owner_user_id = 'owner-a' AND agent_id = 'agent-a'",
		"DELETE FROM im_channel_accounts WHERE owner_user_id = 'owner-a' AND channel_type = 'telegram'",
		"DELETE FROM im_channel_configs WHERE owner_user_id = 'owner-a' AND agent_id = 'agent-a'",
	} {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func assertNoAgentChannelImpact(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"im_channel_configs", "im_channel_accounts", "im_pairings"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s 删除后仍有 %d 条记录", table, count)
		}
	}
}
