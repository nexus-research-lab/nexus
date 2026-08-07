// INPUT: one active durable flow and its encrypted in-process Channel login reference.
// OUTPUT: safe progress or terminal audit synchronized from the Channel control service.
// POS: async completion, expiry, version conflict, and reload-failure reconciliation.
package channelauthorization

import (
	"context"
	"errors"
	"strings"

	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

type safeOutcome struct {
	Code    string
	Message string
}

func (s *Service) synchronize(
	ctx context.Context,
	flow authorizationstore.Flow,
) (*authorizationstore.Flow, error) {
	current, err := s.repository.Get(ctx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return nil, err
	}
	if !authorizationstore.IsActiveStatus(current.Status) {
		return current, nil
	}
	if current.ProcessGeneration != s.processGeneration {
		return nil, authorizationstore.ErrFlowConflict
	}
	if strings.TrimSpace(current.RuntimeRefEncrypted) == "" {
		if !s.now().Before(current.ExpiresAt) {
			return s.expireFlow(ctx, *current)
		}
		return current, nil
	}
	ref, err := s.decryptRuntimeReference(current.RuntimeRefEncrypted)
	if err != nil {
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusError,
			"",
			0,
			safeOutcome{
				Code:    "runtime_reference_invalid",
				Message: "授权运行态引用不可用；未保存或覆盖 Channel 凭据。",
			},
		)
	}
	login, err := s.channels.GetChannelLogin(
		ctx,
		current.OwnerUserID,
		current.ChannelType,
		ref.LoginID,
	)
	if err != nil {
		if errors.Is(err, channelssvc.ErrChannelLoginNotFound) {
			return s.finishFlow(
				ctx,
				*current,
				authorizationstore.StatusError,
				"",
				0,
				safeOutcome{
					Code:    "runtime_session_lost",
					Message: "授权运行态已失效；未保存或覆盖 Channel 凭据。",
				},
			)
		}
		return nil, err
	}
	switch login.Status {
	case channelssvc.ChannelLoginStatusRunning:
		if !s.now().Before(current.ExpiresAt) {
			return s.expireFlow(ctx, *current)
		}
		return s.updateProgress(ctx, *current, authorizationstore.StatusRunning, *login)
	case channelssvc.ChannelLoginStatusVerifyCodeRequired:
		if !s.now().Before(current.ExpiresAt) {
			return s.expireFlow(ctx, *current)
		}
		updated, updateErr := s.updateProgress(
			ctx,
			*current,
			authorizationstore.StatusVerifyCodeRequired,
			*login,
		)
		if updateErr != nil {
			return nil, updateErr
		}
		if current.Status != authorizationstore.StatusVerifyCodeRequired {
			if presentErr := s.presentVerificationCode(ctx, *updated); presentErr != nil {
				_, _ = s.channels.CancelChannelLogin(
					context.Background(),
					updated.OwnerUserID,
					updated.ChannelType,
					ref.LoginID,
				)
				return s.finishFlow(
					ctx,
					*updated,
					authorizationstore.StatusError,
					"",
					0,
					safeOutcome{
						Code:    "verification_card_unavailable",
						Message: "无法安全展示验证码输入卡；授权已取消且未保存凭据。",
					},
				)
			}
		}
		return updated, nil
	case channelssvc.ChannelLoginStatusSucceeded:
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusSucceeded,
			login.AccountID,
			login.CommittedControlVersion,
			safeOutcome{
				Code:    "completed",
				Message: "Channel 凭据已加密保存，候选 runtime 已启动并完成发布。",
			},
		)
	case channelssvc.ChannelLoginStatusExpired:
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusExpired,
			"",
			0,
			safeOutcome{Code: "expired", Message: "授权已过期，未保存 Channel 凭据。"},
		)
	case channelssvc.ChannelLoginStatusCancelled:
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusCancelled,
			"",
			0,
			safeOutcome{Code: "cancelled", Message: "授权已取消，未保存 Channel 凭据。"},
		)
	case channelssvc.ChannelLoginStatusError:
		outcome := outcomeForLoginError(login.Error)
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusError,
			"",
			0,
			outcome,
		)
	default:
		return s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusError,
			"",
			0,
			safeOutcome{
				Code:    "unknown_runtime_status",
				Message: "授权运行态返回未知状态；未保存或覆盖 Channel 凭据。",
			},
		)
	}
}

func (s *Service) updateProgress(
	ctx context.Context,
	flow authorizationstore.Flow,
	status string,
	login channelssvc.ChannelLoginView,
) (*authorizationstore.Flow, error) {
	if flow.Status == status &&
		flow.ResolvedAccountID == strings.TrimSpace(login.AccountID) &&
		flow.CommittedControlVersion == login.CommittedControlVersion {
		return &flow, nil
	}
	if err := s.repository.UpdateProgress(
		ctx,
		flow,
		status,
		login.AccountID,
		login.CommittedControlVersion,
		s.now(),
	); err != nil {
		return nil, err
	}
	return s.repository.Get(ctx, flow.OwnerUserID, flow.FlowID)
}

