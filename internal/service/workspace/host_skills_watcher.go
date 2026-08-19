// INPUT: 服务生命周期、宿主 Skill 源目录与 fsnotify 事件。
// OUTPUT: 去抖、非致命且可停止的宿主 Skill 投影刷新循环。
// POS: 可选宿主资源同步与 nexus-server 生命周期之间的后台协调层。
package workspace

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nexus-research-lab/nexus/internal/config"
)

const hostSkillRefreshDebounce = 500 * time.Millisecond

// WatchHostSkillLibrary 完成首次后台刷新，并监听后续宿主 Skill 变化。
//
// watcher 和单次同步都属于可选能力；任何失败只保留 last-good 并记录日志，
// 不会影响 HTTP 服务健康状态。
func WatchHostSkillLibrary(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) {
	watchHostSkillLibrary(ctx, cfg, logger, nil)
}

func watchHostSkillLibrary(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
	ready chan<- struct{},
) {
	if logger == nil {
		logger = slog.Default()
	}
	result, refreshErr := syncHostSkillLibrary(cfg)
	logHostSkillSyncResult(logger, result, refreshErr)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Warn("宿主 Skill watcher 初始化失败", "err", err)
		signalHostSkillWatcherReady(ready)
		return
	}
	defer watcher.Close()
	updateHostSkillWatches(watcher, result.watchDirectories, logger)
	signalHostSkillWatcherReady(ready)
	if len(watcher.WatchList()) == 0 {
		return
	}

	var refreshTimer *time.Timer
	var refresh <-chan time.Time
	defer func() {
		if refreshTimer != nil {
			refreshTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			if refreshTimer == nil {
				refreshTimer = time.NewTimer(hostSkillRefreshDebounce)
			} else {
				if !refreshTimer.Stop() {
					select {
					case <-refreshTimer.C:
					default:
					}
				}
				refreshTimer.Reset(hostSkillRefreshDebounce)
			}
			refresh = refreshTimer.C
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Warn("宿主 Skill watcher 错误", "err", watchErr)
		case <-refresh:
			refresh = nil
			result, refreshErr = syncHostSkillLibrary(cfg)
			logHostSkillSyncResult(logger, result, refreshErr)
			updateHostSkillWatches(watcher, result.watchDirectories, logger)
		}
	}
}

func updateHostSkillWatches(
	watcher *fsnotify.Watcher,
	directories []string,
	logger *slog.Logger,
) {
	desired := make(map[string]struct{}, len(directories))
	for _, directory := range directories {
		clean := filepath.Clean(directory)
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		desired[clean] = struct{}{}
	}
	active := make(map[string]struct{})
	for _, directory := range watcher.WatchList() {
		active[filepath.Clean(directory)] = struct{}{}
	}
	for directory := range active {
		if _, keep := desired[directory]; keep {
			continue
		}
		if err := watcher.Remove(directory); err != nil && !os.IsNotExist(err) {
			logger.Debug("移除失效宿主 Skill watch 失败", "path", directory, "err", err)
		}
	}
	for directory := range desired {
		if _, exists := active[directory]; exists {
			continue
		}
		if err := watcher.Add(directory); err != nil {
			logger.Warn("添加宿主 Skill watch 失败", "path", directory, "err", err)
		}
	}
}

func signalHostSkillWatcherReady(ready chan<- struct{}) {
	if ready != nil {
		close(ready)
	}
}

func logHostSkillSyncResult(
	logger *slog.Logger,
	result hostSkillSyncResult,
	refreshErr error,
) {
	for _, skipped := range result.skipped {
		logger.Warn(
			"宿主 Skill 投影已跳过异常项",
			"skill", skipped.name,
			"last_good_retained", skipped.retainedLastGood,
			"err", skipped.err,
		)
	}
	if refreshErr != nil {
		logger.Warn("宿主 Skill 刷新失败，继续使用上一版", "err", refreshErr)
	}
}
