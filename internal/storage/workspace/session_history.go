// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_history.go
// @Date   ：2026/04/16 22:00:00
// @Author ：leemysw
// 2026/04/16 22:00:00   Create
// =====================================================

package workspace

import (
	"sort"
	"strings"
	"time"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

type roundTerminalStatus string

const (
	roundStatusRunning     roundTerminalStatus = "running"
	roundStatusSuccess     roundTerminalStatus = "success"
	roundStatusInterrupted roundTerminalStatus = "interrupted"
	roundStatusError       roundTerminalStatus = "error"
)

func normalizeHistoryRows(rows []sessionmodel.Message, activeRoundIDs map[string]struct{}) []sessionmodel.Message {
	compacted := compactMessages(rows)
	materialized := materializeUnfinishedRounds(compacted, activeRoundIDs)
	roundStatus := buildRoundStatus(materialized)
	return normalizeAssistantRows(materialized, roundStatus)
}

func buildRoundStatus(rows []sessionmodel.Message) map[string]roundTerminalStatus {
	statusMap := make(map[string]roundTerminalStatus)
	for _, row := range rows {
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == "" {
			continue
		}
		if _, exists := statusMap[roundID]; !exists {
			statusMap[roundID] = roundStatusRunning
		}
		if strings.TrimSpace(stringFromAny(row["role"])) != "result" {
			continue
		}
		statusMap[roundID] = normalizeRoundStatusValue(row["subtype"])
	}
	return statusMap
}

func normalizeAssistantRows(rows []sessionmodel.Message, statusMap map[string]roundTerminalStatus) []sessionmodel.Message {
	result := make([]sessionmodel.Message, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row["role"])) != "assistant" {
			result = append(result, cloneMessage(row))
			continue
		}
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		assistantStatus := resolveAssistantStatus(statusMap[roundID])
		normalized := cloneMessage(row)
		if assistantStatus != "" {
			normalized["is_complete"] = true
			currentStatus := strings.TrimSpace(stringFromAny(normalized["stream_status"]))
			if currentStatus == "" || currentStatus == "streaming" || currentStatus == "pending" {
				normalized["stream_status"] = assistantStatus
			}
		}
		result = append(result, normalized)
	}
	return result
}

func materializeUnfinishedRounds(rows []sessionmodel.Message, activeRoundIDs map[string]struct{}) []sessionmodel.Message {
	if len(rows) == 0 {
		return rows
	}
	type roundSnapshot struct {
		RoundID         string
		SessionKey      string
		RoomID          string
		ConversationID  string
		AgentID         string
		SessionID       string
		ParentID        string
		LastTimestampMS int64
		HasResult       bool
	}

	rounds := make(map[string]*roundSnapshot)
	for _, row := range rows {
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == "" {
			continue
		}
		snapshot := rounds[roundID]
		if snapshot == nil {
			snapshot = &roundSnapshot{RoundID: roundID}
			rounds[roundID] = snapshot
		}
		snapshot.SessionKey = firstNonEmpty(snapshot.SessionKey, stringFromAny(row["session_key"]))
		snapshot.RoomID = firstNonEmpty(snapshot.RoomID, stringFromAny(row["room_id"]))
		snapshot.ConversationID = firstNonEmpty(snapshot.ConversationID, stringFromAny(row["conversation_id"]))
		snapshot.AgentID = firstNonEmpty(snapshot.AgentID, stringFromAny(row["agent_id"]))
		snapshot.SessionID = firstNonEmpty(snapshot.SessionID, stringFromAny(row["session_id"]))
		snapshot.ParentID = firstNonEmpty(snapshot.ParentID, stringFromAny(row["parent_id"]))
		if ts := messageTimestamp(row); ts > snapshot.LastTimestampMS {
			snapshot.LastTimestampMS = ts
		}
		if strings.TrimSpace(stringFromAny(row["role"])) == "result" {
			snapshot.HasResult = true
		}
	}

	result := make([]sessionmodel.Message, 0, len(rows)+len(rounds))
	result = append(result, rows...)
	for roundID, snapshot := range rounds {
		if snapshot == nil || snapshot.HasResult {
			continue
		}
		if _, isActive := activeRoundIDs[roundID]; isActive {
			continue
		}
		timestamp := snapshot.LastTimestampMS + 1
		if timestamp <= 0 {
			timestamp = time.Now().UnixMilli()
		}
		payload := sessionmodel.Message{
			"message_id":      "result_" + roundID,
			"session_key":     snapshot.SessionKey,
			"room_id":         emptyStringToNil(snapshot.RoomID),
			"conversation_id": emptyStringToNil(snapshot.ConversationID),
			"agent_id":        snapshot.AgentID,
			"round_id":        roundID,
			"session_id":      emptyStringToNil(snapshot.SessionID),
			"role":            "result",
			"timestamp":       timestamp,
			"subtype":         "interrupted",
			"duration_ms":     0,
			"duration_api_ms": 0,
			"num_turns":       0,
			"usage":           map[string]any{},
			"result":          "任务已中断",
			"is_error":        false,
		}
		if strings.TrimSpace(snapshot.ParentID) != "" {
			payload["parent_id"] = snapshot.ParentID
		}
		result = append(result, payload)
	}

	sort.Slice(result, func(i int, j int) bool {
		return messageTimestamp(result[i]) < messageTimestamp(result[j])
	})
	return result
}

