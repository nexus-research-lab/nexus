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
