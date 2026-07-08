package server

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// mountWebAppRoutes 托管 Vite 构建产物，保证前端与 API 同源。
func (s *Server) mountWebAppRoutes() {
	webDistDir, explicit := resolveWebDistDir(s.config.WebDistDir, appfs.Root())
	if webDistDir == "" {
		return
	}
	root, err := filepath.Abs(webDistDir)
	if err != nil {
		if explicit {
			s.api.BaseLogger().Warn("Web 产物目录无效，跳过静态托管", "dir", webDistDir, "err", err)
		}
		return
	}
	indexPath := filepath.Join(root, "index.html")
	if info, statErr := os.Stat(indexPath); statErr != nil || info.IsDir() {
		if explicit {
			s.api.BaseLogger().Warn("Web 产物缺少 index.html，跳过静态托管", "dir", root, "err", statErr)
		}
		return
	}

	fileServer := http.FileServer(http.Dir(root))
	handler := func(writer http.ResponseWriter, request *http.Request) {
		if isAPIRequestPath(request.URL.Path, s.config.APIPrefix) {
			http.NotFound(writer, request)
			return
		}
		start := time.Now()
		recorder := &webStaticResponseRecorder{ResponseWriter: writer}
		relativePath := cleanWebRequestPath(request.URL.Path)
		if relativePath == "" {
			targetPath := webFallbackPath(root, relativePath, indexPath)
			setWebStaticCacheHeaders(writer, relativePath, true)
			http.ServeFile(recorder, request, targetPath)
			logWebStaticRequest(s.api.BaseLogger(), request, relativePath, targetPath, true, recorder, time.Since(start))
			return
		}
		targetPath := filepath.Join(root, relativePath)
		if info, statErr := os.Stat(targetPath); statErr == nil && !info.IsDir() {
			setWebStaticCacheHeaders(writer, relativePath, false)
			fileServer.ServeHTTP(recorder, request)
			logWebStaticRequest(s.api.BaseLogger(), request, relativePath, targetPath, false, recorder, time.Since(start))
			return
		}
		targetPath = webFallbackPath(root, relativePath, indexPath)
		setWebStaticCacheHeaders(writer, relativePath, true)
		http.ServeFile(recorder, request, targetPath)
		logWebStaticRequest(s.api.BaseLogger(), request, relativePath, targetPath, true, recorder, time.Since(start))
	}

	s.router.Get("/*", handler)
	s.router.Head("/*", handler)
	s.api.BaseLogger().Info("已启用 Web 静态托管", "dir", root)
}

func resolveWebDistDir(configuredDir string, appRoot string) (string, bool) {
	if webDistDir := strings.TrimSpace(configuredDir); webDistDir != "" {
		return webDistDir, true
	}
	root := strings.TrimSpace(appRoot)
	if root == "" {
		return "", false
	}
	defaultDir := filepath.Join(root, "web", "dist")
	indexPath := filepath.Join(defaultDir, "index.html")
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		return defaultDir, false
	}
	return "", false
}

func webFallbackPath(root string, relativePath string, indexPath string) string {
	fileName := webFallbackFileName(relativePath)
	if fileName == "index.html" {
		return indexPath
	}
	targetPath := filepath.Join(root, fileName)
	if info, statErr := os.Stat(targetPath); statErr == nil && !info.IsDir() {
		return targetPath
	}
	return indexPath
}

func webFallbackFileName(relativePath string) string {
	switch {
	case relativePath == "":
		return "index.html"
	case relativePath == "settings":
		return "settings.html"
	case relativePath == "capability/connectors/oauth/callback":
		return "oauth-callback.html"
	case strings.HasPrefix(relativePath, "assets/"):
		return "index.html"
	default:
		return "app.html"
	}
}

func isAPIRequestPath(rawPath string, apiPrefix string) bool {
	prefix := strings.TrimRight(strings.TrimSpace(apiPrefix), "/")
	if prefix == "" {
		return false
	}
	return rawPath == prefix || strings.HasPrefix(rawPath, prefix+"/")
}

func cleanWebRequestPath(rawPath string) string {
	clean := path.Clean("/" + strings.TrimSpace(rawPath))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." {
		return ""
	}
	return clean
}

type webStaticResponseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (r *webStaticResponseRecorder) WriteHeader(statusCode int) {
	if r.statusCode != 0 {
		return
	}
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *webStaticResponseRecorder) Write(body []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(body)
	r.bytesWritten += n
	return n, err
}

func (r *webStaticResponseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *webStaticResponseRecorder) status() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}

func logWebStaticRequest(
	logger *slog.Logger,
	request *http.Request,
	relativePath string,
	targetPath string,
	usedFallback bool,
	recorder *webStaticResponseRecorder,
	duration time.Duration,
) {
	if logger == nil {
		return
	}
	status := recorder.status()
	fields := []any{
		"method", request.Method,
		"path", request.URL.Path,
		"relative_path", relativePath,
		"kind", webStaticRequestKind(relativePath, targetPath, usedFallback),
		"target", filepath.Base(targetPath),
		"status", status,
		"bytes", recorder.bytesWritten,
		"duration_ms", float64(duration.Microseconds()) / 1000,
	}
	switch {
	case status >= http.StatusInternalServerError:
		logger.Error("桌面 Web 静态资源请求", fields...)
	case status >= http.StatusBadRequest:
		logger.Warn("桌面 Web 静态资源请求", fields...)
	default:
		logger.Debug("桌面 Web 静态资源请求", fields...)
	}
}

func webStaticRequestKind(relativePath string, targetPath string, usedFallback bool) string {
	isHTML := strings.HasSuffix(targetPath, ".html")
	if usedFallback {
		if isHTML {
			return "html_fallback"
		}
		return "fallback"
	}
	if strings.HasPrefix(relativePath, "assets/") {
		return "asset"
	}
	if isHTML {
		return "html_file"
	}
	return "file"
}

func setWebStaticCacheHeaders(writer http.ResponseWriter, relativePath string, usedFallback bool) {
	if cacheControl := webStaticCacheControl(relativePath, usedFallback); cacheControl != "" {
		writer.Header().Set("Cache-Control", cacheControl)
	}
}

func webStaticCacheControl(relativePath string, usedFallback bool) string {
	if usedFallback || strings.HasSuffix(relativePath, ".html") || relativePath == "" {
		return "no-cache"
	}
	if strings.HasPrefix(relativePath, "assets/") {
		return "public, max-age=31536000, immutable"
	}
	return ""
}
