// INPUT: GitHub nexus-cua release metadata, archive bytes, checksum, and the
// local Nexus state root.
// OUTPUT: A verified macOS or Windows CUA binary installed into a private
// versioned directory, with a small durable enablement record.
// POS: Computer Use distribution boundary; it never grants desktop permissions
// and never downloads executable code from an unverified source.
package computeruse

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

const (
	maxReleaseMetadataBytes = 4 << 20
	maxArchiveBytes         = 512 << 20
	maxArchiveMembers       = 2048
	maxArchiveMemberBytes   = 256 << 20
	maxManifestBytes        = 256 << 10
	maxDownloadTimeout      = 15 * time.Minute
	maxRequestTimeout       = 30 * time.Second
)

var (
	ErrCUANotInstalled     = errors.New("Computer Use 尚未安装")
	ErrCUANoRelease        = errors.New("GitHub 暂无可用的 nexus-cua Release")
	ErrCUAReleaseRejected  = errors.New("nexus-cua Release 未通过安装校验")
	ErrCUAChecksumMismatch = errors.New("nexus-cua 安装包 checksum 校验失败")
	semverPattern          = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	sha256Pattern          = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// CUAInstallationStatus is the browser-safe host installation projection.
// It deliberately omits the private binary path, endpoint, and token.
type CUAInstallationStatus struct {
	Installed    bool   `json:"installed"`
	Enabled      bool   `json:"enabled"`
	Version      string `json:"version,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Source       string `json:"source,omitempty"`
	ReleaseURL   string `json:"release_url,omitempty"`
	Message      string `json:"message,omitempty"`
}

type CUAInstallResult struct {
	Status       CUAInstallationStatus
	BinaryPath   string
	Version      string
	Architecture string
}

// CUAInstallerOptions configures one installer. ExpectedVersion and
// ArchiveSHA256 default to the source-reviewed pin; AllowUnaccepted is a
// development escape hatch that also bypasses both pins.
type CUAInstallerOptions struct {
	Root            string
	ReleaseAPIURL   string
	Client          *http.Client
	AllowUnaccepted bool
	ExpectedVersion string
	ArchiveSHA256   map[string]string
}

type CUAInstaller struct {
	mu              sync.Mutex
	root            string
	releaseAPIURL   string
	client          *http.Client
	allowUnaccepted bool
	expectedVersion string
	archiveSHA256   map[string]string
}

func NewCUAInstaller(cfg config.Config) *CUAInstaller {
	root := cuaStateRoot(cfg)
	releaseURL := strings.TrimSpace(os.Getenv("NEXUS_CUA_RELEASE_API_URL"))
	if releaseURL == "" {
		releaseURL = pinnedCUAReleaseAPIURL()
	}
	return NewCUAInstallerWithOptions(CUAInstallerOptions{
		Root:            root,
		ReleaseAPIURL:   releaseURL,
		Client:          &http.Client{Timeout: maxDownloadTimeout},
		AllowUnaccepted: os.Getenv("NEXUS_CUA_ALLOW_UNACCEPTED_RELEASE") == "1",
	})
}

func NewCUAInstallerWithOptions(options CUAInstallerOptions) *CUAInstaller {
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = filepath.Join(os.TempDir(), "nexus-computer-use")
	}
	releaseURL := strings.TrimSpace(options.ReleaseAPIURL)
	if releaseURL == "" {
		releaseURL = pinnedCUAReleaseAPIURL()
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: maxDownloadTimeout}
	}
	expectedVersion := normalizeCUAVersion(options.ExpectedVersion)
	if expectedVersion == "" {
		expectedVersion = pinnedCUAVersion
	}
	archiveSHA256 := options.ArchiveSHA256
	if archiveSHA256 == nil {
		archiveSHA256 = pinnedCUAArchiveSHA256
	}
	return &CUAInstaller{
		root:            filepath.Clean(root),
		releaseAPIURL:   releaseURL,
		client:          client,
		allowUnaccepted: options.AllowUnaccepted,
		expectedVersion: expectedVersion,
		archiveSHA256:   archiveSHA256,
	}
}

func (i *CUAInstaller) InstalledBinaryPath() string {
	state, err := i.readState()
	if err != nil || !state.Installed || state.Version == "" || state.Architecture == "" {
		return ""
	}
	path := filepath.Join(i.root, "versions", state.Version+"-"+state.Architecture, currentCUABinaryName())
	if !isExecutableRegularFile(path) {
		return ""
	}
	return path
}

func (i *CUAInstaller) Status() CUAInstallationStatus {
	state, err := i.readState()
	status := CUAInstallationStatus{Architecture: currentCUAArchitecture()}
	if err != nil {
		status.Message = "CUA 安装状态无法读取"
		return status
	}
	status.Enabled = state.Enabled
	status.Version = state.Version
	status.Architecture = state.Architecture
	status.Source = state.Source
	status.ReleaseURL = state.ReleaseURL
	if i.InstalledBinaryPath() != "" {
		status.Installed = true
	} else {
		status.Enabled = false
		status.Installed = false
		if status.Message == "" {
			status.Message = "尚未安装 nexus-cua"
		}
	}
	return status
}

func (i *CUAInstaller) Enabled() bool {
	state, err := i.readState()
	return err == nil && state.Enabled && i.InstalledBinaryPath() != ""
}

func (i *CUAInstaller) SetEnabled(enabled bool) error {
	return i.setEnabledForBinary(enabled, i.InstalledBinaryPath())
}

func (i *CUAInstaller) setEnabledForBinary(enabled bool, binaryPath string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	state, err := i.readState()
	if err != nil {
		return err
	}
	if enabled && !isExecutableRegularFile(binaryPath) {
		return ErrCUANotInstalled
	}
	state.Enabled = enabled
	return i.writeState(state)
}

func (i *CUAInstaller) Install(ctx context.Context) (CUAInstallResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return CUAInstallResult{}, fmt.Errorf("Computer Use 安装器不支持当前平台: %s", runtime.GOOS)
	}
	if err := os.MkdirAll(i.root, 0o700); err != nil {
		return CUAInstallResult{}, err
	}
	if err := os.Chmod(i.root, 0o700); err != nil {
		return CUAInstallResult{}, err
	}
	release, err := i.fetchRelease(ctx)
	if err != nil {
		return CUAInstallResult{}, err
	}
	archiveAsset, checksumAsset, err := selectCUAAssets(release.Assets, currentCUAPlatform(), currentCUAArchitecture())
	if err != nil {
		return CUAInstallResult{}, err
	}
	archivePath, err := i.download(ctx, archiveAsset.URL, "archive")
	if err != nil {
		return CUAInstallResult{}, err
	}
	defer os.Remove(archivePath)
	checksumPath, err := i.download(ctx, checksumAsset.URL, "checksum")
	if err != nil {
		return CUAInstallResult{}, err
	}
	defer os.Remove(checksumPath)
	checksum, err := parseChecksumFile(checksumPath, archiveAsset.Name)
	if err != nil {
		return CUAInstallResult{}, err
	}
	actual, err := fileSHA256(archivePath)
	if err != nil {
		return CUAInstallResult{}, err
	}
	if !strings.EqualFold(actual, checksum) {
		return CUAInstallResult{}, fmt.Errorf("%w: expected=%s actual=%s", ErrCUAChecksumMismatch, checksum, actual)
	}
	// The release's own checksum only proves transport integrity. Accepted
	// installs must also match the digest reviewed in source.
	if !i.allowUnaccepted {
		pinned, ok := i.archiveSHA256[archiveAsset.Name]
		if !ok {
			return CUAInstallResult{}, fmt.Errorf("%w: %s 没有源码固定的 digest", ErrCUAReleaseRejected, archiveAsset.Name)
		}
		if !strings.EqualFold(actual, pinned) {
			return CUAInstallResult{}, fmt.Errorf("%w: pinned=%s actual=%s", ErrCUAChecksumMismatch, pinned, actual)
		}
	}

	staging, err := os.MkdirTemp(i.root, ".staging-")
	if err != nil {
		return CUAInstallResult{}, err
	}
	defer os.RemoveAll(staging)
	manifest, binaryPath, err := extractCUAArchive(archivePath, staging, release.TagName, i.allowUnaccepted)
	if err != nil {
		return CUAInstallResult{}, err
	}
	version := normalizeCUAVersion(manifest.Version)
	architecture := normalizeCUAArchitecture(manifest.Architecture)
	if version == "" || architecture == "" {
		return CUAInstallResult{}, fmt.Errorf("%w: manifest version 或 architecture 无效", ErrCUAReleaseRejected)
	}
	versionRoot := filepath.Join(i.root, "versions", version+"-"+architecture)
	if err := os.MkdirAll(filepath.Dir(versionRoot), 0o700); err != nil {
		return CUAInstallResult{}, err
	}
	versionTemp, err := os.MkdirTemp(filepath.Dir(versionRoot), ".version-")
	if err != nil {
		return CUAInstallResult{}, err
	}
	defer os.RemoveAll(versionTemp)
	installedBinary := filepath.Join(versionTemp, currentCUABinaryName())
	if err := copyRegularFile(binaryPath, installedBinary, 0o700); err != nil {
		return CUAInstallResult{}, err
	}
	if existing, err := os.Lstat(versionRoot); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 || !existing.IsDir() {
			return CUAInstallResult{}, fmt.Errorf("%w: 版本目录无效", ErrCUAReleaseRejected)
		}
		existingPath := filepath.Join(versionRoot, currentCUABinaryName())
		digest, err := fileSHA256(existingPath)
		if err != nil || !isExecutableRegularFile(existingPath) || !strings.EqualFold(digest, manifest.Files["runtime"].SHA256) {
			return CUAInstallResult{}, fmt.Errorf("%w: 已有版本与下载内容不一致", ErrCUAReleaseRejected)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return CUAInstallResult{}, err
	} else if err := os.Rename(versionTemp, versionRoot); err != nil {
		return CUAInstallResult{}, err
	}
	state, _ := i.readState()
	state.Installed = true
	state.Version = version
	state.Architecture = architecture
	state.Source = "github_release"
	state.ReleaseURL = release.HTMLURL
	state.SourceRevision = manifest.SourceRevision
	if err := i.writeState(state); err != nil {
		return CUAInstallResult{}, err
	}
	return CUAInstallResult{
		Status:       i.Status(),
		BinaryPath:   filepath.Join(versionRoot, currentCUABinaryName()),
		Version:      version,
		Architecture: architecture,
	}, nil
}

type cuaInstallState struct {
	Installed      bool   `json:"installed"`
	Enabled        bool   `json:"enabled"`
	Version        string `json:"version,omitempty"`
	Architecture   string `json:"architecture,omitempty"`
	Source         string `json:"source,omitempty"`
	ReleaseURL     string `json:"release_url,omitempty"`
	SourceRevision string `json:"source_revision,omitempty"`
}

func (i *CUAInstaller) statePath() string { return filepath.Join(i.root, "state.json") }

func (i *CUAInstaller) readState() (cuaInstallState, error) {
	data, err := os.ReadFile(i.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return cuaInstallState{}, nil
	}
	if err != nil {
		return cuaInstallState{}, err
	}
	if len(data) > maxManifestBytes {
		return cuaInstallState{}, errors.New("Computer Use 安装状态过大")
	}
	var state cuaInstallState
	if err := json.Unmarshal(data, &state); err != nil {
		return cuaInstallState{}, err
	}
	if state.Version != "" && normalizeCUAVersion(state.Version) != state.Version {
		return cuaInstallState{}, errors.New("Computer Use 安装状态版本无效")
	}
	return state, nil
}

func (i *CUAInstaller) writeState(state cuaInstallState) error {
	if err := os.MkdirAll(i.root, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(i.root, ".state-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, i.statePath())
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	HTMLURL    string               `json:"html_url"`
	Draft      bool                 `json:"draft"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (i *CUAInstaller) fetchRelease(ctx context.Context) (githubRelease, error) {
	requestCtx, cancel := context.WithTimeout(ctx, maxRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, i.releaseAPIURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Nexus-CUA-Installer")
	response, err := i.client.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return githubRelease{}, ErrCUANoRelease
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("GitHub Release 请求失败: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseMetadataBytes+1))
	if err != nil {
		return githubRelease{}, err
	}
	if len(data) > maxReleaseMetadataBytes {
		return githubRelease{}, errors.New("GitHub Release metadata 过大")
	}
	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return githubRelease{}, err
	}
	version := normalizeCUAVersion(release.TagName)
	if release.Draft || release.Prerelease || version == "" {
		return githubRelease{}, ErrCUAReleaseRejected
	}
	if !i.allowUnaccepted && version != i.expectedVersion {
		return githubRelease{}, fmt.Errorf("%w: release=%s pinned=%s", ErrCUAReleaseRejected, version, i.expectedVersion)
	}
	return release, nil
}

func (i *CUAInstaller) download(ctx context.Context, rawURL, label string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://") && !i.allowUnaccepted {
		return "", fmt.Errorf("%s 下载地址不是 HTTPS", label)
	}
	requestCtx, cancel := context.WithTimeout(ctx, maxDownloadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Nexus-CUA-Installer")
	response, err := i.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%s 下载失败: HTTP %d", label, response.StatusCode)
	}
	temp, err := os.CreateTemp(i.root, ".download-")
	if err != nil {
		return "", err
	}
	path := temp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(path)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return "", err
	}
	if _, err := io.Copy(temp, io.LimitReader(response.Body, maxArchiveBytes+1)); err != nil {
		temp.Close()
		return "", err
	}
	info, err := temp.Stat()
	if err != nil {
		temp.Close()
		return "", err
	}
	limit := int64(maxArchiveBytes)
	if label == "checksum" {
		limit = maxManifestBytes
	}
	if info.Size() > limit {
		temp.Close()
		return "", fmt.Errorf("%s 超过大小限制", label)
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	success = true
	return path, nil
}

type selectedCUAAsset struct {
	Name string
	URL  string
}

func selectCUAAssets(assets []githubReleaseAsset, platform, architecture string) (selectedCUAAsset, selectedCUAAsset, error) {
	type candidate struct {
		asset githubReleaseAsset
		score int
	}
	var archives []candidate
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !strings.Contains(name, "-"+platform+"-") || !isArchiveName(name) || asset.URL == "" {
			continue
		}
		score, ok := architectureScore(name, architecture)
		if ok && (platform != "windows" || score == 3) {
			archives = append(archives, candidate{asset: asset, score: score})
		}
	}
	if len(archives) == 0 {
		return selectedCUAAsset{}, selectedCUAAsset{}, ErrCUAReleaseRejected
	}
	sort.SliceStable(archives, func(left, right int) bool {
		if archives[left].score != archives[right].score {
			return archives[left].score > archives[right].score
		}
		return archives[left].asset.Name < archives[right].asset.Name
	})
	archive := selectedCUAAsset{Name: archives[0].asset.Name, URL: archives[0].asset.URL}
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, archive.Name+".sha256") {
			return archive, selectedCUAAsset{Name: asset.Name, URL: asset.URL}, nil
		}
	}
	return selectedCUAAsset{}, selectedCUAAsset{}, ErrCUAReleaseRejected
}

