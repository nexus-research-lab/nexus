// INPUT: 即将开始新一轮的 DM 会话。
// OUTPUT: 后台预热的 transcript 解析缓存与投影检查点；不产生任何对外可见的结果。
// POS: 进程重启后第一轮结束时的历史刷新也能走纯追加增量路径；尽力而为，失败只退化为完整重建。
package workspace

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	// 小 transcript 的完整重建本身很快，不值得额外解析一遍。
	transcriptPrewarmMinBytes = 1 << 20
	transcriptPrewarmTimeout  = time.Minute
	transcriptPrewarmAttempts = 3
	transcriptPrewarmRetry    = 500 * time.Millisecond
)

// transcriptPrewarmSlots 限制全局同时预热的会话数，满时直接跳过，不排队。
var transcriptPrewarmSlots = make(chan struct{}, 2)

// PrewarmTranscriptHistory 在 DM 轮次开始时于后台解析 transcript 并建立投影检查点。
// 轮次期间 runtime 只追加 transcript，轮次结束后的历史刷新因此只需处理新增条目。
// 调用立即返回；分段、fork 与小 transcript 不预热，缓存已有检查点时也不重复解析。
func (s *AgentHistoryStore) PrewarmTranscriptHistory(workspacePath string, sessionValue protocol.Session) {
	if s == nil || !transcriptPrewarmEligible(sessionValue) {
		return
	}
	select {
	case transcriptPrewarmSlots <- struct{}{}:
	default:
		return
	}
	key := filepath.Join(workspacePath, sessionValue.SessionKey)
	if !s.beginTranscriptPrewarm(key) {
		<-transcriptPrewarmSlots
		return
	}
	go func() {
		defer func() {
			s.endTranscriptPrewarm(key)
			<-transcriptPrewarmSlots
		}()
		ctx, cancel := context.WithTimeout(context.Background(), transcriptPrewarmTimeout)
		defer cancel()
		s.prewarmTranscriptHistory(ctx, workspacePath, sessionValue)
	}()
}

func transcriptPrewarmEligible(sessionValue protocol.Session) bool {
	if sessionValue.SessionID == nil || strings.TrimSpace(*sessionValue.SessionID) == "" {
		return false
	}
	if segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool); segmented {
		return false
	}
	_, forkMessageID := pendingForkTranscript(sessionValue)
	return forkMessageID == ""
}

// prewarmTranscriptHistory 同步执行一次预热，返回缓存中是否已有可用的投影检查点。
// 读取期间 runtime 恰好写入时解析结果不会进入缓存，稍后有界重试。
func (s *AgentHistoryStore) prewarmTranscriptHistory(ctx context.Context, workspacePath string, sessionValue protocol.Session) bool {
	sessionID := strings.TrimSpace(*sessionValue.SessionID)
	for attempt := 0; attempt < transcriptPrewarmAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(transcriptPrewarmRetry):
			}
		}
		path, size, ok := s.transcriptPrewarmTarget(ctx, workspacePath, sessionID)
		if !ok || size < transcriptPrewarmMinBytes {
			return false
		}
		if s.transcriptProjectionCached(path) {
			return true
		}
		if _, err := s.readHistoryRowsContext(ctx, workspacePath, sessionValue); err != nil {
			return false
		}
		if s.transcriptProjectionCached(path) {
			return true
		}
	}
	return false
}

func (s *AgentHistoryStore) transcriptPrewarmTarget(ctx context.Context, workspacePath string, sessionID string) (string, int64, bool) {
	transcriptPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
	if err != nil {
		return "", 0, false
	}
	root, relative, info, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		return "", 0, false
	}
	defer root.Close()
	// 与 readTranscriptEntriesLineageAtContext 的缓存键保持一致。
	return filepath.Join(root.Name(), relative), info.Size(), true
}

func (s *AgentHistoryStore) transcriptProjectionCached(path string) bool {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	entry, exists := s.cache.entries[path]
	return exists && entry.Lineage != 0 && entry.Projection != nil
}

func (s *AgentHistoryStore) beginTranscriptPrewarm(key string) bool {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	if s.cache.prewarming == nil {
		s.cache.prewarming = make(map[string]struct{})
	}
	if _, running := s.cache.prewarming[key]; running {
		return false
	}
	s.cache.prewarming[key] = struct{}{}
	return true
}

func (s *AgentHistoryStore) endTranscriptPrewarm(key string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.prewarming, key)
}
