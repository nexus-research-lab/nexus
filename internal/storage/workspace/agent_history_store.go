// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：agent_history_store.go
// @Date   ：2026/04/19 16:24:00
// @Author ：leemysw
// 2026/04/19 16:24:00   Create
// =====================================================

package workspace

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
	"github.com/nexus-research-lab/nexus/internal/message"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"golang.org/x/text/unicode/norm"
)

const (
	maxTranscriptCacheEntries     = 12
	maxTranscriptSanitizedLength  = 200
	transcriptScannerBufferBytes  = 16 * 1024 * 1024
	transcriptReadBufferBytes     = 64 * 1024
	transcriptSessionSearchTimout = 5 * time.Second
	overlayKindField              = "nexus_overlay_kind"
	overlayKindRoundMarker        = "round_marker"
)

var transcriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

type transcriptCacheEntry struct {
	FileSize      int64
	ModifiedUnix  int64
	LastAccessUTC int64
	Messages      []sessionmodel.Message
}

type transcriptEntry struct {
	Index int
	Data  map[string]any
}

type transcriptRoundMarker struct {
	RoundID   string
	Content   string
	Timestamp int64
}

// AgentHistoryStore 负责读取 transcript 历史，并与 Nexus overlay 合并。
type AgentHistoryStore struct {
	paths *Store
	files *SessionFileStore

	cacheMu      sync.RWMutex
	messageCache map[string]transcriptCacheEntry
}

// NewAgentHistoryStore 创建 DM 历史读写门面。
func NewAgentHistoryStore(root string) *AgentHistoryStore {
	return &AgentHistoryStore{
		paths:        New(root),
		files:        NewSessionFileStore(root),
		messageCache: make(map[string]transcriptCacheEntry),
	}
}

// DeleteTranscriptSession 删除单个 Claude transcript 文件。
func (s *AgentHistoryStore) DeleteTranscriptSession(workspacePath string, sessionID string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" {
		return false, nil
	}

	transcriptPath, err := s.resolveTranscriptPath(workspacePath, sessionID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := os.Remove(transcriptPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	s.invalidateTranscriptCache(transcriptPath)
	if err := removeDirectoryIfEmpty(filepath.Dir(transcriptPath)); err != nil {
		return true, err
	}
	return true, nil
}

// DeleteTranscriptProject 删除整个 workspace 对应的 transcript 项目目录。
func (s *AgentHistoryStore) DeleteTranscriptProject(workspacePath string) (bool, error) {
	projectDir := findTranscriptProjectDir(canonicalizeTranscriptPath(workspacePath))
	if strings.TrimSpace(projectDir) == "" {
		return false, nil
	}
	if _, err := os.Stat(projectDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if err := os.RemoveAll(projectDir); err != nil {
		return false, err
	}
	s.invalidateTranscriptCachePrefix(projectDir)
	return true, nil
}

// AppendOverlayMessage 追加一条 Nexus overlay 消息。
func (s *AgentHistoryStore) AppendOverlayMessage(workspacePath string, sessionKey string, message sessionmodel.Message) error {
	return s.files.appendJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey), message)
}

// AppendRoundMarker 记录一条 transcript round 对齐标记。
func (s *AgentHistoryStore) AppendRoundMarker(
	workspacePath string,
	sessionKey string,
	roundID string,
	content string,
	timestamp int64,
) error {
	return s.files.appendJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey), map[string]any{
		overlayKindField: overlayKindRoundMarker,
		"round_id":       strings.TrimSpace(roundID),
		"content":        strings.TrimSpace(content),
		"timestamp":      timestamp,
	})
}

// ReadMessages 读取 DM 历史。
func (s *AgentHistoryStore) ReadMessages(
	workspacePath string,
	sessionValue sessionmodel.Session,
	activeRoundIDs []string,
) ([]sessionmodel.Message, error) {
	rows, err := s.readHistoryRows(workspacePath, sessionValue)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs)), nil
}

