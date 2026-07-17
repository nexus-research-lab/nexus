// INPUT: Navi 请求打开的公网 HTTP(S) URL。
// OUTPUT: 注入安全导航桥的真实 HTML 页面快照。
// POS: operation service 的浏览器页面边界；负责防 SSRF、限流读取和可嵌入化。
package operation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	browserPageMaxBytes = 4 * 1024 * 1024
	browserPageTimeout  = 15 * time.Second
	browserPageWorkers  = 4
)

var (
	ErrInvalidBrowserPageURL     = errors.New("invalid browser page URL")
	ErrBrowserPageAddressDenied  = errors.New("browser page address denied")
	ErrBrowserPageTooLarge       = errors.New("browser page too large")
	ErrBrowserPageUnsupported    = errors.New("browser page content unsupported")
	ErrBrowserPageUpstream       = errors.New("browser page upstream failure")
	deniedBrowserAddressPrefixes = mustBrowserAddressPrefixes(
		"100.64.0.0/10",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"2001:db8::/32",
	)
)

// BrowserPageDocument 是 Navi 可安全放入 sandbox iframe 的页面。
type BrowserPageDocument struct {
	URL  string
	HTML []byte
}

type browserPageFetcher struct {
	client   *http.Client
	lookupIP func(context.Context, string) ([]net.IPAddr, error)
	slots    chan struct{}
}

func newBrowserPageFetcher() *browserPageFetcher {
	fetcher := &browserPageFetcher{
		lookupIP: net.DefaultResolver.LookupIPAddr,
		slots:    make(chan struct{}, browserPageWorkers),
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = fetcher.dialPublicAddress
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.TLSHandshakeTimeout = 8 * time.Second
	fetcher.client = &http.Client{
		Transport: transport,
		Timeout:   browserPageTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 6 {
				return fmt.Errorf("%w: redirect limit exceeded", ErrBrowserPageUpstream)
			}
			return validateBrowserPageURL(request.URL)
		},
	}
	return fetcher
}

// FetchBrowserPage 抓取一个不携带用户 Cookie 的公网页面，并转换为 Navi 页面快照。
func (s *Service) FetchBrowserPage(ctx context.Context, rawURL string) (*BrowserPageDocument, error) {
	if s.browserPages == nil {
		s.browserPages = newBrowserPageFetcher()
	}
	return s.browserPages.fetch(ctx, rawURL)
}

func (f *browserPageFetcher) fetch(ctx context.Context, rawURL string) (*BrowserPageDocument, error) {
	target, err := parseBrowserPageURL(rawURL)
	if err != nil {
		return nil, err
	}
	select {
	case f.slots <- struct{}{}:
		defer func() { <-f.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBrowserPageURL, err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.7")
	request.Header.Set("User-Agent", "Mozilla/5.0 Nexus-Navi/1.0")

	response, err := f.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", classifyBrowserPageRequestError(err), err)
	}
	defer func() { _ = response.Body.Close() }()

	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, fmt.Errorf("%w: %s", ErrBrowserPageUnsupported, contentType)
	}
	if response.ContentLength > browserPageMaxBytes {
		return nil, ErrBrowserPageTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, browserPageMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrowserPageUpstream, err)
	}
	if len(payload) > browserPageMaxBytes {
		return nil, ErrBrowserPageTooLarge
	}
	finalURL := response.Request.URL
	rewritten, err := rewriteBrowserPageHTML(payload, contentType, finalURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrowserPageUpstream, err)
	}
	return &BrowserPageDocument{
		URL:  finalURL.String(),
		HTML: rewritten,
	}, nil
}

func (f *browserPageFetcher) dialPublicAddress(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || !isAllowedBrowserPagePort(port) {
		return nil, ErrBrowserPageAddressDenied
	}
	addresses, err := f.resolvePublicAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var dialErrors []error
	for _, ip := range addresses {
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
	}
	return nil, fmt.Errorf("%w: %v", ErrBrowserPageUpstream, errors.Join(dialErrors...))
}

func (f *browserPageFetcher) resolvePublicAddresses(ctx context.Context, host string) ([]net.IP, error) {
	if parsed := net.ParseIP(host); parsed != nil {
		if isPublicBrowserAddress(parsed) {
			return []net.IP{parsed}, nil
		}
		return nil, ErrBrowserPageAddressDenied
	}
	resolved, err := f.lookupIP(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrowserPageUpstream, err)
	}
	addresses := make([]net.IP, 0, len(resolved))
	for _, item := range resolved {
		if isPublicBrowserAddress(item.IP) {
			addresses = append(addresses, item.IP)
		}
	}
	if len(addresses) == 0 {
		return nil, ErrBrowserPageAddressDenied
	}
	return addresses, nil
}

