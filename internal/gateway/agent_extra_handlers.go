// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：agent_extra_handlers.go
// @Date   ：2026/04/16 20:48:00
// @Author ：leemysw
// 2026/04/16 20:48:00   Create
// =====================================================

package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleUpdateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload agent2.UpdateRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.agentService.UpdateAgent(request.Context(), chi.URLParam(request, "agent_id"), payload)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "名称") || strings.Contains(err.Error(), "不可") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteAgent(writer http.ResponseWriter, request *http.Request) {
	err := s.agentService.DeleteAgent(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不可删除") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}
