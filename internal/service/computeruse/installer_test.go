package computeruse

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestCUAStateRootAnchorsBesideSQLiteDatabase(t *testing.T) {
	databaseDir := t.TempDir()
	installer := NewCUAInstaller(config.Config{
		DatabaseURL: "sqlite://" + filepath.Join(databaseDir, "nexus.db"),
	})
	expected := filepath.Join(databaseDir, "computer-use")
	if installer.root != expected {
		t.Fatalf("installer root = %q, want %q", installer.root, expected)
	}
	supervisor := NewProcessSupervisor(config.Config{
		DatabaseURL: filepath.Join(databaseDir, "nexus.db"),
	})
	if supervisor.stateRoot != expected {
		t.Fatalf("supervisor stateRoot = %q, want %q", supervisor.stateRoot, expected)
	}
}

func TestCUAInstallerDownloadsVerifiesAndPersistsRelease(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("native desktop installer requires macOS or Windows")
	}
	archiveName := "nexus-cua-v0.1.0-" + currentCUAPlatform() + "-" + currentCUAArchitecture() + ".tar.gz"
	binary := []byte("nexuscua-test-binary")
	archive := makeCUATarGz(t, binary, "release_accepted")
	digest := sha256.Sum256(archive)
	checksum := hex.EncodeToString(digest[:]) + "  " + archiveName + "\n"
	server := newCUAReleaseServer(t, archiveName, archive, checksum)
	defer server.Close()

	installer := NewCUAInstallerWithOptions(CUAInstallerOptions{
		Root:          t.TempDir(),
		ReleaseAPIURL: server.URL + "/latest",
		Client:        server.Client(),
	})
	result, err := installer.Install(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "0.1.0" || result.Architecture != currentCUAArchitecture() {
		t.Fatalf("install result = %#v", result)
	}
	installed, err := os.ReadFile(result.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(binary) {
		t.Fatalf("installed binary = %q", installed)
	}
	if installer.Status().Enabled {
		t.Fatal("installation must not enable CUA")
	}
	if err := installer.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !installer.Enabled() {
		t.Fatal("enabled state was not persisted")
	}
	reloaded := NewCUAInstallerWithOptions(CUAInstallerOptions{Root: installer.root})
	if !reloaded.Enabled() || reloaded.InstalledBinaryPath() == "" {
		t.Fatal("reloaded installer lost its state")
	}
	if _, err := installer.Install(context.Background()); err != nil || !installer.Enabled() {
		t.Fatalf("idempotent installation did not preserve enablement: %v", err)
	}
	if err := os.WriteFile(result.BinaryPath, []byte("different-runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(context.Background()); !errors.Is(err, ErrCUAReleaseRejected) {
		t.Fatalf("conflicting same-version installation was accepted: %v", err)
	}
	retained, err := os.ReadFile(result.BinaryPath)
	if err != nil || string(retained) != "different-runtime" {
		t.Fatalf("failed installation replaced the existing version: %v", err)
	}
}

func TestCUAInstallerRejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("native desktop installer requires macOS or Windows")
	}
	archiveName := "nexus-cua-v0.1.0-" + currentCUAPlatform() + "-" + currentCUAArchitecture() + ".tar.gz"
	archive := makeCUATarGz(t, []byte("nexuscua"), "release_accepted")
	server := newCUAReleaseServer(t, archiveName, archive, fmt.Sprintf("%064x  %s\n", 0, archiveName))
	defer server.Close()
	root := t.TempDir()
	installer := NewCUAInstallerWithOptions(CUAInstallerOptions{
		Root:            root,
		ReleaseAPIURL:   server.URL + "/latest",
		Client:          server.Client(),
		AllowUnaccepted: true,
	})
	if _, err := installer.Install(context.Background()); err == nil || !errors.Is(err, ErrCUAChecksumMismatch) {
		t.Fatalf("Install error = %v, want checksum mismatch", err)
	}
	if installer.InstalledBinaryPath() != "" {
		t.Fatal("checksum failure left an installed binary")
	}
}

