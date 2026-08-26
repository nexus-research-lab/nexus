// INPUT: host-pinned manifest identity, release artifacts, and managed package root.
// OUTPUT: verified side-by-side nexus-cua installs and an atomic current-version marker.
// POS: executable supply-chain boundary; the runtime itself never downloads code.
package computeruse

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	maxManifestBytes = 8 << 20
	maxArchiveBytes  = 512 << 20
)

var packageVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.-]{0,127}$`)

type PackageConfig struct {
	Available      bool
	Root           string
	CommandPath    string
	TargetVersion  string
	ManifestURL    string
	ManifestSHA256 string
	GOOS           string
	GOARCH         string
	HTTPClient     *http.Client
	Now            func() time.Time
}

type BinarySelection struct {
	Path            string
	Version         string
	ProtocolVersion string
	Source          PackageSource
}

type PackageManager struct {
	config PackageConfig
	mu     sync.Mutex
	cache  *verifiedBinaryCache
}

type verifiedBinaryCache struct {
	selection BinarySelection
	size      int64
	modified  time.Time
	digest    string
}

func NewPackageManager(config PackageConfig) *PackageManager {
	config.Root = cleanOptionalPath(config.Root)
	config.CommandPath = cleanOptionalPath(config.CommandPath)
	config.TargetVersion = strings.TrimSpace(config.TargetVersion)
	config.ManifestURL = strings.TrimSpace(config.ManifestURL)
	config.ManifestSHA256 = strings.ToLower(strings.TrimSpace(config.ManifestSHA256))
	if config.GOOS == "" {
		config.GOOS = runtime.GOOS
	}
	if config.GOARCH == "" {
		config.GOARCH = runtime.GOARCH
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &PackageManager{config: config}
}

func (manager *PackageManager) Status(ctx context.Context) PackageStatus {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.statusLocked(ctx)
}

func (manager *PackageManager) statusLocked(ctx context.Context) PackageStatus {
	status := PackageStatus{
		Available:     manager.config.Available,
		TargetVersion: manager.config.TargetVersion,
		Platform:      normalizedReleaseOS(manager.config.GOOS),
		Architecture:  normalizedReleaseArchitecture(manager.config.GOARCH),
	}
	if !manager.config.Available {
		status.Message = ErrUnavailable.Error()
		return status
	}
	if manager.config.CommandPath != "" {
		status.Source = PackageSourceOverride
		status.ExecutablePath = manager.config.CommandPath
		version, err := manager.cachedOverrideVersionLocked(ctx, false)
		if err != nil {
			status.Message = PublicErrorMessage(err)
			return status
		}
		status.Installed = true
		status.Version = version
		status.ProtocolVersion = ProtocolVersion
		status.Message = "Computer Use executable is managed by NEXUS_CUA_COMMAND_PATH"
		return status
	}
	selection, err := manager.currentManagedBinaryLocked(false)
	if err == nil {
		status.Installed = true
		status.Source = PackageSourceManaged
		status.Version = selection.Version
		status.ProtocolVersion = selection.ProtocolVersion
		status.ExecutablePath = selection.Path
		status.CanRemove = true
	}
	installConfigured := validSHA256(manager.config.ManifestSHA256) && manager.config.ManifestURL != ""
	status.CanInstall = !status.Installed && installConfigured
	status.CanUpdate = status.Installed && status.Version != status.TargetVersion && installConfigured
	if err != nil && !errors.Is(err, ErrNotInstalled) {
		status.Message = PublicErrorMessage(err)
	} else if !installConfigured && !status.Installed {
		status.Message = "signed Computer Use release manifest is not configured"
	} else if status.CanUpdate {
		status.Message = "a pinned Computer Use update is available"
	}
	return status
}

func (manager *PackageManager) CurrentBinary(ctx context.Context) (BinarySelection, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if !manager.config.Available {
		return BinarySelection{}, ErrUnavailable
	}
	if manager.config.CommandPath != "" {
		version, err := manager.cachedOverrideVersionLocked(ctx, true)
		if err != nil {
			return BinarySelection{}, err
		}
		return BinarySelection{
			Path: manager.config.CommandPath, Version: version,
			ProtocolVersion: ProtocolVersion, Source: PackageSourceOverride,
		}, nil
	}
	return manager.currentManagedBinaryLocked(true)
}

func (manager *PackageManager) currentManagedBinaryLocked(forceVerify bool) (BinarySelection, error) {
	marker, err := readJSONFile[currentPackage](filepath.Join(manager.config.Root, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return BinarySelection{}, ErrNotInstalled
	}
	if err != nil {
		return BinarySelection{}, fmt.Errorf("read Computer Use current package: %w", err)
	}
	if marker.SchemaVersion != InstallSchemaVersion || !validPackageVersion(marker.Version) {
		return BinarySelection{}, errors.New("Computer Use current package marker is invalid")
	}
	versionRoot := filepath.Join(manager.config.Root, "versions", marker.Version)
	record, err := readJSONFile[installRecord](filepath.Join(versionRoot, "install.json"))
	if err != nil {
		return BinarySelection{}, fmt.Errorf("read Computer Use install record: %w", err)
	}
	if err := validateInstallRecord(record, marker.Version); err != nil {
		return BinarySelection{}, err
	}
	binaryPath := filepath.Join(versionRoot, record.BinaryName)
	info, err := os.Stat(binaryPath)
	if err != nil || !info.Mode().IsRegular() {
		return BinarySelection{}, errors.New("installed Computer Use executable is missing")
	}
	selection := BinarySelection{
		Path: binaryPath, Version: record.Version, ProtocolVersion: record.ProtocolVersion,
		Source: PackageSourceManaged,
	}
	if !forceVerify && manager.cache != nil && manager.cache.selection == selection &&
		manager.cache.size == info.Size() && manager.cache.modified.Equal(info.ModTime()) &&
		strings.EqualFold(manager.cache.digest, record.BinarySHA256) {
		return selection, nil
	}
	actual, err := fileSHA256(binaryPath)
	if err != nil {
		return BinarySelection{}, err
	}
	if !strings.EqualFold(actual, record.BinarySHA256) {
		return BinarySelection{}, errors.New("installed Computer Use executable checksum mismatch")
	}
	manager.cache = &verifiedBinaryCache{
		selection: selection, size: info.Size(), modified: info.ModTime(), digest: actual,
	}
	return selection, nil
}

func (manager *PackageManager) Install(ctx context.Context) (PackageStatus, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if !manager.config.Available {
		return PackageStatus{}, ErrUnavailable
	}
	if manager.config.CommandPath != "" {
		return PackageStatus{}, ErrExternallyManaged
	}
	if !validPackageVersion(manager.config.TargetVersion) {
		return PackageStatus{}, errors.New("Computer Use target version is invalid")
	}
	if !validSHA256(manager.config.ManifestSHA256) {
		return PackageStatus{}, errors.New("signed Computer Use manifest digest is not configured")
	}
	manifestBytes, err := manager.fetchBytes(ctx, manager.config.ManifestURL, maxManifestBytes)
	if err != nil {
		return PackageStatus{}, fmt.Errorf("download Computer Use manifest: %w", err)
	}
	if digestBytes(manifestBytes) != manager.config.ManifestSHA256 {
		return PackageStatus{}, errors.New("Computer Use manifest checksum mismatch")
	}
	manifest, err := decodeReleaseManifest(manifestBytes)
	if err != nil {
		return PackageStatus{}, err
	}
	if manifest.Version != manager.config.TargetVersion || manifest.ProtocolVersion != ProtocolVersion {
		return PackageStatus{}, errors.New("Computer Use manifest version or protocol does not match the pinned target")
	}
	artifact, err := selectReleaseArtifact(manifest.Artifacts, manager.config.GOOS, manager.config.GOARCH)
	if err != nil {
		return PackageStatus{}, err
	}
	if err := os.MkdirAll(filepath.Join(manager.config.Root, "versions"), 0o700); err != nil {
		return PackageStatus{}, fmt.Errorf("create Computer Use package root: %w", err)
	}
	stageRoot, err := os.MkdirTemp(manager.config.Root, ".install-")
	if err != nil {
		return PackageStatus{}, fmt.Errorf("create Computer Use staging directory: %w", err)
	}
	defer os.RemoveAll(stageRoot)
	if err := os.Chmod(stageRoot, 0o700); err != nil && runtime.GOOS != "windows" {
		return PackageStatus{}, err
	}
	archivePath := filepath.Join(stageRoot, "package.archive")
	archiveDigest, err := manager.downloadFile(ctx, artifact.URL, archivePath, maxArchiveBytes)
	if err != nil {
		return PackageStatus{}, fmt.Errorf("download Computer Use package: %w", err)
	}
	if !strings.EqualFold(archiveDigest, artifact.ArchiveSHA256) {
		return PackageStatus{}, errors.New("Computer Use archive checksum mismatch")
	}
	binaryName := executableName(manager.config.GOOS)
	binaryPath := filepath.Join(stageRoot, binaryName)
	if err := extractPackageBinary(archivePath, artifact, binaryPath); err != nil {
		return PackageStatus{}, err
	}
	binaryDigest, err := fileSHA256(binaryPath)
	if err != nil {
		return PackageStatus{}, err
	}
	if !strings.EqualFold(binaryDigest, artifact.BinarySHA256) {
		return PackageStatus{}, errors.New("Computer Use executable checksum mismatch")
	}
	version, err := inspectCLIVersion(ctx, binaryPath)
	if err != nil {
		return PackageStatus{}, err
	}
	if version != manifest.Version {
		return PackageStatus{}, fmt.Errorf("Computer Use executable version %q does not match manifest %q", version, manifest.Version)
	}
	doctor, err := inspectDoctor(ctx, binaryPath)
	if err != nil {
		return PackageStatus{}, err
	}
	if doctor.ProtocolVersion != ProtocolVersion {
		return PackageStatus{}, ErrProtocolMismatch
	}
	record := installRecord{
		SchemaVersion: InstallSchemaVersion, Version: manifest.Version,
		ProtocolVersion: manifest.ProtocolVersion, Platform: normalizedReleaseOS(manager.config.GOOS),
		Architecture: normalizedReleaseArchitecture(manager.config.GOARCH), BinaryName: binaryName,
		BinarySHA256: strings.ToLower(artifact.BinarySHA256), ArchiveSHA256: strings.ToLower(artifact.ArchiveSHA256),
		InstalledAt: manager.config.Now().UTC(), Source: PackageSourceManaged,
	}
	if err := writeJSONAtomic(filepath.Join(stageRoot, "install.json"), record, 0o600); err != nil {
		return PackageStatus{}, err
	}
	if err := os.Remove(archivePath); err != nil {
		return PackageStatus{}, err
	}
	versionRoot := filepath.Join(manager.config.Root, "versions", manifest.Version)
	if _, err := os.Stat(versionRoot); err == nil {
		existing, readErr := readJSONFile[installRecord](filepath.Join(versionRoot, "install.json"))
		if readErr != nil || existing.BinarySHA256 != record.BinarySHA256 || existing.ArchiveSHA256 != record.ArchiveSHA256 {
			return PackageStatus{}, errors.New("Computer Use version directory already exists with different verified content")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return PackageStatus{}, err
	} else if err := os.Rename(stageRoot, versionRoot); err != nil {
		return PackageStatus{}, fmt.Errorf("activate Computer Use version directory: %w", err)
	}
	marker := currentPackage{SchemaVersion: InstallSchemaVersion, Version: manifest.Version}
	if err := writeJSONAtomic(filepath.Join(manager.config.Root, "current.json"), marker, 0o600); err != nil {
		return PackageStatus{}, err
	}
	manager.cache = nil
	return manager.statusLocked(ctx), nil
}

func (manager *PackageManager) Remove() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.config.CommandPath != "" {
		return ErrExternallyManaged
	}
	if manager.config.Root == "" || manager.config.Root == "." || filepath.Dir(manager.config.Root) == manager.config.Root {
		return errors.New("refusing to remove invalid Computer Use package root")
	}
	if err := os.RemoveAll(manager.config.Root); err != nil {
		return fmt.Errorf("remove Computer Use packages: %w", err)
	}
	manager.cache = nil
	return nil
}

func (manager *PackageManager) cachedOverrideVersionLocked(ctx context.Context, forceVerify bool) (string, error) {
	info, err := os.Stat(manager.config.CommandPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("Computer Use executable override is unavailable")
	}
	if !forceVerify && manager.cache != nil && manager.cache.selection.Source == PackageSourceOverride &&
		manager.cache.selection.Path == manager.config.CommandPath && manager.cache.size == info.Size() &&
		manager.cache.modified.Equal(info.ModTime()) {
		return manager.cache.selection.Version, nil
	}
	version, err := inspectCLIVersion(ctx, manager.config.CommandPath)
	if err != nil {
		return "", errors.New("Computer Use executable override failed version validation")
	}
	manager.cache = &verifiedBinaryCache{
		selection: BinarySelection{
			Path: manager.config.CommandPath, Version: version,
			ProtocolVersion: ProtocolVersion, Source: PackageSourceOverride,
		},
		size: info.Size(), modified: info.ModTime(),
	}
	return version, nil
}

func (manager *PackageManager) Doctor(ctx context.Context) (DoctorReport, error) {
	status := manager.Status(ctx)
	report := DoctorReport{Package: status}
	selection, err := manager.CurrentBinary(ctx)
	if err != nil {
		report.Message = PublicErrorMessage(err)
		return report, err
	}
	doctor, err := inspectDoctor(ctx, selection.Path)
	if err != nil {
		report.Message = PublicErrorMessage(err)
		return report, err
	}
	report.Healthy = doctor.ProtocolVersion == ProtocolVersion
	report.RuntimeVersion = doctor.RuntimeVersion
	report.ProtocolVersion = doctor.ProtocolVersion
	report.Platform = doctor.Platform
	report.Capabilities = doctor.Capabilities
	report.Permissions = doctor.Permissions
	if !report.Healthy {
		report.Message = ErrProtocolMismatch.Error()
		return report, ErrProtocolMismatch
	}
	return report, nil
}

type doctorOutput struct {
	ProtocolVersion string            `json:"protocol_version"`
	RuntimeVersion  string            `json:"-"`
	Platform        string            `json:"-"`
	Capabilities    map[string]any    `json:"capabilities"`
	Permissions     map[string]string `json:"permissions"`
}

func inspectDoctor(ctx context.Context, binary string) (doctorOutput, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(commandCtx, binary, "doctor", "--compact")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return doctorOutput{}, fmt.Errorf("run Computer Use doctor: %w: %s", err, boundedText(stderr.String(), 2048))
	}
	decoder := json.NewDecoder(io.LimitReader(&stdout, maxManifestBytes))
	decoder.UseNumber()
	var output doctorOutput
	if err := decoder.Decode(&output); err != nil {
		return doctorOutput{}, fmt.Errorf("decode Computer Use doctor output: %w", err)
	}
	if output.ProtocolVersion == "" || output.Capabilities == nil || output.Permissions == nil {
		return doctorOutput{}, errors.New("Computer Use doctor output is incomplete")
	}
	output.RuntimeVersion = stringMapValue(output.Capabilities, "runtime_version")
	output.Platform = stringMapValue(output.Capabilities, "platform")
	return output, nil
}

func inspectCLIVersion(ctx context.Context, binary string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, binary, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read Computer Use version: %w: %s", err, boundedText(string(output), 1024))
	}
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) < 2 || fields[0] != "nexus-cua" || !validPackageVersion(fields[len(fields)-1]) {
		return "", errors.New("Computer Use version output is invalid")
	}
	return fields[len(fields)-1], nil
}

func (manager *PackageManager) fetchBytes(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if err := validatePackageURL(rawURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := manager.doPackageRequest(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, errors.New("download exceeds configured size bound")
	}
	return payload, nil
}

func (manager *PackageManager) downloadFile(ctx context.Context, rawURL, path string, limit int64) (string, error) {
	if err := validatePackageURL(rawURL); err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	response, err := manager.doPackageRequest(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	completed := false
	defer func() {
		_ = file.Close()
		if !completed {
			_ = os.Remove(path)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", err
	}
	if written > limit {
		return "", errors.New("download exceeds configured size bound")
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	completed = true
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (manager *PackageManager) doPackageRequest(request *http.Request) (*http.Response, error) {
	client := *manager.config.HTTPClient
	configuredRedirect := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("Computer Use package download exceeded the redirect limit")
		}
		if err := validatePackageURL(next.URL.String()); err != nil {
			return err
		}
		if configuredRedirect != nil {
			return configuredRedirect(next, via)
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.Request == nil || response.Request.URL == nil {
		response.Body.Close()
		return nil, errors.New("Computer Use package response has no final URL")
	}
	if err := validatePackageURL(response.Request.URL.String()); err != nil {
		response.Body.Close()
		return nil, err
	}
	return response, nil
}

func decodeReleaseManifest(payload []byte) (ReleaseManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var manifest ReleaseManifest
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("decode Computer Use manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return manifest, err
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || !validPackageVersion(manifest.Version) ||
		manifest.ProtocolVersion == "" || len(manifest.Artifacts) == 0 {
		return manifest, errors.New("Computer Use manifest is incomplete")
	}
	for _, artifact := range manifest.Artifacts {
		if !validSHA256(artifact.ArchiveSHA256) || !validSHA256(artifact.BinarySHA256) ||
			strings.TrimSpace(artifact.BinaryPath) == "" {
			return manifest, errors.New("Computer Use manifest contains an invalid artifact")
		}
		if err := validatePackageURL(artifact.URL); err != nil {
			return manifest, err
		}
	}
	return manifest, nil
}

func selectReleaseArtifact(artifacts []ReleaseArtifact, goos, goarch string) (ReleaseArtifact, error) {
	wantOS := normalizedReleaseOS(goos)
	wantArch := normalizedReleaseArchitecture(goarch)
	var exact, universal []ReleaseArtifact
	for _, artifact := range artifacts {
		if strings.ToLower(strings.TrimSpace(artifact.OS)) != wantOS {
			continue
		}
		arch := strings.ToLower(strings.TrimSpace(artifact.Architecture))
		if arch == wantArch {
			exact = append(exact, artifact)
		} else if wantOS == "macos" && arch == "universal" {
			universal = append(universal, artifact)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) == 0 && len(universal) == 1 {
		return universal[0], nil
	}
	return ReleaseArtifact{}, fmt.Errorf("Computer Use manifest has no unique artifact for %s/%s", wantOS, wantArch)
}

func validateInstallRecord(record installRecord, expectedVersion string) error {
	if record.SchemaVersion != InstallSchemaVersion || record.Version != expectedVersion ||
		record.ProtocolVersion != ProtocolVersion || !validPackageVersion(record.Version) ||
		filepath.Base(record.BinaryName) != record.BinaryName || !validSHA256(record.BinarySHA256) {
		return errors.New("Computer Use install record is invalid")
	}
	return nil
}

func validatePackageURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("Computer Use package URL is invalid")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := strings.TrimSpace(parsed.Hostname())
	if parsed.Scheme == "http" && (strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()) {
		return nil
	}
	return errors.New("Computer Use package URL must use HTTPS")
}

func normalizedReleaseOS(goos string) string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin", "macos":
		return "macos"
	case "windows":
		return "windows"
	default:
		return strings.ToLower(strings.TrimSpace(goos))
	}
}

func normalizedReleaseArchitecture(goarch string) string {
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	default:
		return strings.ToLower(strings.TrimSpace(goarch))
	}
}

func executableName(goos string) string {
	if normalizedReleaseOS(goos) == "windows" {
		return "nexus-cua.exe"
	}
	return "nexus-cua"
}

func validPackageVersion(value string) bool {
	return packageVersionPattern.MatchString(strings.TrimSpace(value))
}

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
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

func readJSONFile[T any](path string) (T, error) {
	var result T
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxManifestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return result, err
	}
	return result, nil
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryPath, path)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("JSON document must contain exactly one value")
	}
	return nil
}

func stringMapValue(value map[string]any, key string) string {
	result, _ := value[key].(string)
	return strings.TrimSpace(result)
}

func boundedText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

func cleanOptionalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}
