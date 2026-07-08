package protocol

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestGeneratedTypeScriptInSync 防止 web/src/types/generated/protocol.ts 与协议定义漂移。
func TestGeneratedTypeScriptInSync(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	generatedPath := filepath.Join(repoRoot, "web", "src", "types", "generated", "protocol.ts")
	content, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("读取生成文件失败: %v", err)
	}
	if string(content) != TypeScriptDefinitions() {
		t.Fatal("web/src/types/generated/protocol.ts 与协议定义不一致，请运行 go generate ./internal/protocol")
	}
}
