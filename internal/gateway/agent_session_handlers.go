// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：agent_session_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListAgents(writer http.ResponseWriter, request *http.Request) {
	agents, err := s.agentService.ListAgents(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, agents)
}

func (s *Server) handleAgentRuntimeStatuses(writer http.ResponseWriter, request *http.Request) {
	agents, err := s.agentService.ListAgents(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	statuses := make([]map[string]any, 0, len(agents))
	for _, item := range agents {
		runningCount := s.runtime.CountRunningRounds(item.AgentID)
		if s.roomRealtime != nil {
			runningCount += s.roomRealtime.CountRunningTasks(item.AgentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		statuses = append(statuses, map[string]any{
			"agent_id":           item.AgentID,
			"running_task_count": runningCount,
			"status":             status,
		})
	}
	s.writeSuccess(writer, statuses)
}

func (s *Server) handleGetAgent(writer http.ResponseWriter, request *http.Request) {
	agentID := chi.URLParam(request, "agent_id")
	agentValue, err := s.agentService.GetAgent(request.Context(), agentID)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, agentValue)
}

func (s *Server) handleValidateAgentName(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	excludeAgentID := request.URL.Query().Get("exclude_agent_id")
	result, err := s.agentService.ValidateName(request.Context(), name, excludeAgentID)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, result)
}

func (s *Server) handleCreateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload agent2.CreateRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}

	created, err := s.agentService.CreateAgent(request.Context(), payload)
	if err != nil {
		if strings.Contains(err.Error(), "名称") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, created)
}

func (s *Server) handleListAgentSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := s.session.ListAgentSessions(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleListSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := s.session.ListSessions(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleCreateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionsvc.CreateRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.session.CreateSession(request.Context(), payload)
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionMutationUnsupported) || isClientMessageError(err) {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionsvc.UpdateRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.session.UpdateSession(request.Context(), chi.URLParam(request, "session_key"), payload)
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionMutationUnsupported) || isClientMessageError(err) {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteSession(writer http.ResponseWriter, request *http.Request) {
	err := s.session.DeleteSession(request.Context(), chi.URLParam(request, "session_key"))
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}
