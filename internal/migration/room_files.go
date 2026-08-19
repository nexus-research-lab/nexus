// INPUT: 旧 app/rooms、已完成 schema migration 的 Room 数据库与用户状态根。
// OUTPUT: 按 owner 拆分到 users/<owner>/state/rooms 的 Room 文件状态。
// POS: Room 文件从宿主共享目录迁入宿主用户状态根的一次性安全迁移。
package migration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const roomStateMigrationName = "20260727_move_room_state_to_user_state"

type conversationOwnerLookup interface {
	LookupConversationOwnerUserID(context.Context, string) (string, error)
}

type roomFileMigrationResult struct {
	conversations int
	wakeEvents    int
	quarantined   int
}

// RunRoomFiles 将旧宿主 Room 文件迁入各自 owner 的宿主状态根。
//
// 迁移按目录原子收口；调用方应记录告警并继续启动。未写完成标记时，
// 下一次启动会依靠 JSONL 去重继续处理尚未迁移的文件。
func RunRoomFiles(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	appRoot := appfs.AppDir()
	markerPath := workspaceFileMigrationMarker(appRoot, roomStateMigrationName)
	applied, err := workspaceFileMigrationApplied(markerPath)
	if err != nil {
		return err
	}

	legacyRoot := filepath.Join(appRoot, "rooms")
	info, err := os.Lstat(legacyRoot)
	if errors.Is(err, os.ErrNotExist) {
		if applied {
			return nil
		}
		return writeWorkspaceFileMigrationMarker(markerPath)
	}
	if err != nil {
		return fmt.Errorf("检查旧 Room 状态根 %q: %w", legacyRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("旧 Room 状态根不是安全目录: %q", legacyRoot)
	}
	if applied {
		// v0.1.30 曾在旧状态根迁入 app 之前写下空完成标记。完成标记不能
		// 压过后来发现的真实源目录，否则 Room 历史会永久留在旧位置。
		logger.Warn("Room 迁移完成标记早于旧目录，重新执行迁移", "legacy_root", legacyRoot)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return fmt.Errorf("读取旧 Room 状态根 %q: %w", legacyRoot, err)
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开 Room 文件迁移数据库: %w", err)
	}
	defer db.Close()
	owners := roomrepo.NewSQLRepository(cfg.DatabaseDriver, db)
	result, err := migrateLegacyRoomFiles(ctx, appfs.StateRoot(), legacyRoot, entries, owners, logger)
	if err != nil {
		return err
	}
	if !applied {
		if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
			return err
		}
	}
	logger.Info("Room 文件状态迁移完成",
		"migration", roomStateMigrationName,
		"conversations", result.conversations,
		"wake_events", result.wakeEvents,
		"quarantined", result.quarantined,
	)
	return nil
}

func migrateLegacyRoomFiles(
	ctx context.Context,
	stateRoot string,
	legacyRoot string,
	entries []os.DirEntry,
	owners conversationOwnerLookup,
	logger *slog.Logger,
) (roomFileMigrationResult, error) {
	migrator := &legacyRoomFileMigrator{
		ctx:            ctx,
		stateRoot:      stateRoot,
		legacyRoot:     legacyRoot,
		quarantineRoot: filepath.Join(stateRoot, "app", ".migration-quarantine", "room-state-v1"),
		owners:         owners,
		logger:         logger,
	}
	for _, entry := range entries {
		if err := migrator.migrateEntry(entry); err != nil {
			return migrator.result, err
		}
	}
	if err := os.Remove(legacyRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		return migrator.result, fmt.Errorf("移除旧 Room 状态根 %q: %w", legacyRoot, err)
	}
	return migrator.result, nil
}

type legacyRoomFileMigrator struct {
	ctx            context.Context
	stateRoot      string
	legacyRoot     string
	quarantineRoot string
	owners         conversationOwnerLookup
	logger         *slog.Logger
	result         roomFileMigrationResult
}