func parseBrowserPageURL(rawURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBrowserPageURL, err)
	}
	if err = validateBrowserPageURL(target); err != nil {
		return nil, err
	}
	target.Fragment = ""
	return target, nil
}

func validateBrowserPageURL(target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		return ErrInvalidBrowserPageURL
	}
	if target.User != nil || strings.EqualFold(target.Hostname(), "localhost") {
		return ErrBrowserPageAddressDenied
	}
	port := target.Port()
	if port != "" && !isAllowedBrowserPagePort(port) {
		return ErrBrowserPageAddressDenied
	}
	if parsed := net.ParseIP(target.Hostname()); parsed != nil && !isPublicBrowserAddress(parsed) {
		return ErrBrowserPageAddressDenied
	}
	return nil
}

func isAllowedBrowserPagePort(port string) bool {
	return port == "80" || port == "443"
}

func isPublicBrowserAddress(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range deniedBrowserAddressPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func classifyBrowserPageRequestError(err error) error {
	if errors.Is(err, ErrInvalidBrowserPageURL) {
		return ErrInvalidBrowserPageURL
	}
	if errors.Is(err, ErrBrowserPageAddressDenied) {
		return ErrBrowserPageAddressDenied
	}
	return ErrBrowserPageUpstream
}

func mustBrowserAddressPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func rewriteBrowserPageHTML(
	payload []byte,
	contentType string,
	pageURL *url.URL,
) ([]byte, error) {
	reader, err := charset.NewReader(bytes.NewReader(payload), contentType)
	if err != nil {
		return nil, err
	}
	document, err := html.Parse(reader)
	if err != nil {
		return nil, err
	}
	head := findBrowserHTMLElement(document, "head")
	if head == nil {
		return nil, errors.New("html document has no head")
	}
	removeBrowserPageBlockingNodes(head)
	head.InsertBefore(&html.Node{
		Type: html.ElementNode,
		Data: "base",
		Attr: []html.Attribute{{Key: "href", Val: pageURL.String()}},
	}, head.FirstChild)
	script := &html.Node{Type: html.ElementNode, Data: "script"}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: browserPageNavigationBridge})
	head.AppendChild(script)

	var output bytes.Buffer
	if err = html.Render(&output, document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func findBrowserHTMLElement(root *html.Node, name string) *html.Node {
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, name) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if match := findBrowserHTMLElement(child, name); match != nil {
			return match
		}
	}
	return nil
}

func removeBrowserPageBlockingNodes(root *html.Node) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if isBrowserPageBlockingNode(child) {
			root.RemoveChild(child)
		} else {
			removeBrowserPageBlockingNodes(child)
		}
		child = next
	}
}

func isBrowserPageBlockingNode(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if strings.EqualFold(node.Data, "base") {
		return true
	}
	if !strings.EqualFold(node.Data, "meta") {
		return false
	}
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, "http-equiv") {
			value := strings.TrimSpace(attribute.Val)
			if strings.EqualFold(value, "content-security-policy") || strings.EqualFold(value, "refresh") {
				return true
			}
		}
	}
	return false
}

const browserPageNavigationBridge = `(() => {
  const send = (value) => {
    try {
      const target = new URL(value, document.baseURI);
      if (target.protocol !== "http:" && target.protocol !== "https:") return;
      parent.postMessage({ source: "nexus-navi-proxy", type: "navigate", url: target.href }, "*");
    } catch (_) {}
  };
  document.addEventListener("click", (event) => {
    const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
    if (!link || link.hasAttribute("download")) return;
    const href = link.getAttribute("href");
    if (!href || href.startsWith("#")) return;
    event.preventDefault();
    send(link.href);
  }, true);
  document.addEventListener("submit", (event) => {
	const form = event.target;
	if (!(form instanceof HTMLFormElement)) return;
	event.preventDefault();
	if (form.method.toLowerCase() !== "get") return;
    const target = new URL(form.action || document.baseURI, document.baseURI);
    const values = new FormData(form);
    for (const [key, value] of values.entries()) {
      if (typeof value === "string") target.searchParams.append(key, value);
    }
    send(target.href);
  }, true);
})();`
