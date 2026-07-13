package channels

import (
	"context"
	"errors"
	"strings"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func (s *ControlService) StartChannelLogin(
	ctx context.Context,
	ownerUserID string,
	channelType string,
) (*ChannelLoginView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	if _, ok := channelCatalogByType(channelType); !ok {
		return nil, ErrChannelNotFound
	}
	if channelType != ChannelTypeWeixinPersonal {
		return nil, ErrChannelLoginUnsupported
	}

	row, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("channel config is required before login")
	}
	publicConfig, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		return nil, err
	}
	secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return nil, err
	}
	baseURL := firstNonEmpty(publicConfig["base_url"], channeladapters.DefaultPersonalWeixinBaseURL)
	client := s.newPersonalWeixinLoginClient(baseURL, publicConfig)

	store := s.effectiveChannelLoginStore()
	activeKey := channelLoginActiveKey(ownerUserID, channelType)
	now := time.Now()
	store.mu.Lock()
	store.pruneLocked(now)
	if activeID := store.active[activeKey]; activeID != "" {
		if session := store.sessions[activeID]; session != nil {
			activeView := session.snapshot()
			if channelLoginIsActive(activeView.Status) {
				store.mu.Unlock()
				return &activeView, nil
			}
		}
		delete(store.active, activeKey)
	}
	store.mu.Unlock()

	localTokens, err := s.personalWeixinLocalTokens(ctx, row, secrets)
	if err != nil {
		return nil, err
	}
	qrResponse, err := client.StartQRCode(ctx, localTokens)
	if err != nil {
		return nil, err
	}
	qrcode := strings.TrimSpace(qrResponse.QRCode)
	qrPayload := firstNonEmpty(qrResponse.QRCodeImageContent, qrcode)
	if qrcode == "" || qrPayload == "" {
		return nil, errors.New("weixin QR login did not return qrcode")
	}

	loginID := s.idFactory("channel_login")
	session := &channelLoginSession{
		ownerUserID: ownerUserID,
		channelType: channelType,
		activeKey:   activeKey,
		verifyCh:    make(chan struct{}, 1),
		client:      client,
		qrcode:      qrcode,
		view: ChannelLoginView{
			LoginID:       loginID,
			ChannelType:   channelType,
			Status:        ChannelLoginStatusRunning,
			Command:       "Nexus iLink QR login",
			QRPayload:     qrPayload,
			QRPayloadType: "text",
			Output:        "用手机微信扫描二维码，以继续连接。\n",
			StartedAt:     now,
			UpdatedAt:     now,
		},
	}

	store.mu.Lock()
	store.sessions[loginID] = session
	store.active[activeKey] = loginID
	store.mu.Unlock()

	timeout := s.loginTimeout
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	session.cancel = cancel
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
		return nil, errors.New("verify_code is required")
	}
	session.submitVerifyCode(code)
	view := session.snapshot()
	return &view, nil
}
