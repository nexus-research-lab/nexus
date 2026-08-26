// INPUT: sidecar-private screenshot metadata and the current Agent workspace boundary.
// OUTPUT: checksum-verified round-scoped screenshot projection without private sidecar paths.
// POS: artifact trust and filesystem confinement boundary between nexus-cua and Agent runtime.
package computeruse

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const maxScreenshotArtifactBytes = 128 << 20

func (supervisor *Supervisor) readArtifact(epoch uint64, artifact nexuscua.ScreenshotArtifact) ([]byte, error) {
	if supervisor == nil || strings.TrimSpace(artifact.Path) == "" {
		return nil, errors.New("Computer Use screenshot artifact is unavailable")
	}
	supervisor.mu.Lock()
	process := supervisor.process
	if process == nil || process.epoch != epoch {
		supervisor.mu.Unlock()
		return nil, ErrEpochChanged
	}
	artifactRoot := process.artifactRoot
	supervisor.mu.Unlock()
	relative, err := filepath.Rel(artifactRoot, filepath.Clean(artifact.Path))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, errors.New("Computer Use screenshot escaped the private artifact root")
	}
	root, err := confinedfs.Open(artifactRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.OpenFileNoSymlink(filepath.ToSlash(relative), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxScreenshotArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxScreenshotArtifactBytes || uint64(len(content)) != artifact.ByteLength {
		return nil, errors.New("Computer Use screenshot size does not match metadata")
	}
	digest := sha256.Sum256(content)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), artifact.SHA256) {
		return nil, errors.New("Computer Use screenshot checksum mismatch")
	}
	return content, nil
}

func projectScreenshot(
	supervisor *Supervisor,
	actorWorkspace string,
	roundSegment string,
	epoch uint64,
	artifact nexuscua.ScreenshotArtifact,
) (string, error) {
	actorWorkspace = cleanOptionalPath(actorWorkspace)
	if actorWorkspace == "" || actorWorkspace == "." {
		return "", errors.New("Computer Use round has no trusted Agent workspace")
	}
	content, err := supervisor.readArtifact(epoch, artifact)
	if err != nil {
		return "", err
	}
	root, err := confinedfs.Open(actorWorkspace)
	if err != nil {
		return "", fmt.Errorf("open Agent workspace for Computer Use artifact: %w", err)
	}
	defer root.Close()
	extension := ".bin"
	switch strings.ToLower(strings.TrimSpace(artifact.MIMEType)) {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	}
	name := safeArtifactSegment(string(artifact.ArtifactRef)) + extension
	relative := filepath.ToSlash(filepath.Join(".nexus", "computer-use", roundSegment, name))
	if err = root.WriteFileAtomic(relative, content, 0o600); err != nil {
		return "", err
	}
	return filepath.Join(actorWorkspace, filepath.FromSlash(relative)), nil
}

func removeRoundArtifacts(actorWorkspace, roundSegment string) {
	actorWorkspace = cleanOptionalPath(actorWorkspace)
	if actorWorkspace == "" || actorWorkspace == "." {
		return
	}
	root, err := confinedfs.Open(actorWorkspace)
	if err != nil {
		return
	}
	defer root.Close()
	_ = root.RemoveAll(filepath.ToSlash(filepath.Join(".nexus", "computer-use", roundSegment)))
}

func safeArtifactSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "screenshot"
	}
	digest := sha256.Sum256([]byte(value))
	return "screenshot-" + hex.EncodeToString(digest[:8])
}
