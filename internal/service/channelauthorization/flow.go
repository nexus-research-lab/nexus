// INPUT: owner-main actor, Channel/account target, opaque flow ID, or human code handoff.
// OUTPUT: start/status/cancel/code-prompt state machine with encrypted ephemeral data.
// POS: Channel authorization orchestration; model and human surfaces stay separated.
package channelauthorization

import (
	"context"
	"errors"
	"strings"
	"time"

	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

func (s *Service) Start(
	ctx context.Context,
	actor Actor,
	input StartInput,
) (*View, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	binding, err := s.authorize(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = s.Initialize(ctx); err != nil {
		return nil, err
	}
	if s.channels == nil {
		return nil, errors.New("Channel 授权未装配 Channel control")
	}
	if s.presenter == nil {
		return nil, errors.New("Channel 授权未装配人类展示通道")
	}
	channelType := strings.ToLower(strings.TrimSpace(input.ChannelType))
	if channelType == "" {
		return nil, errors.New("channel_type is required")
	}
	expectedAccountID := strings.TrimSpace(input.AccountID)
	binding.ChannelType = channelType
	binding.AccountBinding = accountBinding(expectedAccountID)
	if err = binding.Validate(); err != nil {
		return nil, err
	}
	startVersion, err := s.channels.GetChannelControlVersion(ctx, binding.OwnerUserID)
	if err != nil {
		return nil, err
	}
	flowID, err := s.idFactory("channel_authorization")
	if err != nil {
		return nil, err
	}
	flowGeneration, err := s.idFactory("generation")
	if err != nil {
		return nil, err
	}
	now := s.now()
	flow := authorizationstore.Flow{
		Binding:             binding,
		FlowID:              flowID,
		StartControlVersion: startVersion,
		FlowGeneration:      flowGeneration,
		ProcessGeneration:   s.processGeneration,
		Status:              authorizationstore.StatusStarting,
		ExpiresAt:           now.Add(s.flowTTL),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err = s.repository.Create(ctx, flow); err != nil {
		return nil, err
	}

	login, startErr := s.channels.StartChannelLoginForAuthorizationAtVersion(
		ctx,
		binding.OwnerUserID,
		channelType,
		expectedAccountID,
		flowGeneration,
		startVersion,
	)
	if startErr != nil {
		terminal, finishErr := s.finishFlow(
			ctx,
			flow,
			authorizationstore.StatusError,
			"",
			0,
			outcomeForError(startErr),
		)
		if finishErr != nil {
			return nil, errors.Join(startErr, finishErr)
		}
		return viewForFlow(*terminal), safeStartError(startErr)
	}

	presentationToken, err := s.idFactory("presentation")
	if err != nil {
		return s.failStartedFlow(ctx, flow, login, "presentation_token_failed")
	}
	expiresAt := login.ExpiresAt.UTC()
	if expiresAt.IsZero() || expiresAt.After(flow.ExpiresAt) {
		expiresAt = flow.ExpiresAt
	}
	flow.ExpiresAt = expiresAt
	presentation := humanPresentationForLogin(
		flow,
		*login,
		presentationToken,
		PresentationKindQRCode,
	)
	runtimeCiphertext, err := s.encryptValue(runtimeReference{
		LoginID: login.LoginID, ChannelType: login.ChannelType,
	})
	if err != nil {
		return s.failStartedFlow(ctx, flow, login, "runtime_reference_encrypt_failed")
	}
	presentationCiphertext, err := s.encryptValue(presentation)
	if err != nil {
		return s.failStartedFlow(ctx, flow, login, "presentation_encrypt_failed")
	}
	if err = s.repository.AttachRuntime(
		ctx,
		flow,
		runtimeCiphertext,
		presentationCiphertext,
		authorizationstore.StatusRunning,
		expiresAt,
		s.now(),
	); err != nil {
		_, _ = s.channels.CancelChannelLogin(
			context.Background(),
			binding.OwnerUserID,
			channelType,
			login.LoginID,
		)
		return nil, err
	}
	flow.Status = authorizationstore.StatusRunning
	flow.RuntimeRefEncrypted = runtimeCiphertext
	flow.HumanPresentationEncrypted = presentationCiphertext

	if err = s.presenter.PresentChannelAuthorization(ctx, presentation); err != nil {
		return s.failStartedFlow(ctx, flow, login, "human_presentation_failed")
	}
	s.startMonitor(flow)
	return viewForFlow(flow), nil
}

func (s *Service) Status(
	ctx context.Context,
	actor Actor,
	flowID string,
) (*View, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	binding, flow, err := s.loadAuthorizedFlow(ctx, actor, flowID)
	if err != nil {
		return nil, err
	}
	_ = binding
	flow, err = s.synchronize(ctx, *flow)
	if err != nil {
		return nil, err
	}
	return viewForFlow(*flow), nil
}

func (s *Service) Cancel(
	ctx context.Context,
	actor Actor,
	flowID string,
) (*View, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	_, flow, err := s.loadAuthorizedFlow(ctx, actor, flowID)
	if err != nil {
		return nil, err
	}
	if !authorizationstore.IsActiveStatus(flow.Status) {
		return viewForFlow(*flow), nil
	}
	ref, err := s.decryptRuntimeReference(flow.RuntimeRefEncrypted)
	if err == nil {
		login, cancelErr := s.channels.CancelChannelLogin(
			ctx,
			flow.OwnerUserID,
			flow.ChannelType,
			ref.LoginID,
		)
		if cancelErr != nil ||
			login == nil ||
			login.Status != channelssvc.ChannelLoginStatusCancelled {
			// A completion that has already claimed the authorization commit
			// fence wins. Reconcile its exact terminal status instead of
			// overwriting a published credential set with a cancelled audit.
			current, synchronizeErr := s.synchronize(ctx, *flow)
			if synchronizeErr != nil {
				return nil, errors.Join(cancelErr, synchronizeErr)
			}
			return viewForFlow(*current), nil
		}
	}
	terminal, err := s.finishFlow(
		ctx,
		*flow,
		authorizationstore.StatusCancelled,
		"",
		0,
		safeOutcome{Code: "cancelled_by_human", Message: "授权已由当前用户取消。"},
	)
	s.stopMonitor(flow.FlowID)
	if err != nil {
		return nil, err
	}
	return viewForFlow(*terminal), nil
}

// RequestVerificationCode replays the native code-entry card. The MCP tool
// deliberately has no code argument, preventing transcript/tool-log leakage.
func (s *Service) RequestVerificationCode(
	ctx context.Context,
	actor Actor,
	flowID string,
) (*View, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	_, flow, err := s.loadAuthorizedFlow(ctx, actor, flowID)
	if err != nil {
		return nil, err
	}
	flow, err = s.synchronize(ctx, *flow)
	if err != nil {
		return nil, err
	}
	if flow.Status != authorizationstore.StatusVerifyCodeRequired {
		return nil, errors.New("当前 Channel 授权尚未请求验证码")
	}
	presentation, err := s.decryptHumanPresentation(flow.HumanPresentationEncrypted)
	if err != nil {
		return nil, err
	}
	presentation.Kind = PresentationKindVerificationCode
	presentation.QRPayload = ""
	presentation.QRPayloadType = ""
	presentation.Prompt = "请在 Nexus 的安全输入卡中填写手机上显示的验证码。验证码不会发送给智能体。"
	bindPresentationRoute(&presentation, *flow)
	if s.presenter == nil {
		return nil, errors.New("Channel 授权未装配人类展示通道")
	}
	if err = s.presenter.PresentChannelAuthorization(ctx, presentation); err != nil {
		return nil, errors.New("无法向当前用户展示验证码输入卡")
	}
	return viewForFlow(*flow), nil
}

// SubmitHumanVerificationCode is called only by the authenticated native UI.
// It consumes an exact presentation token and route; code is never persisted.
func (s *Service) SubmitHumanVerificationCode(
	ctx context.Context,
	submission HumanVerificationCodeSubmission,
) (*View, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	if err := s.Initialize(ctx); err != nil {
		return nil, err
	}
	flow, err := s.repository.Get(ctx, submission.OwnerUserID, submission.FlowID)
	if err != nil {
		return nil, err
	}
	if err = requireExactHumanSubmission(*flow, submission); err != nil {
		return nil, err
	}
	if _, err = s.verifyFlowHuman(ctx, flow.Binding, true); err != nil {
		return nil, err
	}
	if !s.now().Before(flow.ExpiresAt) {
		terminal, finishErr := s.expireFlow(ctx, *flow)
		if finishErr != nil {
			return nil, finishErr
		}
		return viewForFlow(*terminal), errors.New("Channel 授权已过期")
	}
	if flow.Status != authorizationstore.StatusVerifyCodeRequired {
		return nil, errors.New("当前 Channel 授权不接受验证码")
	}
	presentation, err := s.decryptHumanPresentation(flow.HumanPresentationEncrypted)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(submission.PresentationToken) != presentation.PresentationToken {
		return nil, errors.New("Channel 授权展示 token 已失效")
	}
	code := strings.TrimSpace(submission.Code)
	if code == "" {
		return nil, errors.New("验证码不能为空")
	}
	ref, err := s.decryptRuntimeReference(flow.RuntimeRefEncrypted)
	if err != nil {
		return nil, err
	}
	_, err = s.channels.SubmitChannelLoginVerifyCode(
		ctx,
		flow.OwnerUserID,
		flow.ChannelType,
		ref.LoginID,
		channelssvc.SubmitChannelLoginVerifyCodeRequest{VerifyCode: code},
	)
	code = ""
	submission.Code = ""
	if err != nil {
		return nil, errors.New("验证码未被当前授权会话接受")
	}
	if err = s.repository.UpdateProgress(
		ctx,
		*flow,
		authorizationstore.StatusRunning,
		flow.ResolvedAccountID,
		flow.CommittedControlVersion,
		s.now(),
	); err != nil {
		return nil, err
	}
	flow.Status = authorizationstore.StatusRunning
	flow.UpdatedAt = s.now()
	return viewForFlow(*flow), nil
}

func (s *Service) Completion(
	ctx context.Context,
	actor Actor,
	flowID string,
) (*Completion, error) {
	endOperation, err := s.beginOperation()
	if err != nil {
		return nil, err
	}
	defer endOperation()
	_, flow, err := s.loadAuthorizedFlow(ctx, actor, flowID)
	if err != nil {
		return nil, err
	}
	if authorizationstore.IsActiveStatus(flow.Status) {
		flow, err = s.synchronize(ctx, *flow)
		if err != nil {
			return nil, err
		}
	}
	if !authorizationstore.IsTerminalStatus(flow.Status) {
		return nil, errors.New("Channel 授权尚未完成")
	}
	audit, err := s.repository.GetAudit(ctx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return nil, err
	}
	return &Completion{
		FlowID:                  audit.FlowID,
		ChannelType:             audit.ChannelType,
		AccountBinding:          audit.AccountBinding,
		ResolvedAccountID:       audit.ResolvedAccountID,
		Status:                  audit.Status,
		OutcomeCode:             audit.OutcomeCode,
		OutcomeMessage:          audit.OutcomeMessage,
		StartControlVersion:     audit.StartControlVersion,
		CommittedControlVersion: audit.CommittedControlVersion,
		Generation:              audit.FlowGeneration,
		CompletedAt:             audit.CompletedAt,
	}, nil
}

func (s *Service) loadAuthorizedFlow(
	ctx context.Context,
	actor Actor,
	flowID string,
) (authorizationstore.Binding, *authorizationstore.Flow, error) {
	current, err := s.authorize(ctx, actor)
	if err != nil {
		return authorizationstore.Binding{}, nil, err
	}
	if err = s.Initialize(ctx); err != nil {
		return authorizationstore.Binding{}, nil, err
	}
	flow, err := s.repository.Get(ctx, current.OwnerUserID, strings.TrimSpace(flowID))
	if err != nil {
		return authorizationstore.Binding{}, nil, err
	}
	if err = requireFlowActorBinding(current, flow.Binding); err != nil {
		return authorizationstore.Binding{}, nil, err
	}
	return current, flow, nil
}

func (s *Service) failStartedFlow(
	ctx context.Context,
	flow authorizationstore.Flow,
	login *channelssvc.ChannelLoginView,
	code string,
) (*View, error) {
	if login != nil && strings.TrimSpace(login.LoginID) != "" {
		_, _ = s.channels.CancelChannelLogin(
			context.Background(),
			flow.OwnerUserID,
			flow.ChannelType,
			login.LoginID,
		)
	}
	terminal, err := s.finishFlow(
		ctx,
		flow,
		authorizationstore.StatusError,
		"",
		0,
		safeOutcome{
			Code:    code,
			Message: "无法安全建立人类授权展示，授权已取消且未保存凭据。",
		},
	)
	if err != nil {
		return nil, err
	}
	return viewForFlow(*terminal), errors.New("无法安全建立 Channel 人类授权展示")
}

func accountBinding(accountID string) string {
	if strings.TrimSpace(accountID) == "" {
		return "new"
	}
	return "account:" + strings.TrimSpace(accountID)
}

func humanPresentationForLogin(
	flow authorizationstore.Flow,
	login channelssvc.ChannelLoginView,
	token string,
	kind string,
) HumanPresentation {
	return HumanPresentation{
		FlowID:                 flow.FlowID,
		PresentationToken:      token,
		Kind:                   kind,
		ChannelType:            flow.ChannelType,
		AccountBinding:         flow.AccountBinding,
		QRPayload:              login.QRPayload,
		QRPayloadType:          login.QRPayloadType,
		Prompt:                 humanPrompt(login),
		PrincipalUserID:        flow.PrincipalUserID,
		PrincipalAuthMethod:    flow.PrincipalAuthMethod,
		PrincipalAuthSessionID: flow.PrincipalAuthSessionID,
		AgentID:                flow.AgentID,
		BusinessSessionKey:     flow.BusinessSessionKey,
		RootRoundID:            flow.RootRoundID,
		RuntimeLeaseSessionKey: flow.RuntimeLeaseSessionKey,
		RuntimeLeaseRoundID:    flow.RuntimeLeaseRoundID,
		ExpiresAt:              flow.ExpiresAt,
	}
}

func humanPrompt(login channelssvc.ChannelLoginView) string {
	if strings.TrimSpace(login.QRPayload) == "" {
		return "请在 Nexus 的授权卡中完成平台验证。"
	}
	return "请在 Nexus 的授权卡中扫描二维码。二维码不会发送给智能体。"
}

func requireExactHumanSubmission(
	flow authorizationstore.Flow,
	submission HumanVerificationCodeSubmission,
) error {
	if strings.TrimSpace(submission.OwnerUserID) != flow.OwnerUserID ||
		strings.TrimSpace(submission.PrincipalUserID) != flow.PrincipalUserID ||
		strings.TrimSpace(submission.PrincipalAuthSessionID) != flow.PrincipalAuthSessionID ||
		strings.TrimSpace(submission.AgentID) != flow.AgentID ||
		strings.TrimSpace(submission.BusinessSessionKey) != flow.BusinessSessionKey ||
		strings.TrimSpace(submission.RootRoundID) != flow.RootRoundID ||
		strings.TrimSpace(submission.RuntimeLeaseSessionKey) != flow.RuntimeLeaseSessionKey ||
		strings.TrimSpace(submission.RuntimeLeaseRoundID) != flow.RuntimeLeaseRoundID {
		return errors.New("验证码提交与原始 principal、Agent、session、root round 或 runtime lease 不匹配")
	}
	return nil
}

func safeStartError(err error) error {
	switch {
	case errors.Is(err, channelssvc.ErrChannelControlVersionConflict):
		return errors.New("Channel 配置版本已变化，请重新开始授权")
	case errors.Is(err, channelssvc.ErrChannelLoginUnsupported):
		return errors.New("该 Channel 不支持扫码授权")
	default:
		return errors.New("无法启动 Channel 授权")
	}
}

func viewForFlow(flow authorizationstore.Flow) *View {
	return &View{
		FlowID:                  flow.FlowID,
		ChannelType:             flow.ChannelType,
		AccountBinding:          flow.AccountBinding,
		ResolvedAccountID:       flow.ResolvedAccountID,
		Status:                  flow.Status,
		StartControlVersion:     flow.StartControlVersion,
		CommittedControlVersion: flow.CommittedControlVersion,
		Generation:              flow.FlowGeneration,
		HumanActionRequired: flow.Status == authorizationstore.StatusRunning ||
			flow.Status == authorizationstore.StatusVerifyCodeRequired,
		Message:    safeStatusMessage(flow),
		ExpiresAt:  flow.ExpiresAt,
		FinishedAt: flow.FinishedAt,
	}
}

func safeStatusMessage(flow authorizationstore.Flow) string {
	switch flow.Status {
	case authorizationstore.StatusStarting:
		return "正在建立安全授权会话。"
	case authorizationstore.StatusRunning:
		return "授权卡已发送给当前用户，等待扫码确认。"
	case authorizationstore.StatusVerifyCodeRequired:
		return "平台需要验证码；请让当前用户在 Nexus 安全输入卡中填写。"
	case authorizationstore.StatusSucceeded:
		return "Channel 授权完成，凭据已加密保存并通过运行态重载核验。"
	case authorizationstore.StatusExpired:
		return "Channel 授权已过期，请重新开始。"
	case authorizationstore.StatusCancelled:
		return "Channel 授权已取消。"
	case authorizationstore.StatusRestartInvalidated:
		return "服务重启后原授权已安全失效，请重新开始。"
	default:
		if strings.TrimSpace(flow.OutcomeMessage) != "" {
			return flow.OutcomeMessage
		}
		return "Channel 授权未完成，未覆盖较新的配置。"
	}
}

func (s *Service) startMonitor(flow authorizationstore.Flow) {
	ctx, cancel := context.WithCancel(context.Background())
	s.monitorMu.Lock()
	if previous := s.monitors[flow.FlowID]; previous != nil {
		previous()
	}
	s.monitors[flow.FlowID] = cancel
	s.monitorWG.Add(1)
	s.monitorMu.Unlock()
	go s.monitor(ctx, flow)
}

func (s *Service) stopMonitor(flowID string) {
	s.monitorMu.Lock()
	cancel := s.monitors[flowID]
	delete(s.monitors, flowID)
	s.monitorMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) monitor(ctx context.Context, flow authorizationstore.Flow) {
	defer s.monitorWG.Done()
	defer s.stopMonitorWithoutCancel(flow.FlowID)
	interval := s.monitorInterval
	if interval <= 0 {
		interval = defaultMonitorInterval
	}
	timer := time.NewTicker(interval)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		endOperation, beginErr := s.beginOperation()
		if beginErr != nil {
			return
		}
		current, err := s.synchronize(ctx, flow)
		endOperation()
		if err == nil {
			flow = *current
			if authorizationstore.IsTerminalStatus(flow.Status) {
				return
			}
		} else if errors.Is(err, authorizationstore.ErrFlowConflict) ||
			errors.Is(err, authorizationstore.ErrFlowNotFound) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (s *Service) stopMonitorWithoutCancel(flowID string) {
	s.monitorMu.Lock()
	delete(s.monitors, flowID)
	s.monitorMu.Unlock()
}
