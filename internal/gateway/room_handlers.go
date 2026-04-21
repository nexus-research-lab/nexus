// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：room_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/launcher"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	room2 "github.com/nexus-research-lab/nexus/internal/room"
	session "github.com/nexus-research-lab/nexus/internal/session"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListRooms(writer http.ResponseWriter, request *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := s.roomService.ListRooms(request.Context(), limit)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleGetRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := s.roomService.GetRoom(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleGetRoomContexts(writer http.ResponseWriter, request *http.Request) {
	items, err := s.roomService.GetRoomContexts(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleConversationMessages(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	limit := 0
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit <= 0 {
			s.writeFailure(writer, http.StatusBadRequest, "limit 参数错误")
			return
		}
		limit = parsedLimit
	}
	beforeRoundID := strings.TrimSpace(request.URL.Query().Get("before_round_id"))
	beforeRoundTimestamp := int64(0)
	if rawBeforeTimestamp := strings.TrimSpace(request.URL.Query().Get("before_round_timestamp")); rawBeforeTimestamp != "" {
		parsedBeforeTimestamp, parseErr := strconv.ParseInt(rawBeforeTimestamp, 10, 64)
		if parseErr != nil || parsedBeforeTimestamp <= 0 {
			s.writeFailure(writer, http.StatusBadRequest, "before_round_timestamp 参数错误")
			return
		}
		beforeRoundTimestamp = parsedBeforeTimestamp
	}

	contextValue, err := s.roomService.GetConversationContext(request.Context(), conversationID)
	if errors.Is(err, room2.ErrConversationNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if contextValue.Room.ID != roomID {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}

	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	if contextValue.Room.RoomType == room2.RoomTypeDM {
		primarySession := findPrimaryConversationSession(contextValue.Sessions)
		if primarySession == nil || strings.TrimSpace(primarySession.AgentID) == "" {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		sessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			strings.TrimSpace(primarySession.AgentID),
			room2.RoomTypeDM,
		)
	}

	items, err := s.session.GetSessionMessagesPage(request.Context(), sessionKey, session.MessagePageRequest{
		Limit:                limit,
		BeforeRoundID:        beforeRoundID,
		BeforeRoundTimestamp: beforeRoundTimestamp,
	})
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleCreateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload room2.CreateRoomRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.roomService.CreateRoom(request.Context(), payload)
	if errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func findPrimaryConversationSession(sessions []room2.SessionRecord) *room2.SessionRecord {
	for index := range sessions {
		if sessions[index].IsPrimary {
			return &sessions[index]
		}
	}
	if len(sessions) == 0 {
		return nil
	}
	return &sessions[0]
}

func (s *Server) handleUpdateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload room2.UpdateRoomRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.roomService.UpdateRoom(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "room_updated")
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteRoom(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptRoom(request.Context(), roomID, "room 已删除")
	}
	err := s.roomService.DeleteRoom(request.Context(), roomID)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), roomID, protocol.EventTypeRoomDeleted, map[string]any{
		"room_id": roomID,
	})
	if s.roomSubs != nil {
		s.roomSubs.RemoveRoom(roomID)
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleEnsureDirectRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := s.roomService.EnsureDirectRoom(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleAddRoomMember(writer http.ResponseWriter, request *http.Request) {
	var payload room2.AddRoomMemberRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.roomService.AddRoomMember(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberAdded, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": payload.AgentID,
	})
	s.writeSuccess(writer, item)
}

func (s *Server) handleRemoveRoomMember(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	agentID := chi.URLParam(request, "agent_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptAgentTasks(request.Context(), roomID, agentID, "成员已移出 room")
	}
	item, err := s.roomService.RemoveRoomMember(request.Context(), roomID, agentID)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrRoomMemberNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberRemoved, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": agentID,
	})
	s.writeSuccess(writer, item)
}

func (s *Server) handleCreateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload room2.CreateConversationRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.roomService.CreateConversation(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_created")
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload room2.UpdateConversationRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.roomService.UpdateConversation(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		payload,
	)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrConversationNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_updated")
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteConversation(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptConversation(request.Context(), conversationID, "对话已删除")
	}
	item, err := s.roomService.DeleteConversation(
		request.Context(),
		roomID,
		conversationID,
	)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrConversationNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), roomID, conversationID, "conversation_deleted")
	s.writeSuccess(writer, item)
}

func (s *Server) handleLauncherQuery(writer http.ResponseWriter, request *http.Request) {
	var payload launcher.QueryRequest
	if !s.bindJSON(writer, request, &payload) {
		return
	}
	item, err := s.launcher.Query(request.Context(), payload.Query)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleLauncherSuggestions(writer http.ResponseWriter, request *http.Request) {
	item, err := s.launcher.Suggestions(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleLauncherBootstrap(writer http.ResponseWriter, request *http.Request) {
	item, err := s.launcher.Bootstrap(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}
