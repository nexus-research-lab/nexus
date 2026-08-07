package channelauthorization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"

	_ "modernc.org/sqlite"
)

const authorizationTestKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

type fakeAuthority struct{}

type fakeHumanVerifier struct{}

type revocableHumanVerifier struct {
	mu      sync.RWMutex
	revoked bool
}

func (fakeHumanVerifier) VerifyInteractiveHuman(
	_ context.Context,
	principal *authctx.Principal,
) (*authctx.Principal, error) {
	if principal != nil {
		return principal, nil
	}
	sessionID := "auth-session-a"
	return &authctx.Principal{
		UserID:     "owner-a",
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
		SessionID:  &sessionID,
	}, nil
}

func (fakeHumanVerifier) VerifyBoundInteractiveHuman(
	_ context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, error) {
	return &authctx.Principal{
		UserID:     userID,
		Role:       authctx.RoleOwner,
		AuthMethod: authMethod,
		SessionID:  &sessionID,
	}, nil
}

func (fakeHumanVerifier) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, func(), error) {
	principal, err := fakeHumanVerifier{}.VerifyBoundInteractiveHuman(
		ctx,
		userID,
		authMethod,
		sessionID,
	)
	if err != nil {
		return nil, nil, err
	}
	return principal, func() {}, nil
}

func (v *revocableHumanVerifier) VerifyInteractiveHuman(
	ctx context.Context,
	principal *authctx.Principal,
) (*authctx.Principal, error) {
	return fakeHumanVerifier{}.VerifyInteractiveHuman(ctx, principal)
}

func (v *revocableHumanVerifier) VerifyBoundInteractiveHuman(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, error) {
	v.mu.RLock()
	revoked := v.revoked
	v.mu.RUnlock()
	if revoked {
		return nil, errors.New("password session is expired, revoked, or inactive")
	}
	return fakeHumanVerifier{}.VerifyBoundInteractiveHuman(
		ctx,
		userID,
		authMethod,
		sessionID,
	)
}

func (v *revocableHumanVerifier) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, func(), error) {
	v.mu.RLock()
	if v.revoked {
		v.mu.RUnlock()
		return nil, nil, errors.New("password session is expired, revoked, or inactive")
	}
	principal, err := fakeHumanVerifier{}.VerifyBoundInteractiveHuman(
		ctx,
		userID,
		authMethod,
		sessionID,
	)
	if err != nil {
		v.mu.RUnlock()
		return nil, nil, err
	}
	var releaseOnce sync.Once
	return principal, func() {
		releaseOnce.Do(v.mu.RUnlock)
	}, nil
}

func (v *revocableHumanVerifier) revoke() {
	v.mu.Lock()
	v.revoked = true
	v.mu.Unlock()
}

func (fakeAuthority) Inspect(
	_ context.Context,
	actor configurationsvc.Actor,
	_ []string,
	_ bool,
) (*configurationsvc.Inspection, error) {
	if !actor.IsMainAgent ||
		actor.ContextKind != configurationsvc.ContextKindAgent ||
		actor.ContextID != actor.AgentID ||
		actor.SessionKey != actor.LeaseSessionKey ||
		actor.RoundID != actor.LeaseRoundID {
		return nil, errors.New("not owner-main private dm")
	}
	return &configurationsvc.Inspection{
		Authority: configurationsvc.AuthorityOwnerMain,
		Context: configurationsvc.ScopeRef{
			Kind: configurationsvc.ScopeKindOwner,
			ID:   actor.OwnerUserID,
		},
	}, nil
}

type fakeChannelControl struct {
	mu               sync.Mutex
	version          int64
	nextID           int
	logins           map[string]channelssvc.ChannelLoginView
	submittedCodes   []string
	startErr         error
	cancelled        []string
	lastAccountID    string
	lastStartVersion int64
}

func newFakeChannelControl() *fakeChannelControl {
	return &fakeChannelControl{
		version: 2,
		logins:  make(map[string]channelssvc.ChannelLoginView),
	}
}

