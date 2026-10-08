// INPUT: 当前 sidecar 自身的 Go 构建身份及可执行文件位置。
// OUTPUT: 与自身固定 Bridge 依赖匹配的随包 helper，缺失或替换依赖则失败。
// POS: 正式 App 装配入口；不从任务环境或资源清单推导可信版本。
package runtimebootstrap

import (
	"errors"
	"os"
	"runtime/debug"
)

// LoadCurrent 以正在运行的 sidecar 编译依赖为版本权威。
func LoadCurrent() (Artifact, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Artifact{}, errors.New("sidecar build identity unavailable")
	}
	version, err := pinnedBridgeVersion(info)
	if err != nil {
		return Artifact{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return Artifact{}, err
	}
	return Load(executable, version)
}

func pinnedBridgeVersion(info *debug.BuildInfo) (string, error) {
	for _, dep := range info.Deps {
		if dep.Path != bridgeModule {
			continue
		}
		if dep.Replace != nil || dep.Version == "" || dep.Version == "(devel)" {
			return "", errors.New("sidecar requires a pinned, unreplaced Bridge dependency")
		}
		return dep.Version, nil
	}
	return "", errors.New("sidecar Bridge dependency unavailable")
}
