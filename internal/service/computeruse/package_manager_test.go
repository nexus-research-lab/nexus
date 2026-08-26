// INPUT: local release manifests, native helper executables, and in-memory package servers.
// OUTPUT: regression evidence for package selection, checksum, activation, doctor, and removal boundaries.
// POS: managed nexus-cua supply-chain contract tests; never depend on a public release.
package computeruse

import (
	"archive/zip"
	"bytes"
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
	"strings"
	"testing"
	"time"
)

const packageTestVersion = "0.1.0-alpha.1"

func TestMain(m *testing.M) {
	if os.Getenv("NEXUS_CUA_PACKAGE_HELPER") == "1" || os.Getenv("NEXUS_CUA_SIDECAR_HELPER") == "1" {
		for _, argument := range os.Args[1:] {
			switch argument {
			case "--version":
				fmt.Println("nexus-cua " + packageTestVersion)
				os.Exit(0)
			case "doctor":
				fmt.Printf(`{"protocol_version":%q,"capabilities":{"runtime_version":%q,"platform":%q},"permissions":{"screen_capture":"granted","accessibility":"granted","input_control":"granted"}}`, ProtocolVersion, packageTestVersion, normalizedReleaseOS(runtime.GOOS))
				os.Exit(0)
			case "serve":
				if os.Getenv("NEXUS_CUA_SIDECAR_HELPER") == "exit" {
					fmt.Fprintln(os.Stderr, "helper exited before ready")
					os.Exit(9)
				}
				if os.Getenv("NEXUS_CUA_SIDECAR_HELPER") == "1" {
					fmt.Fprintln(os.Stdout, "helper sidecar running")
					for {
						time.Sleep(time.Hour)
					}
				}
			}
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestPackageManagerInstallsPinnedVerifiedArtifactAndRemovesManagedRoot(t *testing.T) {
	t.Setenv("NEXUS_CUA_PACKAGE_HELPER", "1")
	binary, err := os.ReadFile(mustExecutablePath(t))
	if err != nil {
		t.Fatal(err)
	}
	binaryName := executableName(runtime.GOOS)
	archive := zipPackage(t, "release/"+binaryName, binary)
	archiveDigest := checksum(archive)
	binaryDigest := checksum(binary)
	var manifest []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/manifest.json":
			_, _ = writer.Write(manifest)
		case "/package.zip":
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	manifest, err = json.Marshal(ReleaseManifest{
		SchemaVersion: ManifestSchemaVersion, Version: packageTestVersion,
		ProtocolVersion: ProtocolVersion,
		Artifacts: []ReleaseArtifact{{
			OS: normalizedReleaseOS(runtime.GOOS), Architecture: normalizedReleaseArchitecture(runtime.GOARCH),
			URL: server.URL + "/package.zip", Format: "zip", ArchiveSHA256: archiveDigest,
			BinaryPath: "release/" + binaryName, BinarySHA256: binaryDigest,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "managed-computer-use")
	manager := NewPackageManager(PackageConfig{
		Available: true, Root: root, TargetVersion: packageTestVersion,
		ManifestURL: server.URL + "/manifest.json", ManifestSHA256: checksum(manifest),
		HTTPClient: server.Client(),
	})

	before := manager.Status(context.Background())
	if before.Installed || !before.CanInstall {
		t.Fatalf("initial Status() = %+v", before)
	}
	installed, err := manager.Install(context.Background())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !installed.Installed || installed.Version != packageTestVersion || installed.Source != PackageSourceManaged {
		t.Fatalf("installed Status() = %+v", installed)
	}
	report, err := manager.Doctor(context.Background())
	if err != nil || !report.Healthy || report.ProtocolVersion != ProtocolVersion {
		t.Fatalf("Doctor() = %+v, %v", report, err)
	}
	if err = manager.Remove(); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("managed root still exists: %v", err)
	}
}

func TestPackageManagerRejectsManifestDigestBeforeDownloadingArtifact(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()
	manager := NewPackageManager(PackageConfig{
		Available: true, Root: filepath.Join(t.TempDir(), "packages"), TargetVersion: packageTestVersion,
		ManifestURL: server.URL + "/manifest.json", ManifestSHA256: strings.Repeat("0", 64),
		HTTPClient: server.Client(),
	})
	if _, err := manager.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "manifest checksum mismatch") {
		t.Fatalf("Install() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want manifest only", requests)
	}
}

func TestPackageManagerRefusesToRemoveExternalOverride(t *testing.T) {
	executable := mustExecutablePath(t)
	manager := NewPackageManager(PackageConfig{Available: true, CommandPath: executable})
	if err := manager.Remove(); !errors.Is(err, ErrExternallyManaged) {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("external executable changed: %v", err)
	}
}

func TestSelectReleaseArtifactUsesExactArchitectureBeforeMacUniversal(t *testing.T) {
	artifacts := []ReleaseArtifact{
		{OS: "macos", Architecture: "universal", URL: "https://example.invalid/universal"},
		{OS: "macos", Architecture: "aarch64", URL: "https://example.invalid/arm"},
	}
	selected, err := selectReleaseArtifact(artifacts, "darwin", "arm64")
	if err != nil || !strings.HasSuffix(selected.URL, "/arm") {
		t.Fatalf("selectReleaseArtifact() = %+v, %v", selected, err)
	}
}

func TestExtractPackageBinaryRejectsUnsafeDeclaredPath(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "package.zip")
	if err := os.WriteFile(archivePath, zipPackage(t, "nexus-cua", []byte("binary")), 0o600); err != nil {
		t.Fatal(err)
	}
	err := extractPackageBinary(archivePath, ReleaseArtifact{Format: "zip", BinaryPath: "../nexus-cua"}, filepath.Join(t.TempDir(), "nexus-cua"))
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("extractPackageBinary() error = %v", err)
	}
}

func mustExecutablePath(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func zipPackage(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o700)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func checksum(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
