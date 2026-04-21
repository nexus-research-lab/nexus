// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：workspace_handlers.go
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
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleWorkspaceFiles(writer http.ResponseWriter, request *http.Request) {
	items, err := s.workspace.ListFiles(request.Context(), chi.URLParam(request, "agent_id"))
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

func (s *Server) handleWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	item, err := s.workspace.GetFile(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) || strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.workspace.UpdateFile(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.Content)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleCreateWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path      string `json:"path"`
		EntryType string `json:"entry_type"`
		Content   string `json:"content"`
	}
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.workspace.CreateEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.EntryType, payload.Content)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "存在") || strings.Contains(err.Error(), "仅支持") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleRenameWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.workspace.RenameEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.NewPath)
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "相同") || strings.Contains(err.Error(), "存在") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	item, err := s.workspace.DeleteEntry(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}
