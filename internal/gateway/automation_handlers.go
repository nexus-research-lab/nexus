// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：automation_handlers.go
// @Date   ：2026/04/11 15:41:00
// @Author ：leemysw
// 2026/04/11 15:41:00   Create
// =====================================================

package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"

	"github.com/go-chi/chi/v5"
)

type scheduledTaskCreatePayload struct {
	Name          string                        `json:"name"`
	AgentID       string                        `json:"agent_id"`
	Schedule      automationsvc.Schedule        `json:"schedule"`
	Instruction   string                        `json:"instruction"`
	SessionTarget *automationsvc.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *automationsvc.DeliveryTarget `json:"delivery,omitempty"`
	Source        *automationsvc.Source         `json:"source,omitempty"`
	Enabled       *bool                         `json:"enabled,omitempty"`
}

type scheduledTaskUpdatePayload struct {
	Name          *string                       `json:"name,omitempty"`
	Schedule      *automationsvc.Schedule       `json:"schedule,omitempty"`
	Instruction   *string                       `json:"instruction,omitempty"`
	SessionTarget *automationsvc.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *automationsvc.DeliveryTarget `json:"delivery,omitempty"`
	Source        *automationsvc.Source         `json:"source,omitempty"`
	Enabled       *bool                         `json:"enabled,omitempty"`
}

type scheduledTaskStatusPayload struct {
	Enabled bool `json:"enabled"`
}

type heartbeatUpdatePayload struct {
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

type heartbeatWakePayload struct {
	Mode string  `json:"mode"`
	Text *string `json:"text,omitempty"`
}

func (s *Server) handleListScheduledTasks(writer http.ResponseWriter, request *http.Request) {
	items, err := s.automation.ListTasks(request.Context(), strings.TrimSpace(request.URL.Query().Get("agent_id")))
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleCreateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskCreatePayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	sessionTarget := automationsvc.SessionTarget{}
	if payload.SessionTarget != nil {
		sessionTarget = *payload.SessionTarget
	}
	delivery := automationsvc.DeliveryTarget{}
	if payload.Delivery != nil {
		delivery = *payload.Delivery
	}
	source := automationsvc.Source{}
	if payload.Source != nil {
		source = *payload.Source
	}
	source.Kind = automationsvc.SourceKindUserPage
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	item, err := s.automation.CreateTask(request.Context(), automationsvc.CreateJobInput{
		Name:          payload.Name,
		AgentID:       payload.AgentID,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        source,
		Enabled:       enabled,
	})
	if err != nil {
		if isClientMessageError(err) || isStructuredSessionKeyError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskUpdatePayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.automation.UpdateTask(request.Context(), chi.URLParam(request, "job_id"), automationsvc.UpdateJobInput{
		Name:          payload.Name,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		SessionTarget: payload.SessionTarget,
		Delivery:      payload.Delivery,
		Source:        payload.Source,
		Enabled:       payload.Enabled,
	})
	if err != nil {
		if errors.Is(err, automationsvc.ErrJobNotFound) {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if isClientMessageError(err) || isStructuredSessionKeyError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteScheduledTask(writer http.ResponseWriter, request *http.Request) {
	if err := s.automation.DeleteTask(request.Context(), chi.URLParam(request, "job_id")); err != nil {
		if errors.Is(err, automationsvc.ErrJobNotFound) {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"job_id": chi.URLParam(request, "job_id")})
}

func (s *Server) handleRunScheduledTask(writer http.ResponseWriter, request *http.Request) {
	item, err := s.automation.RunTaskNow(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, automationsvc.ErrJobNotFound) {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if isClientMessageError(err) || isStructuredSessionKeyError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateScheduledTaskStatus(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskStatusPayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.automation.UpdateTaskStatus(request.Context(), chi.URLParam(request, "job_id"), payload.Enabled)
	if err != nil {
		if errors.Is(err, automationsvc.ErrJobNotFound) {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if isClientMessageError(err) || isStructuredSessionKeyError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleListScheduledTaskRuns(writer http.ResponseWriter, request *http.Request) {
	items, err := s.automation.ListTaskRuns(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, automationsvc.ErrJobNotFound) {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleGetHeartbeat(writer http.ResponseWriter, request *http.Request) {
	item, err := s.automation.GetHeartbeatStatus(request.Context(), chi.URLParam(request, "agent_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateHeartbeat(writer http.ResponseWriter, request *http.Request) {
	var payload heartbeatUpdatePayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.automation.UpdateHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), automationsvc.HeartbeatUpdateInput{
		Enabled:      payload.Enabled,
		EverySeconds: payload.EverySeconds,
		TargetMode:   payload.TargetMode,
		AckMaxChars:  payload.AckMaxChars,
	})
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleWakeHeartbeat(writer http.ResponseWriter, request *http.Request) {
	var payload heartbeatWakePayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.automation.WakeHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), automationsvc.HeartbeatWakeRequest{
		Mode: payload.Mode,
		Text: payload.Text,
	})
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}