func (f *fakeChannelControl) GetChannelControlVersion(context.Context, string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.version, nil
}

func (f *fakeChannelControl) StartChannelLoginForAuthorizationAtVersion(
	_ context.Context,
	_ string,
	channelType string,
	accountID string,
	_ string,
	version int64,
) (*channelssvc.ChannelLoginView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAccountID = accountID
	f.lastStartVersion = version
	if f.startErr != nil {
		return nil, f.startErr
	}
	f.nextID++
	loginID := fmt.Sprintf("internal-login-%d", f.nextID)
	now := time.Now().UTC()
	item := channelssvc.ChannelLoginView{
		LoginID:             loginID,
		ChannelType:         channelType,
		Status:              channelssvc.ChannelLoginStatusRunning,
		QRPayload:           "secret-device-qr-payload",
		QRPayloadType:       "text",
		StartControlVersion: version,
		StartedAt:           now,
		UpdatedAt:           now,
		ExpiresAt:           now.Add(10 * time.Minute),
	}
	f.logins[loginID] = item
	copyItem := item
	return &copyItem, nil
}

func (f *fakeChannelControl) GetChannelLogin(
	_ context.Context,
	_ string,
	_ string,
	loginID string,
) (*channelssvc.ChannelLoginView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.logins[loginID]
	if !ok {
		return nil, channelssvc.ErrChannelLoginNotFound
	}
	copyItem := item
	return &copyItem, nil
}

func (f *fakeChannelControl) CancelChannelLogin(
	_ context.Context,
	_ string,
	_ string,
	loginID string,
) (*channelssvc.ChannelLoginView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.logins[loginID]
	if !ok {
		return nil, channelssvc.ErrChannelLoginNotFound
	}
	if item.Status == channelssvc.ChannelLoginStatusCancelled ||
		item.Status == channelssvc.ChannelLoginStatusSucceeded ||
		item.Status == channelssvc.ChannelLoginStatusError ||
		item.Status == channelssvc.ChannelLoginStatusExpired {
		copyItem := item
		return &copyItem, nil
	}
	item.Status = channelssvc.ChannelLoginStatusCancelled
	f.logins[loginID] = item
	f.cancelled = append(f.cancelled, loginID)
	copyItem := item
	return &copyItem, nil
}

func (f *fakeChannelControl) CancelChannelLoginAndWait(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	loginID string,
) (*channelssvc.ChannelLoginView, error) {
	return f.CancelChannelLogin(ctx, ownerUserID, channelType, loginID)
}

func (f *fakeChannelControl) SubmitChannelLoginVerifyCode(
	_ context.Context,
	_ string,
	_ string,
	loginID string,
	request channelssvc.SubmitChannelLoginVerifyCodeRequest,
) (*channelssvc.ChannelLoginView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.logins[loginID]
	if !ok {
		return nil, channelssvc.ErrChannelLoginNotFound
	}
	if item.Status != channelssvc.ChannelLoginStatusVerifyCodeRequired {
		return nil, channelssvc.ErrChannelLoginState
	}
	f.submittedCodes = append(f.submittedCodes, request.VerifyCode)
	item.Status = channelssvc.ChannelLoginStatusRunning
	f.logins[loginID] = item
	copyItem := item
	return &copyItem, nil
}

func (f *fakeChannelControl) setStatus(status string, accountID string, committedVersion int64, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for loginID, item := range f.logins {
		item.Status = status
		item.AccountID = accountID
		item.CommittedControlVersion = committedVersion
		item.Error = message
		item.UpdatedAt = time.Now().UTC()
		f.logins[loginID] = item
	}
}

type recordingPresenter struct {
	mu    sync.Mutex
	items []HumanPresentation
	err   error
}

func (p *recordingPresenter) PresentChannelAuthorization(
	_ context.Context,
	item HumanPresentation,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = append(p.items, item)
	return p.err
}

func (p *recordingPresenter) latest(t *testing.T) HumanPresentation {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) == 0 {
		t.Fatal("missing human presentation")
	}
	return p.items[len(p.items)-1]
}

