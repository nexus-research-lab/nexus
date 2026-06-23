package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWebDistDir(t *testing.T) {
	t.Parallel()

	t.Run("explicit dir wins", func(t *testing.T) {
		t.Parallel()
		got, explicit := resolveWebDistDir("  /tmp/web-dist  ", "/unused")
		if got != "/tmp/web-dist" {
			t.Fatalf("resolveWebDistDir explicit dir = %q, want /tmp/web-dist", got)
		}
		if !explicit {
			t.Fatal("resolveWebDistDir explicit flag = false, want true")
		}
	})

	t.Run("default package dist", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		distDir := filepath.Join(root, "web", "dist")
		if err := os.MkdirAll(distDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
			t.Fatal(err)
		}

		got, explicit := resolveWebDistDir("", root)
		if got != distDir {
			t.Fatalf("resolveWebDistDir default dir = %q, want %q", got, distDir)
		}
		if explicit {
			t.Fatal("resolveWebDistDir explicit flag = true, want false")
		}
	})

	t.Run("missing default dist", func(t *testing.T) {
		t.Parallel()
		got, explicit := resolveWebDistDir("", t.TempDir())
		if got != "" {
			t.Fatalf("resolveWebDistDir missing default = %q, want empty", got)
		}
		if explicit {
			t.Fatal("resolveWebDistDir explicit flag = true, want false")
		}
	})
}

func TestIsAPIRequestPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawPath   string
		apiPrefix string
		want      bool
	}{
		{name: "api prefix root", rawPath: "/nexus/v1", apiPrefix: "/nexus/v1", want: true},
		{name: "api child path", rawPath: "/nexus/v1/health", apiPrefix: "/nexus/v1", want: true},
		{name: "api sibling path", rawPath: "/nexus/v10/health", apiPrefix: "/nexus/v1", want: false},
		{name: "web route", rawPath: "/app", apiPrefix: "/nexus/v1", want: false},
		{name: "empty prefix", rawPath: "/nexus/v1/health", apiPrefix: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAPIRequestPath(tt.rawPath, tt.apiPrefix); got != tt.want {
				t.Fatalf("isAPIRequestPath(%q, %q) = %v, want %v", tt.rawPath, tt.apiPrefix, got, tt.want)
			}
		})
	}
}

func TestCleanWebRequestPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawPath string
		want    string
	}{
		{name: "root", rawPath: "/", want: ""},
		{name: "app route", rawPath: "/app", want: "app"},
		{name: "asset", rawPath: "/assets/index.js", want: "assets/index.js"},
		{name: "path traversal collapsed", rawPath: "/assets/../index.html", want: "index.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cleanWebRequestPath(tt.rawPath); got != tt.want {
				t.Fatalf("cleanWebRequestPath(%q) = %q, want %q", tt.rawPath, got, tt.want)
			}
		})
	}
}

func TestWebFallbackFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		relativePath string
		want         string
	}{
		{name: "root app entry", relativePath: "", want: "index.html"},
		{name: "settings entry", relativePath: "settings", want: "settings.html"},
		{
			name:         "oauth callback entry",
			relativePath: "capability/connectors/oauth/callback",
			want:         "oauth-callback.html",
		},
		{name: "asset miss keeps index", relativePath: "assets/missing.js", want: "index.html"},
		{name: "app route", relativePath: "app", want: "app.html"},
		{name: "room route", relativePath: "rooms/r1", want: "app.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webFallbackFileName(tt.relativePath); got != tt.want {
				t.Fatalf("webFallbackFileName(%q) = %q, want %q", tt.relativePath, got, tt.want)
			}
		})
	}
}

func TestWebStaticRequestKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		relativePath string
		targetPath   string
		usedFallback bool
		want         string
	}{
		{
			name:         "html fallback",
			relativePath: "app",
			targetPath:   "/dist/app.html",
			usedFallback: true,
			want:         "html_fallback",
		},
		{
			name:         "asset",
			relativePath: "assets/app.js",
			targetPath:   "/dist/assets/app.js",
			usedFallback: false,
			want:         "asset",
		},
		{
			name:         "html file",
			relativePath: "app.html",
			targetPath:   "/dist/app.html",
			usedFallback: false,
			want:         "html_file",
		},
		{
			name:         "file",
			relativePath: "favicon.ico",
			targetPath:   "/dist/favicon.ico",
			usedFallback: false,
			want:         "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webStaticRequestKind(tt.relativePath, tt.targetPath, tt.usedFallback); got != tt.want {
				t.Fatalf("webStaticRequestKind(%q, %q, %v) = %q, want %q", tt.relativePath, tt.targetPath, tt.usedFallback, got, tt.want)
			}
		})
	}
}

func TestWebStaticCacheControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		relativePath string
		usedFallback bool
		want         string
	}{
		{name: "hashed asset", relativePath: "assets/app.123.js", want: "public, max-age=31536000, immutable"},
		{name: "html file", relativePath: "app.html", want: "no-cache"},
		{name: "fallback route", relativePath: "rooms/r1", usedFallback: true, want: "no-cache"},
		{name: "root fallback", relativePath: "", usedFallback: true, want: "no-cache"},
		{name: "plain file", relativePath: "logo.webp", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webStaticCacheControl(tt.relativePath, tt.usedFallback); got != tt.want {
				t.Fatalf("webStaticCacheControl(%q, %v) = %q, want %q", tt.relativePath, tt.usedFallback, got, tt.want)
			}
		})
	}
}
