import AppKit
import WebKit

final class WebViewHost: NSObject, WKNavigationDelegate, WKUIDelegate {
  let webView: WKWebView
  private let runtime: SidecarRuntimeConfig
  private let surfaceName: String
  private let startupTimeline: DesktopStartupTimeline?
  private let bridgeHandler: DesktopBridgeHandler
  private let lifecycleHandler: DesktopLifecycleHandler
  private var lastRequestedURL: URL?

  init(
    runtime: SidecarRuntimeConfig,
    surfaceName: String,
    startupTimeline: DesktopStartupTimeline? = nil,
    onWebReady: @escaping @MainActor () -> Void,
    openRoute: @escaping (DesktopWebRoute) -> Void,
    closeLauncher: @escaping () -> Void,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any]
  ) throws {
    self.runtime = runtime
    self.surfaceName = surfaceName
    self.startupTimeline = startupTimeline
    bridgeHandler = DesktopBridgeHandler(
      runtime: runtime,
      startupTimeline: startupTimeline,
      openRoute: openRoute,
      closeLauncher: closeLauncher,
      globalShortcutStatusProvider: globalShortcutStatusProvider,
      globalShortcutEnabledUpdater: globalShortcutEnabledUpdater,
      globalShortcutAcceleratorUpdater: globalShortcutAcceleratorUpdater,
      globalShortcutAcceleratorResetter: globalShortcutAcceleratorResetter
    )
    lifecycleHandler = DesktopLifecycleHandler(
      runtime: runtime,
      surfaceName: surfaceName,
      startupTimeline: startupTimeline,
      onWebReady: onWebReady
    )
    webView = WKWebView(
      frame: .zero,
      configuration: try WebViewConfigurationFactory.make(
        runtime: runtime,
        bridgeHandler: bridgeHandler,
        lifecycleHandler: lifecycleHandler
      )
    )
    super.init()
    bridgeHandler.attach(webView: webView)
    webView.navigationDelegate = self
    webView.uiDelegate = self
    webView.underPageBackgroundColor = .clear
    webView.setValue(false, forKey: "drawsBackground")
    webView.allowsBackForwardNavigationGestures = false
    startupTimeline?.mark("webview.created", metadata: ["surface": surfaceName])
  }

  func load(_ url: URL? = nil) {
    let targetURL = url ?? runtime.webURL
    lastRequestedURL = targetURL
    startupTimeline?.mark("webview.cookie_begin", metadata: webMetadata(url: targetURL))
    installDesktopSessionCookie {
      self.startupTimeline?.mark("webview.load_begin", metadata: self.webMetadata(url: targetURL))
      self.webView.load(URLRequest(url: targetURL))
    }
  }

  func reload() {
    startupTimeline?.mark("webview.reload", metadata: webMetadata(url: webView.url))
    webView.reload()
  }

  func webView(
    _ webView: WKWebView,
    decidePolicyFor navigationAction: WKNavigationAction,
    decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
  ) {
    guard let url = navigationAction.request.url else {
      decisionHandler(.cancel)
      return
    }
    if isInternalNavigation(url) {
      lastRequestedURL = url
      decisionHandler(.allow)
      return
    }
    if DesktopExternalURLPolicy.canOpen(url) {
      do {
        try DesktopExternalURLPolicy.open(url)
        startupTimeline?.mark("webview.external_url_opened", metadata: webMetadata(
          url: url,
          extra: ["navigation_type": navigationType(navigationAction.navigationType)]
        ))
      } catch {
        startupTimeline?.mark("webview.external_url_failed", metadata: webMetadata(
          url: url,
          extra: ["error": error.localizedDescription]
        ))
        NSLog("[Nexus WebView] external URL open failed: \(error.localizedDescription)")
      }
      decisionHandler(.cancel)
      return
    }

    startupTimeline?.mark("webview.navigation_blocked", metadata: webMetadata(
      url: url,
      extra: ["reason": "unsupported_scheme"]
    ))
    NSLog("[Nexus WebView] blocked navigation: \(url.absoluteString)")
    decisionHandler(.cancel)
  }

  func webView(
    _ webView: WKWebView,
    createWebViewWith configuration: WKWebViewConfiguration,
    for navigationAction: WKNavigationAction,
    windowFeatures: WKWindowFeatures
  ) -> WKWebView? {
    guard let url = navigationAction.request.url else {
      return nil
    }
    if isInternalNavigation(url) {
      lastRequestedURL = url
      webView.load(URLRequest(url: url))
      return nil
    }
    if DesktopExternalURLPolicy.canOpen(url) {
      do {
        try DesktopExternalURLPolicy.open(url)
        startupTimeline?.mark("webview.popup_external_url_opened", metadata: webMetadata(url: url))
      } catch {
        startupTimeline?.mark("webview.popup_external_url_failed", metadata: webMetadata(
          url: url,
          extra: ["error": error.localizedDescription]
        ))
        NSLog("[Nexus WebView] popup URL open failed: \(error.localizedDescription)")
      }
    } else {
      startupTimeline?.mark("webview.popup_blocked", metadata: webMetadata(
        url: url,
        extra: ["reason": "unsupported_scheme"]
      ))
    }
    return nil
  }

  func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
    startupTimeline?.mark("webview.navigation_started", metadata: webMetadata(url: lastRequestedURL ?? webView.url))
  }

  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
    startupTimeline?.mark("webview.navigation_finished", metadata: webMetadata(url: webView.url ?? lastRequestedURL))
    probeDesktopBridge()
  }

  func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
    startupTimeline?.mark("webview.navigation_failed", metadata: webMetadata(
      url: webView.url ?? lastRequestedURL,
      extra: ["error": error.localizedDescription]
    ))
    NSLog("[Nexus WebView] navigation failed: \(error.localizedDescription)")
  }

  func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
    startupTimeline?.mark("webview.provisional_navigation_failed", metadata: webMetadata(
      url: webView.url ?? lastRequestedURL,
      extra: ["error": error.localizedDescription]
    ))
    NSLog("[Nexus WebView] provisional navigation failed: \(error.localizedDescription)")
  }

  func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
    let targetURL = webView.url ?? lastRequestedURL
    var metadata = webMetadata(url: targetURL)
    if let diagnosticsURL = DesktopDiagnosticsReport.writeRuntimeIssue(
      prefix: "webcontent-terminated",
      reason: "WebContent process terminated",
      runtime: runtime,
      startupTimeline: startupTimeline,
      details: metadata
    ) {
      metadata["diagnostics_path"] = diagnosticsURL.path
    }
    startupTimeline?.mark("webview.content_process_terminated", metadata: metadata)
    NSLog("[Nexus WebView] content process terminated, reloading current route.")
    webView.reload()
  }

  func webView(
    _ webView: WKWebView,
    contextMenuItemsForElement elementInfo: [String: Any],
    defaultMenuItems: [NSMenuItem]
  ) -> [NSMenuItem] {
    startupTimeline?.mark("webview.context_menu_suppressed", metadata: ["surface": surfaceName])
    return []
  }

  private func isInternalNavigation(_ url: URL) -> Bool {
    guard let scheme = url.scheme?.lowercased() else {
      return false
    }
    if scheme == "about" {
      return true
    }
    guard DesktopWebOriginPolicy.isURLAllowed(url, runtime: runtime) else {
      return false
    }
    return true
  }

  private func webMetadata(url: URL?, extra: [String: String] = [:]) -> [String: String] {
    var metadata = extra
    metadata["surface"] = surfaceName
    guard let url else {
      return metadata
    }
    metadata["path"] = url.path.isEmpty ? "/" : url.path
    if let queryKeys = queryKeys(url: url) {
      metadata["query_keys"] = queryKeys
    }
    if let scheme = url.scheme?.lowercased(), !scheme.isEmpty {
      metadata["scheme"] = scheme
    }
    if let host = url.host?.trimmingCharacters(in: .whitespacesAndNewlines), !host.isEmpty {
      metadata["host"] = host
    }
    return metadata
  }

  private func navigationType(_ type: WKNavigationType) -> String {
    switch type {
    case .linkActivated:
      return "link_activated"
    case .formSubmitted:
      return "form_submitted"
    case .backForward:
      return "back_forward"
    case .reload:
      return "reload"
    case .formResubmitted:
      return "form_resubmitted"
    case .other:
      return "other"
    @unknown default:
      return "unknown"
    }
  }

  private func queryKeys(url: URL) -> String? {
    guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
          let queryItems = components.queryItems,
          !queryItems.isEmpty else {
      return nil
    }
    let keys = queryItems.map(\.name).filter { !$0.isEmpty }
    if keys.isEmpty {
      return "unknown"
    }
    return Array(Set(keys)).sorted().joined(separator: ",")
  }

  private func installDesktopSessionCookie(completion: @escaping () -> Void) {
    guard let host = runtime.webURL.host,
          let cookie = HTTPCookie(properties: [
            .domain: host,
            .path: "/",
            .name: "nexus_desktop_token",
            .value: runtime.sessionToken,
          ]) else {
      completion()
      return
    }
    webView.configuration.websiteDataStore.httpCookieStore.setCookie(cookie) {
      DispatchQueue.main.async {
        completion()
      }
    }
  }

  private func probeDesktopBridge() {
    let script = """
    [
      typeof window.webkit !== "undefined",
      Boolean(window.webkit?.messageHandlers?.nexusDesktopLifecycle),
      Boolean(window.webkit?.messageHandlers?.nexusDesktop),
      Boolean(window.__NEXUS_DESKTOP_RUNTIME__)
    ].join("|")
    """
    webView.evaluateJavaScript(script) { [weak self] result, error in
      guard let self else {
        return
      }
      var metadata: [String: String] = ["surface": self.surfaceName]
      if let error {
        metadata["error"] = error.localizedDescription
        self.startupTimeline?.mark("webview.bridge_probe", metadata: metadata)
        return
      }
      if let text = result as? String {
        let values = text.split(separator: "|").map(String.init)
        if values.count == 4 {
          metadata["has_webkit"] = values[0]
          metadata["has_lifecycle_handler"] = values[1]
          metadata["has_bridge_handler"] = values[2]
          metadata["has_runtime"] = values[3]
        } else {
          metadata["result"] = text
        }
      } else if let result {
        metadata["result_type"] = String(describing: type(of: result))
      } else {
        metadata["result_type"] = "nil"
      }
      self.startupTimeline?.mark("webview.bridge_probe", metadata: metadata)
    }
  }
}
