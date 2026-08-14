// INPUT: owner-confined workspace、精确 session_key、配置版本与删除清理引用。
// OUTPUT: 持久 deleting/deleted tombstone、一次性删除 lease 与普通 writer admission。
// POS: session 目录之外的删除真相源；删除后晚到 runtime writer 不得以 version=1 复活。
package workspace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	sessionLifecycleStateActive   = "active"
	sessionLifecycleStateDeleting = "deleting"
	sessionLifecycleStateDeleted  = "deleted"
)

type sessionLifecycleRecord struct {
	SessionKey           string    `json:"session_key"`
	OwnerUserID          string    `json:"owner_user_id"`
	WorkspacePath        string    `json:"workspace_path"`
	State                string    `json:"state"`
	Generation           int64     `json:"generation"`
	ConfigurationVersion int64     `json:"configuration_version,omitempty"`
	DeleteToken          string    `json:"delete_token,omitempty"`
	CleanupSessionID     string    `json:"cleanup_session_id,omitempty"`
	CleanupSessionIDs    []string  `json:"cleanup_session_ids,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// SessionDeletionLease 是 storage 删除栅栏的不可伪造进程内句柄。
// 字段保持私有，调用方只能交回创建它的 SessionFileStore。
type SessionDeletionLease struct {
	workspacePath string
	sessionKey    string
	deleteToken   string
}

// PendingSessionDeletion 是宿主启动恢复器可见的最小删除记录。
// DeleteToken 只封装在 Lease 私有字段内，不对模型、协议或审计投影。
type PendingSessionDeletion struct {
	OwnerUserID          string
	WorkspacePath        string
	SessionKey           string
	ConfigurationVersion int64
	CleanupSessionID     string
	CleanupSessionIDs    []string
	Committed            bool
	CleanupComplete      bool
	Lease                SessionDeletionLease
}

// BeginSessionDeletion 在关闭 runtime 之前持久写入 deleting tombstone。
func (s *SessionFileStore) BeginSessionDeletion(
	workspacePath string,
	sessionKey string,
	expectedConfigurationVersion int64,
	cleanupSessionID string,
) (SessionDeletionLease, error) {
	return s.BeginSessionDeletionWithTranscriptIDs(
		workspacePath,
		sessionKey,
		expectedConfigurationVersion,
		[]string{cleanupSessionID},
	)
}

// BeginSessionDeletionWithTranscriptIDs 持久保存删除所需的完整 transcript lineage。
func (s *SessionFileStore) BeginSessionDeletionWithTranscriptIDs(
	workspacePath string,
	sessionKey string,
	expectedConfigurationVersion int64,
	cleanupSessionIDs []string,
) (SessionDeletionLease, error) {
	if expectedConfigurationVersion < 1 {
		return SessionDeletionLease{}, errors.New("expected session configuration_version 必须大于 0")
	}
	lease, _, err := s.beginSessionDeletion(
		workspacePath,
		sessionKey,
		&expectedConfigurationVersion,
		cleanupSessionIDs,
		false,
	)
	return lease, err
}

// BeginSessionArtifactDeletion 为 Room/Automation 等内部生命周期清理建立同一
// 持久 tombstone。即使 meta 尚未落盘或已先行消失，也要保留 admission fence，
// 避免晚到 runtime writer 重新创建目录。
func (s *SessionFileStore) BeginSessionArtifactDeletion(
	workspacePath string,
	sessionKey string,
	cleanupSessionID string,
) (SessionDeletionLease, int64, error) {
	return s.BeginSessionArtifactDeletionWithTranscriptIDs(
		workspacePath,
		sessionKey,
		[]string{cleanupSessionID},
	)
}

// BeginSessionArtifactDeletionWithTranscriptIDs 为内部 artifact 删除固化完整 lineage。
func (s *SessionFileStore) BeginSessionArtifactDeletionWithTranscriptIDs(
	workspacePath string,
	sessionKey string,
	cleanupSessionIDs []string,
) (SessionDeletionLease, int64, error) {
	return s.beginSessionDeletion(
		workspacePath,
		sessionKey,
		nil,
		cleanupSessionIDs,
		true,
	)
}

func (s *SessionFileStore) beginSessionDeletion(
	workspacePath string,
	sessionKey string,
	expectedConfigurationVersion *int64,
	cleanupSessionIDs []string,
	allowMissing bool,
) (SessionDeletionLease, int64, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, sessionKey)
	defer unlock()

	current, _, err := s.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		return SessionDeletionLease{}, 0, err
	}
	if current == nil && !allowMissing {
		return SessionDeletionLease{}, 0, os.ErrNotExist
	}
	configurationVersion := int64(0)
	if current != nil {
		configurationVersion = current.ConfigurationVersion
	}
	if expectedConfigurationVersion != nil &&
		configurationVersion != *expectedConfigurationVersion {
		return SessionDeletionLease{}, 0, fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			*expectedConfigurationVersion,
			configurationVersion,
		)
	}
	previous, err := s.readSessionLifecycle(workspacePath, sessionKey)
	if err != nil {
		return SessionDeletionLease{}, 0, err
	}
	if previous != nil &&
		(previous.State == sessionLifecycleStateDeleting ||
			previous.State == sessionLifecycleStateDeleted) {
		return SessionDeletionLease{}, 0, ErrSessionDeleted
	}
	token, err := newSessionDeletionToken()
	if err != nil {
		return SessionDeletionLease{}, 0, err
	}
	generation := configurationVersion + 1
	if previous != nil && previous.Generation >= generation {
		generation = previous.Generation + 1
	}
	cleanupSessionIDs = normalizedCleanupSessionIDs(cleanupSessionIDs)
	cleanupSessionID := ""
	if len(cleanupSessionIDs) > 0 {
		cleanupSessionID = cleanupSessionIDs[0]
	}
	record := sessionLifecycleRecord{
		SessionKey:           sessionKey,
		OwnerUserID:          strings.TrimSpace(s.ownerUserID),
		WorkspacePath:        filepath.Clean(strings.TrimSpace(workspacePath)),
		State:                sessionLifecycleStateDeleting,
		Generation:           generation,
		ConfigurationVersion: configurationVersion,
		DeleteToken:          token,
		CleanupSessionID:     strings.TrimSpace(cleanupSessionID),
		CleanupSessionIDs:    cleanupSessionIDs,
		UpdatedAt:            time.Now().UTC(),
	}
	if err = s.writeSessionLifecycle(workspacePath, record); err != nil {
		return SessionDeletionLease{}, 0, err
	}
	return SessionDeletionLease{
		workspacePath: strings.TrimSpace(workspacePath),
		sessionKey:    sessionKey,
		deleteToken:   token,
	}, configurationVersion, nil
}

// AbortSessionDeletion 在目录尚未提交删除时解除本次 deleting tombstone。
func (s *SessionFileStore) AbortSessionDeletion(lease SessionDeletionLease) error {
	if !validSessionDeletionLease(lease) {
		return nil
	}
	unlock := lockSessionMutation(s.ownerUserID, lease.workspacePath, lease.sessionKey)
	defer unlock()
	_, err := s.requireSessionDeletionLeaseLocked(lease)
	if err != nil {
		return err
	}
	return s.removeSessionLifecycle(lease.workspacePath, lease.sessionKey)
}

// CommitSessionDeletion 以 lease 和 configuration_version 删除精确 session 目录，
// 并把 tombstone 推进到 deleted。删除后普通 Upsert 永久 fail closed。
func (s *SessionFileStore) CommitSessionDeletion(
	lease SessionDeletionLease,
	expectedConfigurationVersion int64,
) (bool, error) {
	if !validSessionDeletionLease(lease) {
		return false, errors.New("invalid session deletion lease")
	}
	unlock := lockSessionMutation(s.ownerUserID, lease.workspacePath, lease.sessionKey)
	defer unlock()
	record, err := s.requireSessionDeletionLeaseLocked(lease)
	if err != nil {
		return false, err
	}
	if record.ConfigurationVersion != expectedConfigurationVersion {
		return false, fmt.Errorf(
			"%w: lifecycle expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			expectedConfigurationVersion,
			record.ConfigurationVersion,
		)
	}
	deleted, err := s.deleteSessionLocked(
		lease.workspacePath,
		lease.sessionKey,
		&expectedConfigurationVersion,
	)
	if err != nil {
		return false, err
	}
	// 目录删除与 tombstone finalization 之间崩溃时，恢复器会看到 deleting
	// 但目录已不存在；这仍是已提交删除，必须继续推进 deleted。
	record.State = sessionLifecycleStateDeleted
	record.Generation++
	record.UpdatedAt = time.Now().UTC()
	if err = s.writeSessionLifecycle(lease.workspacePath, *record); err != nil {
		return true, fmt.Errorf("session directory deleted but lifecycle finalization failed: %w", err)
	}
	return deleted || record.State == sessionLifecycleStateDeleted, nil
}

// CompleteSessionDeletionCleanup 清除 tombstone 中仅供宿主重试清理的 SDK transcript 引用。
func (s *SessionFileStore) CompleteSessionDeletionCleanup(lease SessionDeletionLease) error {
	if !validSessionDeletionLease(lease) {
		return nil
	}
	unlock := lockSessionMutation(s.ownerUserID, lease.workspacePath, lease.sessionKey)
	defer unlock()
	record, err := s.readSessionLifecycle(lease.workspacePath, lease.sessionKey)
	if err != nil {
		return err
	}
	if record == nil || record.State != sessionLifecycleStateDeleted ||
		record.DeleteToken != lease.deleteToken {
		return errors.New("session deletion cleanup lease is stale")
	}
	record.DeleteToken = ""
	record.CleanupSessionID = ""
	record.CleanupSessionIDs = nil
	record.UpdatedAt = time.Now().UTC()
	return s.writeSessionLifecycle(lease.workspacePath, *record)
}

func (s *SessionFileStore) requireSessionWritableLocked(
	workspacePath string,
	sessionKey string,
) error {
	record, err := s.readSessionLifecycle(workspacePath, sessionKey)
	if err != nil {
		return err
	}
	if record == nil || record.State == "" || record.State == sessionLifecycleStateActive {
		return nil
	}
	return fmt.Errorf("%w: state=%s", ErrSessionDeleted, record.State)
}

func (s *SessionFileStore) requireSessionDeletionLeaseLocked(
	lease SessionDeletionLease,
) (*sessionLifecycleRecord, error) {
	record, err := s.readSessionLifecycle(lease.workspacePath, lease.sessionKey)
	if err != nil {
		return nil, err
	}
	if record == nil ||
		record.State != sessionLifecycleStateDeleting ||
		record.DeleteToken != lease.deleteToken {
		return nil, errors.New("session deletion lease is stale")
	}
	return record, nil
}

func (s *SessionFileStore) readSessionLifecycle(
	workspacePath string,
	sessionKey string,
) (*sessionLifecycleRecord, error) {
	root, err := s.openSessionLifecycleRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	payload, err := root.ReadFile(sessionLifecycleFileName(workspacePath, sessionKey))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record sessionLifecycleRecord
	if err = json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}
	if strings.TrimSpace(record.SessionKey) != strings.TrimSpace(sessionKey) {
		return nil, fmt.Errorf(
			"%w: lifecycle requested=%q stored=%q",
			ErrSessionStorageIdentityMismatch,
			strings.TrimSpace(sessionKey),
			strings.TrimSpace(record.SessionKey),
		)
	}
	if strings.TrimSpace(s.ownerUserID) != "" &&
		strings.TrimSpace(record.OwnerUserID) != strings.TrimSpace(s.ownerUserID) {
		return nil, fmt.Errorf(
			"%w: lifecycle owner requested=%q stored=%q",
			ErrSessionStorageIdentityMismatch,
			strings.TrimSpace(s.ownerUserID),
			strings.TrimSpace(record.OwnerUserID),
		)
	}
	if filepath.Clean(strings.TrimSpace(record.WorkspacePath)) !=
		filepath.Clean(strings.TrimSpace(workspacePath)) {
		return nil, fmt.Errorf(
			"%w: lifecycle workspace requested=%q stored=%q",
			ErrSessionStorageIdentityMismatch,
			filepath.Clean(strings.TrimSpace(workspacePath)),
			filepath.Clean(strings.TrimSpace(record.WorkspacePath)),
		)
	}
	return &record, nil
}

func (s *SessionFileStore) writeSessionLifecycle(
	workspacePath string,
	record sessionLifecycleRecord,
) error {
	root, err := s.openSessionLifecycleRoot(true)
	if err != nil {
		return err
	}
	defer root.Close()
	relative := sessionLifecycleFileName(workspacePath, record.SessionKey)
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFileAtomic(relative, payload, storageFileMode(0o600))
}

func (s *SessionFileStore) removeSessionLifecycle(
	workspacePath string,
	sessionKey string,
) error {
	root, err := s.openSessionLifecycleRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	err = root.RemoveAll(sessionLifecycleFileName(workspacePath, sessionKey))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func sessionLifecycleFileName(workspacePath string, sessionKey string) string {
	physicalIdentity := filepath.Clean(strings.TrimSpace(workspacePath)) + "\x00" +
		encodeSessionDirName(strings.TrimSpace(sessionKey))
	sum := sha256.Sum256([]byte(physicalIdentity))
	return hex.EncodeToString(sum[:]) + ".json"
}

func (s *SessionFileStore) openSessionLifecycleRoot(create bool) (*confinedfs.Root, error) {
	managedRoot, target := s.sessionLifecycleRootPaths()
	return openManagedSubtree(managedRoot, target, create, storagePrivateDirectoryMode())
}

func (s *SessionFileStore) sessionLifecycleRootPaths() (string, string) {
	if strings.TrimSpace(s.ownerUserID) != "" {
		return s.paths.StateRoot, filepath.Join(
			appfs.UserStateRootAt(s.paths.StateRoot, s.ownerUserID),
			"session-lifecycle",
		)
	}
	// 未绑定 owner 的门面只供底层单元测试和 legacy host 调用。产品服务总是
	// 使用 ForOwner，因此权威记录位于 users/<owner>/state。
	root := filepath.Clean(strings.TrimSpace(s.paths.WorkspaceRoot))
	return root, filepath.Join(root, ".nexus-host-state", "session-lifecycle")
}

// ListSessionDeletionRecords 扫描 owner state。已完成记录也必须返回，宿主重启时
// 才能重新安装永久 runtime admission fence。
func (s *SessionFileStore) ListSessionDeletionRecords() ([]PendingSessionDeletion, error) {
	return s.listSessionDeletionRecords()
}

// ListPendingSessionDeletions 只返回尚需提交目录删除或 transcript cleanup 的记录。
func (s *SessionFileStore) ListPendingSessionDeletions() ([]PendingSessionDeletion, error) {
	records, err := s.listSessionDeletionRecords()
	if err != nil {
		return nil, err
	}
	pending := make([]PendingSessionDeletion, 0, len(records))
	for _, record := range records {
		if !record.CleanupComplete {
			pending = append(pending, record)
		}
	}
	return pending, nil
}

func (s *SessionFileStore) listSessionDeletionRecords() ([]PendingSessionDeletion, error) {
	if s == nil || s.paths == nil {
		return nil, errors.New("workspace storage root is nil")
	}
	usersRoot, err := openManagedSubtree(
		s.paths.StateRoot,
		filepath.Join(s.paths.StateRoot, "users"),
		false,
		0o700,
	)
	if errors.Is(err, os.ErrNotExist) {
		return []PendingSessionDeletion{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer usersRoot.Close()
	ownerEntries, err := fs.ReadDir(usersRoot.FS(), ".")
	if err != nil {
		return nil, err
	}
	result := make([]PendingSessionDeletion, 0)
	for _, ownerEntry := range ownerEntries {
		if !ownerEntry.IsDir() {
			continue
		}
		lifecycleRoot, openErr := usersRoot.OpenRootNoSymlink(filepath.ToSlash(filepath.Join(
			ownerEntry.Name(),
			"state",
			"session-lifecycle",
		)))
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		entries, readErr := fs.ReadDir(lifecycleRoot.FS(), ".")
		if readErr != nil {
			lifecycleRoot.Close()
			return nil, readErr
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			payload, readErr := lifecycleRoot.ReadFile(entry.Name())
			if readErr != nil {
				lifecycleRoot.Close()
				return nil, readErr
			}
			var record sessionLifecycleRecord
			if readErr = json.Unmarshal(payload, &record); readErr != nil {
				lifecycleRoot.Close()
				return nil, readErr
			}
			if appfs.UserPathSegment(record.OwnerUserID) != ownerEntry.Name() ||
				!s.paths.workspacePathBelongsToOwner(record.OwnerUserID, record.WorkspacePath) ||
				entry.Name() != sessionLifecycleFileName(record.WorkspacePath, record.SessionKey) {
				lifecycleRoot.Close()
				return nil, fmt.Errorf(
					"%w: invalid pending lifecycle record %s/%s",
					ErrSessionStorageIdentityMismatch,
					ownerEntry.Name(),
					entry.Name(),
				)
			}
			if record.State != sessionLifecycleStateDeleting &&
				record.State != sessionLifecycleStateDeleted {
				continue
			}
			cleanupSessionIDs := normalizedCleanupSessionIDs(append(
				record.CleanupSessionIDs,
				record.CleanupSessionID,
			))
			cleanupComplete := record.State == sessionLifecycleStateDeleted &&
				strings.TrimSpace(record.DeleteToken) == "" &&
				len(cleanupSessionIDs) == 0
			if !cleanupComplete && strings.TrimSpace(record.DeleteToken) == "" {
				lifecycleRoot.Close()
				return nil, errors.New("pending session deletion is missing delete_token")
			}
			result = append(result, PendingSessionDeletion{
				OwnerUserID:          strings.TrimSpace(record.OwnerUserID),
				WorkspacePath:        filepath.Clean(strings.TrimSpace(record.WorkspacePath)),
				SessionKey:           strings.TrimSpace(record.SessionKey),
				ConfigurationVersion: record.ConfigurationVersion,
				CleanupSessionID:     strings.TrimSpace(record.CleanupSessionID),
				CleanupSessionIDs:    cleanupSessionIDs,
				Committed:            record.State == sessionLifecycleStateDeleted,
				CleanupComplete:      cleanupComplete,
				Lease: SessionDeletionLease{
					workspacePath: filepath.Clean(strings.TrimSpace(record.WorkspacePath)),
					sessionKey:    strings.TrimSpace(record.SessionKey),
					deleteToken:   strings.TrimSpace(record.DeleteToken),
				},
			})
		}
		if closeErr := lifecycleRoot.Close(); closeErr != nil {
			return nil, closeErr
		}
	}
	sort.Slice(result, func(left int, right int) bool {
		if result[left].OwnerUserID != result[right].OwnerUserID {
			return result[left].OwnerUserID < result[right].OwnerUserID
		}
		return result[left].SessionKey < result[right].SessionKey
	})
	return result, nil
}

func normalizedCleanupSessionIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func newSessionDeletionToken() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func validSessionDeletionLease(lease SessionDeletionLease) bool {
	return strings.TrimSpace(lease.workspacePath) != "" &&
		strings.TrimSpace(lease.sessionKey) != "" &&
		strings.TrimSpace(lease.deleteToken) != ""
}
