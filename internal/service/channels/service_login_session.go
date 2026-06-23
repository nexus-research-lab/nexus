package channels

import (
	"context"
	"strings"
	"time"
)

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

func trimChannelLoginOutput(output string) string {
	if len(output) <= channelLoginOutputLimit {
		return output
	}
	return output[len(output)-channelLoginOutputLimit:]
}
