package computeruse

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCUAInstallerDownloadsVerifiesAndPersistsRelease(t *testing.T) {
	archiveName := "nexus-cua-v0.1.0-macos-universal-release.tar.gz"
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
	if result.Version != "0.1.0" || result.Architecture != "universal2" {
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
}

func TestCUAInstallerRejectsChecksumMismatch(t *testing.T) {
	archiveName := "nexus-cua-v0.1.0-macos-universal-release.tar.gz"
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
		Platform:       "macos",
		Architecture:   "universal2",
		Files: map[string]struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		}{"runtime": {Path: "nexuscua", SHA256: sha256Hex(binary)}},
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
	write("nexuscua", binary, 0o700)
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
