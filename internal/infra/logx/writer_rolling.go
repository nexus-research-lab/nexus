package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileOptions 描述日志文件滚动策略。
type FileOptions struct {
	Enabled     bool
	Path        string
	RotateDaily bool
	MaxSizeMB   int
	MaxAgeDays  int
	MaxBackups  int
	Compress    bool
	NowFn       func() time.Time
}

type rollingFileWriter struct {
	options         FileOptions
	currentDate     string
	currentPath     string
	lastCleanupDate string
	writer          *lumberjack.Logger
	mu              sync.Mutex
}

func newRollingFileWriter(options FileOptions) (io.Writer, error) {
	if !options.Enabled {
		return nil, nil
	}
	normalized := normalizeFileOptions(options)
	if normalized.Path == "" {
		return nil, nil
	}

	writer := &rollingFileWriter{options: normalized}
	if err := writer.rotateIfNeeded(normalized.NowFn()); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rollingFileWriter) Write(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(w.options.NowFn()); err != nil {
		return 0, err
	}
	if w.writer == nil {
		return 0, fmt.Errorf("log writer is not initialized")
	}
	return w.writer.Write(payload)
}

func (w *rollingFileWriter) rotateIfNeeded(now time.Time) error {
	dateKey := ""
	if w.options.RotateDaily {
		dateKey = now.Format("2006-01-02")
	}
	if w.writer != nil && (!w.options.RotateDaily || w.currentDate == dateKey) {
		return nil
	}

	targetPath := buildActiveLogPath(w.options.Path, now, w.options.RotateDaily)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	if w.writer != nil {
		_ = w.writer.Close()
	}

	w.writer = &lumberjack.Logger{
		Filename:   targetPath,
		MaxSize:    w.options.MaxSizeMB,
		MaxAge:     w.options.MaxAgeDays,
		MaxBackups: w.options.MaxBackups,
		Compress:   w.options.Compress,
		LocalTime:  true,
	}
	w.currentDate = dateKey
	w.currentPath = targetPath

	if w.options.RotateDaily {
		if err := w.cleanupOldFiles(now); err != nil {
			return err
		}
	}
	return nil
}

func (w *rollingFileWriter) cleanupOldFiles(now time.Time) error {
	cleanupDate := now.Format("2006-01-02")
	if cleanupDate == w.lastCleanupDate {
		return nil
	}
	w.lastCleanupDate = cleanupDate

	matches, err := filepath.Glob(dailyLogGlobPattern(w.options.Path))
	if err != nil {
		return fmt.Errorf("glob log files: %w", err)
	}

	type fileInfo struct {
		path    string
		modTime time.Time
	}

	candidates := make([]fileInfo, 0, len(matches))
	cutoff := now.AddDate(0, 0, -maxInt(w.options.MaxAgeDays, 0))
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || info.IsDir() {
			continue
		}
		if w.options.MaxAgeDays > 0 && info.ModTime().Before(cutoff) {
			_ = os.Remove(match)
			continue
		}
		candidates = append(candidates, fileInfo{
			path:    match,
			modTime: info.ModTime(),
		})
	}

	if w.options.MaxBackups <= 0 || len(candidates) <= w.options.MaxBackups+1 {
		return nil
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	for _, item := range candidates[w.options.MaxBackups+1:] {
		_ = os.Remove(item.path)
	}
	return nil
}

func normalizeFileOptions(options FileOptions) FileOptions {
	result := options
	result.Path = expandHomePath(strings.TrimSpace(result.Path))
	if result.NowFn == nil {
		result.NowFn = time.Now
	}
	if result.MaxSizeMB <= 0 {
		result.MaxSizeMB = 10
	}
	if result.MaxAgeDays < 0 {
		result.MaxAgeDays = 0
	}
	if result.MaxBackups < 0 {
		result.MaxBackups = 0
	}
	return result
}

func buildActiveLogPath(basePath string, now time.Time, rotateDaily bool) string {
	if !rotateDaily {
		return expandHomePath(strings.TrimSpace(basePath))
	}

	basePath = expandHomePath(strings.TrimSpace(basePath))
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(filepath.Base(basePath), ext)
	dir := filepath.Dir(basePath)
	filename := fmt.Sprintf("%s-%s%s", base, now.Format("2006-01-02"), ext)
	return filepath.Join(dir, filename)
}

func dailyLogGlobPattern(basePath string) string {
	basePath = expandHomePath(strings.TrimSpace(basePath))
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(filepath.Base(basePath), ext)
	dir := filepath.Dir(basePath)
	if ext == "" {
		return filepath.Join(dir, base+"-*")
	}
	return filepath.Join(dir, base+"-*"+ext+"*")
}

func expandHomePath(raw string) string {
	if raw == "" || raw == "~" {
		return raw
	}
	if !strings.HasPrefix(raw, "~/") {
		return raw
	}
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return raw
	}
	return filepath.Join(homeDir, strings.TrimPrefix(raw, "~/"))
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
