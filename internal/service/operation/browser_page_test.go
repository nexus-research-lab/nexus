package operation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestParseBrowserPageURLRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	unsafeTargets := []string{
		"",
		"ftp://example.com/file",
		"http://localhost/admin",
		"http://127.0.0.1:80/admin",
		"http://[::1]/admin",
		"http://10.0.0.8/admin",
		"http://169.254.169.254/latest/meta-data",
		"https://example.com:8443/admin",
		"https://user:password@example.com/",
	}
	for _, target := range unsafeTargets {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			if _, err := parseBrowserPageURL(target); err == nil {
				t.Fatalf("不安全地址应被拒绝: %q", target)
			}
		})
	}

	parsed, err := parseBrowserPageURL("https://example.com/docs/page#section")
	if err != nil {
		t.Fatalf("公网 HTTPS 地址应通过基础校验: %v", err)
	}
	if parsed.String() != "https://example.com/docs/page" {
		t.Fatalf("页面抓取地址应移除 fragment，实际: %q", parsed.String())
	}
}

func TestRewriteBrowserPageHTMLCreatesSandboxNavigationSnapshot(t *testing.T) {
	t.Parallel()

	pageURL, err := url.Parse("https://example.com/docs/page")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`<!doctype html><html><head>
<base href="https://old.example/">
<meta http-equiv="Content-Security-Policy" content="frame-ancestors 'none'">
<meta http-equiv="refresh" content="0;url=https://escape.example/">
<title>Docs</title></head><body><a href="next">Next</a></body></html>`)
	rewritten, err := rewriteBrowserPageHTML(payload, "text/html; charset=utf-8", pageURL)
	if err != nil {
		t.Fatalf("重写 HTML 失败: %v", err)
	}
	html := string(rewritten)
	if strings.Count(html, `<base href="https://example.com/docs/page"`) != 1 {
		t.Fatalf("页面应只保留最终 URL base: %s", html)
	}
	if strings.Contains(strings.ToLower(html), "content-security-policy") ||
		strings.Contains(strings.ToLower(html), "http-equiv=\"refresh\"") {
		t.Fatalf("页面内阻断嵌入或自动跳转的 meta 未清理: %s", html)
	}
	if !strings.Contains(html, "nexus-navi-proxy") || !strings.Contains(html, `type: "navigate"`) {
		t.Fatalf("页面缺少 Navi 导航桥: %s", html)
	}
}

func TestBrowserPageFetcherReturnsRewrittenHTMLWithoutUserCookies(t *testing.T) {
	t.Parallel()

	var received *http.Request
	fetcher := &browserPageFetcher{
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			received = request.Clone(request.Context())
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`<html><head><title>Navi</title></head><body>Ready</body></html>`)),
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Request:    request,
				StatusCode: http.StatusOK,
			}, nil
		})},
		slots: make(chan struct{}, 1),
	}

	document, err := fetcher.fetch(context.Background(), "https://example.com/start")
	if err != nil {
		t.Fatalf("抓取页面失败: %v", err)
	}
	if received == nil || received.Header.Get("Cookie") != "" {
		t.Fatalf("Navi 页面抓取不得携带用户 Cookie: %+v", received)
	}
	if received.Header.Get("User-Agent") != "Mozilla/5.0 Nexus-Navi/1.0" {
		t.Fatalf("页面抓取 User-Agent 不正确: %q", received.Header.Get("User-Agent"))
	}
	if document.URL != "https://example.com/start" || !strings.Contains(string(document.HTML), "nexus-navi-proxy") {
		t.Fatalf("页面快照不正确: url=%q html=%s", document.URL, string(document.HTML))
	}
}

func TestBrowserPageFetcherPropagatesContextDeadline(t *testing.T) {
	t.Parallel()

	fetcher := &browserPageFetcher{
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})},
		slots: make(chan struct{}, 1),
	}
	_, err := fetcher.fetch(context.Background(), "https://example.com/")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("超时语义应保留给 HTTP 层，实际: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
