// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：auth_handlers.go
// @Date   ：2026/04/12 01:14:00
// @Author ：leemysw
// 2026/04/12 01:14:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"

	auth2 "github.com/nexus-research-lab/nexus/internal/auth"
)

type authLoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthStatus(writer http.ResponseWriter, request *http.Request) {
	if s.auth == nil {
		s.writeFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}
	status, err := s.auth.BuildStatusPayload(request.Context(), request)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, status)
}

func (s *Server) handleAuthLogin(writer http.ResponseWriter, request *http.Request) {
	if s.auth == nil {
		s.writeFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}

	var payload authLoginPayload
	if !s.bindJSON(writer, request, &payload) {
		return
	}

	if err := s.auth.Logout(request.Context(), s.auth.ExtractSessionToken(request)); err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := s.auth.Login(request.Context(), auth2.LoginInput{
		Username:  payload.Username,
		Password:  payload.Password,
		ClientIP:  auth2.ResolveClientIP(request),
		UserAgent: request.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, auth2.ErrPasswordLoginDisabled):
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth2.ErrInvalidCredentials):
			s.writeFailure(writer, http.StatusUnauthorized, err.Error())
		default:
			if isClientMessageError(err) {
				s.writeFailure(writer, http.StatusBadRequest, err.Error())
				return
			}
			s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     s.auth.CookieName(),
		Value:    result.SessionToken,
		MaxAge:   s.auth.SessionMaxAge(),
		Path:     s.auth.CookiePath(),
		HttpOnly: true,
		SameSite: s.auth.CookieSameSite(),
		Secure:   s.auth.CookieSecure(),
	})
	s.writeSuccess(writer, result.Status)
}

func (s *Server) handleAuthLogout(writer http.ResponseWriter, request *http.Request) {
	if s.auth == nil {
		s.writeFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}

	if err := s.auth.Logout(request.Context(), s.auth.ExtractSessionToken(request)); err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     s.auth.CookieName(),
		Value:    "",
		MaxAge:   -1,
		Path:     s.auth.CookiePath(),
		HttpOnly: true,
		SameSite: s.auth.CookieSameSite(),
		Secure:   s.auth.CookieSecure(),
	})

	state, ok := auth2.StateFromContext(request.Context())
	if !ok {
		var err error
		state, err = s.auth.GetState(request.Context())
		if err != nil {
			s.writeFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.writeSuccess(writer, auth2.StatusPayload{
		AuthRequired:         state.AuthRequired,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		Authenticated:        !state.AuthRequired,
		Username:             nil,
		SetupRequired:        state.SetupRequired,
		AccessTokenEnabled:   state.AccessTokenEnabled,
	})
}