func isArchiveName(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".zip")
}

func architectureScore(name, architecture string) (int, bool) {
	architecture = normalizeCUAArchitecture(architecture)
	isUniversal := strings.Contains(name, "universal")
	isARM := strings.Contains(name, "arm64") || strings.Contains(name, "aarch64")
	isIntel := strings.Contains(name, "x86_64") || strings.Contains(name, "amd64") || strings.Contains(name, "intel")
	if architecture == "arm64" {
		if isIntel && !isARM && !isUniversal {
			return 0, false
		}
		if isARM {
			return 3, true
		}
		if isUniversal {
			return 2, true
		}
		return 1, true
	}
	if architecture == "x86_64" {
		if isARM && !isIntel && !isUniversal {
			return 0, false
		}
		if isIntel {
			return 3, true
		}
		if isUniversal {
			return 2, true
		}
		return 1, true
	}
	return 0, false
}

func parseChecksumFile(path, archiveName string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxManifestBytes {
		return "", errors.New("checksum 文件过大")
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 || !sha256Pattern.MatchString(fields[0]) {
			continue
		}
		if len(fields) == 1 || strings.EqualFold(strings.TrimPrefix(fields[len(fields)-1], "*"), archiveName) {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", ErrCUAChecksumMismatch
}

type cuaManifest struct {
	Contract       string `json:"contract"`
	ReleaseState   string `json:"release_state"`
	CandidateKind  string `json:"candidate_kind"`
	Version        string `json:"version"`
	Protocol       string `json:"protocol_version"`
	SourceRevision string `json:"source_revision"`
	Platform       string `json:"platform"`
	Architecture   string `json:"architecture"`
	Files          map[string]struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

func extractCUAArchive(archivePath, destination, tag string, allowUnaccepted bool) (cuaManifest, string, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		// Downloaded files are intentionally extensionless; tar/gzip is the
		// release default. ZIP is selected from the release asset name by the
		// caller in future revisions, while content sniffing keeps this helper
		// deterministic for tests.
	}
	if fileLooksLikeGzip(archivePath) {
		return extractTarGzip(archivePath, destination, tag, allowUnaccepted)
	}
	return extractZip(archivePath, destination, tag, allowUnaccepted)
}

func fileLooksLikeGzip(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	var header [2]byte
	_, err = io.ReadFull(file, header[:])
	return err == nil && header[0] == 0x1f && header[1] == 0x8b
}

func extractTarGzip(path, destination, tag string, allowUnaccepted bool) (cuaManifest, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return cuaManifest{}, "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return cuaManifest{}, "", err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, maxArchiveBytes))
	return extractArchiveEntries(destination, tag, allowUnaccepted, func() (archiveEntry, error) {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return archiveEntry{}, io.EOF
			}
			return archiveEntry{}, err
		}
		return archiveEntry{
			name:   header.Name,
			mode:   header.Mode,
			kind:   header.Typeflag,
			size:   header.Size,
			reader: io.LimitReader(tarReader, maxArchiveMemberBytes+1),
		}, nil
	})
}