func makeCUATarGz(t *testing.T, binary []byte, releaseState string) []byte {
	t.Helper()
	manifest := cuaManifest{
		Contract:       "nexus.cua.release-candidate.v1",
		ReleaseState:   releaseState,
		CandidateKind:  "release",
		Version:        "0.1.0",
		Protocol:       "nexus.cua.v1",
		SourceRevision: "dacde8d",
		Platform:       currentCUAPlatform(),
		Architecture:   currentCUAArchitecture(),
		Files: map[string]struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		}{"runtime": {Path: currentCUABinaryName(), SHA256: sha256Hex(binary)}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	write := func(name string, data []byte, mode int64) {
		t.Helper()
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", manifestBytes, 0o600)
	write(currentCUABinaryName(), binary, 0o700)
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func newCUAReleaseServer(t *testing.T, archiveName string, archive []byte, checksum string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest":
			_ = json.NewEncoder(writer).Encode(githubRelease{
				TagName: "v0.1.0",
				HTMLURL: "https://github.com/nexus-research-lab/nexus-cua/releases/tag/v0.1.0",
				Assets: []githubReleaseAsset{
					{Name: archiveName, URL: server.URL + "/archive"},
					{Name: archiveName + ".sha256", URL: server.URL + "/checksum"},
				},
			})
		case "/archive":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(archive)
		case "/checksum":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(checksum))
		default:
			http.NotFound(writer, request)
		}
	}))
	return server
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func TestCUAAssetsMatchPlatformAndArchitecture(t *testing.T) {
	names := []string{
		"nexuscua-0.1.0-macos-universal2.tar.gz",
		"nexuscua-0.1.0-windows-x86_64.zip",
		"nexuscua-0.1.0-windows-arm64.zip",
	}
	var assets []githubReleaseAsset
	for _, name := range names {
		assets = append(assets, githubReleaseAsset{Name: name, URL: "https://example.test/" + name},
			githubReleaseAsset{Name: name + ".sha256", URL: "https://example.test/" + name + ".sha256"})
	}
	for _, test := range []struct{ platform, architecture, name string }{
		{"macos", "arm64", names[0]},
		{"windows", "x86_64", names[1]},
		{"windows", "arm64", names[2]},
	} {
		archive, _, err := selectCUAAssets(assets, test.platform, test.architecture)
		if err != nil || archive.Name != test.name {
			t.Fatalf("%s/%s selected %q: %v", test.platform, test.architecture, archive.Name, err)
		}
	}
}

func TestCUAZipIncludesBoundedLicenseInventory(t *testing.T) {
	binary := []byte("native-runtime")
	gzipReader, err := gzip.NewReader(bytes.NewReader(makeCUATarGz(t, binary, "release_accepted")))
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		writer, err := zipWriter.Create(header.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(writer, tarReader); err != nil {
			t.Fatal(err)
		}
	}
	for index := range 200 {
		writer, err := zipWriter.Create(fmt.Sprintf("licenses/dependency-%d/LICENSE.txt", index))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte("license notice")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "runtime.zip")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, executable, err := extractCUAArchive(path, t.TempDir(), "v0.1.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if !isExecutableRegularFile(executable) {
		t.Fatal("extracted runtime was not recognized as executable")
	}
	manifest.Platform = "unsupported"
	if err := validateCUAManifest(manifest, "v0.1.0", executable, false); !errors.Is(err, ErrCUAReleaseRejected) {
		t.Fatalf("wrong-platform manifest was accepted: %v", err)
	}
	manifest.Platform = currentCUAPlatform()
	delete(manifest.Files, "runtime")
	if err := validateCUAManifest(manifest, "v0.1.0", executable, false); !errors.Is(err, ErrCUAReleaseRejected) {
		t.Fatalf("manifest without a runtime digest was accepted: %v", err)
	}
	for _, name := range []string{"../outside", "C:relative.exe", "runtime.exe:stream"} {
		if _, err := safeArchivePath(name); err == nil {
			t.Fatalf("unsafe archive path accepted: %q", name)
		}
	}
}

func TestConfiguredCUAEnablementSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, currentCUABinaryName())
	if err := os.WriteFile(binary, []byte("configured-runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DatabaseURL: filepath.Join(root, "nexus.db"), ComputerUseBinaryPath: binary}
	service := NewService(cfg)
	if !service.InstallationStatus().Installed {
		t.Fatal("configured runtime was not recognized")
	}
	if err := service.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(cfg)
	if !restarted.Enabled() {
		t.Fatal("configured runtime enablement was lost on restart")
	}
	if err := restarted.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if NewService(cfg).Enabled() {
		t.Fatal("disabled configured runtime was enabled on restart")
	}
}
