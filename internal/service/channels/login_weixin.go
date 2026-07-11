package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func (s *ControlService) runPersonalWeixinLoginSession(
	ctx context.Context,
	cancel context.CancelFunc,
	session *channelLoginSession,
	row *channelConfigRow,
) {
	defer cancel()
	defer s.finishChannelLoginSession(session)
	flow := personalWeixinLoginFlow{service: s, ctx: ctx, session: session, row: row}
	flow.run()
}

type personalWeixinLoginFlow struct {
	service *ControlService
	ctx     context.Context
	session *channelLoginSession
	row     *channelConfigRow
}

type personalWeixinLoginStep struct {
	finished bool
	retryIn  time.Duration
	err      error
}

type personalWeixinLoginStatusHandler func(
	*personalWeixinLoginFlow,
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep

var personalWeixinLoginStatusHandlers = map[string]personalWeixinLoginStatusHandler{
	"":                    (*personalWeixinLoginFlow).waitForConfirmation,
	"wait":                (*personalWeixinLoginFlow).waitForConfirmation,
	"scaned":              (*personalWeixinLoginFlow).handleScanned,
	"need_verifycode":     (*personalWeixinLoginFlow).handleVerifyCode,
	"verify_code_blocked": (*personalWeixinLoginFlow).handleVerifyCodeBlocked,
	"expired":             (*personalWeixinLoginFlow).handleExpired,
	"binded_redirect":     (*personalWeixinLoginFlow).handleBoundRedirect,
	"scaned_but_redirect": (*personalWeixinLoginFlow).handleScannedRedirect,
	"confirmed":           (*personalWeixinLoginFlow).handleConfirmed,
}

func (f *personalWeixinLoginFlow) run() {
	var terminalErr error
	for f.ctx.Err() == nil {
		step := f.poll()
		if step.finished {
			return
		}
		if step.err != nil {
			terminalErr = step.err
			break
		}
		if step.retryIn > 0 && !waitChannelLoginRetry(f.ctx, step.retryIn) {
			break
		}
	}
	f.finishStopped(terminalErr)
}

func (f *personalWeixinLoginFlow) poll() personalWeixinLoginStep {
	status, err := f.session.client.PollQRCodeStatus(f.ctx, f.session.qrcode, f.session.takeVerifyCode())
	if err != nil {
		f.session.appendOutput("扫码状态刷新失败，稍后重试。\n")
		return personalWeixinLoginStep{retryIn: time.Second}
	}
	handler := personalWeixinLoginStatusHandlers[strings.TrimSpace(status.Status)]
	if handler == nil {
		f.session.finish(ChannelLoginStatusError, "未知扫码状态: "+status.Status)
		return personalWeixinLoginStep{finished: true}
	}
	return handler(f, status)
}

func (f *personalWeixinLoginFlow) waitForConfirmation(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	return personalWeixinLoginStep{retryIn: time.Second}
}

func (f *personalWeixinLoginFlow) handleScanned(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	f.session.appendOutput("已扫码，正在等待手机确认。\n")
	return personalWeixinLoginStep{retryIn: time.Second}
}

func (f *personalWeixinLoginFlow) handleVerifyCode(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	code, err := f.session.waitVerifyCode(f.ctx)
	if err != nil {
		return personalWeixinLoginStep{err: err}
	}
	f.session.setVerifyCode(code)
	return personalWeixinLoginStep{}
}

func (f *personalWeixinLoginFlow) handleVerifyCodeBlocked(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	f.session.finish(ChannelLoginStatusError, "多次输入错误，请重新拉起二维码后再试")
	return personalWeixinLoginStep{finished: true}
}

func (f *personalWeixinLoginFlow) handleExpired(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	f.session.finish(ChannelLoginStatusExpired, "二维码已过期，请重新拉起二维码")
	return personalWeixinLoginStep{finished: true}
}

func (f *personalWeixinLoginFlow) handleBoundRedirect(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	f.session.finish(ChannelLoginStatusSucceeded, "")
	f.session.appendOutput("已连接过此微信账号，无需重复连接。\n")
	return personalWeixinLoginStep{finished: true}
}

func (f *personalWeixinLoginFlow) handleScannedRedirect(
	channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	f.session.appendOutput("微信服务已重定向，继续等待确认。\n")
	return personalWeixinLoginStep{retryIn: time.Second}
}

func (f *personalWeixinLoginFlow) handleConfirmed(
	status channeladapters.PersonalWeixinQRStatusResponse,
) personalWeixinLoginStep {
	if strings.TrimSpace(status.BotToken) == "" || strings.TrimSpace(status.IlinkBotID) == "" {
		f.session.finish(ChannelLoginStatusError, "登录失败：微信服务未返回账号凭据")
		return personalWeixinLoginStep{finished: true}
	}
	if err := f.service.savePersonalWeixinLoginCredentials(context.Background(), f.row, status); err != nil {
		f.session.finish(ChannelLoginStatusError, "保存微信账号失败: "+err.Error())
		return personalWeixinLoginStep{finished: true}
	}
	f.session.setAccount(status.IlinkBotID, status.IlinkUserID)
	f.session.finish(ChannelLoginStatusSucceeded, "")
	f.session.appendOutput("微信已连接，Nexus 将自动接收和回投消息。\n")
	return personalWeixinLoginStep{finished: true}
}

func (f *personalWeixinLoginFlow) finishStopped(err error) {
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		f.session.finish(ChannelLoginStatusError, err.Error())
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(f.ctx.Err(), context.DeadlineExceeded) {
		f.session.finish(ChannelLoginStatusExpired, "微信扫码登录已超时，请重新拉起二维码")
		return
	}
	f.session.finish(ChannelLoginStatusCancelled, "微信扫码登录已取消")
}

func (s *ControlService) savePersonalWeixinLoginCredentials(
	ctx context.Context,
	row *channelConfigRow,
	status channeladapters.PersonalWeixinQRStatusResponse,
) error {
	if row == nil {
		return errors.New("channel config is required before login")
	}
	config, err := s.preparePersonalWeixinLoginStorage(ctx, row, status)
	if err != nil {
		return err
	}
	if err = s.persistPersonalWeixinLoginStorage(ctx, row, status, config); err != nil {
		return err
	}
	return s.refreshPersonalWeixinLoginRouter(ctx, row, config.configJSON)
}

type personalWeixinLoginStorage struct {
	publicConfig map[string]string
	configJSON   string
}

func (s *ControlService) preparePersonalWeixinLoginStorage(
	ctx context.Context,
	row *channelConfigRow,
	status channeladapters.PersonalWeixinQRStatusResponse,
) (personalWeixinLoginStorage, error) {
	publicConfig, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		return personalWeixinLoginStorage{}, err
	}
	if publicConfig == nil {
		publicConfig = map[string]string{}
	}
	secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return personalWeixinLoginStorage{}, err
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	if err = s.saveLegacyPersonalWeixinAccount(ctx, row, publicConfig, secrets); err != nil {
		return personalWeixinLoginStorage{}, err
	}
	nextPublicConfig := normalizeStringMap(publicConfig)
	delete(nextPublicConfig, "account_id")
	delete(nextPublicConfig, "user_id")
	nextPublicConfig["base_url"] = firstNonEmpty(status.BaseURL, nextPublicConfig["base_url"], channeladapters.DefaultPersonalWeixinBaseURL)
	configJSON, err := encodeStringMap(nextPublicConfig)
	if err != nil {
		return personalWeixinLoginStorage{}, err
	}
	return personalWeixinLoginStorage{
		publicConfig: nextPublicConfig,
		configJSON:   configJSON,
	}, nil
}