func TestServiceKeepsQRAndVerificationCodeOutOfModelAndAudit(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	presenter := &recordingPresenter{}
	service := newAuthorizationTestService(t, db, control, presenter)
	defer func() { _ = service.Close(context.Background()) }()
	actor := testAuthorizationActor("round-1")

	started, err := service.Start(context.Background(), actor, StartInput{
		ChannelType: "weixin-personal",
		AccountID:   "wx-account-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.FlowID == "" || strings.Contains(mustJSON(t, started), "secret-device") ||
		strings.Contains(mustJSON(t, started), "internal-login") {
		t.Fatalf("model start view leaked ephemeral material: %+v", started)
	}
	presentation := presenter.latest(t)
	if presentation.QRPayload != "secret-device-qr-payload" ||
		presentation.BusinessSessionKey != actor.SessionKey {
		t.Fatalf("native presentation missing trusted QR route: %+v", presentation)
	}
	assertDatabaseDoesNotContain(t, db, "secret-device-qr-payload")
	assertDatabaseDoesNotContain(t, db, "internal-login-1")

	control.setStatus(channelssvc.ChannelLoginStatusVerifyCodeRequired, "", 0, "")
	waiting, err := service.Status(context.Background(), actor, started.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Status != authorizationstore.StatusVerifyCodeRequired {
		t.Fatalf("status = %+v", waiting)
	}
	codeCard := presenter.latest(t)
	if codeCard.Kind != PresentationKindVerificationCode ||
		codeCard.QRPayload != "" {
		t.Fatalf("verification code card is unsafe: %+v", codeCard)
	}
	if strings.Contains(mustJSON(t, waiting), "246810") {
		t.Fatal("model view unexpectedly contains verification code")
	}
	submission := humanSubmission(actor, codeCard, "246810")
	if _, err = service.SubmitHumanVerificationCode(context.Background(), submission); err != nil {
		t.Fatal(err)
	}
	assertDatabaseDoesNotContain(t, db, "246810")

	control.setStatus(channelssvc.ChannelLoginStatusSucceeded, "wx-account-1", 3, "")
	completed, err := service.Status(context.Background(), actor, started.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != authorizationstore.StatusSucceeded ||
		completed.CommittedControlVersion != 3 {
		t.Fatalf("completion = %+v", completed)
	}
	assertDatabaseDoesNotContain(t, db, "secret-device-qr-payload")
	assertDatabaseDoesNotContain(t, db, "internal-login-1")
	assertDatabaseDoesNotContain(t, db, "246810")
	audit, err := service.Completion(context.Background(), actor, started.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Status != authorizationstore.StatusSucceeded ||
		audit.ResolvedAccountID != "wx-account-1" ||
		strings.Contains(mustJSON(t, audit), "secret-device") {
		t.Fatalf("unsafe completion audit: %+v", audit)
	}
}

func TestServiceCloseWaitsForCommitLeaseAndAuditsActiveFlow(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	service := newAuthorizationTestService(
		t,
		db,
		control,
		&recordingPresenter{},
	)
	actor := testAuthorizationActor("round-close")
	started, err := service.Start(
		context.Background(),
		actor,
		StartInput{ChannelType: "weixin-personal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	releaseCommit, err := service.AcquireChannelLoginAuthorizationCommit(
		context.Background(),
		channelssvc.ChannelLoginAuthorizationCommit{
			OwnerUserID:          actor.OwnerUserID,
			ChannelType:          started.ChannelType,
			LoginID:              "internal-login-1",
			AuthorizationBinding: started.Generation,
			StartControlVersion:  started.StartControlVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- service.Close(context.Background())
	}()
	select {
	case err = <-closeResult:
		t.Fatalf("Close returned before commit lease release: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	releaseCommit()
	select {
	case err = <-closeResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not drain the released commit lease")
	}

	flow, err := service.repository.Get(
		context.Background(),
		actor.OwnerUserID,
		started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != authorizationstore.StatusCancelled ||
		flow.OutcomeCode != "service_closed" {
		t.Fatalf("closed flow = %+v", flow)
	}
	audit, err := service.repository.GetAudit(
		context.Background(),
		actor.OwnerUserID,
		started.FlowID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Status != authorizationstore.StatusCancelled ||
		audit.OutcomeCode != "service_closed" ||
		len(control.cancelled) != 1 {
		t.Fatalf(
			"close audit=%+v cancelled=%+v",
			audit,
			control.cancelled,
		)
	}
	if _, err = service.Start(
		context.Background(),
		actor,
		StartInput{ChannelType: "weixin-personal"},
	); err == nil || !strings.Contains(err.Error(), "正在关闭") {
		t.Fatalf("closed service accepted new authorization: %v", err)
	}
}

func TestAuthorizationCommitLeaseExcludesCancelAndCompletionWinsAtExpiry(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	service := newAuthorizationTestService(
		t,
		db,
		control,
		&recordingPresenter{},
	)
	defer func() { _ = service.Close(context.Background()) }()
	actor := testAuthorizationActor("round-commit-cancel")
	started, err := service.Start(
		context.Background(),
		actor,
		StartInput{ChannelType: "weixin-personal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	service.stopMonitor(started.FlowID)
	service.monitorWG.Wait()

	releaseCommit, err := service.AcquireChannelLoginAuthorizationCommit(
		context.Background(),
		channelssvc.ChannelLoginAuthorizationCommit{
			OwnerUserID:          actor.OwnerUserID,
			ChannelType:          started.ChannelType,
			LoginID:              "internal-login-1",
			AuthorizationBinding: started.Generation,
			StartControlVersion:  started.StartControlVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	control.setStatus(
		channelssvc.ChannelLoginStatusSucceeded,
		"wx-account-commit",
		3,
		"",
	)
	service.now = func() time.Time {
		return started.ExpiresAt.Add(time.Second)
	}

	type cancelResult struct {
		view *View
		err  error
	}
	resultCh := make(chan cancelResult, 1)
	go func() {
		view, cancelErr := service.Cancel(
			context.Background(),
			actor,
			started.FlowID,
		)
		resultCh <- cancelResult{view: view, err: cancelErr}
	}()
	select {
	case result := <-resultCh:
		t.Fatalf(
			"Cancel crossed the active commit lease: view=%+v err=%v",
			result.view,
			result.err,
		)
	case <-time.After(30 * time.Millisecond):
	}

	releaseCommit()
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.view == nil ||
			result.view.Status != authorizationstore.StatusSucceeded ||
			result.view.ResolvedAccountID != "wx-account-commit" ||
			result.view.CommittedControlVersion != 3 {
			t.Fatalf("completion lost to cancel/expiry: %+v", result.view)
		}
	case <-time.After(time.Second):
		t.Fatal("Cancel did not resume after the commit lease was released")
	}
	if len(control.cancelled) != 0 {
		t.Fatalf("completed login was cancelled: %+v", control.cancelled)
	}
}

func TestAuthorizationCommitLeaseBlocksHumanSessionRevocation(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	verifier := &revocableHumanVerifier{}
	service := NewService(
		config.Config{
			DatabaseDriver:          "sqlite",
			ConnectorCredentialsKey: authorizationTestKey,
		},
		db,
		fakeAuthority{},
		verifier,
		control,
		&recordingPresenter{},
	)
	service.monitorInterval = time.Hour
	defer func() { _ = service.Close(context.Background()) }()
	actor := testAuthorizationActor("round-session-lease")
	started, err := service.Start(
		context.Background(),
		actor,
		StartInput{ChannelType: "weixin-personal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	releaseCommit, err := service.AcquireChannelLoginAuthorizationCommit(
		context.Background(),
		channelssvc.ChannelLoginAuthorizationCommit{
			OwnerUserID:          actor.OwnerUserID,
			ChannelType:          started.ChannelType,
			LoginID:              "internal-login-1",
			AuthorizationBinding: started.Generation,
			StartControlVersion:  started.StartControlVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			releaseCommit()
		}
	}()

	revoked := make(chan struct{})
	go func() {
		verifier.revoke()
		close(revoked)
	}()
	select {
	case <-revoked:
		t.Fatal("human session revocation crossed the active commit lease")
	case <-time.After(30 * time.Millisecond):
	}
	releaseCommit()
	released = true
	select {
	case <-revoked:
	case <-time.After(time.Second):
		t.Fatal("human session revocation did not resume after commit release")
	}

	release, err := service.AcquireChannelLoginAuthorizationCommit(
		context.Background(),
		channelssvc.ChannelLoginAuthorizationCommit{
			OwnerUserID:          actor.OwnerUserID,
			ChannelType:          started.ChannelType,
			LoginID:              "internal-login-1",
			AuthorizationBinding: started.Generation,
			StartControlVersion:  started.StartControlVersion,
		},
	)
	if release != nil {
		release()
		t.Fatal("revoked session received another authorization commit lease")
	}
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("post-revocation commit error = %v", err)
	}
}

func TestAuthorizationCommitLeaseRejectsRevokedHumanSession(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	verifier := &revocableHumanVerifier{}
	service := NewService(
		config.Config{
			DatabaseDriver:          "sqlite",
			ConnectorCredentialsKey: authorizationTestKey,
		},
		db,
		fakeAuthority{},
		verifier,
		control,
		&recordingPresenter{},
	)
	service.monitorInterval = time.Hour
	defer func() { _ = service.Close(context.Background()) }()
	actor := testAuthorizationActor("round-revoked")
	started, err := service.Start(
		context.Background(),
		actor,
		StartInput{ChannelType: "weixin-personal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	verifier.revoke()
	release, err := service.AcquireChannelLoginAuthorizationCommit(
		context.Background(),
		channelssvc.ChannelLoginAuthorizationCommit{
			OwnerUserID:          actor.OwnerUserID,
			ChannelType:          started.ChannelType,
			LoginID:              "internal-login-1",
			AuthorizationBinding: started.Generation,
			StartControlVersion:  started.StartControlVersion,
		},
	)
	if release != nil {
		release()
		t.Fatal("revoked human session received a commit lease")
	}
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked human session commit error = %v", err)
	}
}

func TestServiceRestoresFlowAcrossRoundsAndRejectsCrossScopeHumanSubmission(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	presenter := &recordingPresenter{}
	service := newAuthorizationTestService(t, db, control, presenter)
	defer func() { _ = service.Close(context.Background()) }()
	actor := testAuthorizationActor("round-a")
	started, err := service.Start(context.Background(), actor, StartInput{
		ChannelType: "weixin-personal",
	})
	if err != nil {
		t.Fatal(err)
	}

	crossRound := testAuthorizationActor("round-b")
	if _, err = service.Status(
		context.Background(),
		crossRound,
		started.FlowID,
	); err != nil {
		t.Fatalf("same principal private DM must restore flow across rounds: %v", err)
	}
	crossSession := actor
	crossSession.SessionKey = "agent:other:websocket:dm"
	crossSession.LeaseSessionKey = crossSession.SessionKey
	if _, err = service.Cancel(context.Background(), crossSession, started.FlowID); err == nil {
		t.Fatalf("cross-session cancellation must fail: %v", err)
	}

	control.setStatus(channelssvc.ChannelLoginStatusVerifyCodeRequired, "", 0, "")
	if _, err = service.Status(context.Background(), actor, started.FlowID); err != nil {
		t.Fatal(err)
	}
	card := presenter.latest(t)
	submission := humanSubmission(actor, card, "112233")
	submission.RuntimeLeaseRoundID = "different-lease"
	if _, err = service.SubmitHumanVerificationCode(context.Background(), submission); err == nil {
		t.Fatal("cross-lease human submission must fail")
	}
	if len(control.submittedCodes) != 0 {
		t.Fatalf("rejected code reached Channel control: %+v", control.submittedCodes)
	}
}

func TestServiceExpiresAndCancelsUnderlyingLogin(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	service := newAuthorizationTestService(t, db, control, &recordingPresenter{})
	defer func() { _ = service.Close(context.Background()) }()
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	service.flowTTL = time.Minute
	actor := testAuthorizationActor("round-expiry")
	started, err := service.Start(context.Background(), actor, StartInput{ChannelType: "weixin-personal"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	expired, err := service.Status(context.Background(), actor, started.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Status != authorizationstore.StatusExpired || len(control.cancelled) != 1 {
		t.Fatalf("expired flow did not cancel login: view=%+v cancelled=%+v", expired, control.cancelled)
	}
	audit, err := service.Completion(context.Background(), actor, started.FlowID)
	if err != nil || audit.OutcomeCode != "expired" {
		t.Fatalf("expiry audit = %+v err=%v", audit, err)
	}
}

func TestServiceRestartExplicitlyInvalidatesAndScrubsActiveFlow(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	actor := testAuthorizationActor("round-restart")
	first := newAuthorizationTestService(t, db, control, &recordingPresenter{})
	started, err := first.Start(context.Background(), actor, StartInput{ChannelType: "weixin-personal"})
	if err != nil {
		t.Fatal(err)
	}
	second := NewService(
		config.Config{DatabaseDriver: "sqlite", ConnectorCredentialsKey: authorizationTestKey},
		db,
		fakeAuthority{},
		fakeHumanVerifier{},
		control,
		&recordingPresenter{},
	)
	second.monitorInterval = time.Hour
	defer func() { _ = first.Close(context.Background()) }()
	defer func() { _ = second.Close(context.Background()) }()
	if err = second.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	view, err := second.Status(context.Background(), actor, started.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != authorizationstore.StatusRestartInvalidated {
		t.Fatalf("restarted flow = %+v", view)
	}
	assertFlowEphemeralColumnsScrubbed(t, db, started.FlowID)
	audit, err := second.Completion(context.Background(), actor, started.FlowID)
	if err != nil || audit.OutcomeCode != "server_restarted" {
		t.Fatalf("restart audit = %+v err=%v", audit, err)
	}
}

func TestServiceReportsVersionConflictAndReloadRollbackWithoutRawError(t *testing.T) {
	tests := []struct {
		name        string
		rawError    string
		outcomeCode string
	}{
		{
			name:        "version conflict",
			rawError:    "Channel 配置版本已变化 expected=2 token=raw-secret",
			outcomeCode: "channel_version_conflict",
		},
		{
			name:        "runtime reload",
			rawError:    "Channel 候选 runtime 启动失败，上一份可运行配置已保留 token=raw-secret",
			outcomeCode: "runtime_reload_failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := newAuthorizationTestDB(t)
			control := newFakeChannelControl()
			service := newAuthorizationTestService(t, db, control, &recordingPresenter{})
			defer func() { _ = service.Close(context.Background()) }()
			actor := testAuthorizationActor("round-failure")
			started, err := service.Start(context.Background(), actor, StartInput{ChannelType: "feishu"})
			if err != nil {
				t.Fatal(err)
			}
			control.setStatus(channelssvc.ChannelLoginStatusError, "", 0, test.rawError)
			failed, err := service.Status(context.Background(), actor, started.FlowID)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(mustJSON(t, failed), "raw-secret") {
				t.Fatalf("model failure leaked raw error: %+v", failed)
			}
			completion, err := service.Completion(context.Background(), actor, started.FlowID)
			if err != nil {
				t.Fatal(err)
			}
			if completion.OutcomeCode != test.outcomeCode ||
				strings.Contains(mustJSON(t, completion), "raw-secret") {
				t.Fatalf("completion = %+v", completion)
			}
		})
	}
}

func TestServiceBindsStartVersionAndAccount(t *testing.T) {
	db := newAuthorizationTestDB(t)
	control := newFakeChannelControl()
	service := newAuthorizationTestService(t, db, control, &recordingPresenter{})
	defer func() { _ = service.Close(context.Background()) }()
	_, err := service.Start(
		context.Background(),
		testAuthorizationActor("round-binding"),
		StartInput{ChannelType: "weixin-personal", AccountID: "exact-account"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if control.lastStartVersion != 2 || control.lastAccountID != "exact-account" {
		t.Fatalf(
			"start binding mismatch: version=%d account=%q",
			control.lastStartVersion,
			control.lastAccountID,
		)
	}
}

func newAuthorizationTestService(
	t *testing.T,
	db *sql.DB,
	control *fakeChannelControl,
	presenter *recordingPresenter,
) *Service {
	t.Helper()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", ConnectorCredentialsKey: authorizationTestKey},
		db,
		fakeAuthority{},
		fakeHumanVerifier{},
		control,
		presenter,
	)
	service.monitorInterval = time.Hour
	var mu sync.Mutex
	counter := 0
	service.idFactory = func(prefix string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		counter++
		return fmt.Sprintf("%s-%d", prefix, counter), nil
	}
	return service
}

func newAuthorizationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func testAuthorizationActor(roundID string) Actor {
	sessionKey := "agent:main-agent:websocket:dm"
	return Actor{
		OwnerUserID:        "owner-a",
		AgentID:            "main-agent",
		SessionKey:         sessionKey,
		RoundID:            roundID,
		LeaseSessionKey:    sessionKey,
		LeaseRoundID:       roundID,
		IsMainAgent:        true,
		ContextKind:        configurationsvc.ContextKindAgent,
		ContextID:          "main-agent",
		PrincipalRole:      "owner",
		AuthMethod:         authctx.AuthMethodPassword,
		AuthSessionID:      "auth-session-a",
		RoundLeaseRequired: true,
	}
}

func humanSubmission(
	actor Actor,
	presentation HumanPresentation,
	code string,
) HumanVerificationCodeSubmission {
	return HumanVerificationCodeSubmission{
		FlowID:                 presentation.FlowID,
		PresentationToken:      presentation.PresentationToken,
		OwnerUserID:            actor.OwnerUserID,
		PrincipalUserID:        actor.OwnerUserID,
		PrincipalAuthSessionID: actor.AuthSessionID,
		AgentID:                actor.AgentID,
		BusinessSessionKey:     actor.SessionKey,
		RootRoundID:            actor.RoundID,
		RuntimeLeaseSessionKey: actor.LeaseSessionKey,
		RuntimeLeaseRoundID:    actor.LeaseRoundID,
		Code:                   code,
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func assertDatabaseDoesNotContain(t *testing.T, db *sql.DB, secret string) {
	t.Helper()
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM channel_authorization_flows
WHERE COALESCE(runtime_ref_encrypted, '') LIKE ?
   OR COALESCE(human_presentation_encrypted, '') LIKE ?
   OR outcome_code LIKE ?
   OR outcome_message LIKE ?`,
		"%"+secret+"%",
		"%"+secret+"%",
		"%"+secret+"%",
		"%"+secret+"%",
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("database contains plaintext secret %q", secret)
	}
	var auditCount int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM channel_authorization_audit
WHERE outcome_code LIKE ? OR outcome_message LIKE ?`,
		"%"+secret+"%",
		"%"+secret+"%",
	).Scan(&auditCount)
	if err != nil {
		t.Fatal(err)
	}
	if auditCount != 0 {
		t.Fatalf("audit contains plaintext secret %q", secret)
	}
}

func assertFlowEphemeralColumnsScrubbed(t *testing.T, db *sql.DB, flowID string) {
	t.Helper()
	var runtimeRef sql.NullString
	var presentation sql.NullString
	if err := db.QueryRow(
		`SELECT runtime_ref_encrypted, human_presentation_encrypted
FROM channel_authorization_flows WHERE flow_id = ?`,
		flowID,
	).Scan(&runtimeRef, &presentation); err != nil {
		t.Fatal(err)
	}
	if runtimeRef.Valid || presentation.Valid {
		t.Fatalf("terminal flow retained ephemeral ciphertext: runtime=%v presentation=%v", runtimeRef, presentation)
	}
}
