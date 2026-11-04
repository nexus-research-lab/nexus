// INPUT: Native Windows verbatim paths and the host-owned artifact root.
// OUTPUT: Equivalent path spellings remain readable while outside files fail.
// POS: Windows screenshot delivery regression coverage.
package computeruse

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestScreenshotArtifactAcceptsWindowsVerbatimPaths(t *testing.T) {
	service, _, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	path := filepath.Join(supervisor.artifact, "frame.png")
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, mime, err := round.readArtifact(`\\?\` + path)
	if err != nil || mime != "image/png" || encoded != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("verbatim screenshot read failed: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, png, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := round.readArtifact(`\\?\` + outside); err == nil {
		t.Fatal("verbatim prefix admitted an outside artifact")
	}
}
