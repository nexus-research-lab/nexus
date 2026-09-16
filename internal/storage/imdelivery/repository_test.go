package imdelivery

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func repositoryForTest(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "im.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), db
}
func TestDeliverySendClaimSurvivesRepositoryRestart(t *testing.T) {
	r, db := repositoryForTest(t)
	ctx := context.Background()
	d := Delivery{ID: "delivery", OwnerUserID: "owner", Source: Source{AgentID: "agent", SessionKey: "source"}, TargetSessionKey: "im", PairingID: "pairing", Content: "草案"}
	if _, err := r.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.ClaimSend(ctx, "owner", d.ID, false); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	restarted := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	if ok, err := restarted.ClaimSend(ctx, "owner", d.ID, false); err != nil || ok {
		t.Fatalf("unknown effect was replayed: %v %v", ok, err)
	}
	loaded, err := restarted.SaveDelivery(ctx, d)
	if err != nil || loaded.State != "unknown" {
		t.Fatalf("durable intent: %+v %v", loaded, err)
	}
	d.Content = "changed"
	if _, err = restarted.SaveDelivery(ctx, d); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed retry accepted: %v", err)
	}
	if _, err = restarted.GetDelivery(ctx, "other", d.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("owner leak %v", err)
	}
	if err = r.FinishSend(ctx, "owner", d.ID, "sent", "receipt"); err != nil {
		t.Fatal(err)
	}
	if ok, err := restarted.ClaimSend(ctx, "owner", d.ID, false); err != nil || ok {
		t.Fatal("sent delivery replayed")
	}
}
func TestRepliesKeepIndependentHumanFeedbackAndImmutableIntent(t *testing.T) {
	r, db := repositoryForTest(t)
	ctx := context.Background()
	d := Delivery{ID: "d", OwnerUserID: "owner", Source: Source{AgentID: "a", SessionKey: "source"}, TargetSessionKey: "im", Content: "same"}
	if _, err := r.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	d.ID = "d2"
	if _, err := r.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	records, err := r.List(ctx, "owner", "im", "", 0, 10)
	if err != nil || len(records) != 2 {
		t.Fatalf("logical deliveries collapsed %v %v", records, err)
	}
	reply := Reply{ID: "r", OwnerUserID: "owner", DeliveryID: "d", Input: Input{ID: "human2", SessionKey: "im"}, ContentSources: []string{"human1"}, Content: "增加人力", ForwardedContent: "原话增加人力"}
	if _, err = r.SaveReply(ctx, reply); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.TransitionReply(ctx, "owner", "r", "pending", "accepted"); err != nil || !ok {
		t.Fatal(err)
	}
	restarted := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	duplicate, err := restarted.SaveReply(ctx, reply)
	if err != nil || duplicate.State != "accepted" {
		t.Fatalf("retry lost admission %v %v", duplicate, err)
	}
	changed := reply
	changed.Content = "批准"
	if _, err = r.SaveReply(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("feedback overwritten %v", err)
	}
	reply.ID = "r2"
	reply.Input.ID = "human3"
	if _, err = r.SaveReply(ctx, reply); err != nil {
		t.Fatal(err)
	}
	if _, err = r.VerifyReply(ctx, "owner", "r", "wrong", "a", "原话增加人力"); !errors.Is(err, ErrConflict) {
		t.Fatal("accepted wrong session")
	}
	for i := 0; i < 2; i++ {
		claimed, err := r.TransitionReply(ctx, "owner", "r", "accepted", "started")
		if err != nil || claimed != (i == 0) {
			t.Fatalf("claim %d %v %v", i, claimed, err)
		}
	}
}
