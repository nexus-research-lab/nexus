package auth

import (
	"cmp"
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Service) buildStatusPayload(state State, principal *Principal) StatusPayload {
	result := StatusPayload{
		AuthRequired:         state.AuthRequired,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		Authenticated:        !state.AuthRequired && !state.SetupRequired,
		SetupRequired:        state.SetupRequired,
		AccessTokenEnabled:   state.AccessTokenEnabled,
	}
	if principal == nil {
		return result
	}
	result.Authenticated = true
	result.Username = stringPointer(principal.Username)
	result.UserID = stringPointer(principal.UserID)
	result.DisplayName = stringPointer(principal.DisplayName)
	result.Role = stringPointer(principal.Role)
	result.Avatar = stringPointer(principal.Avatar)
	result.AuthMethod = stringPointer(principal.AuthMethod)
	return result
}

func (s *Service) resolveRequestPrincipal(ctx context.Context, request *http.Request, state State) (*Principal, error) {
	if request == nil {
		return nil, nil
	}
	if s.desktopAuthBypassEnabled() {
		return s.desktopLocalPrincipal(ctx)
	}
	sessionToken := s.ExtractSessionToken(request)
	if sessionToken != "" {
		principal, err := s.resolveSessionPrincipal(ctx, sessionToken)
		if err != nil {
			return nil, err
		}
		if principal != nil {
			return principal, nil
		}
	}
	if !state.AccessTokenEnabled {
		return nil, nil
	}
	return s.resolveBearerPrincipal(request), nil
}

func (s *Service) resolveSessionPrincipal(ctx context.Context, sessionToken string) (*Principal, error) {
	record, user, err := s.repository.GetActiveSessionByTokenHash(ctx, hashSessionToken(sessionToken), s.now())
	if err != nil {
		return nil, err
	}
	if record == nil || user == nil || user.Status != UserStatusActive {
		return nil, nil
	}
	if err = s.repository.TouchSession(ctx, record.SessionID, s.now()); err != nil {
		return nil, err
	}
	return &Principal{
		UserID:      user.UserID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Avatar:      user.Avatar,
		AuthMethod:  AuthMethodPassword,
		SessionID:   stringPointer(record.SessionID),
	}, nil
}

func (s *Service) resolveBearerPrincipal(request *http.Request) *Principal {
	accessToken := strings.TrimSpace(s.config.AccessToken)
	if accessToken == "" || request == nil {
		return nil
	}
	providedToken := extractBearerToken(request.Header.Get("Authorization"))
	if providedToken == "" {
		query := request.URL.Query()
		providedToken = cmp.Or(
			strings.TrimSpace(query.Get("access_token")),
			strings.TrimSpace(query.Get("token")),
		)
	}
	if providedToken == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(accessToken)) != 1 {
		return nil
	}
	return &Principal{
		UserID:      "access-token-bootstrap",
		Username:    "access-token",
		DisplayName: "ACCESS_TOKEN",
		Role:        RoleOwner,
		AuthMethod:  AuthMethodBearer,
	}
}