func extractZip(path, destination, tag string, allowUnaccepted bool) (cuaManifest, string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return cuaManifest{}, "", err
	}
	defer archive.Close()
	index := 0
	return extractArchiveEntries(destination, tag, allowUnaccepted, func() (archiveEntry, error) {
		if index >= len(archive.File) {
			return archiveEntry{}, io.EOF
		}
		item := archive.File[index]
		index++
		reader, err := item.Open()
		if err != nil {
			return archiveEntry{}, err
		}
		return archiveEntry{
			name:   item.Name,
			mode:   int64(item.Mode()),
			kind:   zipEntryType(item),
			size:   int64(item.UncompressedSize64),
			reader: reader,
			close:  reader.Close,
		}, nil
	})
}

type archiveEntry struct {
	name   string
	mode   int64
	kind   byte
	size   int64
	reader io.Reader
	close  func() error
}

func extractArchiveEntries(destination, tag string, allowUnaccepted bool, next func() (archiveEntry, error)) (cuaManifest, string, error) {
	var manifest cuaManifest
	var manifestFound bool
	var binaryPath string
	memberCount := 0
	var totalBytes int64
	for {
		entry, err := next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return cuaManifest{}, "", err
		}
		memberCount++
		if memberCount > maxArchiveMembers {
			return cuaManifest{}, "", errors.New("nexus-cua archive 文件数量超过上限")
		}
		if entry.close != nil {
			defer entry.close()
		}
		name, err := safeArchivePath(entry.name)
		if err != nil {
			return cuaManifest{}, "", err
		}
		if entry.kind == tar.TypeSymlink || entry.kind == tar.TypeLink || entry.kind == 's' {
			return cuaManifest{}, "", errors.New("nexus-cua archive 不允许链接文件")
		}
		if entry.kind != tar.TypeReg && entry.kind != 0 {
			continue
		}
		if entry.size < 0 || entry.size > maxArchiveMemberBytes {
			return cuaManifest{}, "", errors.New("nexus-cua archive 成员超过大小限制")
		}
		totalBytes += entry.size
		if totalBytes > maxArchiveBytes {
			return cuaManifest{}, "", errors.New("nexus-cua archive 总大小超过上限")
		}
		path := filepath.Join(destination, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return cuaManifest{}, "", err
		}
		if filepath.Base(path) == "manifest.json" {
			if manifestFound {
				return cuaManifest{}, "", errors.New("nexus-cua archive 包含多个 manifest.json")
			}
			data, err := io.ReadAll(io.LimitReader(entry.reader, maxManifestBytes+1))
			if err != nil {
				return cuaManifest{}, "", err
			}
			if len(data) > maxManifestBytes {
				return cuaManifest{}, "", errors.New("nexus-cua manifest 过大")
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return cuaManifest{}, "", err
			}
			manifestFound = true
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return cuaManifest{}, "", err
			}
			continue
		}
		if filepath.Base(path) == currentCUABinaryName() {
			if binaryPath != "" {
				return cuaManifest{}, "", errors.New("nexus-cua archive 包含多个 nexuscua")
			}
			if err := copyReader(entry.reader, path, entry.size, 0o700); err != nil {
				return cuaManifest{}, "", err
			}
			binaryPath = path
			continue
		}
		if err := drainReader(entry.reader, entry.size); err != nil {
			return cuaManifest{}, "", err
		}
	}
	if !manifestFound || binaryPath == "" {
		return cuaManifest{}, "", fmt.Errorf("%w: archive 缺少 manifest.json 或 nexuscua", ErrCUAReleaseRejected)
	}
	if err := validateCUAManifest(manifest, tag, binaryPath, allowUnaccepted); err != nil {
		return cuaManifest{}, "", err
	}
	return manifest, binaryPath, nil
}

