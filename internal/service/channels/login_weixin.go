// INPUT: 个人微信扫码登录状态、账号凭据和当前渠道配置。
// OUTPUT: 串行持久化的账号配置与可观测的 runtime 热重载结果。
// POS: 个人微信登录完成链路，必须与同 owner+channel 的增删改共享写锁。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	defer session.markDone()
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
	if f.session.expectedAccountID != "" &&
		f.session.expectedAccountID != strings.TrimSpace(status.IlinkBotID) {
		f.session.finish(
			ChannelLoginStatusError,
			"扫码返回账号与授权目标不匹配，未保存任何凭据",
		)
		return personalWeixinLoginStep{finished: true}
	}
	releaseCommit, err := f.service.acquireChannelLoginAuthorizationCommit(
		f.ctx,
		f.session,
	)
	if err != nil {
		f.session.finish(ChannelLoginStatusError, err.Error())
		return personalWeixinLoginStep{finished: true}
	}
	if !f.session.claimCompletion() {
		releaseCommit()
		return personalWeixinLoginStep{finished: true}
	}
	defer releaseCommit()
	committedVersion, err := f.service.savePersonalWeixinLoginCredentials(
		context.Background(),
		f.row,
		status,
		f.session.snapshot().StartControlVersion,
	)
	if err != nil {
		f.session.releaseCompletion()
		f.session.finish(ChannelLoginStatusError, "保存微信账号失败: "+err.Error())
		return personalWeixinLoginStep{finished: true}
	}
	f.session.setCommittedControlVersion(committedVersion)
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
	expectedVersion int64,
) (int64, error) {
	if row == nil {
		return 0, errors.New("channel config is required before login")
	}
	unlockControl := s.lockControlMutation(row.OwnerUserID)
	defer unlockControl()
	unlockChannel := s.lockChannelMutation(row.OwnerUserID, row.ChannelType)
	defer unlockChannel()

	reloadSnapshot, err := s.captureChannelReloadSnapshot(ctx, row.OwnerUserID, row.ChannelType)
	if err != nil {
		return 0, err
	}
	if reloadSnapshot.version != expectedVersion {
		return 0, channelControlVersionError(
			expectedVersion,
			ErrChannelControlVersionConflict,
		)
	}
	var current *channelConfigRow
	var config personalWeixinLoginStorage
	committedVersion, err := s.withChannelControlMutation(ctx, row.OwnerUserID, expectedVersion, func(tx *sql.Tx) error {
		var loadErr error
		current, loadErr = s.getChannelConfigRowFrom(ctx, tx, row.OwnerUserID, row.ChannelType)
		if loadErr != nil {
			return loadErr
		}
		if current == nil {
			return ErrChannelNotFound
		}
		config, loadErr = s.preparePersonalWeixinLoginStorage(ctx, tx, current, status)
		if loadErr != nil {
			return loadErr
		}
		return s.persistPersonalWeixinLoginStorage(ctx, tx, current, status, config)
	})
	if err != nil {
		return 0, channelControlVersionError(expectedVersion, err)
	}
	if err = s.refreshPersonalWeixinLoginRouter(ctx, current, config.configJSON); err != nil {
		if restoreErr := s.restoreChannelReloadSnapshot(ctx, reloadSnapshot, committedVersion); restoreErr != nil {
			return 0, errors.Join(
				fmt.Errorf("%w: %v", ErrChannelRuntimeReload, err),
				fmt.Errorf("恢复授权前 Channel 配置失败: %w", restoreErr),
			)
		}
		return 0, fmt.Errorf(
			"%w: 候选 runtime 启动失败，上一份可运行配置已保留: %v",
			ErrChannelRuntimeReload,
			err,
		)
	}
	return committedVersion, nil
}

type personalWeixinLoginStorage struct {
	publicConfig map[string]string
	configJSON   string
}

func (s *ControlService) preparePersonalWeixinLoginStorage(
	ctx context.Context,
	store channelStore,
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
	if err = s.saveLegacyPersonalWeixinAccount(ctx, store, row, publicConfig, secrets); err != nil {
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
	store channelStore,
	row *channelConfigRow,
	status channeladapters.PersonalWeixinQRStatusResponse,
	config personalWeixinLoginStorage,
) error {
	if err := s.savePersonalWeixinAccount(ctx, store, row, config.publicConfig, status); err != nil {
		return err
	}
	return s.upsertChannelConfigRowWith(ctx, store, channelConfigRow{
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
	return s.reloadChannelRuntime(ctx, row.OwnerUserID, row.ChannelType, configJSON, sql.NullString{})
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
