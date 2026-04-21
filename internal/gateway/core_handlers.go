// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：core_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import "net/http"

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"status": "ok",
		},
	})
}

func (s *Server) handleRuntimeOptions(writer http.ResponseWriter, request *http.Request) {
	defaultAgent, err := s.agentService.GetDefaultAgent(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	defaultProvider, err := s.providers.DefaultProvider(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data": map[string]any{
			"default_agent_id":       defaultAgent.AgentID,
			"default_agent_provider": defaultProvider,
		},
	})
}
