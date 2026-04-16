// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：model_request.go
// @Date   ：2026/04/16 22:03:49
// @Author ：leemysw
// 2026/04/16 22:03:49   Create
// =====================================================

package room

import roommodel "github.com/nexus-research-lab/nexus/internal/model/room"

// CreateRoomRequest 表示创建房间请求。
type CreateRoomRequest = roommodel.CreateRoomRequest

// UpdateRoomRequest 表示更新房间请求。
type UpdateRoomRequest = roommodel.UpdateRoomRequest

// AddRoomMemberRequest 表示追加成员请求。
type AddRoomMemberRequest = roommodel.AddRoomMemberRequest

// CreateConversationRequest 表示创建话题请求。
type CreateConversationRequest = roommodel.CreateConversationRequest

// UpdateConversationRequest 表示更新话题请求。
type UpdateConversationRequest = roommodel.UpdateConversationRequest