func (s *ControlService) persistPersonalWeixinLoginStorage(
	ctx context.Context,
	row *channelConfigRow,
	status channeladapters.PersonalWeixinQRStatusResponse,
	config personalWeixinLoginStorage,
) error {
	if err := s.savePersonalWeixinAccount(ctx, row, config.publicConfig, status); err != nil {
		return err
	}
	return s.upsertChannelConfigRow(ctx, channelConfigRow{
		OwnerUserID:          row.OwnerUserID,
		ChannelType:          row.ChannelType,
		AgentID:              row.AgentID,
		Status:               ChannelConfigStatusConfigured,
		ConfigJSON:           config.configJSON,
		CredentialsEncrypted: sql.NullString{},
	})
}

func (s *ControlService) refreshPersonalWeixinLoginRouter(
	ctx context.Context,
	row *channelConfigRow,
	configJSON string,
) error {
	runtimeStatus := ChannelConfigStatusConfigured
	runtimeError := ""
	if err := s.configureRouterChannel(ctx, row.OwnerUserID, row.ChannelType, configJSON, sql.NullString{}); err != nil {
		runtimeStatus = ChannelConfigStatusError
		runtimeError = err.Error()
	} else if s.router != nil && s.router.IsReadyForOwner(row.OwnerUserID, row.ChannelType) {
		runtimeStatus = ChannelConfigStatusConnected
	}
	return s.updateChannelConfigRuntimeState(ctx, row.OwnerUserID, row.ChannelType, runtimeStatus, runtimeError)
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