func (m *legacyRoomFileMigrator) migrateEntry(entry os.DirEntry) error {
	sourcePath := filepath.Join(m.legacyRoot, entry.Name())
	switch {
	case entry.Name() == ".DS_Store":
		return removeMigrationSource(sourcePath)
	case entry.Name() == "directed_message_wakes.jsonl" && entry.Type().IsRegular():
		if unsafePath, reason, found, err := firstUnsafeRoomPath(sourcePath); err != nil {
			return err
		} else if found {
			return m.quarantine(
				sourcePath,
				"旧 Room wake 日志是不安全文件，已移入宿主隔离区",
				"unsafe_path", unsafePath,
				"reason", reason,
			)
		}
		count, quarantined, err := migrateLegacyRoomWakeFile(
			m.ctx,
			m.stateRoot,
			sourcePath,
			m.quarantineRoot,
			m.owners,
		)
		m.result.wakeEvents += count
		m.result.quarantined += quarantined
		return err
	case entry.IsDir():
		return m.migrateConversation(sourcePath, entry.Name())
	default:
		return m.quarantine(sourcePath, "旧 Room 根包含未知条目，已移入宿主隔离区")
	}
}

func (m *legacyRoomFileMigrator) migrateConversation(sourcePath string, directoryName string) error {
	unsafePath, reason, found, err := firstUnsafeRoomPath(sourcePath)
	if err != nil {
		return err
	}
	if found {
		return m.quarantine(
			sourcePath,
			"旧 Room 状态包含不安全文件，已移入宿主隔离区",
			"unsafe_path", unsafePath,
			"reason", reason,
		)
	}

	conversationID, ownerUserID, err := resolveLegacyRoomDirectoryOwner(
		m.ctx,
		sourcePath,
		directoryName,
		m.owners,
	)
	if err != nil {
		return err
	}
	if conversationID == "" || ownerUserID == "" {
		empty, emptyErr := directoryEmpty(sourcePath)
		if emptyErr != nil {
			return emptyErr
		}
		if empty {
			return removeMigrationSource(sourcePath)
		}
		return m.quarantine(sourcePath, "旧 Room 状态无法确认 owner，已移入宿主隔离区")
	}
	if err = validateLegacyRoomDirectory(sourcePath, conversationID); err != nil {
		return m.quarantine(
			sourcePath,
			"旧 Room 状态内容归属不一致，已移入宿主隔离区",
			"reason", err.Error(),
		)
	}

	pathStore := workspacestore.New("")
	pathStore.StateRoot = m.stateRoot
	stateTarget := pathStore.RoomConversationDir(ownerUserID, conversationID)
	assetTarget := pathStore.RoomConversationAssetDir(ownerUserID, conversationID)
	for _, target := range []string{stateTarget, assetTarget} {
		if unsafePath, found, checkErr := unsafeMigrationTarget(m.stateRoot, target); checkErr != nil {
			return checkErr
		} else if found {
			return m.quarantine(
				sourcePath,
				"Room 迁移目标包含符号链接，旧状态已移入宿主隔离区",
				"target", target,
				"unsafe_path", unsafePath,
			)
		}
	}

	if err = mergeLegacyRoomConversation(
		sourcePath,
		stateTarget,
		assetTarget,
		conversationID,
		ownerUserID,
	); err != nil {
		return fmt.Errorf("迁移 Room conversation %s: %w", conversationID, err)
	}
	if err = hardenMigratedRoomFiles(
		stateTarget,
		assetTarget,
		appfs.RuntimeIsolationEnforced(),
	); err != nil {
		return err
	}
	m.result.conversations++
	return nil
}

func hardenMigratedRoomFiles(
	stateTarget string,
	assetTarget string,
	launcherManagesPermissions bool,
) error {
	if !shouldHardenMigratedPermissions(launcherManagesPermissions) {
		return nil
	}
	if err := chmodPrivateTree(stateTarget); err != nil {
		return err
	}
	if _, err := os.Lstat(assetTarget); err == nil {
		return chmodPrivateTree(assetTarget)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (m *legacyRoomFileMigrator) quarantine(sourcePath string, message string, fields ...any) error {
	if err := moveMigrationQuarantine(sourcePath, m.quarantineRoot); err != nil {
		return err
	}
	m.result.quarantined++
	fields = append([]any{"path", sourcePath}, fields...)
	m.logger.Warn(message, fields...)
	return nil
}

func removeMigrationSource(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func resolveLegacyRoomDirectoryOwner(
	ctx context.Context,
	sourcePath string,
	directoryName string,
	owners conversationOwnerLookup,
) (string, string, error) {
	candidates := legacyConversationIDCandidates(directoryName)
	if conversationID := conversationIDFromRoomFiles(sourcePath); conversationID != "" {
		candidates = append([]string{conversationID}, candidates...)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, conversationID := range candidates {
		conversationID = strings.TrimSpace(conversationID)
		if conversationID == "" {
			continue
		}
		if _, ok := seen[conversationID]; ok {
			continue
		}
		seen[conversationID] = struct{}{}
		ownerUserID, err := owners.LookupConversationOwnerUserID(ctx, conversationID)
		if err != nil {
			return "", "", err
		}
		if ownerUserID != "" {
			return conversationID, ownerUserID, nil
		}
	}
	return "", "", nil
}

func legacyConversationIDCandidates(directoryName string) []string {
	name := strings.TrimSpace(directoryName)
	result := make([]string, 0, 6)
	if strings.HasPrefix(name, "room-") {
		result = append(result, strings.TrimPrefix(name, "room-"))
	}
	for _, encoding := range []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	} {
		decoded, err := encoding.DecodeString(name)
		if err == nil && len(decoded) > 0 {
			result = append(result, string(decoded))
		}
	}
	result = append(result, name)
	return result
}

func conversationIDFromRoomFiles(root string) string {
	for _, name := range []string{
		"overlay.jsonl",
		"messages.jsonl",
		"directed_messages.jsonl",
		"directed_message_cursors.jsonl",
		"public_handoffs.jsonl",
	} {
		if conversationID := conversationIDFromJSONL(filepath.Join(root, name)); conversationID != "" {
			return conversationID
		}
	}
	return ""
}

func conversationIDFromJSONL(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 8<<20))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		var value any
		if json.Unmarshal(scanner.Bytes(), &value) != nil {
			continue
		}
		if conversationID := nestedString(value, "conversation_id"); conversationID != "" {
			return conversationID
		}
	}
	return ""
}

