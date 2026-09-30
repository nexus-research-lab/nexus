// INPUT: 可信 sidecar 路径和产品固定的 Bridge 模块版本。
// OUTPUT: 通过随包清单/内容/Go 构建身份校验的 helper 路径与 SHA256。
// POS: App 资源身份校验，不以清单替代代码签名或原生接口可用性证明。
package runtimebootstrap

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const bridgeModule = "github.com/nexus-research-lab/nexus-agent-sdk-bridge"
const helperRelative = "bin/nexus-runtime-bootstrap"

type Manifest struct {
	Version       int    `json:"version"`
	RelativePath  string `json:"relative_path"`
	BridgeVersion string `json:"bridge_version"`
	Architecture  string `json:"architecture"`
	SHA256        string `json:"sha256"`
}

type Artifact struct {
	Path     string
	Manifest Manifest
}

// Load 只从 sidecar 同 bundle 的固定资源位置读取；不接受任务提供的替代路径。
func Load(sidecarPath, expectedBridgeVersion string) (Artifact, error) {
	var zero Artifact
	if !filepath.IsAbs(sidecarPath) || expectedBridgeVersion == "" || !strings.HasPrefix(expectedBridgeVersion, "v") {
		return zero, errors.New("trusted sidecar and canonical Bridge version required")
	}
	canonical, err := filepath.EvalSymlinks(sidecarPath)
	if err != nil {
		return zero, err
	}
	macos := filepath.Dir(canonical)
	contents := filepath.Dir(macos)
	if filepath.Base(canonical) != "nexus-server" || filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" {
		return zero, errors.New("sidecar is not in canonical App bundle")
	}
	root, err := confinedfs.Open(contents)
	if err != nil {
		return zero, err
	}
	defer root.Close()
	resources, err := root.OpenRootNoSymlink("Resources")
	if err != nil {
		return zero, err
	}
	defer resources.Close()
	file, err := resources.OpenFileNoSymlink("runtime-bootstrap.json", os.O_RDONLY, 0)
	if err != nil {
		return zero, err
	}
	bytes, err := io.ReadAll(io.LimitReader(file, 4097))
	closeErr := file.Close()
	if err := errors.Join(err, closeErr); err != nil {
		return zero, err
	}
	if len(bytes) > 4096 {
		return zero, errors.New("bootstrap manifest exceeds limit")
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return zero, fmt.Errorf("invalid bootstrap manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return zero, errors.New("bootstrap manifest has trailing content")
	}
	if manifest.Version != 1 || manifest.RelativePath != helperRelative || manifest.BridgeVersion != expectedBridgeVersion || manifest.Architecture != runtime.GOARCH || len(manifest.SHA256) != 64 || strings.ToLower(manifest.SHA256) != manifest.SHA256 {
		return zero, errors.New("bootstrap manifest identity mismatch")
	}
	if _, err := hex.DecodeString(manifest.SHA256); err != nil {
		return zero, errors.New("invalid bootstrap digest")
	}
	helper, err := resources.OpenFileNoSymlink(helperRelative, os.O_RDONLY, 0)
	if err != nil {
		return zero, err
	}
	defer helper.Close()
	info, err := helper.Stat()
	if err != nil {
		return zero, err
	}
	if info.Mode()&0111 == 0 {
		return zero, errors.New("bootstrap is not executable")
	}
	build, err := buildinfo.Read(helper)
	if err != nil {
		return zero, fmt.Errorf("read bootstrap build identity: %w", err)
	}
	if build.Path != bridgeModule+"/cmd/nexus-runtime-bootstrap" || build.Main.Path != bridgeModule || build.Main.Version != expectedBridgeVersion || build.Main.Replace != nil {
		return zero, errors.New("bootstrap Bridge build identity mismatch")
	}
	settings := map[string]string{}
	for _, setting := range build.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["CGO_ENABLED"] != "1" || settings["GOOS"] != "darwin" || settings["GOARCH"] != manifest.Architecture {
		return zero, errors.New("bootstrap is not matching native macOS cgo build")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, helper); err != nil {
		return zero, err
	}
	if hex.EncodeToString(digest.Sum(nil)) != manifest.SHA256 {
		return zero, errors.New("bootstrap binary digest mismatch")
	}
	// 校验后确认名称仍绑定相同 inode；启动器放行前仍会重新校验固定摘要。
	current, err := resources.Lstat(helperRelative)
	if err != nil {
		return zero, err
	}
	if !os.SameFile(info, current) || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return zero, errors.New("bootstrap changed during verification")
	}
	return Artifact{Path: filepath.Join(resources.Name(), helperRelative), Manifest: manifest}, nil
}
