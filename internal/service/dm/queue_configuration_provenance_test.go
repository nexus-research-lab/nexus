package dm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestDirectUserDMQueueKeepsConfigurationContextAfterAdmissionClaim(t *testing.T) {
	service, client, sources, bindings, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	ctx := trustedDMQueueContext()

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "keep the first round running",
		RoundID:                     "round-provenance-first",
		UserMessageID:               "message-provenance-first",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuePrompt(t, prompts, "keep the first round running")

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "update my safe settings",
		RoundID:                     "round-provenance-queued",
		UserMessageID:               "message-provenance-queued",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	finishDMQueueRound(client, "result-provenance-first")
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuedHumanBinding(t, bindings)
	expectDMQueuePrompt(t, prompts, "update my safe settings")

	status := waitForDMQueueAdmissionStatus(t, db, "round-provenance-queued", queueadmissionstore.StatusConsumed)
	if status != queueadmissionstore.StatusConsumed {
		t.Fatalf("queue admission status = %q, want consumed", status)
	}
	finishDMQueueRound(client, "result-provenance-queued")
}

func TestTamperedDMQueuePayloadLosesConfigurationContextAndRevokesAdmission(t *testing.T) {
	service, client, sources, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	ctx := trustedDMQueueContext()

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "keep the first round running",
		RoundID:                     "round-tamper-first",
		UserMessageID:               "message-tamper-first",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuePrompt(t, prompts, "keep the first round running")

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "original queued request",
		RoundID:                     "round-tamper-queued",
		UserMessageID:               "message-tamper-queued",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("queued items = %+v, err=%v", items, err)
	}
	tampered := items[0]
	tampered.Content = "tampered destructive request"
	if _, err = service.inputQueue.Enqueue(location, tampered); err != nil {
		t.Fatal(err)
	}

	finishDMQueueRound(client, "result-tamper-first")
	expectDMQueueSource(t, sources, "agent_queue")
	expectDMQueuePrompt(t, prompts, "tampered destructive request")

	status := waitForDMQueueAdmissionStatus(t, db, "round-tamper-queued", queueadmissionstore.StatusRevoked)
	if status != queueadmissionstore.StatusRevoked {
		t.Fatalf("tampered queue admission status = %q, want revoked", status)
	}
	finishDMQueueRound(client, "result-tamper-queued")
}

type rejectingDMQueueAdmissionStore struct {
	err error
}

func (s rejectingDMQueueAdmissionStore) Record(context.Context, queueadmissionstore.Admission) error {
	return s.err
}

func (rejectingDMQueueAdmissionStore) Claim(context.Context, queueadmissionstore.Binding) (queueadmissionstore.Claim, bool, error) {
	return queueadmissionstore.Claim{}, false, nil
}

func (rejectingDMQueueAdmissionStore) Release(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Consume(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Revoke(context.Context, queueadmissionstore.Binding) error {
	return nil
}

func TestDMQueueAdmissionFailureKeepsAndRecoversDurableUserInput(t *testing.T) {
	service, client, _, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	admissionErr := errors.New("queue admission unavailable")
	service.SetQueueAdmissionStore(rejectingDMQueueAdmissionStore{err: admissionErr})
	request := InputQueueRequest{
		SessionKey: sessionKey, ClientMessageID: "client-dm-queue-retained",
		Action: "enqueue", Content: "keep this DM input for retry",
		TrustedConfigurationContext: true,
	}
	if _, err := service.HandleInputQueue(trustedDMQueueContext(), request); !errors.Is(err, admissionErr) {
		t.Fatalf("HandleInputQueue error = %v, want %v", err, admissionErr)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 ||
		items[0].ClientMessageID != request.ClientMessageID ||
		items[0].Content != request.Content {
		t.Fatalf("durable DM input was lost after admission failure: %+v", items)
	}

	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(service.config, db))
	retry, err := service.HandleInputQueue(trustedDMQueueContext(), request)
	if err != nil {
		t.Fatalf("retry retained DM input: %v", err)
	}
	if !retry.Duplicate || retry.ItemID != items[0].ID {
		t.Fatalf("retry result = %+v, want original item %q", retry, items[0].ID)
	}
	expectDMQueuePrompt(t, prompts, request.Content)
	finishDMQueueRound(client, "result-retained-dm-queue")
}

func trustedDMQueueContext() context.Context {
	return authctx.WithState(context.Background(), authctx.State{AuthRequired: false})
}

func newDMQueueProvenanceFixture(
	t *testing.T,
) (*Service, *fakeDMClient, <-chan string, <-chan authctx.QueuedHumanPrincipalBinding, <-chan string, *sql.DB, string) {
	t.Helper()
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	prompts := make(chan string, 4)
	client.onQuery = func(_ context.Context, prompt string) {
		prompts <- prompt
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	sessionKey := protocol.BuildAgentSessionKey(
		cfg.DefaultAgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"queue-provenance",
		"",
	)
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runtimeManager.CloseSession(closeCtx, sessionKey)
	})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(cfg, db))
	sources := make(chan string, 4)
	bindings := make(chan authctx.QueuedHumanPrincipalBinding, 2)
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		sourceContextType string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		sources <- sourceContextType
		if binding, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok {
			bindings <- binding
		}
		return nil
	})
	return service, client, sources, bindings, prompts, db, sessionKey
}

func expectDMQueuedHumanBinding(
	t *testing.T,
	bindings <-chan authctx.QueuedHumanPrincipalBinding,
) {
	t.Helper()
	select {
	case binding := <-bindings:
		if binding.UserID != authctx.SystemUserID ||
			binding.AuthMethod != authctx.AuthMethodLocal ||
			binding.SessionID != "" {
			t.Fatalf("queued DM human binding = %+v", binding)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for queued DM human binding")
	}
}

func expectDMQueueSource(t *testing.T, sources <-chan string, want string) {
	t.Helper()
	select {
	case got := <-sources:
		if got != want {
			t.Fatalf("configuration source = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for configuration source %q", want)
	}
}

func expectDMQueuePrompt(t *testing.T, prompts <-chan string, content string) {
	t.Helper()
	select {
	case prompt := <-prompts:
		if !strings.Contains(prompt, content) {
			t.Fatalf("runtime prompt does not contain %q: %s", content, prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for prompt containing %q", content)
	}
}

func finishDMQueueRound(client *fakeDMClient, resultID string) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      resultID,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success", DurationMS: 1, NumTurns: 1},
	}
}

func waitForDMQueueAdmissionStatus(
	t *testing.T,
	db *sql.DB,
	itemID string,
	want string,
) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		err := db.QueryRow(
			`SELECT status FROM configuration_queue_admissions
WHERE queue_item_id = ? AND scope = 'dm'`,
			itemID,
		).Scan(&status)
		if err == nil && status == want {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	return status
}