func validateCUAManifest(manifest cuaManifest, tag, binaryPath string, allowUnaccepted bool) error {
	if strings.ToLower(manifest.Platform) != currentCUAPlatform() {
		return ErrCUAReleaseRejected
	}
	switch manifest.Contract {
	case "nexus.cua.release.v1":
		// Published by the nexus-cua release pipeline. Acceptance is the
		// archive digest pinned in pin.go, checked before extraction.
		if !allowUnaccepted && manifest.ReleaseState != "published" {
			return fmt.Errorf("%w: release_state=%s", ErrCUAReleaseRejected, manifest.ReleaseState)
		}
	case "nexus.cua.release-candidate.v1":
		if !allowUnaccepted && manifest.ReleaseState != "release_accepted" {
			return fmt.Errorf("%w: release_state=%s", ErrCUAReleaseRejected, manifest.ReleaseState)
		}
	default:
		return ErrCUAReleaseRejected
	}
	if normalizeCUAVersion(manifest.Version) == "" || normalizeCUAVersion(tag) != normalizeCUAVersion(manifest.Version) {
		return ErrCUAReleaseRejected
	}
	architecture := normalizeCUAArchitecture(manifest.Architecture)
	if architecture != currentCUAArchitecture() && !(currentCUAPlatform() == "macos" && architecture == "universal2") {
		return ErrCUAReleaseRejected
	}
	runtimeRecord := manifest.Files["runtime"]
	if !sha256Pattern.MatchString(runtimeRecord.SHA256) || filepath.Base(runtimeRecord.Path) != currentCUABinaryName() {
		return ErrCUAReleaseRejected
	}
	digest, err := fileSHA256(binaryPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(runtimeRecord.SHA256, digest) {
		return ErrCUAChecksumMismatch
	}
	return nil
}

func safeArchivePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := filepath.Clean(filepath.FromSlash(name))
	if name == "" || strings.Contains(name, ":") || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("nexus-cua archive 路径无效")
	}
	return clean, nil
}

