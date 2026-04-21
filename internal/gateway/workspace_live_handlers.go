// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：workspace_live_handlers.go
// @Date   ：2026/04/12 11:49:00
// @Author ：leemysw
// 2026/04/12 11:49:00   Create
// =====================================================

package gateway

import (
	"context"
	"errors"
	"strings"
)

func (s *Server) handleSubscribeWorkspace(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	agentID := strings.TrimSpace(stringValue(inbound["agent_id"]))
	if agentID == "" {
		s.sendGatewayError(
			ctx,
			sender,
			"",
			"invalid_workspace_subscription",
			errors.New("agent_id is required"),
			map[string]any{
				"type": "subscribe_workspace",
			},
		)
		return
	}
	if s.workspaceSubs == nil {
		return
	}
	if err := s.workspaceSubs.Subscribe(ctx, sender, agentID); err != nil {
		s.sendGatewayError(ctx, sender, "", "workspace_subscription_error", err, map[string]any{
			"type":     "subscribe_workspace",
			"agent_id": agentID,
		})
	}
}

func (s *Server) handleUnsubscribeWorkspace(sender *websocketSender, inbound map[string]any) {
	if s.workspaceSubs == nil {
		return
	}
	agentID := strings.TrimSpace(stringValue(inbound["agent_id"]))
	if agentID == "" {
		return
	}
	s.workspaceSubs.Unsubscribe(sender, agentID)
}
