package appdelta

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// 写入同目录临时文件，只有完整摘要匹配才替换已有文件。
func verifiedFile(path, expected string, write func(io.Writer) error) error {
	if digest, e := hex.DecodeString(expected); e != nil || len(digest) != sha256.Size {
		return errors.New("invalid package checksum")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".nexus-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	out := bufio.NewWriter(io.MultiWriter(f, hash))
	err = write(out)
	if err == nil {
		err = out.Flush()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !strings.EqualFold(expected, hex.EncodeToString(hash.Sum(nil))) {
		return errors.New("update checksum mismatch")
	}
	return os.Rename(f.Name(), path)
}

// Remember 只保留一个已校验的完整安装包作为后续版本的差分基准。
func Remember(cache, packagePath, expected string) error {
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}
	src, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer src.Close()
	return verifiedFile(filepath.Join(cache, "base"), expected, func(w io.Writer) error { _, err := io.Copy(w, src); return err })
}

func get(client *http.Client, address string) (io.ReadCloser, error) {
	res, err := client.Get(address)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("delta asset: %s", res.Status)
	}
	return res.Body, nil
}

func Reconstruct(cache, address, target, expected string) error {
	return reconstruct(&http.Client{Timeout: 10 * time.Minute}, cache, address, target, expected)
}

func reconstruct(client *http.Client, cache, address, target, expected string) error {
	base, err := os.Open(filepath.Join(cache, "base"))
	if err != nil {
		return err
	}
	defer base.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, base); err != nil {
		return err
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("invalid package URL")
	}
	u.Path += ".from-" + hex.EncodeToString(hash.Sum(nil)) + ".delta"
	u.RawPath = ""
	sumURL := *u
	sumURL.Path += ".sha256"
	sums, err := get(client, sumURL.String())
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(sums, 4096))
	sums.Close()
	if err != nil {
		return err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return errors.New("missing delta checksum")
	}
	digest, err := hex.DecodeString(fields[0])
	if err != nil || len(digest) != sha256.Size {
		return errors.New("invalid delta checksum")
	}
	payload, err := get(client, u.String())
	if err != nil {
		return err
	}
	defer payload.Close()
	patch, err := os.CreateTemp(filepath.Dir(target), ".nexus-delta-*")
	if err != nil {
		return err
	}
	defer os.Remove(patch.Name())
	defer patch.Close()
	patchHash := sha256.New()
	n, err := io.Copy(io.MultiWriter(patch, patchHash), io.LimitReader(payload, maxPackage+1))
	if err != nil {
		return err
	}
	if n > maxPackage || !strings.EqualFold(fields[0], hex.EncodeToString(patchHash.Sum(nil))) {
		return errors.New("delta checksum mismatch")
	}
	if _, err = patch.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return verifiedFile(target, expected, func(w io.Writer) error { return Apply(base, bufio.NewReader(patch), w) })
}

// Adopt 在旧更新缓存清理前保留与当前版本、构建号和架构相符的安装包。
func Adopt(cache, version, build string) error {
	if _, err := os.Stat(filepath.Join(cache, "update-base", "base")); err == nil {
		return nil
	}
	version, _, _ = strings.Cut(version, "+")
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) || !regexp.MustCompile(`^[0-9]+$`).MatchString(build) {
		return errors.New("invalid build identity")
	}
	name := "NexusSetup-" + version + "-" + build + ".exe"
	if runtime.GOOS == "darwin" {
		arch := "arm64"
		if runtime.GOARCH == "amd64" {
			arch = "intel"
		}
		name = "Nexus-macos-" + arch + "-" + version + "-" + build + ".dmg"
	}
	updates := filepath.Join(cache, "updates")
	dirs, err := os.ReadDir(updates)
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		candidate := filepath.Join(updates, d.Name(), name)
		raw, err := os.ReadFile(candidate + ".sha256")
		if err != nil {
			continue
		}
		fields := strings.Fields(string(raw))
		if len(fields) > 0 && Remember(filepath.Join(cache, "update-base"), candidate, fields[0]) == nil {
			return nil
		}
	}
	return os.ErrNotExist
}
