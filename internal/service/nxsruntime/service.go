package nxsruntime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"

	bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
)

const (
	commandPathEnvName = "NEXUS_NXS_COMMAND_PATH"
	cacheDirEnvName    = "NEXUS_NXS_RUNTIME_CACHE_DIR"
	cacheDirName       = "nexus-agent-sdk-bridge"
)

// RuntimeStatus 表示 nxs runtime 在当前主机上的可用状态。
type RuntimeStatus struct {
	Available   bool   `json:"available"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source,omitempty"`
	CanDownload bool   `json:"can_download"`
	Message     string `json:"message,omitempty"`
}

// Service 负责探测和拉取 nxs runtime。
type Service struct {
	getenv      func(string) string
	userCache   func() (string, error)
	appRoot     func() string
	runtimePath func() (string, error)
	stat        func(string) (os.FileInfo, error)
	walkDir     func(string, fs.WalkDirFunc) error
}

// NewService 创建 nxs runtime 服务。
func NewService() *Service {
	return &Service{
		getenv:      os.Getenv,
		userCache:   os.UserCacheDir,
		appRoot:     appfs.Root,
		runtimePath: bridgenxs.RuntimePath,
		stat:        os.Stat,
		walkDir:     filepath.WalkDir,
	}
}

// Status 只检查本地已存在的 nxs runtime，不触发下载。
func (s *Service) Status() RuntimeStatus {
	service := s.withDefaults()
	if status, ok := service.statusFromEnv(); ok {
		return status
	}
	if status, ok := service.statusFromAppRoot(); ok {
		return status
	}
	if status, ok := service.statusFromCache(); ok {
		return status
	}
	return RuntimeStatus{
		Available:   false,
		CanDownload: true,
		Message:     "当前未找到可用 nxs runtime，可以下载后再切换。",
	}
}

// Download 通过 bridge resolver 下载并缓存 nxs runtime。
func (s *Service) Download() (RuntimeStatus, error) {
	service := s.withDefaults()
	if status, ok := service.statusFromEnv(); ok {
		if status.Available {
			return status, nil
		}
		return status, errors.New(status.Message)
	}
	path, err := service.runtimePath()
	if err != nil {
		return RuntimeStatus{
			Available:   false,
			CanDownload: true,
			Message:     fmt.Sprintf("nxs runtime 下载失败：%v", err),
		}, err
	}
	if !service.isExecutable(path) {
		err := fmt.Errorf("downloaded nxs runtime is not executable: %s", path)
		return RuntimeStatus{
			Available:   false,
			CanDownload: true,
			Message:     "nxs runtime 下载完成但文件不可执行。",
		}, err
	}
	return RuntimeStatus{
		Available:   true,
		Path:        filepath.Clean(path),
		Source:      "cache",
		CanDownload: false,
	}, nil
}

func (s *Service) withDefaults() *Service {
	if s == nil {
		return NewService()
	}
	result := *s
	if result.getenv == nil {
		result.getenv = os.Getenv
	}
	if result.userCache == nil {
		result.userCache = os.UserCacheDir
	}
	if result.appRoot == nil {
		result.appRoot = appfs.Root
	}
	if result.runtimePath == nil {
		result.runtimePath = bridgenxs.RuntimePath
	}
	if result.stat == nil {
		result.stat = os.Stat
	}
	if result.walkDir == nil {
		result.walkDir = filepath.WalkDir
	}
	return &result
}

func (s *Service) statusFromEnv() (RuntimeStatus, bool) {
	path := strings.TrimSpace(s.getenv(commandPathEnvName))
	if path == "" {
		return RuntimeStatus{}, false
	}
	if s.isExecutable(path) {
		return RuntimeStatus{
			Available:   true,
			Path:        filepath.Clean(path),
			Source:      "env",
			CanDownload: false,
		}, true
	}
	return RuntimeStatus{
		Available:   false,
		Path:        filepath.Clean(path),
		Source:      "env",
		CanDownload: false,
		Message:     "NEXUS_NXS_COMMAND_PATH 指向的 nxs 不可执行，请修正或清空后再下载。",
	}, true
}

func (s *Service) statusFromAppRoot() (RuntimeStatus, bool) {
	path := filepath.Join(strings.TrimSpace(s.appRoot()), "bin", executableName())
	if !s.isExecutable(path) {
		return RuntimeStatus{}, false
	}
	return RuntimeStatus{
		Available:   true,
		Path:        filepath.Clean(path),
		Source:      "app_root",
		CanDownload: false,
	}, true
}

func (s *Service) statusFromCache() (RuntimeStatus, bool) {
	root := strings.TrimSpace(s.getenv(cacheDirEnvName))
	if root == "" {
		cacheDir, err := s.userCache()
		if err != nil || strings.TrimSpace(cacheDir) == "" {
			return RuntimeStatus{}, false
		}
		root = filepath.Join(cacheDir, cacheDirName, "runtimes", "nxs")
	}
	path := s.findCachedRuntime(root)
	if path == "" {
		return RuntimeStatus{}, false
	}
	cleanPath := filepath.Clean(path)
	return RuntimeStatus{
		Available:   true,
		Path:        cleanPath,
		Source:      "cache",
		CanDownload: false,
	}, true
}

func (s *Service) findCachedRuntime(root string) string {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return ""
	}
	var found string
	_ = s.walkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found != "" || entry == nil || entry.IsDir() || entry.Name() != executableName() {
			return nil
		}
		if s.isExecutable(path) {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func (s *Service) isExecutable(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := s.stat(path)
	if err != nil || info == nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "nxs.exe"
	}
	return "nxs"
}
