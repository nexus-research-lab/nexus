package inbox

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"

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
	open, _ := r.ListOpen(ctx, "inbox-a", time.Now().UTC())
	if len(open) != 1 || open[0].Status != StatusDelivering {
		t.Fatalf("只释放指定 round 的 delivering: %+v", open)
	}
	if err = r.ReleaseDelivering(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	open, _ = r.ListOpen(ctx, "inbox-a", time.Now().UTC())
	if len(open) != 1 || open[0].EventID != "e2" || open[0].Status != StatusPending || open[0].RoundID != "" {
		t.Fatalf("启动释放应把 delivering 退回 pending: %+v", open)
	}
	inboxes, err := r.ListPendingInboxes(ctx, time.Now().UTC())
	if err != nil || len(inboxes) != 1 || inboxes[0] != (PendingInbox{OwnerUserID: "owner-1", InboxKey: "inbox-a"}) {
		t.Fatalf("pending inboxes = %+v, %v", inboxes, err)
	}
}

func appendFutureEvent(t *testing.T, r *Repository, id string, kind string, availableAt time.Time) {
	t.Helper()
	if _, _, err := r.Append(context.Background(), Event{
		EventID: id, OwnerUserID: "owner-1", InboxKey: "inbox-a", Kind: kind,
		DedupeKey: "d-" + id, AvailableAt: availableAt,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryFutureEventsStayHiddenUntilDue(t *testing.T) {
	r := newInboxTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	appendFutureEvent(t, r, "later", KindFollowUp, now.Add(time.Hour))
	appendFutureEvent(t, r, "soon", KindFollowUp, now.Add(10*time.Minute))

	if open, _ := r.ListOpen(ctx, "inbox-a", now); len(open) != 0 {
		t.Fatalf("未到点事件不能被取走: %+v", open)
	}
	if inboxes, _ := r.ListPendingInboxes(ctx, now); len(inboxes) != 0 {
		t.Fatalf("未到点 inbox 不该被唤醒: %v", inboxes)
	}
	next, err := r.NextAvailableAt(ctx, now)
	if err != nil || next == nil || !next.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("next = %v, %v", next, err)
	}
	pending, _ := r.ListPendingKind(ctx, "inbox-a", KindFollowUp)
	if len(pending) != 2 || pending[0].EventID != "soon" {
		t.Fatalf("pending 应按到点时间排序: %+v", pending)
	}
	open, _ := r.ListOpen(ctx, "inbox-a", now.Add(30*time.Minute))
	if len(open) != 1 || open[0].EventID != "soon" {
		t.Fatalf("到点后只取已到点事件: %+v", open)
	}
}

func TestRepositoryCancelKindAndDefer(t *testing.T) {
	r := newInboxTestRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	appendFutureEvent(t, r, "echo-1", KindEcho, now.Add(-time.Minute))
	appendTestEvent(t, r, "inbox-a", "user-1", "u1")

	if moved, _ := r.Transition(ctx, "inbox-a", []string{"echo-1"}, StatusPending, StatusDelivering, ""); len(moved) != 1 {
		t.Fatal("claim echo failed")
	}
	deferred, err := r.Defer(ctx, "inbox-a", "echo-1", now.Add(time.Hour))
	if err != nil || !deferred {
		t.Fatalf("defer = %v, %v", deferred, err)
	}
	if open, _ := r.ListOpen(ctx, "inbox-a", now); len(open) != 1 || open[0].EventID != "user-1" {
		t.Fatalf("顺延后的 echo 不应可见: %+v", open)
	}
	cancelled, err := r.CancelKind(ctx, "inbox-a", KindEcho)
	if err != nil || !slices.Equal(cancelled, []string{"echo-1"}) {
		t.Fatalf("cancel = %v, %v", cancelled, err)
	}
	if pending, _ := r.ListPendingKind(ctx, "inbox-a", KindEcho); len(pending) != 0 {
		t.Fatalf("取消后不应剩 echo: %+v", pending)
	}
	if next, _ := r.NextAvailableAt(ctx, now); next != nil {
		t.Fatalf("没有未来事件时 next 应为空: %v", next)
	}
}

func TestRepositoryGateCountsEveryStopAndResume(t *testing.T) {
	r := newInboxTestRepository(t)
	ctx := context.Background()
	if gate, err := r.Gate(ctx, "inbox-a"); err != nil || gate != (Gate{}) {
		t.Fatalf("从未停止的闸门 = %+v %v", gate, err)
	}
	gate, err := r.SetPaused(ctx, "owner-1", "inbox-a", true)
	if err != nil || !gate.Paused || gate.Revision != 1 {
		t.Fatalf("停止后 = %+v %v", gate, err)
	}
	if gate, err = r.SetPaused(ctx, "owner-1", "inbox-a", false); err != nil || gate.Paused || gate.Revision != 2 {
		t.Fatalf("恢复后 = %+v %v", gate, err)
	}
	if _, err = r.SetPaused(ctx, "owner-2", "inbox-a", true); err == nil {
		t.Fatal("其他 owner 不能改写闸门")
	}
	if gate, _ = r.Gate(ctx, "inbox-a"); gate.Paused || gate.Revision != 2 {
		t.Fatalf("越权写入不应生效: %+v", gate)
	}
}

func TestRepositoryFindByDedupeAndAppendWithStatus(t *testing.T) {
	r := newInboxTestRepository(t)
	ctx := context.Background()
	if _, found, err := r.FindByDedupe(ctx, "inbox-a", "missing"); err != nil || found {
		t.Fatalf("未知键 found=%v err=%v", found, err)
	}
	alias, _, err := r.Append(ctx, Event{
		EventID: "alias", OwnerUserID: "owner-1", InboxKey: "inbox-a", Kind: KindHostRecovery,
		DedupeKey: "k1", Status: StatusConsumed, Payload: Payload{Content: "merged"},
	})
	if err != nil || alias.Status != StatusConsumed {
		t.Fatalf("带状态写入 = %+v %v", alias, err)
	}
	found, ok, err := r.FindByDedupe(ctx, "inbox-a", "k1")
	if err != nil || !ok || found.EventID != "alias" || found.Status != StatusConsumed {
		t.Fatalf("按键查找 = %+v ok=%v err=%v", found, ok, err)
	}
	if open, _ := r.ListOpen(ctx, "inbox-a", time.Now().UTC()); len(open) != 0 {
		t.Fatalf("已收口的记录行不应出现在待处理中: %+v", open)
	}
}