func nestedString(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		if result, ok := typed[key].(string); ok && strings.TrimSpace(result) != "" {
			return strings.TrimSpace(result)
		}
		for _, child := range typed {
			if result := nestedString(child, key); result != "" {
				return result
			}
		}
	case []any:
		for _, child := range typed {
			if result := nestedString(child, key); result != "" {
				return result
			}
		}
	}
	return ""
}

func validateLegacyRoomDirectory(root string, conversationID string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && entry.Name() == "attachments" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		lines, err := readLegacyRoomJSONLLines(path, conversationID, "")
		if err != nil {
			return err
		}
		_ = lines
		return nil
	})
}

func readLegacyRoomJSONLLines(
	path string,
	conversationID string,
	ownerUserID string,
) ([][]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := nonemptyJSONLLines(content)
	normalized := make([][]byte, 0, len(lines))
	for index, line := range lines {
		var value any
		if err = json.Unmarshal(line, &value); err != nil {
			return nil, fmt.Errorf("%s 第 %d 行不是有效 JSON: %w", path, index+1, err)
		}
		conversationIDs := make(map[string]struct{})
		collectNestedStrings(value, "conversation_id", conversationIDs)
		for valueConversationID := range conversationIDs {
			if valueConversationID != conversationID {
				return nil, fmt.Errorf(
					"%s 第 %d 行 conversation_id=%q 与目录归属 %q 不一致",
					path,
					index+1,
					valueConversationID,
					conversationID,
				)
			}
		}
		if ownerUserID != "" {
			rewriteNestedString(value, "owner_user_id", ownerUserID)
			line, err = json.Marshal(value)
			if err != nil {
				return nil, err
			}
		}
		normalized = append(normalized, bytes.Clone(line))
	}
	return normalized, nil
}

func collectNestedStrings(value any, key string, result map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			if childKey == key {
				if text, ok := child.(string); ok {
					if text = strings.TrimSpace(text); text != "" {
						result[text] = struct{}{}
					}
				}
				continue
			}
			collectNestedStrings(child, key, result)
		}
	case []any:
		for _, child := range typed {
			collectNestedStrings(child, key, result)
		}
	}
}

func rewriteNestedString(value any, key string, replacement string) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			if childKey == key {
				typed[childKey] = replacement
				continue
			}
			rewriteNestedString(child, key, replacement)
		}
	case []any:
		for _, child := range typed {
			rewriteNestedString(child, key, replacement)
		}
	}
}

