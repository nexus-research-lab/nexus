package inbox

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func newInboxTestRepository(t *testing.T) *Repository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inbox.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, filepath.Join("..", "..", "..", "db", "migrations", "sqlite")); err != nil {
		t.Fatal(err)
	}
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func appendTestEvent(t *testing.T, r *Repository, inboxKey string, id string, dedupe string) (Event, bool) {
	t.Helper()
	event, duplicate, err := r.Append(context.Background(), Event{
		EventID: id, OwnerUserID: "owner-1", InboxKey: inboxKey, Kind: KindUserMessage,
		DedupeKey: dedupe, Payload: Payload{Content: "content " + id},
	})
	if err != nil {
		t.Fatal(err)
	}
	return event, duplicate
}

func TestRepositoryAppendIsIdempotentAndOrdered(t *testing.T) {
	r := newInboxTestRepository(t)
	first, duplicate := appendTestEvent(t, r, "inbox-a", "e1", "c1")
	if duplicate || first.Seq != 1 || first.Status != StatusPending {
		t.Fatalf("first append = %+v duplicate=%v", first, duplicate)
	}
	again, duplicate := appendTestEvent(t, r, "inbox-a", "e1-retry", "c1")
	if !duplicate || again.EventID != "e1" {
		t.Fatalf("重复 dedupe_key 应返回原事件: %+v duplicate=%v", again, duplicate)
	}
	second, _ := appendTestEvent(t, r, "inbox-a", "e2", "c2")
	other, _ := appendTestEvent(t, r, "inbox-b", "e3", "c1")
	if second.Seq != 2 || other.Seq != 1 {
		t.Fatalf("seq 应按 inbox 单调递增: second=%d other=%d", second.Seq, other.Seq)
	}
	if _, _, err := r.Append(context.Background(), Event{
		EventID: "e4", OwnerUserID: "owner-2", InboxKey: "inbox-a", Kind: KindUserMessage, DedupeKey: "c1",
	}); err == nil {
		t.Fatal("其他 owner 复用 dedupe_key 必须被拒绝")
	}
}

func TestRepositoryTransitionsOnlyFromExpectedStatus(t *testing.T) {
	r := newInboxTestRepository(t)
	ctx := context.Background()
	appendTestEvent(t, r, "inbox-a", "e1", "c1")
	appendTestEvent(t, r, "inbox-a", "e2", "c2")

	moved, err := r.Transition(ctx, "inbox-a", []string{"e1", "e2"}, StatusPending, StatusDelivering, "round-1")
	if err != nil || !slices.Equal(moved, []string{"e1", "e2"}) {
		t.Fatalf("claim = %v, %v", moved, err)
	}
	if moved, _ = r.Transition(ctx, "inbox-a", []string{"e1"}, StatusPending, StatusWithdrawn, ""); len(moved) != 0 {
		t.Fatalf("已取走的事件不能撤回: %v", moved)
	}
	if moved, _ = r.Transition(ctx, "inbox-a", []string{"e1"}, StatusDelivering, StatusConsumed, "round-1"); !slices.Equal(moved, []string{"e1"}) {
		t.Fatalf("consume = %v", moved)
	}
	if err = r.ReleaseDelivering(ctx, "inbox-a", "round-other"); err != nil {
		t.Fatal(err)
	}
	open, _ := r.ListOpen(ctx, "inbox-a")
	if len(open) != 1 || open[0].Status != StatusDelivering {
		t.Fatalf("只释放指定 round 的 delivering: %+v", open)
	}
	if err = r.ReleaseDelivering(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	open, _ = r.ListOpen(ctx, "inbox-a")
	if len(open) != 1 || open[0].EventID != "e2" || open[0].Status != StatusPending || open[0].RoundID != "" {
		t.Fatalf("启动释放应把 delivering 退回 pending: %+v", open)
	}
	inboxes, err := r.ListPendingInboxes(ctx)
	if err != nil || len(inboxes) != 1 || inboxes[0] != (PendingInbox{OwnerUserID: "owner-1", InboxKey: "inbox-a"}) {
		t.Fatalf("pending inboxes = %+v, %v", inboxes, err)
	}
}
