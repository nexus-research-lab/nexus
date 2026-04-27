package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRollingFileWriterCreatesDailyLogFile(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)

	writer, err := newRollingFileWriter(FileOptions{
		Enabled:     true,
		Path:        filepath.Join(tempDir, "logger.log"),
		RotateDaily: true,
		MaxSizeMB:   1,
		MaxAgeDays:  7,
		MaxBackups:  7,
		NowFn: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("创建滚动日志写入器失败: %v", err)
	}

	if _, err = writer.Write([]byte("hello\n")); err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	expectedPath := filepath.Join(tempDir, "logger-2026-04-11.log")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("读取日期日志文件失败: %v", err)
	}
	if !strings.Contains(string(content), "hello") {
		t.Fatalf("日志文件内容不正确: %s", string(content))
	}
}

func TestRollingFileWriterRotatesBySize(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)

	writer, err := newRollingFileWriter(FileOptions{
		Enabled:     true,
		Path:        filepath.Join(tempDir, "logger.log"),
		RotateDaily: true,
		MaxSizeMB:   1,
		MaxAgeDays:  7,
		MaxBackups:  7,
		NowFn: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("创建滚动日志写入器失败: %v", err)
	}

	chunk := []byte(strings.Repeat("a", 700*1024))
	if _, err = writer.Write(chunk); err != nil {
		t.Fatalf("第一次写入日志失败: %v", err)
	}
	if _, err = writer.Write(chunk); err != nil {
		t.Fatalf("第二次写入日志失败: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "logger-2026-04-11*.log*"))
	if err != nil {
		t.Fatalf("匹配滚动日志文件失败: %v", err)
	}
	if len(matches) < 2 {
		t.Fatalf("期望触发大小轮转，实际文件数量=%d, files=%v", len(matches), matches)
	}
}