func zipEntryType(item *zip.File) byte {
	mode := item.Mode()
	if mode&os.ModeSymlink != 0 {
		return 's'
	}
	if item.FileInfo().IsDir() {
		return tar.TypeDir
	}
	return tar.TypeReg
}

func copyReader(reader io.Reader, destination string, size, mode int64) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, io.LimitReader(reader, size+1)); err != nil {
		return err
	}
	if info, err := file.Stat(); err != nil || info.Size() != size {
		if err != nil {
			return err
		}
		return errors.New("nexus-cua archive 成员大小不匹配")
	}
	return file.Chmod(os.FileMode(mode))
}

func copyRegularFile(source, destination string, mode os.FileMode) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("nexuscua 必须是普通文件")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	return copyReader(input, destination, info.Size(), int64(mode))
}

func drainReader(reader io.Reader, size int64) error {
	_, err := io.Copy(io.Discard, io.LimitReader(reader, size+1))
	return err
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func isExecutableRegularFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(path), ".exe")
	}
	return info.Mode()&0o111 != 0
}

func currentCUAPlatform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func currentCUABinaryName() string {
	if runtime.GOOS == "windows" {
		return "nexuscua.exe"
	}
	return "nexuscua"
}

func normalizeCUAVersion(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	if !semverPattern.MatchString(value) {
		return ""
	}
	return value
}

func currentCUAArchitecture() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

func normalizeCUAArchitecture(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "aarch64", "arm64", "universal", "universal2":
		if strings.HasPrefix(value, "universal") {
			return "universal2"
		}
		return "arm64"
	case "x86_64", "amd64", "intel":
		return "x86_64"
	default:
		return ""
	}
}
