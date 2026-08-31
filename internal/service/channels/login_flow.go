package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func (s *ControlService) StartChannelLogin(
	ctx context.Context,
	ownerUserID string,
	channelType string,
) (*ChannelLoginView, error) {
	version, err := s.GetChannelControlVersion(ctx, ownerUserID)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	view, err := s.StartChannelLoginAtVersion(ctx, ownerUserID, channelType, version)
	return view, err
}

// StartChannelLoginAtVersion binds an interactive authorization to the exact
// Channel control generation visible when the human starts scanning. Its
// eventual credential write cannot overwrite a newer configuration.
func (s *ControlService) StartChannelLoginAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedVersion int64,
) (*ChannelLoginView, error) {
	return s.startChannelLoginAtVersion(
		ctx,
		ownerUserID,
		channelType,
		"",
		"",
		expectedVersion,
	)
}

// StartChannelLoginForAccountAtVersion additionally fences a caller-supplied
// account identity. A platform response for another account is rejected before
// credentials are persisted.
func (s *ControlService) StartChannelLoginForAccountAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedAccountID string,
	expectedVersion int64,
) (*ChannelLoginView, error) {
	return s.startChannelLoginAtVersion(
		ctx,
		ownerUserID,
		channelType,
		strings.TrimSpace(expectedAccountID),
		"",
		expectedVersion,
	)
}

// StartChannelLoginForAuthorizationAtVersion prevents one conversational flow
// from adopting or cancelling another HTTP/MCP authorization.
func (s *ControlService) StartChannelLoginForAuthorizationAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedAccountID string,
	authorizationBinding string,
	expectedVersion int64,
) (*ChannelLoginView, error) {
	authorizationBinding = strings.TrimSpace(authorizationBinding)
	if authorizationBinding == "" {
		return nil, errors.New("channel login authorization binding is required")
	}
	return s.startChannelLoginAtVersion(
		ctx,
		ownerUserID,
		channelType,
		strings.TrimSpace(expectedAccountID),
		authorizationBinding,
		expectedVersion,
	)
}

func (s *ControlService) startChannelLoginAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedAccountID string,
	authorizationBinding string,
	expectedVersion int64,
) (*ChannelLoginView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return nil, err
	}
	currentVersion, err := s.GetChannelControlVersion(ctx, ownerUserID)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	if currentVersion != expectedVersion {
		return nil, channelControlVersionError(
			expectedVersion,
			ErrChannelControlVersionConflict,
		)
	}
	catalog, ok := channelCatalogByType(channelType)
	if !ok {
		return nil, ErrChannelNotFound
	}
	if !catalog.SupportsQRCode {
		return nil, ErrChannelLoginUnsupported
	}

	row, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	if row == nil {
		return nil, classifyChannelControlError(
			ErrChannelConfigRequired,
			errors.New("channel config is required before login"),
		)
	}
	store := s.effectiveChannelLoginStore()
	activeKey := channelLoginActiveKey(ownerUserID, channelType)
	now := time.Now()
	store.mu.Lock()
	store.pruneLocked(now)
	if activeID := store.active[activeKey]; activeID != "" {
		if session := store.sessions[activeID]; session != nil {
			activeView := session.snapshot()
			if channelLoginIsActive(activeView.Status) &&
				activeView.StartControlVersion == expectedVersion &&
				session.expectedAccountID == expectedAccountID &&
				session.authorizationBinding == authorizationBinding {
				store.mu.Unlock()
				return &activeView, nil
			}
			if channelLoginIsActive(activeView.Status) {
				if session.authorizationBinding != "" || authorizationBinding != "" {
					store.mu.Unlock()
					return nil, fmt.Errorf(
						"%w: another bound Channel authorization is active",
						ErrChannelLoginState,
					)
				}
				_, _ = session.cancelLogin()
			}
		}
		delete(store.active, activeKey)
	}
	store.mu.Unlock()

	if channelType != ChannelTypeWeixinPersonal {
		return s.startRegisteredChannelLogin(
			ctx,
			row,
			activeKey,
			now,
			expectedAccountID,
			authorizationBinding,
			expectedVersion,
		)
	}
	publicConfig, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	baseURL := firstNonEmpty(publicConfig["base_url"], channeladapters.DefaultPersonalWeixinBaseURL)
	client := s.newPersonalWeixinLoginClient(baseURL, publicConfig)
	localTokens, err := s.personalWeixinLocalTokens(ctx, row, secrets)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	qrResponse, err := client.StartQRCode(ctx, localTokens)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationUnknown, err)
	}
	qrcode := strings.TrimSpace(qrResponse.QRCode)
	qrPayload := firstNonEmpty(qrResponse.QRCodeImageContent, qrcode)
	if qrcode == "" || qrPayload == "" {
		return nil, channelControlMutationFailure(
			ControlMutationUnknown,
			errors.New("weixin QR login did not return qrcode"),
		)
	}

	timeout := s.loginTimeout
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	expiresAt := now.Add(timeout)
	loginID := s.idFactory("channel_login")
	session := &channelLoginSession{
		ownerUserID:          ownerUserID,
		channelType:          channelType,
		activeKey:            activeKey,
		expectedAccountID:    expectedAccountID,
		authorizationBinding: authorizationBinding,
		verifyCh:             make(chan struct{}, 1),
		done:                 make(chan struct{}),
		client:               client,
		qrcode:               qrcode,
		view: ChannelLoginView{
			LoginID:             loginID,
			ChannelType:         channelType,
			Status:              ChannelLoginStatusRunning,
			Command:             "Nexus iLink QR login",
			QRPayload:           qrPayload,
			QRPayloadType:       "text",
			Output:              "用手机微信扫描二维码，以继续连接。\n",
			StartControlVersion: expectedVersion,
			StartedAt:           now,
			UpdatedAt:           now,
			ExpiresAt:           expiresAt,
		},
	}

	store.mu.Lock()
	store.sessions[loginID] = session
	store.active[activeKey] = loginID
	store.mu.Unlock()

	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	session.setCancel(cancel)
	view := session.snapshot()
	go s.runPersonalWeixinLoginSession(runCtx, cancel, session, row)
	return &view, nil
}

