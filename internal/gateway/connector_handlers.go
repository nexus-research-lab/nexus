// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：connector_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"net/http"
	"strings"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/connectors"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListConnectors(writer http.ResponseWriter, request *http.Request) {
	items, err := s.connectors.ListConnectors(
		request.Context(),
		request.URL.Query().Get("q"),
		request.URL.Query().Get("category"),
		request.URL.Query().Get("status"),
	)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleConnectorCategories(writer http.ResponseWriter, request *http.Request) {
	s.writeSuccess(writer, s.connectors.GetCategories())
}

func (s *Server) handleConnectorCount(writer http.ResponseWriter, request *http.Request) {
	count, err := s.connectors.GetConnectedCount(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"count": count})
}

func (s *Server) handleConnectorDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.GetConnectorDetail(request.Context(), chi.URLParam(request, "connector_id"))
	if strings.Contains(strings.ToLower(errString(err)), "not found") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectorAuthURL(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.GetAuthURL(request.Context(), chi.URLParam(request, "connector_id"), request.URL.Query().Get("redirect_uri"))
	if strings.Contains(strings.ToLower(errString(err)), "未知连接器") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectorOAuthCallback(writer http.ResponseWriter, request *http.Request) {
	var payload connectorsvc.OAuthCallbackRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.connectors.CompleteOAuthCallback(request.Context(), payload)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectConnector(writer http.ResponseWriter, request *http.Request) {
	var payload map[string]string
	if !s.bindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	item, err := s.connectors.Connect(request.Context(), chi.URLParam(request, "connector_id"), payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDisconnectConnector(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.Disconnect(request.Context(), chi.URLParam(request, "connector_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}
