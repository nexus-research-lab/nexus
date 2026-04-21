// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：channel_handlers.go
// @Date   ：2026/04/12 00:13:00
// @Author ：leemysw
// 2026/04/12 00:13:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/channels"
)

func (s *Server) handleChannelIngress(writer http.ResponseWriter, request *http.Request) {
	s.handleChannelIngressByName(writer, request, "")
}

func (s *Server) handleInternalChannelIngress(writer http.ResponseWriter, request *http.Request) {
	s.handleChannelIngressByName(writer, request, channels.ChannelTypeInternal)
}

func (s *Server) handleDiscordChannelIngress(writer http.ResponseWriter, request *http.Request) {
	s.handleChannelIngressByName(writer, request, channels.ChannelTypeDiscord)
}

func (s *Server) handleTelegramChannelIngress(writer http.ResponseWriter, request *http.Request) {
	s.handleChannelIngressByName(writer, request, channels.ChannelTypeTelegram)
}

func (s *Server) handleChannelIngressByName(
	writer http.ResponseWriter,
	request *http.Request,
	channelName string,
) {
	if s.ingress == nil {
		s.writeFailure(writer, http.StatusServiceUnavailable, "channel ingress is not configured")
		return
	}

	var payload channels.IngressRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	if strings.TrimSpace(channelName) != "" {
		payload.Channel = channelName
	}

	result, err := s.ingress.Accept(request.Context(), payload)
	if err != nil {
		if isChannelIngressClientError(err) || isStructuredSessionKeyError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, result)
}

func isChannelIngressClientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, channels.ErrIngressChannelRequired) || errors.Is(err, channels.ErrIngressRefRequired) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "content is required") ||
		strings.Contains(message, "agent_id 与 session_key 不一致") ||
		strings.Contains(message, "channel 与 session_key 不一致") ||
		strings.Contains(message, "仅支持 agent session_key") ||
		strings.Contains(message, "requires")
}