func (s *ControlService) GetChannelLogin(
	_ context.Context,
	ownerUserID string,
	channelType string,
	loginID string,
) (*ChannelLoginView, error) {
	session, err := s.getChannelLoginSession(ownerUserID, channelType, loginID)
	if err != nil {
		return nil, err
	}
	view := session.snapshot()
	return &view, nil
}

func (s *ControlService) SubmitChannelLoginVerifyCode(
	_ context.Context,
	ownerUserID string,
	channelType string,
	loginID string,
	request SubmitChannelLoginVerifyCodeRequest,
) (*ChannelLoginView, error) {
	session, err := s.getChannelLoginSession(ownerUserID, channelType, loginID)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(request.VerifyCode)
	if code == "" {
		return nil, invalidChannelControl(errors.New("verify_code is required"))
	}
	if err = session.submitVerifyCode(code); err != nil {
		return nil, err
	}
	view := session.snapshot()
	return &view, nil
}

// CancelChannelLogin stops one exact owner+channel login. A completion that
// already claimed its commit fence wins; callers must then read the final state.
func (s *ControlService) CancelChannelLogin(
	_ context.Context,
	ownerUserID string,
	channelType string,
	loginID string,
) (*ChannelLoginView, error) {
	session, err := s.getChannelLoginSession(ownerUserID, channelType, loginID)
	if err != nil {
		return nil, err
	}
	view, err := session.cancelLogin()
	if err != nil {
		return &view, err
	}
	s.finishChannelLoginSession(session)
	return &view, nil
}

// CancelChannelLoginAndWait cancels one exact login and waits until its poller
// has left every credential-write and runtime-publication path.
func (s *ControlService) CancelChannelLoginAndWait(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	loginID string,
) (*ChannelLoginView, error) {
	session, err := s.getChannelLoginSession(ownerUserID, channelType, loginID)
	if err != nil {
		return nil, err
	}
	view, cancelErr := session.cancelLogin()
	if cancelErr == nil {
		s.finishChannelLoginSession(session)
	}
	waitErr := session.waitDone(ctx)
	final := session.snapshot()
	if waitErr != nil || cancelErr != nil {
		return &final, errors.Join(cancelErr, waitErr)
	}
	return &view, nil
}

func (s *ControlService) acquireChannelLoginAuthorizationCommit(
	ctx context.Context,
	session *channelLoginSession,
) (func(), error) {
	if session == nil {
		return nil, ErrChannelLoginAuthorizationCommit
	}
	request := session.authorizationCommitRequest()
	if request.AuthorizationBinding == "" {
		return func() {}, nil
	}
	if s == nil || s.authorizationCommitGuard == nil {
		return nil, ErrChannelLoginAuthorizationCommit
	}
	release, err := s.authorizationCommitGuard.AcquireChannelLoginAuthorizationCommit(
		ctx,
		request,
	)
	if err != nil || release == nil {
		return nil, ErrChannelLoginAuthorizationCommit
	}
	return release, nil
}