func migrateLegacyRoomWakeFile(
	ctx context.Context,
	stateRoot string,
	sourcePath string,
	quarantineRoot string,
	owners conversationOwnerLookup,
) (int, int, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return 0, 0, err
	}
	lines := nonemptyJSONLLines(content)
	ownerByWakeID := make(map[string]string)
	grouped := make(map[string][][]byte)
	unresolved := make([][]byte, 0)
	for _, line := range lines {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			unresolved = append(unresolved, line)
			continue
		}
		ownerUserID := ""
		wakeID := nestedString(row["wake"], "wake_id")
		if wakeID == "" {
			wakeID, _ = row["wake_id"].(string)
		}
		wakeID = strings.TrimSpace(wakeID)
		ownerUserID = ownerByWakeID[wakeID]
		conversationIDs := make(map[string]struct{})
		collectNestedStrings(row, "conversation_id", conversationIDs)
		if len(conversationIDs) > 1 {
			unresolved = append(unresolved, line)
			continue
		}
		for conversationID := range conversationIDs {
			ownerUserID, err = owners.LookupConversationOwnerUserID(ctx, conversationID)
			if err != nil {
				return 0, 0, err
			}
		}
		if ownerUserID == "" && wakeID != "" {
			ownerUserID = ownerByWakeID[wakeID]
		}
		if ownerUserID == "" {
			unresolved = append(unresolved, line)
			continue
		}
		if wakeID != "" {
			ownerByWakeID[wakeID] = ownerUserID
		}
		rewriteNestedString(row, "owner_user_id", ownerUserID)
		if wake, ok := row["wake"].(map[string]any); ok {
			wake["owner_user_id"] = ownerUserID
		}
		normalized, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return 0, 0, marshalErr
		}
		line = normalized
		grouped[ownerUserID] = append(grouped[ownerUserID], line)
	}
	for ownerUserID, ownerLines := range grouped {
		targetPath := filepath.Join(
			appfs.UserRoomRootAt(stateRoot, ownerUserID),
			"directed_message_wakes.jsonl",
		)
		if unsafePath, found, checkErr := unsafeMigrationTarget(stateRoot, targetPath); checkErr != nil {
			return 0, 0, checkErr
		} else if found {
			return 0, 0, fmt.Errorf("Room wake 迁移目标包含不安全路径: %s", unsafePath)
		}
		if err = mergeJSONLLines(targetPath, ownerLines); err != nil {
			return 0, 0, err
		}
	}
	if len(unresolved) > 0 {
		if err = mergeJSONLLines(
			filepath.Join(quarantineRoot, "directed_message_wakes.jsonl"),
			unresolved,
		); err != nil {
			return 0, 0, err
		}
	}
	if err = os.Remove(sourcePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, 0, err
	}
	return len(lines) - len(unresolved), len(unresolved), nil
}

