// INPUT: 已保存的频道配置、官方应用注册客户端与二维码轮询结果。
// OUTPUT: 飞书/钉钉/企业微信扫码会话，以及加密落库并重载后的频道凭据。
// POS: channels 控制服务中的官方扫码编排；平台协议由 appregistration 承载。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
)

func (s *ControlService) startRegisteredChannelLogin(
	ctx context.Context,
	row *channelConfigRow,
	activeKey string,
	now time.Time,
) (*ChannelLoginView, error) {
	client := s.newChannelRegistrationClient(row.ChannelType)
	if client == nil {
		return nil, ErrChannelLoginUnsupported
	}
	started, err := client.Start(ctx)
	if err != nil {
		return nil, err
	}
	qrPayload := firstNonEmpty(started.VerificationURIComplete, started.VerificationURI)
	if strings.TrimSpace(started.DeviceCode) == "" || qrPayload == "" {
		return nil, errors.New("扫码注册未返回完整的二维码信息")
	}
	loginID := s.idFactory("channel_login")
	session := &channelLoginSession{
		ownerUserID:        row.OwnerUserID,
		channelType:        row.ChannelType,
		activeKey:          activeKey,
		verifyCh:           make(chan struct{}, 1),
		registrationClient: client,
		deviceCode:         started.DeviceCode,
		pollInterval:       time.Duration(started.Interval) * time.Second,
		view: ChannelLoginView{
			LoginID:       loginID,
			ChannelType:   row.ChannelType,
			Status:        ChannelLoginStatusRunning,
			Command:       "Nexus official QR registration",
			QRPayload:     qrPayload,
			QRPayloadType: "text",
			Output:        channelRegistrationPrompt(row.ChannelType),
			StartedAt:     now,
			UpdatedAt:     now,
		},
	}
	if s.registrationPollInterval > 0 {
		session.pollInterval = s.registrationPollInterval
	}
	store := s.effectiveChannelLoginStore()
	store.mu.Lock()
	store.sessions[loginID] = session
	store.active[activeKey] = loginID
	store.mu.Unlock()

	timeout := s.loginTimeout
	if started.ExpiresIn > 0 {
		timeout = time.Duration(started.ExpiresIn) * time.Second
	}
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	session.cancel = cancel
	view := session.snapshot()
	go s.runRegisteredChannelLogin(runCtx, cancel, session, row)
	return &view, nil
}

