package appdelta

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := bytes.Repeat([]byte("stable!"), blockSize/7+1)
	old = old[:blockSize]
	next := append(bytes.Clone(old), []byte("new installer trailer")...)
	old[100] = 0
	for _, test := range []struct {
		name         string
		base, target []byte
	}{
		{"diff and literal", old, next}, {"copy and literal", next[:blockSize], next}, {"shrink", next, old[:4096]},
	} {
		t.Run(test.name, func(t *testing.T) {
			open := func(name string, data []byte) *os.File {
				t.Helper()
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				f, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { f.Close() })
				return f
			}
			base := open("base", test.base)
			target := open("target", test.target)
			var patch, out bytes.Buffer
			if err := Create(base, target, &patch); err != nil {
				t.Fatal(err)
			}
			if err := Apply(base, bytes.NewReader(patch.Bytes()), &out); err != nil || !bytes.Equal(test.target, out.Bytes()) {
				t.Fatal("reconstruction mismatch", err)
			}
			if err := Apply(base, bytes.NewReader(patch.Bytes()[:patch.Len()-1]), &bytes.Buffer{}); err == nil {
				t.Fatal("accepted incomplete patch")
			}
		})
	}
}

func TestBaselineRejectsInvalidReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "installer")
	cache := filepath.Join(dir, "cache")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte("original")))
	if err := Remember(cache, path, sum); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Remember(cache, path, sum); err == nil {
		t.Fatal("accepted invalid package")
	}
	got, err := os.ReadFile(filepath.Join(cache, "base"))
	if err != nil || string(got) != "original" {
		t.Fatal("lost verified baseline", err)
	}
}

func TestDownloadVerifyReconstruct(t *testing.T) {
	dir := t.TempDir()
	base := []byte("old baseline")
	next := []byte("new release package")
	basePath := filepath.Join(dir, "base")
	target := filepath.Join(dir, "target")
	output := filepath.Join(dir, "downloaded")
	if err := os.WriteFile(basePath, base, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, next, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := os.Open(basePath)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var patch bytes.Buffer
	if err = Create(a, b, &patch); err != nil {
		t.Fatal(err)
	}
	asset := fmt.Sprintf("/NexusSetup.exe.from-%x.delta", sha256.Sum256(base))
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case asset:
			w.Write(patch.Bytes())
		case asset + ".sha256":
			fmt.Fprintf(w, "%x  patch", sha256.Sum256(patch.Bytes()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()
	url := remote.URL + "/NexusSetup.exe"
	if err = reconstruct(remote.Client(), dir, url, output, fmt.Sprintf("%x", sha256.Sum256(next))); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(next, got) {
		t.Fatal("incorrect installer", err)
	}
	if err = reconstruct(remote.Client(), dir, url, output, fmt.Sprintf("%x", sha256.Sum256(base))); err == nil {
		t.Fatal("incorrect target checksum accepted")
	}
}