// ReadMessagesPage 按 round 分页读取 DM 历史。
func (s *AgentHistoryStore) ReadMessagesPage(
	workspacePath string,
	sessionValue sessionmodel.Session,
	activeRoundIDs []string,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) (sessionmodel.MessagePage, error) {
	rows, err := s.readHistoryRows(workspacePath, sessionValue)
	if err != nil {
		return sessionmodel.MessagePage{}, err
	}
	normalizedRows := normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs))
	return paginateNormalizedHistoryRows(
		normalizedRows,
		limit,
		beforeRoundID,
		beforeRoundTimestamp,
		false,
	), nil
}

func (s *AgentHistoryStore) readHistoryRows(
	workspacePath string,
	sessionValue sessionmodel.Session,
) ([]sessionmodel.Message, error) {
	if err := sessionmodel.EnsureTranscriptHistory(sessionValue.Options, sessionValue.SessionKey); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(stringPointerValue(sessionValue.SessionID))
	overlayRows, roundMarkers, err := s.readOverlayRowsAndMarkers(workspacePath, sessionValue.SessionKey)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		return buildOverlayOnlyHistoryRows(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayRows,
			roundMarkers,
		), nil
	}

	transcriptRows, err := s.readTranscriptMessages(
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		sessionID,
		roundMarkers,
	)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// transcript 文件尚未出现时，只返回当前 overlay/round marker。
			return buildOverlayOnlyHistoryRows(
				sessionValue.SessionKey,
				sessionValue.AgentID,
				overlayRows,
				roundMarkers,
			), nil
		}
		return nil, err
	}

	return mergeTranscriptAndOverlayRows(transcriptRows, overlayRows), nil
}

func (s *AgentHistoryStore) readOverlayRowsAndMarkers(
	workspacePath string,
	sessionKey string,
) ([]sessionmodel.Message, []transcriptRoundMarker, error) {
	rows, err := s.files.readJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey))
	if errors.Is(err, os.ErrNotExist) {
		return []sessionmodel.Message{}, []transcriptRoundMarker{}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	messageRows := make([]sessionmodel.Message, 0, len(rows))
	roundMarkers := make([]transcriptRoundMarker, 0)
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row[overlayKindField])) == overlayKindRoundMarker {
			roundMarkers = append(roundMarkers, transcriptRoundMarker{
				RoundID:   strings.TrimSpace(stringFromAny(row["round_id"])),
				Content:   strings.TrimSpace(stringFromAny(row["content"])),
				Timestamp: messageTimestamp(sessionmodel.Message(row)),
			})
			continue
		}
		messageRows = append(messageRows, sessionmodel.Message(row))
	}
	return messageRows, roundMarkers, nil
}

func buildOverlayOnlyHistoryRows(
	sessionKey string,
	agentID string,
	overlayRows []sessionmodel.Message,
	roundMarkers []transcriptRoundMarker,
) []sessionmodel.Message {
	markerRows := materializeRoundMarkerMessages(sessionKey, agentID, roundMarkers)
	combined := make([]sessionmodel.Message, 0, len(markerRows)+len(overlayRows))
	combined = append(combined, markerRows...)
	combined = append(combined, overlayRows...)
	return combined
}

func mergeTranscriptAndOverlayRows(
	transcriptRows []sessionmodel.Message,
	overlayRows []sessionmodel.Message,
) []sessionmodel.Message {
	combined := make([]sessionmodel.Message, 0, len(transcriptRows)+len(overlayRows))
	combined = append(combined, transcriptRows...)
	combined = append(combined, overlayRows...)
	return combined
}

func materializeRoundMarkerMessages(
	sessionKey string,
	agentID string,
	roundMarkers []transcriptRoundMarker,
) []sessionmodel.Message {
	if len(roundMarkers) == 0 {
		return []sessionmodel.Message{}
	}

	rows := make([]sessionmodel.Message, 0, len(roundMarkers))
	for _, marker := range roundMarkers {
		roundID := strings.TrimSpace(marker.RoundID)
		if roundID == "" {
			continue
		}
		rows = append(rows, sessionmodel.Message{
			"message_id":  roundID,
			"session_key": sessionKey,
			"agent_id":    strings.TrimSpace(agentID),
			"round_id":    roundID,
			"role":        "user",
			"content":     strings.TrimSpace(marker.Content),
			"timestamp":   marker.Timestamp,
		})
	}
	return rows
}

