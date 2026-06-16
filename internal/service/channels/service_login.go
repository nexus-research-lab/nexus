package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

const (
	ChannelLoginStatusRunning            = "running"
	ChannelLoginStatusVerifyCodeRequired = "verify_code_required"
	ChannelLoginStatusSucceeded          = "succeeded"
	ChannelLoginStatusError              = "error"
	ChannelLoginStatusExpired            = "expired"
	ChannelLoginStatusCancelled          = "cancelled"

	channelLoginOutputLimit = 64 * 1024
)

var (
	ErrChannelLoginNotFound    = errors.New("channel login not found")
	ErrChannelLoginUnsupported = errors.New("channel login is not supported")
)

type ChannelLoginView struct {
	LoginID        string     `json:"login_id"`
	ChannelType    string     `json:"channel_type"`
	Status         string     `json:"status"`
	Command        string     `json:"command,omitempty"`
	QRPayload      string     `json:"qr_payload,omitempty"`
	QRPayloadType  string     `json:"qr_payload_type,omitempty"`
	Output         string     `json:"output,omitempty"`
	Error          string     `json:"error,omitempty"`
	AccountID      string     `json:"account_id,omitempty"`
	UserID         string     `json:"user_id,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	VerifyCodeHint string     `json:"verify_code_hint,omitempty"`
}

type SubmitChannelLoginVerifyCodeRequest struct {
	VerifyCode string `json:"verify_code"`
}

type personalWeixinLoginClient interface {
	StartQRCode(context.Context, []string) (channeladapters.PersonalWeixinQRCodeResponse, error)
	PollQRCodeStatus(context.Context, string, string) (channeladapters.PersonalWeixinQRStatusResponse, error)
}

type channelLoginStore struct {
	mu       sync.Mutex
	active   map[string]string
	sessions map[string]*channelLoginSession
}

type channelLoginSession struct {
	mu          sync.Mutex
	ownerUserID string
	channelType string
	activeKey   string
	cancel      context.CancelFunc
	verifyCode  string
	verifyCh    chan struct{}
	client      personalWeixinLoginClient
	qrcode      string
	view        ChannelLoginView
}

func newChannelLoginStore() *channelLoginStore {
	return &channelLoginStore{
		active:   map[string]string{},
		sessions: map[string]*channelLoginSession{},
	}
}

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
	qrPayload := firstNonEmpty(qrResponse.QRCodeImageContent, qrResponse.QRCode)
	if strings.TrimSpace(qrResponse.QRCode) == "" || strings.TrimSpace(qrPayload) == "" {
		return nil, errors.New("weixin QR login did not return qrcode")
	}

	loginID := s.idFactory("channel_login")
	session := &channelLoginSession{
		ownerUserID: ownerUserID,
		channelType: channelType,
		activeKey:   activeKey,
		verifyCh:    make(chan struct{}, 1),
		client:      client,
		qrcode:      qrResponse.QRCode,
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

func (s *ControlService) runPersonalWeixinLoginSession(
	ctx context.Context,
	cancel context.CancelFunc,
	session *channelLoginSession,
	row *channelConfigRow,
) {
	defer cancel()
	defer s.finishChannelLoginSession(session)

	var status channeladapters.PersonalWeixinQRStatusResponse
	var err error
	for ctx.Err() == nil {
		status, err = session.client.PollQRCodeStatus(ctx, session.qrcode, session.takeVerifyCode())
		if err != nil {
			session.appendOutput("扫码状态刷新失败，稍后重试。\n")
			if waitChannelLoginRetry(ctx, time.Second) {
				continue
			}
			break
		}
		switch status.Status {
		case "wait", "":
			if waitChannelLoginRetry(ctx, time.Second) {
				continue
			}
		case "scaned":
			session.appendOutput("已扫码，正在等待手机确认。\n")
			if waitChannelLoginRetry(ctx, time.Second) {
				continue
			}
		case "need_verifycode":
			code, waitErr := session.waitVerifyCode(ctx)
			if waitErr != nil {
				err = waitErr
				break
			}
			session.setVerifyCode(code)
		case "verify_code_blocked":
			session.finish(ChannelLoginStatusError, "多次输入错误，请重新拉起二维码后再试")
			return
		case "expired":
			session.finish(ChannelLoginStatusExpired, "二维码已过期，请重新拉起二维码")
			return
		case "binded_redirect":
			session.finish(ChannelLoginStatusSucceeded, "")
			session.appendOutput("已连接过此微信账号，无需重复连接。\n")
			return
		case "scaned_but_redirect":
			session.appendOutput("微信服务已重定向，继续等待确认。\n")
			if waitChannelLoginRetry(ctx, time.Second) {
				continue
			}
		case "confirmed":
			if strings.TrimSpace(status.BotToken) == "" || strings.TrimSpace(status.IlinkBotID) == "" {
				session.finish(ChannelLoginStatusError, "登录失败：微信服务未返回账号凭据")
				return
			}
			if saveErr := s.savePersonalWeixinLoginCredentials(context.Background(), row, status); saveErr != nil {
				session.finish(ChannelLoginStatusError, "保存微信账号失败: "+saveErr.Error())
				return
			}
			session.setAccount(status.IlinkBotID, status.IlinkUserID)
			session.finish(ChannelLoginStatusSucceeded, "")
			session.appendOutput("微信已连接，Nexus 将自动接收和回投消息。\n")
			return
		default:
			session.finish(ChannelLoginStatusError, "未知扫码状态: "+status.Status)
			return
		}
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		session.finish(ChannelLoginStatusError, err.Error())
		return
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		session.finish(ChannelLoginStatusExpired, "微信扫码登录已超时，请重新拉起二维码")
		return
	}
	session.finish(ChannelLoginStatusCancelled, "微信扫码登录已取消")
}

func (s *ControlService) savePersonalWeixinLoginCredentials(
	ctx context.Context,
	row *channelConfigRow,
	status channeladapters.PersonalWeixinQRStatusResponse,
) error {
	if row == nil {
		return errors.New("channel config is required before login")
	}
	publicConfig, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		return err
	}
	if publicConfig == nil {
		publicConfig = map[string]string{}
	}
	secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return err
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	if err = s.saveLegacyPersonalWeixinAccount(ctx, row, publicConfig, secrets); err != nil {
		return err
	}
	nextPublicConfig := normalizeStringMap(publicConfig)
	delete(nextPublicConfig, "account_id")
	delete(nextPublicConfig, "user_id")
	nextPublicConfig["base_url"] = firstNonEmpty(status.BaseURL, nextPublicConfig["base_url"], channeladapters.DefaultPersonalWeixinBaseURL)
	configJSON, err := encodeStringMap(nextPublicConfig)
	if err != nil {
		return err
	}
	if err = s.savePersonalWeixinAccount(ctx, row, nextPublicConfig, status); err != nil {
		return err
	}
	if err = s.upsertChannelConfigRow(ctx, channelConfigRow{
		OwnerUserID:          row.OwnerUserID,
		ChannelType:          row.ChannelType,
		AgentID:              row.AgentID,
		Status:               ChannelConfigStatusConfigured,
		ConfigJSON:           configJSON,
		CredentialsEncrypted: sql.NullString{},
	}); err != nil {
		return err
	}
	runtimeStatus := ChannelConfigStatusConfigured
	runtimeError := ""
	if err = s.configureRouterChannel(ctx, row.OwnerUserID, row.ChannelType, configJSON, sql.NullString{}); err != nil {
		runtimeStatus = ChannelConfigStatusError
		runtimeError = err.Error()
	} else if s.router != nil && s.router.IsReadyForOwner(row.OwnerUserID, row.ChannelType) {
		runtimeStatus = ChannelConfigStatusConnected
	}
	return s.updateChannelConfigRuntimeState(ctx, row.OwnerUserID, row.ChannelType, runtimeStatus, runtimeError)
}

func (s *ControlService) getChannelLoginSession(ownerUserID string, channelType string, loginID string) (*channelLoginSession, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return nil, ErrChannelLoginNotFound
	}
	store := s.effectiveChannelLoginStore()
	store.mu.Lock()
	session := store.sessions[loginID]
	store.mu.Unlock()
	if session == nil || session.ownerUserID != ownerUserID || session.channelType != channelType {
		return nil, ErrChannelLoginNotFound
	}
	return session, nil
}

func (s *ControlService) finishChannelLoginSession(session *channelLoginSession) {
	store := s.effectiveChannelLoginStore()
	store.mu.Lock()
	if store.active[session.activeKey] == session.view.LoginID {
		delete(store.active, session.activeKey)
	}
	store.mu.Unlock()
}

func (s *ControlService) effectiveChannelLoginStore() *channelLoginStore {
	if s.loginStore == nil {
		s.loginStore = newChannelLoginStore()
	}
	return s.loginStore
}

func (s *ControlService) newPersonalWeixinLoginClient(baseURL string, publicConfig map[string]string) personalWeixinLoginClient {
	if s.weixinLoginClientFactory != nil {
		return s.weixinLoginClientFactory(baseURL, publicConfig)
	}
	return channeladapters.NewPersonalWeixinIlinkClient(channeladapters.PersonalWeixinClientConfig{
		BaseURL:            baseURL,
		BotAgent:           publicConfig["bot_agent"],
		IlinkAppID:         publicConfig["ilink_app_id"],
		IlinkClientVersion: publicConfig["ilink_client_version"],
	}, s.httpClient)
}

func (s *channelLoginSession) snapshot() ChannelLoginView {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.view
	if s.view.FinishedAt != nil {
		finishedAt := *s.view.FinishedAt
		view.FinishedAt = &finishedAt
	}
	return view
}

func (s *channelLoginSession) appendOutput(output string) {
	if output == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.Output = trimChannelLoginOutput(s.view.Output + output)
	s.view.UpdatedAt = time.Now()
}

func (s *channelLoginSession) finish(status string, errorMessage string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.Status = status
	s.view.Error = strings.TrimSpace(errorMessage)
	if s.view.Error != "" && !strings.Contains(s.view.Output, s.view.Error) {
		s.view.Output = trimChannelLoginOutput(s.view.Output + s.view.Error + "\n")
	}
	s.view.UpdatedAt = now
	s.view.FinishedAt = &now
}

func (s *channelLoginSession) waitVerifyCode(ctx context.Context) (string, error) {
	s.mu.Lock()
	s.view.Status = ChannelLoginStatusVerifyCodeRequired
	s.view.VerifyCodeHint = "输入手机微信显示的数字，以继续连接"
	s.view.UpdatedAt = time.Now()
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-s.verifyCh:
		return s.takeVerifyCode(), nil
	}
}

func (s *channelLoginSession) submitVerifyCode(code string) {
	s.mu.Lock()
	s.verifyCode = strings.TrimSpace(code)
	s.view.Status = ChannelLoginStatusRunning
	s.view.VerifyCodeHint = ""
	s.view.UpdatedAt = time.Now()
	s.view.Output = trimChannelLoginOutput(s.view.Output + "已提交验证码，继续等待微信确认。\n")
	s.mu.Unlock()
	select {
	case s.verifyCh <- struct{}{}:
	default:
	}
}

func (s *channelLoginSession) setVerifyCode(code string) {
	s.mu.Lock()
	s.verifyCode = strings.TrimSpace(code)
	s.mu.Unlock()
}

func (s *channelLoginSession) takeVerifyCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := strings.TrimSpace(s.verifyCode)
	s.verifyCode = ""
	return code
}

func (s *channelLoginSession) setAccount(accountID string, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.AccountID = strings.TrimSpace(accountID)
	s.view.UserID = strings.TrimSpace(userID)
}

func (s *channelLoginStore) pruneLocked(now time.Time) {
	for loginID, session := range s.sessions {
		view := session.snapshot()
		if channelLoginIsActive(view.Status) {
			continue
		}
		if view.FinishedAt != nil && now.Sub(*view.FinishedAt) > 10*time.Minute {
			delete(s.sessions, loginID)
			if s.active[session.activeKey] == loginID {
				delete(s.active, session.activeKey)
			}
		}
	}
}

func waitChannelLoginRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func channelLoginActiveKey(ownerUserID string, channelType string) string {
	return strings.TrimSpace(ownerUserID) + "\x00" + normalizeIMChannelType(channelType)
}

func channelLoginIsActive(status string) bool {
	switch strings.TrimSpace(status) {
	case ChannelLoginStatusRunning, ChannelLoginStatusVerifyCodeRequired:
		return true
	default:
		return false
	}
}

func trimChannelLoginOutput(output string) string {
	if len(output) <= channelLoginOutputLimit {
		return output
	}
	return output[len(output)-channelLoginOutputLimit:]
}