func refreshSessionMetaFromMessages(base sessionmodel.Session, rows []sessionmodel.Message) sessionmodel.Session {
	meta := base
	compacted := compactMessages(rows)
	meta.MessageCount = len(compacted)
	meta.LastActivity = meta.CreatedAt

	var latest sessionmodel.Message
	for _, row := range compacted {
		if latest == nil || messageTimestamp(row) >= messageTimestamp(latest) {
			latest = row
		}
		if sessionID := strings.TrimSpace(stringFromAny(row["session_id"])); sessionID != "" {
			meta.SessionID = stringPointer(sessionID)
		}
	}
	if latest != nil {
		meta.AgentID = firstNonEmpty(meta.AgentID, stringFromAny(latest["agent_id"]))
		if ts := messageTimestamp(latest); ts > 0 {
			meta.LastActivity = time.UnixMilli(ts).UTC()
		}
		if meta.Options == nil {
			meta.Options = map[string]any{}
		}
		if roundID := strings.TrimSpace(stringFromAny(latest["round_id"])); roundID != "" {
			meta.Options["latest_round_id"] = roundID
		}
		if role := strings.TrimSpace(stringFromAny(latest["role"])); role == "result" {
			meta.Options["latest_result_subtype"] = firstNonEmpty(stringFromAny(latest["subtype"]), "success")
		}
	}
	meta.IsActive = meta.Status == "" || meta.Status == "active"
	if meta.Status == "" {
		meta.Status = "active"
	}
	return meta
}

func normalizeActiveRoundIDs(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeRoundStatusValue(value any) roundTerminalStatus {
	normalized := strings.ToLower(strings.TrimSpace(stringFromAny(value)))
	switch normalized {
	case "", "running":
		return roundStatusRunning
	case "interrupted", "cancelled":
		return roundStatusInterrupted
	case "error":
		return roundStatusError
	default:
		return roundStatusSuccess
	}
}

func resolveAssistantStatus(status roundTerminalStatus) string {
	switch status {
	case roundStatusInterrupted:
		return "cancelled"
	case roundStatusError:
		return "error"
	case roundStatusSuccess:
		return "done"
	default:
		return ""
	}
}

func cloneMessage(message sessionmodel.Message) sessionmodel.Message {
	if len(message) == 0 {
		return sessionmodel.Message{}
	}
	result := make(sessionmodel.Message, len(message))
	for key, value := range message {
		result[key] = value
	}
	return result
}

func emptyStringToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
