package appfs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const appRootEnvName = "NEXUS_APP_ROOT"

var (
	appRootOnce sync.Once
	appRootPath string
)

// Root 返回运行时可用的应用根目录。
func Root() string {
	appRootOnce.Do(func() {
		appRootPath = resolveRoot()
	})
	return appRootPath
}

func resolveRoot() string {
	candidates := []string{}
	if envRoot := strings.TrimSpace(os.Getenv(appRootEnvName)); envRoot != "" {
		candidates = append(candidates, envRoot)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates,
			executableDir,
			filepath.Dir(executableDir),
			filepath.Dir(filepath.Dir(executableDir)),
		)
	}
	candidates = append(candidates, sourceRoot())
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if looksLikeAppRoot(clean) {
			return clean
		}
	}
	return filepath.Clean(sourceRoot())
}

func looksLikeAppRoot(root string) bool {
	skillsInfo, skillErr := os.Stat(filepath.Join(root, "skills"))
	if skillErr != nil || !skillsInfo.IsDir() {
		return false
	}
	if dbInfo, dbErr := os.Stat(filepath.Join(root, "db")); dbErr == nil && dbInfo.IsDir() {
		return true
	}
	if _, goModErr := os.Stat(filepath.Join(root, "go.mod")); goModErr == nil {
		return true
	}
	return false
}

func sourceRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	start := filepath.Dir(file)
	for current := start; current != filepath.Dir(current); current = filepath.Dir(current) {
		if looksLikeAppRoot(current) {
			return filepath.Clean(current)
		}
	}
	return filepath.Clean(start)
}