func (s *AgentHistoryStore) readTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
) ([]sessionmodel.Message, error) {
	transcriptPath, err := s.resolveTranscriptPath(workspacePath, sessionID)
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, err
	}

	if cachedRows, ok := s.readTranscriptCache(transcriptPath, fileInfo); ok {
		return cachedRows, nil
	}

	entries, err := s.readTranscriptEntries(transcriptPath)
	if err != nil {
		return nil, err
	}
	chain := buildPrimaryTranscriptChain(entries)
	projectedRows := projectTranscriptChain(sessionKey, agentID, chain, roundMarkers)
	s.writeTranscriptCache(transcriptPath, fileInfo, projectedRows)
	return projectedRows, nil
}

func (s *AgentHistoryStore) readTranscriptEntries(path string) ([]transcriptEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewScanner(file)
	reader.Buffer(make([]byte, 0, transcriptReadBufferBytes), transcriptScannerBufferBytes)

	results := make([]transcriptEntry, 0)
	for index := 0; reader.Scan(); index++ {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		normalizeTranscriptEntryShape(entry)
		if strings.TrimSpace(stringFromAny(entry["uuid"])) == "" {
			continue
		}
		results = append(results, transcriptEntry{
			Index: index,
			Data:  entry,
		})
	}
	return results, reader.Err()
}

func normalizeTranscriptEntryShape(entry map[string]any) {
	// 兼容 transcript 历史里沿用的 camelCase 字段，统一桥接到当前 SDK 解码需要的 snake_case。
	if entry["session_id"] == nil && entry["sessionId"] != nil {
		entry["session_id"] = entry["sessionId"]
	}

	if strings.TrimSpace(stringFromAny(entry["type"])) != "assistant" {
		return
	}
	messageValue, ok := entry["message"].(map[string]any)
	if !ok {
		return
	}
	if strings.TrimSpace(stringFromAny(messageValue["id"])) != "" {
		return
	}
	if uuid := strings.TrimSpace(stringFromAny(entry["uuid"])); uuid != "" {
		messageValue["id"] = uuid
	}
}

func (s *AgentHistoryStore) resolveTranscriptPath(workspacePath string, sessionID string) (string, error) {
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectDir := findTranscriptProjectDir(canonicalPath)
	if projectDir != "" {
		path := filepath.Join(projectDir, sessionID+".jsonl")
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}

	for _, worktreePath := range listTranscriptWorktreePaths(canonicalPath) {
		if worktreePath == canonicalPath {
			continue
		}
		worktreeDir := findTranscriptProjectDir(worktreePath)
		if worktreeDir == "" {
			continue
		}
		path := filepath.Join(worktreeDir, sessionID+".jsonl")
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func (s *AgentHistoryStore) readTranscriptCache(path string, fileInfo os.FileInfo) ([]sessionmodel.Message, bool) {
	s.cacheMu.RLock()
	entry, exists := s.messageCache[path]
	s.cacheMu.RUnlock()
	if !exists {
		return nil, false
	}
	if entry.FileSize != fileInfo.Size() || entry.ModifiedUnix != fileInfo.ModTime().UnixNano() {
		return nil, false
	}

	s.cacheMu.Lock()
	refreshedEntry := s.messageCache[path]
	refreshedEntry.LastAccessUTC = time.Now().UTC().UnixNano()
	s.messageCache[path] = refreshedEntry
	s.cacheMu.Unlock()
	return entry.Messages, true
}

func (s *AgentHistoryStore) writeTranscriptCache(
	path string,
	fileInfo os.FileInfo,
	rows []sessionmodel.Message,
) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.messageCache[path] = transcriptCacheEntry{
		FileSize:      fileInfo.Size(),
		ModifiedUnix:  fileInfo.ModTime().UnixNano(),
		LastAccessUTC: time.Now().UTC().UnixNano(),
		Messages:      rows,
	}
	s.pruneTranscriptCacheLocked()
}

func (s *AgentHistoryStore) pruneTranscriptCacheLocked() {
	if len(s.messageCache) <= maxTranscriptCacheEntries {
		return
	}

	type cacheCandidate struct {
		Path          string
		LastAccessUTC int64
	}

	candidates := make([]cacheCandidate, 0, len(s.messageCache))
	for path, entry := range s.messageCache {
		candidates = append(candidates, cacheCandidate{
			Path:          path,
			LastAccessUTC: entry.LastAccessUTC,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].LastAccessUTC < candidates[j].LastAccessUTC
	})
	for len(candidates) > maxTranscriptCacheEntries {
		delete(s.messageCache, candidates[0].Path)
		candidates = candidates[1:]
	}
}

func (s *AgentHistoryStore) invalidateTranscriptCache(path string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	delete(s.messageCache, path)
}

func (s *AgentHistoryStore) invalidateTranscriptCachePrefix(prefix string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for path := range s.messageCache {
		if path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator)) {
			delete(s.messageCache, path)
		}
	}
}

func removeDirectoryIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(path)
}

func buildPrimaryTranscriptChain(entries []transcriptEntry) []transcriptEntry {
	if len(entries) == 0 {
		return nil
	}

	byUUID := make(map[string]transcriptEntry, len(entries))
	parentUUIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		uuid := strings.TrimSpace(stringFromAny(entry.Data["uuid"]))
		if uuid == "" {
			continue
		}
		byUUID[uuid] = entry
		parentUUID := strings.TrimSpace(stringFromAny(entry.Data["parentUuid"]))
		if parentUUID != "" {
			parentUUIDs[parentUUID] = struct{}{}
		}
	}

	terminals := make([]transcriptEntry, 0)
	for _, entry := range entries {
		uuid := strings.TrimSpace(stringFromAny(entry.Data["uuid"]))
		if uuid == "" {
			continue
		}
		if _, exists := parentUUIDs[uuid]; exists {
			continue
		}
		if shouldSkipTranscriptEntry(entry.Data) {
			continue
		}
		terminals = append(terminals, entry)
	}
	if len(terminals) == 0 {
		return nil
	}

	sort.Slice(terminals, func(i int, j int) bool {
		return terminals[i].Index > terminals[j].Index
	})

	leaf := terminals[0]
	chain := make([]transcriptEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	current := leaf
	for {
		uuid := strings.TrimSpace(stringFromAny(current.Data["uuid"]))
		if uuid == "" {
			break
		}
		if _, exists := seen[uuid]; exists {
			break
		}
		seen[uuid] = struct{}{}
		chain = append(chain, current)
		parentUUID := strings.TrimSpace(stringFromAny(current.Data["parentUuid"]))
		if parentUUID == "" {
			break
		}
		parent, exists := byUUID[parentUUID]
		if !exists {
			break
		}
		current = parent
	}

	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return chain
}

func shouldSkipTranscriptEntry(entry map[string]any) bool {
	if boolValueAny(entry["isSidechain"]) || boolValueAny(entry["isMeta"]) {
		return true
	}
	return strings.TrimSpace(stringFromAny(entry["teamName"])) != ""
}

