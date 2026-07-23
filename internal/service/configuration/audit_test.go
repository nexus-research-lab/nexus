package configuration

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

func TestBeginAuditAllowsOnlyOneExecutorForRequestID(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	service := NewService(cfg, db, nil, nil, nil, nil, nil, nil, nil)
	actor := Actor{OwnerUserID: "owner", AgentID: "nexus", IsMainAgent: true}
	request := ChangeRequest{
		RequestID: "request-concurrent-1", Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search_api_key":"must-not-leak"}`),
	}
	plan := ChangePlan{Domain: DomainPreferences, Operation: "update", CurrentRevision: "sha256:before"}

	start := make(chan struct{})
	created := make(chan bool, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, won, beginErr := service.beginAudit(t.Context(), actor, request, plan)
			created <- won
			errs <- beginErr
		}()
	}
	close(start)
	wait.Wait()
	close(created)
	close(errs)
	winners := 0
	for won := range created {
		if won {
			winners++
		}
	}
	for beginErr := range errs {
		if beginErr != nil {
			t.Fatalf("beginAudit error: %v", beginErr)
		}
	}
	if winners != 1 {
		t.Fatalf("audit executors = %d, want 1", winners)
	}
	records, err := service.ListChanges(t.Context(), actor, DomainPreferences, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != "applying" {
		t.Fatalf("audit records = %+v", records)
	}
	if string(records[0].Request) == "" || containsText(string(records[0].Request), "must-not-leak") {
		t.Fatalf("audit request leaked secret: %s", records[0].Request)
	}
}

func containsText(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}