func mergeLegacyRoomConversation(
	sourceRoot string,
	stateTargetRoot string,
	assetTargetRoot string,
	conversationID string,
	ownerUserID string,
) error {
	if err := os.MkdirAll(stateTargetRoot, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(sourceRoot, entry.Name())
		if entry.Name() == ".DS_Store" {
			if err = removeMigrationSource(sourcePath); err != nil {
				return err
			}
			continue
		}
		if entry.Name() == "attachments" && entry.IsDir() {
			if err = mergeLegacyAssetDirectory(
				sourcePath,
				filepath.Join(assetTargetRoot, "attachments"),
			); err != nil {
				return err
			}
			continue
		}
		targetName := entry.Name()
		if targetName == "messages.jsonl" {
			targetName = "overlay.jsonl"
		}
		targetPath := filepath.Join(stateTargetRoot, targetName)
		if entry.IsDir() {
			if err = mergeLegacyRoomStateDirectory(
				sourcePath,
				targetPath,
				conversationID,
				ownerUserID,
			); err != nil {
				return err
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(targetName), ".jsonl") {
			lines, readErr := readLegacyRoomJSONLLines(sourcePath, conversationID, ownerUserID)
			if readErr != nil {
				return readErr
			}
			if err = mergeJSONLLines(targetPath, lines); err != nil {
				return err
			}
			if err = removeMigrationSource(sourcePath); err != nil {
				return err
			}
			continue
		}
		if err = mergeLegacyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return os.Remove(sourceRoot)
}

func mergeLegacyRoomStateDirectory(
	sourceRoot string,
	targetRoot string,
	conversationID string,
	ownerUserID string,
) error {
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(sourceRoot, entry.Name())
		targetName := entry.Name()
		if targetName == ".DS_Store" {
			if err = os.Remove(sourcePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if targetName == "messages.jsonl" {
			targetName = "overlay.jsonl"
		}
		targetPath := filepath.Join(targetRoot, targetName)
		if entry.IsDir() {
			if err = mergeLegacyRoomStateDirectory(
				sourcePath,
				targetPath,
				conversationID,
				ownerUserID,
			); err != nil {
				return err
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(targetName), ".jsonl") {
			lines, readErr := readLegacyRoomJSONLLines(sourcePath, conversationID, ownerUserID)
			if readErr != nil {
				return readErr
			}
			if err = mergeJSONLLines(targetPath, lines); err != nil {
				return err
			}
			if err = removeMigrationSource(sourcePath); err != nil {
				return err
			}
			continue
		}
		if err = mergeLegacyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return os.Remove(sourceRoot)
}

func mergeLegacyAssetDirectory(sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(sourceRoot, entry.Name())
		if entry.Name() == ".DS_Store" {
			if err = removeMigrationSource(sourcePath); err != nil {
				return err
			}
			continue
		}
		targetPath := filepath.Join(targetRoot, entry.Name())
		if entry.IsDir() {
			if err = mergeLegacyAssetDirectory(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		if err = mergeLegacyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return os.Remove(sourceRoot)
}

func mergeLegacyFile(sourcePath string, targetPath string) error {
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	targetContent, err := os.ReadFile(targetPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return err
		}
		if err = os.WriteFile(targetPath, sourceContent, 0o600); err != nil {
			return err
		}
	case err != nil:
		return err
	case bytes.Equal(sourceContent, targetContent):
	default:
		sum := sha256.Sum256(sourceContent)
		conflictPath := targetPath + ".legacy-" + hex.EncodeToString(sum[:4])
		if err = os.WriteFile(conflictPath, sourceContent, 0o600); err != nil {
			return err
		}
	}
	return os.Remove(sourcePath)
}

func mergeJSONLLines(targetPath string, sourceLines [][]byte) error {
	existingContent, err := os.ReadFile(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seen := make(map[string]struct{})
	merged := make([][]byte, 0)
	for _, line := range nonemptyJSONLLines(existingContent) {
		key := canonicalJSONLineKey(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, line)
	}
	for _, line := range sourceLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		key := canonicalJSONLineKey(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, bytes.Clone(line))
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return err
	}
	var output bytes.Buffer
	for _, line := range merged {
		output.Write(line)
		output.WriteByte('\n')
	}
	temporary, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".migration-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = temporary.Write(output.Bytes()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, targetPath); err != nil {
		return err
	}
	return nil
}

func canonicalJSONLineKey(line []byte) string {
	var value any
	if json.Unmarshal(line, &value) != nil {
		return string(bytes.TrimSpace(line))
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return string(bytes.TrimSpace(line))
	}
	return string(normalized)
}

func nonemptyJSONLLines(content []byte) [][]byte {
	rawLines := bytes.Split(content, []byte{'\n'})
	lines := make([][]byte, 0, len(rawLines))
	for _, line := range rawLines {
		if line = bytes.TrimSpace(line); len(line) > 0 {
			lines = append(lines, bytes.Clone(line))
		}
	}
	return lines
}

func firstUnsafeRoomPath(root string) (string, string, bool, error) {
	var foundPath string
	var foundReason string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			foundPath = path
			foundReason = "symbolic_link"
			return filepath.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			foundPath = path
			foundReason = "special_file"
			return filepath.SkipAll
		}
		hasMultipleLinks, err := roomFileHasMultipleHardLinks(path, info)
		if err != nil {
			return err
		}
		if hasMultipleLinks {
			foundPath = path
			foundReason = "hard_link"
			return filepath.SkipAll
		}
		return nil
	})
	return foundPath, foundReason, foundPath != "", err
}

func unsafeMigrationTarget(root string, target string) (string, bool, error) {
	path, found, err := firstSymlinkPathComponent(root, target)
	if err != nil || found {
		return path, found, err
	}
	path, _, found, err = firstUnsafeRoomPath(target)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return path, found, err
}

func firstSymlinkPathComponent(root string, target string) (string, bool, error) {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("Room 迁移目标越出状态根: %q", target)
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return "", false, nil
		}
		if statErr != nil {
			return "", false, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return current, true, nil
		}
	}
	return "", false, nil
}

func directoryEmpty(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	_, err = file.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

func moveMigrationQuarantine(sourcePath string, quarantineRoot string) error {
	if err := os.MkdirAll(quarantineRoot, 0o700); err != nil {
		return err
	}
	name := filepath.Base(sourcePath)
	targetPath := filepath.Join(quarantineRoot, name)
	for suffix := 1; ; suffix++ {
		_, err := os.Lstat(targetPath)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
		targetPath = filepath.Join(quarantineRoot, fmt.Sprintf("%s.%d", name, suffix))
	}
	return os.Rename(sourcePath, targetPath)
}

func chmodPrivateTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	})
}
