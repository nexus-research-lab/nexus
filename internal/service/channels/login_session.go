package channels

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (s *channelLoginSession) snapshot() ChannelLoginView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneChannelLoginView(s.view)
}

func (s *channelLoginSession) authorizationCommitRequest() ChannelLoginAuthorizationCommit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ChannelLoginAuthorizationCommit{
		OwnerUserID:          s.ownerUserID,
		ChannelType:          s.channelType,
		LoginID:              s.view.LoginID,
		AuthorizationBinding: s.authorizationBinding,
		StartControlVersion:  s.view.StartControlVersion,
	}
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
	if !channelLoginIsActive(s.view.Status) {
		return
	}
	s.view.Status = status
	s.view.Error = strings.TrimSpace(errorMessage)
	if s.view.Error != "" && !strings.Contains(s.view.Output, s.view.Error) {
		s.view.Output = trimChannelLoginOutput(s.view.Output + s.view.Error + "\n")
	}
	s.view.UpdatedAt = now
	s.view.FinishedAt = &now
	s.committing = false
	s.verifyCode = ""
}

func (s *channelLoginSession) waitVerifyCode(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.cancelRequested || !channelLoginIsActive(s.view.Status) {
		s.mu.Unlock()
		return "", context.Canceled
	}
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

func (s *channelLoginSession) submitVerifyCode(code string) error {
	s.mu.Lock()
	if s.cancelRequested || s.view.Status != ChannelLoginStatusVerifyCodeRequired {
		s.mu.Unlock()
		return ErrChannelLoginState
	}
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
	return nil
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

func (s *channelLoginSession) setCommittedControlVersion(version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.CommittedControlVersion = version
}

func (s *channelLoginSession) claimCompletion() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelRequested || s.committing || !channelLoginIsActive(s.view.Status) {
		return false
	}
	s.committing = true
	return true
}

func (s *channelLoginSession) releaseCompletion() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.committing = false
}

func (s *channelLoginSession) setCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel = cancel
}

func (s *channelLoginSession) cancelLogin() (ChannelLoginView, error) {
	s.mu.Lock()
	if !channelLoginIsActive(s.view.Status) {
		view := cloneChannelLoginView(s.view)
		s.mu.Unlock()
		return view, nil
	}
	if s.committing {
		view := cloneChannelLoginView(s.view)
		s.mu.Unlock()
		return view, errors.New("channel login completion is already committing")
	}
	s.cancelRequested = true
	cancel := s.cancel
	now := time.Now()
	s.view.Status = ChannelLoginStatusCancelled
	s.view.Error = ""
	s.view.Output = trimChannelLoginOutput(s.view.Output + "扫码授权已取消。\n")
	s.view.UpdatedAt = now
	s.view.FinishedAt = &now
	s.verifyCode = ""
	view := cloneChannelLoginView(s.view)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return view, nil
}

func (s *channelLoginSession) markDone() {
	if s == nil {
		return
	}
	s.doneOnce.Do(func() {
		if s.done != nil {
			close(s.done)
		}
	})
}

func (s *channelLoginSession) waitDone(ctx context.Context) error {
	if s == nil || s.done == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return nil
	}
}

func cloneChannelLoginView(value ChannelLoginView) ChannelLoginView {
	result := value
	if value.FinishedAt != nil {
		finishedAt := *value.FinishedAt
		result.FinishedAt = &finishedAt
	}
	return result
}

func trimChannelLoginOutput(output string) string {
	if len(output) <= channelLoginOutputLimit {
		return output
	}
	return output[len(output)-channelLoginOutputLimit:]
}
