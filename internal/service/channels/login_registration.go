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
	expectedAccountID string,
	authorizationBinding string,
	expectedVersion int64,
) (*ChannelLoginView, error) {
	client := s.newChannelRegistrationClient(row.ChannelType)
	if client == nil {
		return nil, ErrChannelLoginUnsupported
	}
	started, err := client.Start(ctx)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationUnknown, err)
	}
	qrPayload := firstNonEmpty(started.VerificationURIComplete, started.VerificationURI)
	if strings.TrimSpace(started.DeviceCode) == "" || qrPayload == "" {
		return nil, channelControlMutationFailure(
			ControlMutationUnknown,
			errors.New("扫码注册未返回完整的二维码信息"),
		)
	}
	timeout := s.loginTimeout
	if started.ExpiresIn > 0 {
		timeout = time.Duration(started.ExpiresIn) * time.Second
	}
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	loginID := s.idFactory("channel_login")
	session := &channelLoginSession{
		ownerUserID:          row.OwnerUserID,
		channelType:          row.ChannelType,
		activeKey:            activeKey,
		expectedAccountID:    strings.TrimSpace(expectedAccountID),
		authorizationBinding: strings.TrimSpace(authorizationBinding),
		verifyCh:             make(chan struct{}, 1),
		done:                 make(chan struct{}),
		registrationClient:   client,
		deviceCode:           started.DeviceCode,
		pollInterval:         time.Duration(started.Interval) * time.Second,
		view: ChannelLoginView{
			LoginID:             loginID,
			ChannelType:         row.ChannelType,
			Status:              ChannelLoginStatusRunning,
			Command:             "Nexus official QR registration",
			QRPayload:           qrPayload,
			QRPayloadType:       "text",
			Output:              channelRegistrationPrompt(row.ChannelType),
			StartControlVersion: expectedVersion,
			StartedAt:           now,
			UpdatedAt:           now,
			ExpiresAt:           now.Add(timeout),
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

	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	session.setCancel(cancel)
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
	defer session.markDone()
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
			accountID := channelRegistrationAccountID(row.ChannelType, result.Credentials)
			if session.expectedAccountID != "" &&
				session.expectedAccountID != strings.TrimSpace(accountID) {
				session.finish(
					ChannelLoginStatusError,
					"扫码返回账号与授权目标不匹配，未保存任何凭据",
				)
				return
			}
			releaseCommit, acquireErr := s.acquireChannelLoginAuthorizationCommit(
				ctx,
				session,
			)
			if acquireErr != nil {
				session.finish(ChannelLoginStatusError, acquireErr.Error())
				return
			}
			if !session.claimCompletion() {
				releaseCommit()
				return
			}
			defer releaseCommit()
			var committedVersion int64
			committedVersion, err = s.saveRegisteredChannelCredentials(
				context.Background(),
				row,
				result.Credentials,
				session.snapshot().StartControlVersion,
			)
			if err != nil {
				session.releaseCompletion()
				session.finish(ChannelLoginStatusError, "保存扫码凭据失败: "+err.Error())
				return
			}
			session.setCommittedControlVersion(committedVersion)
			session.setAccount(accountID, result.UserID)
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
	expectedVersion int64,
) (int64, error) {
	if row == nil {
		return 0, ErrChannelNotFound
	}
	ownerUserID := normalizeChannelOwnerUserID(row.OwnerUserID)
	channelType := normalizeIMChannelType(row.ChannelType)
	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockChannel := s.lockChannelMutation(ownerUserID, channelType)
	defer unlockChannel()

	reloadSnapshot, err := s.captureChannelReloadSnapshot(ctx, ownerUserID, channelType)
	if err != nil {
		return 0, err
	}
	if reloadSnapshot.version != expectedVersion {
		return 0, channelControlVersionError(
			expectedVersion,
			ErrChannelControlVersionConflict,
		)
	}
	var configJSON string
	var credentials sql.NullString
	committedVersion, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		current, loadErr := s.getChannelConfigRowFrom(ctx, tx, ownerUserID, channelType)
		if loadErr != nil {
			return loadErr
		}
		if current == nil {
			return ErrChannelNotFound
		}
		publicConfig, decodeErr := decodeStringMap(current.ConfigJSON)
		if decodeErr != nil {
			return decodeErr
		}
		secrets, decryptErr := s.decryptCredentials(current.CredentialsEncrypted)
		if decryptErr != nil {
			return decryptErr
		}
		if secrets == nil {
			secrets = map[string]string{}
		}
		switch channelType {
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
		publicKey, secretKey, _ := channelManualCredentialPair(channelType)
		if strings.TrimSpace(publicConfig[publicKey]) == "" ||
			strings.TrimSpace(secrets[secretKey]) == "" {
			return errors.New("扫码成功但平台未返回完整的应用凭据")
		}
		configJSON, decodeErr = encodeStringMap(publicConfig)
		if decodeErr != nil {
			return decodeErr
		}
		encrypted, encryptErr := s.encryptCredentials(secrets)
		if encryptErr != nil {
			return encryptErr
		}
		credentials = sql.NullString{String: encrypted, Valid: encrypted != ""}
		return s.upsertChannelConfigRowWith(ctx, tx, channelConfigRow{
			OwnerUserID: ownerUserID, ChannelType: channelType,
			AgentID: current.AgentID, Status: ChannelConfigStatusConfigured,
			ConfigJSON: configJSON, CredentialsEncrypted: credentials,
		})
	})
	if err != nil {
		return 0, channelControlVersionError(expectedVersion, err)
	}
	if err = s.reloadChannelRuntime(ctx, ownerUserID, channelType, configJSON, credentials); err != nil {
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