func projectTranscriptChain(
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []sessionmodel.Message {
	projected := make([]sessionmodel.Message, 0, len(chain))
	currentRoundID := ""
	var processor *message.Processor
	var lastTimestamp int64
	alignedMarkers := alignTranscriptRoundMarkers(chain, roundMarkers)
	markerIndex := 0

	for _, entry := range chain {
		if shouldSkipTranscriptEntry(entry.Data) {
			continue
		}

		decoded, err := sdkprotocol.DecodeMessage(entry.Data)
		if err != nil {
			continue
		}
		entryTimestamp := transcriptEntryTimestamp(entry.Data, entry.Index, lastTimestamp)
		lastTimestamp = entryTimestamp

		switch decoded.Type {
		case sdkprotocol.MessageTypeUser:
			if isTranscriptToolResult(decoded) {
				if processor == nil {
					currentRoundID = firstNonEmpty(strings.TrimSpace(stringFromAny(entry.Data["parentUuid"])), strings.TrimSpace(decoded.UUID))
					processor = newTranscriptProcessor(sessionKey, agentID, currentRoundID, decoded.SessionID)
				}
				output := processor.Process(decoded)
				projected = append(projected, stampTranscriptDurableMessages(output.DurableMessages, entryTimestamp)...)
				continue
			}
			// 中文注释：Claude transcript 会夹杂一类“空 user turn”，
			// 它们不是前端真实输入，不能消费 Nexus 的 round marker。
			// 否则后续真实 assistant 会挂错 round，result 也就无法并回同一轮。
			if !shouldMaterializeTranscriptUserTurn(entry.Data) {
				continue
			}
			marker := consumeTranscriptRoundMarker(alignedMarkers, &markerIndex)
			currentRoundID = firstNonEmpty(marker.RoundID, buildTranscriptRoundID(decoded.UUID))
			processor = newTranscriptProcessor(sessionKey, agentID, currentRoundID, decoded.SessionID)
			userMessage := buildTranscriptUserMessage(
				sessionKey,
				agentID,
				currentRoundID,
				decoded.SessionID,
				entry.Data,
				marker.Content,
				entryTimestamp,
			)
			if userMessage == nil {
				continue
			}
			projected = append(projected, *userMessage)
		case sdkprotocol.MessageTypeAssistant,
			sdkprotocol.MessageTypeSystem,
			sdkprotocol.MessageTypeToolProgress:
			if processor == nil {
				currentRoundID = buildTranscriptRoundID(decoded.UUID)
				processor = newTranscriptProcessor(sessionKey, agentID, currentRoundID, decoded.SessionID)
			}
			output := processor.Process(decoded)
			projected = append(projected, stampTranscriptDurableMessages(output.DurableMessages, entryTimestamp)...)
		case sdkprotocol.MessageTypeResult:
			// result 统一以 Nexus overlay 为真相源。
			// transcript 即使带了 result，也不再直接投影进历史，
			// 避免 assistant/usage 与 runtime result 语义重新混在一起。
			continue
		default:
			continue
		}
	}

	return projected
}

func alignTranscriptRoundMarkers(
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []transcriptRoundMarker {
	if len(roundMarkers) == 0 {
		return nil
	}
	// transcript 只保留当前主链末端的用户轮次，而 overlay 可能保留了更早的 round marker。
	// 这里必须按尾部对齐，保证最新 transcript user 绑定到最新 round marker，
	// 不能从头部顺序消费，否则会把旧轮次用户输入错绑到新的 assistant 回复上。
	transcriptUserCount := countTranscriptUserTurns(chain)
	if transcriptUserCount <= 0 {
		return nil
	}
	if transcriptUserCount >= len(roundMarkers) {
		return append([]transcriptRoundMarker(nil), roundMarkers...)
	}
	startIndex := len(roundMarkers) - transcriptUserCount
	return append([]transcriptRoundMarker(nil), roundMarkers[startIndex:]...)
}

func countTranscriptUserTurns(chain []transcriptEntry) int {
	count := 0
	for _, entry := range chain {
		decoded, err := sdkprotocol.DecodeMessage(entry.Data)
		if err != nil {
			continue
		}
		if decoded.Type == sdkprotocol.MessageTypeUser &&
			!isTranscriptToolResult(decoded) &&
			shouldMaterializeTranscriptUserTurn(entry.Data) {
			count++
		}
	}
	return count
}

func shouldMaterializeTranscriptUserTurn(entry map[string]any) bool {
	return sanitizeTranscriptUserContent(transcriptUserContent(entry)) != ""
}

func consumeTranscriptRoundMarker(markers []transcriptRoundMarker, index *int) transcriptRoundMarker {
	if index == nil {
		return transcriptRoundMarker{}
	}
	for *index < len(markers) {
		marker := markers[*index]
		*index++
		if strings.TrimSpace(marker.RoundID) != "" || strings.TrimSpace(marker.Content) != "" {
			return marker
		}
	}
	return transcriptRoundMarker{}
}

func newTranscriptProcessor(
	sessionKey string,
	agentID string,
	roundID string,
	sessionID string,
) *message.Processor {
	return message.NewProcessor(message.MessageContext{
		SessionKey: sessionKey,
		AgentID:    agentID,
		RoundID:    roundID,
		ParentID:   roundID,
	}, strings.TrimSpace(sessionID))
}

func buildTranscriptUserMessage(
	sessionKey string,
	agentID string,
	roundID string,
	sessionID string,
	entry map[string]any,
	contentOverride string,
	timestamp int64,
) *sessionmodel.Message {
	content := firstNonEmpty(strings.TrimSpace(contentOverride), transcriptUserContent(entry))
	if content == "" {
		return nil
	}
	payload := sessionmodel.Message{
		"message_id":  roundID,
		"session_key": sessionKey,
		"agent_id":    agentID,
		"round_id":    roundID,
		"role":        "user",
		"content":     content,
		"timestamp":   timestamp,
	}
	if strings.TrimSpace(sessionID) != "" {
		payload["session_id"] = strings.TrimSpace(sessionID)
	}
	return &payload
}

func transcriptUserContent(entry map[string]any) string {
	messageValue, _ := entry["message"].(map[string]any)
	contentValue := messageValue["content"]
	if text := sanitizeTranscriptUserContent(strings.TrimSpace(stringFromAny(contentValue))); text != "" {
		return text
	}
	items, _ := contentValue.([]any)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := sanitizeTranscriptUserContent(strings.TrimSpace(stringFromAny(payload["text"]))); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func sanitizeTranscriptUserContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if sessionmodel.IsInternalTranscriptInterruptPrompt(trimmed) {
		return ""
	}
	return trimmed
}

func stampTranscriptDurableMessages(
	rows []sessionmodel.Message,
	timestamp int64,
) []sessionmodel.Message {
	if len(rows) == 0 {
		return nil
	}
	result := make([]sessionmodel.Message, 0, len(rows))
	for _, row := range rows {
		stamped := cloneMessage(row)
		stamped["timestamp"] = timestamp
		result = append(result, stamped)
	}
	return result
}

func isTranscriptToolResult(message sdkprotocol.ReceivedMessage) bool {
	if message.User == nil {
		return false
	}
	if message.User.ToolUseResult != nil {
		return true
	}
	for _, block := range message.User.Message.Content {
		blockType := strings.TrimSpace(string(block.Type()))
		if blockType == "tool_result" || blockType == "server_tool_result" {
			return true
		}
	}
	return false
}

func buildTranscriptRoundID(uuid string) string {
	trimmed := strings.TrimSpace(uuid)
	if trimmed == "" {
		return "transcript_round"
	}
	return trimmed
}

func transcriptEntryTimestamp(entry map[string]any, index int, lastTimestamp int64) int64 {
	value := strings.TrimSpace(stringFromAny(entry["timestamp"]))
	if value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed.UnixMilli()
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed.UnixMilli()
		}
	}
	if lastTimestamp > 0 {
		return lastTimestamp + 1
	}
	return int64(index + 1)
}

