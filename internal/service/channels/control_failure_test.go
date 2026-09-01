package channels

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestDeleteChannelConfigReportsCommittedWhenRuntimeStopFails(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "token"},
		},
	); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	stopErr := errors.New("runtime stop failed")
	router.RegisterForOwner("owner-a", &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     stopErr,
	})

	err := service.DeleteChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram)
	if !errors.Is(err, stopErr) {
		t.Fatalf("delete error = %v", err)
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationCommitted {
		t.Fatalf("delete effect = %q ok=%v", effect, ok)
	}
	row, readErr := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if readErr != nil || row != nil {
		t.Fatalf("committed delete must remain durable: row=%+v err=%v", row, readErr)
	}
}

func TestChannelControlTransactionFailureCarriesNotAppliedEvidence(t *testing.T) {
	db := newChannelTestDB(t)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := service.withChannelControlMutation(
		context.Background(),
		"owner-a",
		0,
		nil,
	)
	if err == nil {
		t.Fatal("closed database mutation unexpectedly succeeded")
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationNotApplied {
		t.Fatalf("begin failure effect = %q ok=%v", effect, ok)
	}
}
