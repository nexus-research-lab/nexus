// INPUT: service shutdown context, current-process active flows, and exact login refs.
// OUTPUT: fenced cancellation, joined monitor/login shutdown, and terminal audits.
// POS: graceful shutdown boundary; after Close returns no old authorization can commit.
package channelauthorization

import (
	"context"
	"errors"

	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

// Close fences new work and commit leases, cancels every exact underlying
// login, waits for monitors and login pollers, and audits every remaining flow.
func (s *Service) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closeErr = s.close(ctx)
	})
	return s.closeErr
}

func (s *Service) close(ctx context.Context) error {
	// Cleanup must outlive an already-cancelled HTTP shutdown context. Returning
	// before pollers exit would violate the no-post-Close-write contract.
	cleanupCtx := context.WithoutCancel(ctx)

	// Writer preference blocks new beginOperation readers while waiting for
	// current operations and already-acquired commit leases to drain.
	s.lifecycleMu.Lock()
	s.closing = true
	s.lifecycleMu.Unlock()

	s.stopAllMonitors()
	flows, listErr := s.repository.ListActiveForProcess(
		cleanupCtx,
		s.processGeneration,
	)
	errs := []error{listErr}
	for index := range flows {
		if err := s.closeActiveFlow(cleanupCtx, flows[index]); err != nil {
			errs = append(errs, err)
		}
	}
	s.monitorWG.Wait()

	// A monitor may have completed a terminal transition while Close waited for
	// the lifecycle writer. Re-listing proves there is no unaudited active row.
	remaining, remainingErr := s.repository.ListActiveForProcess(
		cleanupCtx,
		s.processGeneration,
	)
	errs = append(errs, remainingErr)
	for index := range remaining {
		if err := s.closeActiveFlow(cleanupCtx, remaining[index]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) stopAllMonitors() {
	s.monitorMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.monitors))
	for _, cancel := range s.monitors {
		cancels = append(cancels, cancel)
	}
	s.monitors = make(map[string]context.CancelFunc)
	s.monitorMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Service) closeActiveFlow(
	ctx context.Context,
	flow authorizationstore.Flow,
) error {
	current, err := s.repository.Get(ctx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return err
	}
	if !authorizationstore.IsActiveStatus(current.Status) {
		return nil
	}
	if s.channels == nil {
		_, finishErr := s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusCancelled,
			"",
			0,
			safeOutcome{
				Code:    "service_closed",
				Message: "Channel 授权服务已关闭；授权未保存凭据。",
			},
		)
		return errors.Join(
			errors.New("Channel authorization control is unavailable during shutdown"),
			finishErr,
		)
	}
	ref, refErr := s.decryptRuntimeReference(current.RuntimeRefEncrypted)
	if refErr != nil {
		_, finishErr := s.finishFlow(
			ctx,
			*current,
			authorizationstore.StatusCancelled,
			"",
			0,
			safeOutcome{
				Code:    "service_closed",
				Message: "Channel 授权服务已关闭；授权未保存凭据。",
			},
		)
		return errors.Join(refErr, finishErr)
	}
	login, cancelErr := s.channels.CancelChannelLoginAndWait(
		ctx,
		current.OwnerUserID,
		current.ChannelType,
		ref.LoginID,
	)
	if login != nil && login.Status == channelssvc.ChannelLoginStatusSucceeded {
		_, err = s.finishFlow(
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
		return errors.Join(cancelErr, err)
	}
	_, finishErr := s.finishFlow(
		ctx,
		*current,
		authorizationstore.StatusCancelled,
		"",
		0,
		safeOutcome{
			Code:    "service_closed",
			Message: "Channel 授权服务已关闭；授权未保存凭据。",
		},
	)
	if errors.Is(cancelErr, channelssvc.ErrChannelLoginNotFound) {
		cancelErr = nil
	}
	return errors.Join(cancelErr, finishErr)
}
