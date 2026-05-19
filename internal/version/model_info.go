package version

import (
	"runtime"
	"strings"
)

const (
	// ProjectName 是本地版本信息的项目标识。
	ProjectName = "nexus"

	// ReleasePageURL 指向用户手动下载发布包的入口。
	ReleasePageURL = "https://github.com/nexus-research-lab/nexus/releases/latest"
)

var (
	// AppVersion 由 release 构建通过 ldflags 注入。
	AppVersion = "dev"
	// GitCommit 由 release 构建通过 ldflags 注入。
	GitCommit = ""
	// BuildDate 由 release 构建通过 ldflags 注入。
	BuildDate = ""
)

// Info 描述当前二进制的版本与平台信息。
type Info struct {
	Project    string `json:"project"`
	Version    string `json:"version"`
	GitCommit  string `json:"git_commit,omitempty"`
	BuildDate  string `json:"build_date,omitempty"`
	GoOS       string `json:"goos"`
	GoArch     string `json:"goarch"`
	Target     string `json:"target"`
	ReleaseURL string `json:"release_url"`
}

// Current 返回当前进程的版本信息。
func Current() Info {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	return Info{
		Project:    ProjectName,
		Version:    normalizeVersion(AppVersion),
		GitCommit:  strings.TrimSpace(GitCommit),
		BuildDate:  strings.TrimSpace(BuildDate),
		GoOS:       goos,
		GoArch:     goarch,
		Target:     TargetFor(goos, goarch),
		ReleaseURL: ReleasePageURL,
	}
}

// TargetFor 返回发布包使用的平台标识。
func TargetFor(goos string, goarch string) string {
	return strings.TrimSpace(goos) + "-" + strings.TrimSpace(goarch)
}

func normalizeVersion(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "dev"
	}
	return strings.TrimPrefix(value, "v")
}