func transcriptConfigHomeDir() string {
	if value := strings.TrimSpace(os.Getenv("NEXUS_CONFIG_DIR")); value != "" {
		return norm.NFC.String(value)
	}
	if value := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); value != "" {
		return norm.NFC.String(value)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return norm.NFC.String(filepath.Join(".", ".nexus"))
	}
	return norm.NFC.String(filepath.Join(homeDir, ".nexus"))
}

func transcriptProjectsDir() string {
	return filepath.Join(transcriptConfigHomeDir(), "projects")
}

func canonicalizeTranscriptPath(path string) string {
	if path == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = path
	}
	resolved, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		resolved = absolutePath
	}
	return norm.NFC.String(resolved)
}

func findTranscriptProjectDir(projectPath string) string {
	exact := filepath.Join(transcriptProjectsDir(), sanitizeTranscriptPath(projectPath))
	if isDirectory(exact) {
		return exact
	}
	sanitized := sanitizeTranscriptPath(projectPath)
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return ""
	}
	prefix := sanitized[:maxTranscriptSanitizedLength]
	for _, entry := range readDirectories(transcriptProjectsDir()) {
		if strings.HasPrefix(filepath.Base(entry), prefix+"-") {
			return entry
		}
	}
	return ""
}

func listTranscriptWorktreePaths(cwd string) []string {
	if strings.TrimSpace(cwd) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), transcriptSessionSearchTimout)
	defer cancel()

	command := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	command.Dir = cwd
	output, err := command.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		results = append(results, norm.NFC.String(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))))
	}
	return results
}

func sanitizeTranscriptPath(path string) string {
	sanitized := transcriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return sanitized
	}
	return sanitized[:maxTranscriptSanitizedLength] + "-" + simpleTranscriptHash(path)
}

func simpleTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := []byte{}
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func readDirectories(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	results := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, filepath.Join(root, entry.Name()))
		}
	}
	return results
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boolValueAny(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}
