// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：provider_handlers.go
// @Date   ：2026/04/16 20:48:00
// @Author ：leemysw
// 2026/04/16 20:48:00   Create
// =====================================================

package gateway

import (
	"net/http"
	"strings"

	providercfg "github.com/nexus-research-lab/nexus/internal/provider"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListProviderConfigs(writer http.ResponseWriter, request *http.Request) {
	items, err := s.providers.List(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleListProviderOptions(writer http.ResponseWriter, request *http.Request) {
	item, err := s.providers.ListOptions(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleCreateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CreateInput
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.providers.Create(request.Context(), payload)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.UpdateInput
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.providers.Update(request.Context(), chi.URLParam(request, "provider"), payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteProviderConfig(writer http.ResponseWriter, request *http.Request) {
	provider := chi.URLParam(request, "provider")
	if err := s.providers.Delete(request.Context(), provider); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"provider": provider})
}
