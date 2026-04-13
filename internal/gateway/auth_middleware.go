// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：auth_middleware.go
// @Date   ：2026/04/12 01:14:00
// @Author ：leemysw
// 2026/04/12 01:14:00   Create
// =====================================================

package gateway

import (
	authsvc "github.com/nexus-research-lab/nexus-core/internal/auth"
	"net/http"
	"strings"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(writer, request)
			return
		}

		principal, state, err := s.auth.InspectRequest(request.Context(), request)
		if err != nil {
			s.writeFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}

		ctx := authsvc.WithState(request.Context(), state)
		ctx = authsvc.WithPrincipal(ctx, principal)
		if isPublicAuthRoute(request) || !state.AuthRequired {
			next.ServeHTTP(writer, request.WithContext(ctx))
			return
		}
		if principal == nil {
			s.writeFailure(writer, http.StatusUnauthorized, "未登录或登录状态已过期")
			return
		}
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func isPublicAuthRoute(request *http.Request) bool {
	if request == nil {
		return true
	}
	if request.Method == http.MethodOptions {
		return true
	}
	path := strings.TrimSpace(request.URL.Path)
	switch path {
	case "/agent/v1/health",
		"/agent/v1/runtime/options",
		"/agent/v1/auth/status",
		"/agent/v1/auth/login",
		"/agent/v1/auth/logout":
		return true
	default:
		return false
	}
}