func (s *ControlService) runRegisteredChannelLogin(
	ctx context.Context,
	cancel context.CancelFunc,
	session *channelLoginSession,
	row *channelConfigRow,
) {
	defer cancel()
	defer s.finishChannelLoginSession(session)
	interval := session.pollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for ctx.Err() == nil {
		if !waitChannelLoginRetry(ctx, interval) {
			break
		}
		result, err := session.registrationClient.Poll(ctx, session.deviceCode)
		if err != nil {
			session.appendOutput("扫码状态刷新失败，稍后重试。\n")
			continue
		}
		switch result.Status {
		case appregistration.StatusPending:
			continue
		case appregistration.StatusSlowDown:
			interval += 5 * time.Second
			continue
		case appregistration.StatusExpired:
			session.finish(ChannelLoginStatusExpired, firstNonEmpty(result.Message, "二维码已过期，请重新扫码"))
			return
		case appregistration.StatusFailed:
			session.finish(ChannelLoginStatusError, firstNonEmpty(result.Message, "扫码注册失败"))
			return
		case appregistration.StatusSucceeded:
			if err = s.saveRegisteredChannelCredentials(context.Background(), row, result.Credentials); err != nil {
				session.finish(ChannelLoginStatusError, "保存扫码凭据失败: "+err.Error())
				return
			}
			session.setAccount(channelRegistrationAccountID(row.ChannelType, result.Credentials), result.UserID)
			session.appendOutput(channelRegistrationSuccessMessage(row.ChannelType))
			session.finish(ChannelLoginStatusSucceeded, "")
			return
		default:
			session.finish(ChannelLoginStatusError, fmt.Sprintf("未知扫码注册状态: %s", result.Status))
			return
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		session.finish(ChannelLoginStatusExpired, "二维码已超时，请重新拉起")
		return
	}
	session.finish(ChannelLoginStatusCancelled, "扫码注册已取消")
}

func (s *ControlService) saveRegisteredChannelCredentials(
	ctx context.Context,
	row *channelConfigRow,
	registered map[string]string,
) error {
	publicConfig, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		return err
	}
	secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return err
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	switch row.ChannelType {
	case ChannelTypeFeishu:
		publicConfig["app_id"] = registered["client_id"]
		secrets["app_secret"] = registered["client_secret"]
	case ChannelTypeDingTalk:
		publicConfig["client_id"] = registered["client_id"]
		secrets["client_secret"] = registered["client_secret"]
	case ChannelTypeWeChat:
		publicConfig["bot_id"] = registered["bot_id"]
		secrets["secret"] = registered["secret"]
	default:
		return ErrChannelLoginUnsupported
	}
	publicKey, secretKey, _ := channelManualCredentialPair(row.ChannelType)
	if strings.TrimSpace(publicConfig[publicKey]) == "" || strings.TrimSpace(secrets[secretKey]) == "" {
		return errors.New("扫码成功但平台未返回完整的应用凭据")
	}
	configJSON, err := encodeStringMap(publicConfig)
	if err != nil {
		return err
	}
	encrypted, err := s.encryptCredentials(secrets)
	if err != nil {
		return err
	}
	credentials := sql.NullString{String: encrypted, Valid: encrypted != ""}
	if err = s.upsertChannelConfigRow(ctx, channelConfigRow{
		OwnerUserID:          row.OwnerUserID,
		ChannelType:          row.ChannelType,
		AgentID:              row.AgentID,
		Status:               ChannelConfigStatusConfigured,
		ConfigJSON:           configJSON,
		CredentialsEncrypted: credentials,
	}); err != nil {
		return err
	}
	return s.reloadChannelRuntime(ctx, row.OwnerUserID, row.ChannelType, configJSON, credentials)
}

func (s *ControlService) newChannelRegistrationClient(channelType string) appregistration.Client {
	if s.registrationClientFactory != nil {
		return s.registrationClientFactory(channelType)
	}
	switch channelType {
	case ChannelTypeFeishu:
		return appregistration.NewFeishuClient(s.httpClient, appregistration.FeishuOptions{
			Name:        "Nexus 飞书机器人",
			Description: "连接 Nexus 后用于接收和回复飞书消息。",
			TenantScopes: []string{
				"im:message",
				"im:message:send_as_bot",
				"im:message.reactions:read",
				"im:message.reactions:write",
				"im:resource",
			},
			Events: []string{"im.message.receive_v1", "im.message.reaction.created_v1"},
		})
	case ChannelTypeDingTalk:
		return appregistration.NewDingTalkClient(s.httpClient, "")
	case ChannelTypeWeChat:
		return appregistration.NewWeComClient(s.httpClient, "")
	default:
		return nil
	}
}

func channelRegistrationPrompt(channelType string) string {
	switch channelType {
	case ChannelTypeFeishu:
		return "请使用飞书扫描二维码，选择已有应用或创建新应用，并确认 Nexus 所需权限。\n"
	case ChannelTypeDingTalk:
		return "请使用钉钉扫描二维码，一键创建并授权 Nexus 机器人。\n"
	case ChannelTypeWeChat:
		return "请使用企业微信扫描二维码，绑定 Nexus 智能机器人。\n"
	default:
		return "请扫描二维码继续连接。\n"
	}
}

func channelRegistrationSuccessMessage(channelType string) string {
	switch channelType {
	case ChannelTypeFeishu:
		return "飞书机器人已授权并连接，Nexus 将自动接收和回投消息。\n"
	case ChannelTypeDingTalk:
		return "钉钉机器人已创建并连接，Nexus 将自动接收和回投消息。\n"
	case ChannelTypeWeChat:
		return "企业微信机器人已绑定并连接，Nexus 将自动接收和回投消息。\n"
	default:
		return "扫码连接已完成。\n"
	}
}

func channelRegistrationAccountID(channelType string, credentials map[string]string) string {
	if channelType == ChannelTypeWeChat {
		return strings.TrimSpace(credentials["bot_id"])
	}
	return strings.TrimSpace(credentials["client_id"])
}
