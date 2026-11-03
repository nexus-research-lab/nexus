package workgraphworkflow

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	"modernc.org/sqlite"
)

// Interleave a second connection immediately after COMMIT, before the first
// caller receives its result. This catches post-commit rereads deterministically.
type draftCommitConnector struct {
	dsn         string
	mu          sync.Mutex
	afterCommit func() error
}

func (c *draftCommitConnector) Driver() driver.Driver { return &sqlite.Driver{} }
func (c *draftCommitConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.Driver().Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &draftCommitConn{Conn: conn, connector: c}, nil
}

type draftCommitConn struct {
	driver.Conn
	connector *draftCommitConnector
}

func (c *draftCommitConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &draftCommitTx{Tx: tx, connector: c.connector}, nil
}

type draftCommitTx struct {
	driver.Tx
	connector *draftCommitConnector
}

func (tx *draftCommitTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	tx.connector.mu.Lock()
	after := tx.connector.afterCommit
	tx.connector.afterCommit = nil
	tx.connector.mu.Unlock()
	if after != nil {
		return after()
	}
	return nil
}

func TestDraftWriteReceiptDoesNotReadConcurrentRevisionAfterCommit(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "receipts.db") + "?_pragma=foreign_keys(1)"
	otherDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = otherDB.Close() })
	ensureGooseSQLiteDialect(t)
	if err = goose.Up(otherDB, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	connector := &draftCommitConnector{dsn: dsn}
	db := sql.OpenDB(connector)
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{DatabaseDriver: "sqlite"}
	repository, other := NewRepository(cfg, db), NewRepository(cfg, otherDB)
	now := time.Now().UTC()
	preview := protocol.WorkGraphWorkflowPreview{PreviewID: "preview", Title: "first"}
	_, err = repository.CreateDraft(ctx, protocol.WorkGraphWorkflowDraft{
		PreviewID: "preview", OwnerUserID: "owner", SourceSessionKey: "session", SourceExecutionID: "execution",
		HeadRevision: 1, SelectedRevision: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
		Versions: []protocol.WorkGraphWorkflowPreviewVersion{{Revision: 1, Preview: preview, CreatedAt: now}},
	})
	if err != nil {
		t.Fatal(err)
	}
	connector.afterCommit = func() error {
		next := preview
		next.Title = "third"
		_, err := other.AppendDraftVersion(ctx, "owner", "preview", 2, next, now, now.Add(time.Hour))
		return err
	}
	preview.Title = "second"
	receipt, err := repository.AppendDraftVersion(ctx, "owner", "preview", 1, preview, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.HeadRevision != 2 || receipt.SelectedRevision != 2 || receipt.Preview.Title != "second" || len(receipt.Versions) != 2 {
		t.Fatalf("append receipt included another writer: %#v", receipt)
	}
	current, err := other.GetDraftByID(ctx, "owner", "preview")
	if err != nil || current == nil || current.HeadRevision != 3 {
		t.Fatalf("concurrent append = %#v, %v", current, err)
	}
	connector.afterCommit = func() error {
		_, err := other.SelectDraftVersion(ctx, "owner", "preview", 3, 3, now)
		return err
	}
	receipt, err = repository.SelectDraftVersion(ctx, "owner", "preview", 3, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.HeadRevision != 3 || receipt.SelectedRevision != 1 || receipt.Preview.Title != "first" {
		t.Fatalf("selection receipt included another writer: %#v", receipt)
	}
	current, err = other.GetDraftByID(ctx, "owner", "preview")
	if err != nil || current == nil || current.SelectedRevision != 3 {
		t.Fatalf("concurrent selection = %#v, %v", current, err)
	}
}
