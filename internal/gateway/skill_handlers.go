// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：skill_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"
	"os"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	skillsvc "github.com/nexus-research-lab/nexus/internal/skills"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := s.skills.ListSkills(request.Context(), skillsvc.Query{
		AgentID:     request.URL.Query().Get("agent_id"),
		CategoryKey: request.URL.Query().Get("category_key"),
		SourceType:  request.URL.Query().Get("source_type"),
		Q:           request.URL.Query().Get("q"),
	})
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

func (s *Server) handleGetSkillDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.GetSkillDetail(request.Context(), chi.URLParam(request, "skill_name"), request.URL.Query().Get("agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) || strings.Contains(strings.ToLower(errString(err)), "not found") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleAgentSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := s.skills.GetAgentSkills(request.Context(), chi.URLParam(request, "agent_id"))
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

func (s *Server) handleInstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		SkillName string `json:"skill_name"`
	}
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.skills.InstallSkill(request.Context(), chi.URLParam(request, "agent_id"), payload.SkillName)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(err.Error(), "仅允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUninstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	err := s.skills.UninstallSkill(request.Context(), chi.URLParam(request, "agent_id"), chi.URLParam(request, "skill_name"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleImportLocalSkill(writer http.ResponseWriter, request *http.Request) {
	filePayload, filename, localPath, err := s.parseLocalSkillImportRequest(request)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	var item *skillsvc.Detail
	if len(filePayload) > 0 {
		item, err = s.skills.ImportUploadedArchive(filename, filePayload)
	} else {
		item, err = s.skills.ImportLocalPath(localPath)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "SKILL.md") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteSkill(writer http.ResponseWriter, request *http.Request) {
	err := s.skills.DeleteSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		if strings.Contains(err.Error(), "不允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}