func (s *Service) presentVerificationCode(
	ctx context.Context,
	flow authorizationstore.Flow,
) error {
	if s.presenter == nil {
		return errors.New("Channel 授权未装配人类展示通道")
	}
	presentation, err := s.decryptHumanPresentation(flow.HumanPresentationEncrypted)
	if err != nil {
		return err
	}
	presentation.Kind = PresentationKindVerificationCode
	presentation.QRPayload = ""
	presentation.QRPayloadType = ""
	presentation.Prompt = "请在 Nexus 的安全输入卡中填写手机上显示的验证码。验证码不会发送给智能体。"
	bindPresentationRoute(&presentation, flow)
	return s.presenter.PresentChannelAuthorization(ctx, presentation)
}

func bindPresentationRoute(
	presentation *HumanPresentation,
	flow authorizationstore.Flow,
) {
	if presentation == nil {
		return
	}
	presentation.PrincipalUserID = flow.PrincipalUserID
	presentation.PrincipalAuthMethod = flow.PrincipalAuthMethod
	presentation.PrincipalAuthSessionID = flow.PrincipalAuthSessionID
	presentation.AgentID = flow.AgentID
	presentation.BusinessSessionKey = flow.BusinessSessionKey
	presentation.RootRoundID = flow.RootRoundID
	presentation.RuntimeLeaseSessionKey = flow.RuntimeLeaseSessionKey
	presentation.RuntimeLeaseRoundID = flow.RuntimeLeaseRoundID
}

func (s *Service) expireFlow(
	ctx context.Context,
	flow authorizationstore.Flow,
) (*authorizationstore.Flow, error) {
	if ref, err := s.decryptRuntimeReference(flow.RuntimeRefEncrypted); err == nil {
		_, _ = s.channels.CancelChannelLogin(
			context.Background(),
			flow.OwnerUserID,
			flow.ChannelType,
			ref.LoginID,
		)
	}
	return s.finishFlow(
		ctx,
		flow,
		authorizationstore.StatusExpired,
		"",
		0,
		safeOutcome{Code: "expired", Message: "授权已过期，未保存 Channel 凭据。"},
	)
}

func (s *Service) finishFlow(
	ctx context.Context,
	flow authorizationstore.Flow,
	status string,
	resolvedAccountID string,
	committedVersion int64,
	outcome safeOutcome,
) (*authorizationstore.Flow, error) {
	auditID, err := s.idFactory("channel_authorization_audit")
	if err != nil {
		return nil, err
	}
	return s.repository.Finish(ctx, flow, authorizationstore.TerminalUpdate{
		Status:                  status,
		ResolvedAccountID:       strings.TrimSpace(resolvedAccountID),
		CommittedControlVersion: committedVersion,
		OutcomeCode:             strings.TrimSpace(outcome.Code),
		OutcomeMessage:          strings.TrimSpace(outcome.Message),
		FinishedAt:              s.now(),
		AuditID:                 auditID,
	})
}

func outcomeForError(err error) safeOutcome {
	switch {
	case errors.Is(err, channelssvc.ErrChannelControlVersionConflict):
		return safeOutcome{
			Code:    "channel_version_conflict",
			Message: "Channel 配置版本已变化；授权未覆盖较新的配置。",
		}
	case errors.Is(err, channelssvc.ErrChannelRuntimeReload):
		return safeOutcome{
			Code:    "runtime_reload_failed",
			Message: "候选 Channel runtime 启动失败；上一份可运行配置和版本已恢复。",
		}
	case errors.Is(err, channelssvc.ErrChannelLoginUnsupported):
		return safeOutcome{
			Code:    "unsupported",
			Message: "该 Channel 不支持扫码授权。",
		}
	default:
		return safeOutcome{
			Code:    "authorization_failed",
			Message: "Channel 授权失败；未保存或覆盖较新的配置。",
		}
	}
}

func outcomeForLoginError(message string) safeOutcome {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "version") ||
		strings.Contains(normalized, "配置版本") ||
		strings.Contains(normalized, "control version"):
		return safeOutcome{
			Code:    "channel_version_conflict",
			Message: "Channel 配置版本已变化；授权未覆盖较新的配置。",
		}
	case strings.Contains(normalized, "runtime") ||
		strings.Contains(normalized, "上一份可运行配置"):
		return safeOutcome{
			Code:    "runtime_reload_failed",
			Message: "候选 Channel runtime 启动失败；上一份可运行配置和版本已恢复。",
		}
	case strings.Contains(normalized, "账号与授权目标不匹配"):
		return safeOutcome{
			Code:    "account_binding_mismatch",
			Message: "平台返回的账号与授权目标不匹配；未保存任何凭据。",
		}
	default:
		return safeOutcome{
			Code:    "authorization_failed",
			Message: "Channel 授权失败；未保存或覆盖较新的配置。",
		}
	}
}
