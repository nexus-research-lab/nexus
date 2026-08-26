// INPUT: a checksum-verified release archive and its closed manifest entry.
// OUTPUT: exactly one regular nexus-cua executable in the private staging directory.
// POS: archive traversal, symlink, duplicate-entry, and expansion boundary.
package computeruse

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const (
	maxBinaryBytes         = 256 << 20
	maxArchiveEntries      = 4096
	maxArchiveExpandedSize = 512 << 20
)

func extractPackageBinary(archivePath string, artifact ReleaseArtifact, targetPath string) error {
	entryPath, err := normalizedArchiveEntry(artifact.BinaryPath)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(artifact.Format)) {
	case "tar.gz", "tgz":
		return extractTarGzipBinary(archivePath, entryPath, targetPath)
	case "zip":
		return extractZipBinary(archivePath, entryPath, targetPath)
	default:
		return fmt.Errorf("unsupported Computer Use package format %q", artifact.Format)
	}
}

func extractTarGzipBinary(archivePath, entryPath, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open Computer Use tar.gz: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	found := false
	entries := 0
	var expandedSize int64
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("read Computer Use tar.gz: %w", nextErr)
		}
		entries++
		if entries > maxArchiveEntries || header.Size < 0 || header.Size > maxArchiveExpandedSize-expandedSize {
			return errors.New("Computer Use archive exceeds the expansion bound")
		}
		expandedSize += header.Size
		name, normalizeErr := normalizedArchiveEntry(header.Name)
		if normalizeErr != nil {
			return normalizeErr
		}
		if name != entryPath {
			continue
		}
		if found || header.Typeflag != tar.TypeReg || header.Size < 1 || header.Size > maxBinaryBytes {
			return errors.New("Computer Use archive executable entry is invalid")
		}
		if err := writeBoundedExecutable(targetPath, reader, header.Size); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return errors.New("Computer Use archive does not contain the declared executable")
	}
	return nil
}

func extractZipBinary(archivePath, entryPath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open Computer Use zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveEntries {
		return errors.New("Computer Use zip contains too many entries")
	}
	found := false
	for _, entry := range reader.File {
		name, normalizeErr := normalizedArchiveEntry(entry.Name)
		if normalizeErr != nil {
			return normalizeErr
		}
		if name != entryPath {
			continue
		}
		if found || !entry.Mode().IsRegular() || entry.UncompressedSize64 < 1 || entry.UncompressedSize64 > maxBinaryBytes {
			return errors.New("Computer Use zip executable entry is invalid")
		}
		content, openErr := entry.Open()
		if openErr != nil {
			return openErr
		}
		writeErr := writeBoundedExecutable(targetPath, content, int64(entry.UncompressedSize64))
		closeErr := content.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		found = true
	}
	if !found {
		return errors.New("Computer Use archive does not contain the declared executable")
	}
	return nil
}

func normalizedArchiveEntry(value string) (string, error) {
	normalized := path.Clean(strings.ReplaceAll(strings.TrimSpace(value), `\`, "/"))
	if normalized == "." || normalized == "" || path.IsAbs(normalized) ||
		normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", errors.New("Computer Use archive contains an unsafe path")
	}
	return normalized, nil
}

func writeBoundedExecutable(targetPath string, source io.Reader, expected int64) error {
	if expected < 1 || expected > maxBinaryBytes {
		return errors.New("Computer Use executable exceeds the size bound")
	}
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, maxBinaryBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != expected || written > maxBinaryBytes {
		return errors.New("Computer Use executable size does not match the archive entry")
	}
	return nil
}
