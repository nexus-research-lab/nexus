import AppKit
import WebKit

final class WebViewHost: NSObject, WKNavigationDelegate, WKUIDelegate {
  private static let resumeProbeReloadThreshold = 2

  let webView: WKWebView
  let windowInteractionState = DesktopWindowInteractionState()
  private let runtime: SidecarRuntimeConfig
  private let surfaceName: String
  private let startupTimeline: DesktopStartupTimeline?
  private let bridgeHandler: DesktopBridgeHandler
  private let lifecycleHandler: DesktopLifecycleHandler
  private let windowInteractionHandler: DesktopWindowInteractionHandler
  private var lastRequestedURL: URL?
  private var lastRoute = DesktopWebRoute(path: "/launcher", entry: .app)
  private var resumeCheckInFlight = false
  private var consecutiveResumeProbeFailures = 0
  private var lastResumeCheckAt = Date.distantPast
  private var navigationRequestID = 0
  private var navigationInFlight = false

  private struct ResumeProbeResult {
    let isReady: Bool
    let currentRoute: DesktopWebRoute?
    let snapshot: String
  }

  init(
    runtime: SidecarRuntimeConfig,
    surfaceName: String,
    windowCloseButtonCenter: CGPoint,
    windowControlsLeadingInset: CGFloat,
    startupTimeline: DesktopStartupTimeline? = nil,
    onWebReady: @escaping @MainActor () -> Void,
    openRoute: @escaping (DesktopWebRoute) -> Void,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any],
    updateStarter: @escaping () -> String
  ) throws {
    self.runtime = runtime
    self.surfaceName = surfaceName
    self.startupTimeline = startupTimeline
    bridgeHandler = DesktopBridgeHandler(
      runtime: runtime,
      startupTimeline: startupTimeline,
      openRoute: openRoute,
      globalShortcutStatusProvider: globalShortcutStatusProvider,
      globalShortcutEnabledUpdater: globalShortcutEnabledUpdater,
      globalShortcutAcceleratorUpdater: globalShortcutAcceleratorUpdater,
      globalShortcutAcceleratorResetter: globalShortcutAcceleratorResetter,
      updateStarter: updateStarter
    )
    lifecycleHandler = DesktopLifecycleHandler(
      runtime: runtime,
      surfaceName: surfaceName,
      startupTimeline: startupTimeline,
      onWebReady: onWebReady
    )
    windowInteractionHandler = DesktopWindowInteractionHandler(
      runtime: runtime,
      startupTimeline: startupTimeline,
      interactionState: windowInteractionState
    )
    webView = WKWebView(
      frame: .zero,
      configuration: try WebViewConfigurationFactory.make(
        runtime: runtime,
        windowCloseButtonCenter: windowCloseButtonCenter,
        windowControlsLeadingInset: windowControlsLeadingInset,
        bridgeHandler: bridgeHandler,
        lifecycleHandler: lifecycleHandler,
        windowInteractionHandler: windowInteractionHandler
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
    navigationRequestID += 1
    navigationInFlight = true
    let requestID = navigationRequestID
    lastRequestedURL = targetURL
    updateLastRoute(from: targetURL)
    startupTimeline?.mark("webview.cookie_begin", metadata: webMetadata(
      url: targetURL,
      extra: ["navigation_request_id": "\(requestID)"]
    ))
    installDesktopSessionCookie {
      guard requestID == self.navigationRequestID else {
        self.startupTimeline?.mark("webview.load_skipped_stale", metadata: self.webMetadata(
          url: targetURL,
          extra: [
            "navigation_request_id": "\(requestID)",
            "current_navigation_request_id": "\(self.navigationRequestID)",
          ]
        ))
        return
      }
      self.startupTimeline?.mark("webview.load_begin", metadata: self.webMetadata(
        url: targetURL,
        extra: ["navigation_request_id": "\(requestID)"]
      ))
      if self.webView.load(URLRequest(url: targetURL)) == nil {
        self.navigationInFlight = false
      }
    }
  }

  func reload() {
    navigationRequestID += 1
    navigationInFlight = true
    let requestID = navigationRequestID
    startupTimeline?.mark("webview.reload", metadata: webMetadata(
      url: webView.url,
      extra: ["navigation_request_id": "\(requestID)"]
    ))
    installDesktopSessionCookie {
      guard requestID == self.navigationRequestID else {
        self.startupTimeline?.mark("webview.reload_skipped_stale", metadata: [
          "current_navigation_request_id": "\(self.navigationRequestID)",
          "navigation_request_id": "\(requestID)",
          "surface": self.surfaceName,
        ])
        return
      }
      if self.webView.reload() == nil {
        self.navigationInFlight = false
      }
    }
  }

  func recoverAfterWindowShown(reason: String) {
    if resumeCheckInFlight {
      return
    }

    let observedNavigationRequestID = navigationRequestID
    if shouldSkipResumeCheck(observedNavigationRequestID) {
      skipResumeCheck(reason: reason, observedNavigationRequestID: observedNavigationRequestID)
      return
    }

    let now = Date()
    if now.timeIntervalSince(lastResumeCheckAt) < 5 {
      return
    }

    lastResumeCheckAt = now
    resumeCheckInFlight = true
    startupTimeline?.mark("webview.resume_check_begin", metadata: [
      "path": lastRoute.path,
      "reason": reason,
      "surface": surfaceName,
    ])
    webView.needsDisplay = true
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
      self?.captureResumeProbeIfCurrent(
        reason: reason,
        observedNavigationRequestID: observedNavigationRequestID
      )
    }
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
      updateLastRoute(from: url)
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
      updateLastRoute(from: url)
      load(url)
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

  func webView(
    _ webView: WKWebView,
    runOpenPanelWith parameters: WKOpenPanelParameters,
    initiatedByFrame frame: WKFrameInfo,
    completionHandler: @escaping ([URL]?) -> Void
  ) {
    let panel = NSOpenPanel()
    panel.canChooseFiles = true
    panel.canChooseDirectories = parameters.allowsDirectories
    panel.allowsMultipleSelection = parameters.allowsMultipleSelection
    panel.resolvesAliases = true
    panel.treatsFilePackagesAsDirectories = false
    panel.prompt = "选择"

    startupTimeline?.mark("webview.open_panel_begin", metadata: ["surface": surfaceName])
    let presentResult: (NSApplication.ModalResponse) -> Void = { [weak self] response in
      if response == .OK {
        self?.startupTimeline?.mark("webview.open_panel_selected", metadata: [
          "surface": self?.surfaceName ?? "unknown",
          "count": String(panel.urls.count),
        ])
        completionHandler(panel.urls)
      } else {
        self?.startupTimeline?.mark("webview.open_panel_cancelled", metadata: [
          "surface": self?.surfaceName ?? "unknown",
        ])
        completionHandler(nil)
      }
    }

    if let window = webView.window {
      panel.beginSheetModal(for: window, completionHandler: presentResult)
    } else {
      presentResult(panel.runModal())
    }
  }

  func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
    if !navigationInFlight {
      navigationRequestID += 1
    }
    navigationInFlight = true
    startupTimeline?.mark("webview.navigation_started", metadata: webMetadata(url: lastRequestedURL ?? webView.url))
  }

  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
    navigationInFlight = false
    consecutiveResumeProbeFailures = 0
    updateLastRoute(from: webView.url ?? lastRequestedURL)
    startupTimeline?.mark("webview.navigation_finished", metadata: webMetadata(url: webView.url ?? lastRequestedURL))
    probeDesktopBridge()
  }

  func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
    navigationInFlight = false
    let cancelled = isNavigationCancellation(error)
    startupTimeline?.mark(cancelled ? "webview.navigation_cancelled" : "webview.navigation_failed", metadata: webMetadata(
      url: webView.url ?? lastRequestedURL,
      extra: navigationErrorMetadata(error, webView: webView)
    ))
    NSLog("[Nexus WebView] navigation \(cancelled ? "cancelled" : "failed"): \(error.localizedDescription)")
  }

  func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
    navigationInFlight = false
    let cancelled = isNavigationCancellation(error)
    startupTimeline?.mark(cancelled ? "webview.provisional_navigation_cancelled" : "webview.provisional_navigation_failed", metadata: webMetadata(
      url: webView.url ?? lastRequestedURL,
      extra: navigationErrorMetadata(error, webView: webView)
    ))
    NSLog("[Nexus WebView] provisional navigation \(cancelled ? "cancelled" : "failed"): \(error.localizedDescription)")
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
    load((Self.route(from: targetURL) ?? lastRoute).url(runtime: runtime))
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

  private func captureResumeProbe(completion: @escaping (ResumeProbeResult) -> Void) {
    let script = """
    (() => {
      window.dispatchEvent(new Event("resize"));
      document.documentElement.style.setProperty("--nexus-webview-resume", String(Date.now()));
      if (document.body) {
        document.body.getBoundingClientRect();
      }
      const root = document.getElementById("root");
      return {
        isReady: Boolean(root && root.childElementCount > 0 && document.readyState !== "loading"),
        href: window.location.href,
        path: `${window.location.pathname}${window.location.search}${window.location.hash}`,
        readyState: document.readyState,
        title: document.title,
        hasRoot: Boolean(root),
        rootChildren: root ? root.childElementCount : -1,
        bodyChildren: document.body ? document.body.childElementCount : -1,
        bodyTextLength: document.body ? document.body.innerText.length : -1
      };
    })();
    """
    webView.evaluateJavaScript(script) { result, error in
      if let error {
        completion(ResumeProbeResult(
          isReady: false,
          currentRoute: nil,
          snapshot: "\(type(of: error)): \(error.localizedDescription)"
        ))
        return
      }

      completion(Self.parseResumeProbe(result))
    }
  }

  private func captureResumeProbeIfCurrent(
    reason: String,
    observedNavigationRequestID: Int
  ) {
    if shouldSkipResumeCheck(observedNavigationRequestID) {
      skipResumeCheck(reason: reason, observedNavigationRequestID: observedNavigationRequestID)
      return
    }
    captureResumeProbe { [weak self] probe in
      guard let self else {
        return
      }
      if self.shouldSkipResumeCheck(observedNavigationRequestID) {
        self.skipResumeCheck(reason: reason, observedNavigationRequestID: observedNavigationRequestID)
        return
      }
      self.handleResumeProbe(reason: reason, probe: probe)
    }
  }

  private func shouldSkipResumeCheck(_ observedNavigationRequestID: Int) -> Bool {
    Self.shouldSkipResumeCheck(
      observedNavigationRequestID: observedNavigationRequestID,
      currentNavigationRequestID: navigationRequestID,
      navigationInFlight: navigationInFlight,
      webViewIsLoading: webView.isLoading
    )
  }

  static func shouldSkipResumeCheck(
    observedNavigationRequestID: Int,
    currentNavigationRequestID: Int,
    navigationInFlight: Bool,
    webViewIsLoading: Bool
  ) -> Bool {
    observedNavigationRequestID != currentNavigationRequestID
      || navigationInFlight
      || webViewIsLoading
  }

  private func skipResumeCheck(reason: String, observedNavigationRequestID: Int) {
    resumeCheckInFlight = false
    startupTimeline?.mark("webview.resume_check_skipped", metadata: [
      "current_navigation_request_id": "\(navigationRequestID)",
      "navigation_request_id": "\(observedNavigationRequestID)",
      "path": lastRoute.path,
      "reason": reason,
      "skip_reason": observedNavigationRequestID == navigationRequestID
        ? "navigation_in_flight"
        : "navigation_changed",
      "surface": surfaceName,
    ])
  }

  private func handleResumeProbe(reason: String, probe: ResumeProbeResult) {
    defer {
      resumeCheckInFlight = false
    }

    if let currentRoute = probe.currentRoute {
      lastRoute = currentRoute
    }

    if probe.isReady {
      consecutiveResumeProbeFailures = 0
      startupTimeline?.mark("webview.resume_check_ready", metadata: [
        "path": lastRoute.path,
        "reason": reason,
        "surface": surfaceName,
      ])
      return
    }

    consecutiveResumeProbeFailures += 1
    let recoveryRoute = probe.currentRoute ?? lastRoute
    var metadata: [String: String] = [
      "failure_count": "\(consecutiveResumeProbeFailures)",
      "path": recoveryRoute.path,
      "probe": "empty_or_loading_root",
      "reason": reason,
      "surface": surfaceName,
    ]
    if !probe.snapshot.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      metadata["probe_snapshot"] = Self.trimMetadata(probe.snapshot)
    }

    if consecutiveResumeProbeFailures >= Self.resumeProbeReloadThreshold,
       let diagnosticsURL = DesktopDiagnosticsReport.writeRuntimeIssue(
        prefix: "webview-resume-failed",
        reason: "empty_or_loading_root",
        runtime: runtime,
        startupTimeline: startupTimeline,
        details: [
          "failure_count": consecutiveResumeProbeFailures,
          "probe_snapshot": Self.trimDiagnosticDetail(probe.snapshot),
          "resume_reason": reason,
          "route_path": recoveryRoute.path,
        ]
       ) {
      metadata["diagnostics_path"] = diagnosticsURL.path
    }

    startupTimeline?.mark("webview.resume_reload", metadata: metadata)
    load(recoveryRoute.url(runtime: runtime))
  }

  private func updateLastRoute(from url: URL?) {
    guard let route = Self.route(from: url) else {
      return
    }
    lastRoute = route
  }

  private static func parseResumeProbe(_ result: Any?) -> ResumeProbeResult {
    let snapshot = diagnosticString(result)
    guard let record = result as? [String: Any] else {
      return ResumeProbeResult(isReady: false, currentRoute: nil, snapshot: snapshot)
    }
    let isReady = record["isReady"] as? Bool ?? false
    let path = record["path"] as? String ?? ""
    return ResumeProbeResult(isReady: isReady, currentRoute: route(fromProbePath: path), snapshot: snapshot)
  }

  private static func route(from url: URL?) -> DesktopWebRoute? {
    guard let url else {
      return nil
    }
    guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
      return nil
    }
    var path = components.path.isEmpty ? "/" : components.path
    if let query = components.percentEncodedQuery, !query.isEmpty {
      path += "?\(query)"
    }
    if let fragment = components.percentEncodedFragment, !fragment.isEmpty {
      path += "#\(fragment)"
    }
    return route(fromProbePath: path)
  }

  private static func route(fromProbePath path: String) -> DesktopWebRoute? {
    let candidate = path.trimmingCharacters(in: .whitespacesAndNewlines)
    guard candidate.hasPrefix("/"), !candidate.hasPrefix("//") else {
      return nil
    }
    guard let components = URLComponents(string: "http://nexus.local\(candidate)") else {
      return DesktopWebRoute.appRoute(candidate)
    }

    if let desktopRoute = components.queryItems?.first(where: { $0.name == "desktop_route" })?.value,
       let route = DesktopWebRoute.appRoute(desktopRoute) {
      return route
    }

    var routeValue = components.path.isEmpty ? "/" : components.path
    if let query = components.percentEncodedQuery, !query.isEmpty {
      routeValue += "?\(query)"
    }
    if let fragment = components.percentEncodedFragment, !fragment.isEmpty {
      routeValue += "#\(fragment)"
    }
    return DesktopWebRoute.appRoute(routeValue)
  }

  private static func diagnosticString(_ value: Any?) -> String {
    guard let value else {
      return ""
    }
    if JSONSerialization.isValidJSONObject(value),
       let data = try? JSONSerialization.data(withJSONObject: value, options: [.sortedKeys]),
       let text = String(data: data, encoding: .utf8) {
      return text
    }
    return "\(value)"
  }

  private static func trimMetadata(_ value: String) -> String {
    let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
    let maxLength = 240
    if normalized.count <= maxLength {
      return normalized
    }
    let endIndex = normalized.index(normalized.startIndex, offsetBy: maxLength)
    return String(normalized[..<endIndex]) + "..."
  }

  private static func trimDiagnosticDetail(_ value: String) -> String {
    let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
    let maxLength = 4096
    if normalized.count <= maxLength {
      return normalized
    }
    let endIndex = normalized.index(normalized.startIndex, offsetBy: maxLength)
    return String(normalized[..<endIndex]) + "..."
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

  private func navigationErrorMetadata(_ error: Error, webView: WKWebView) -> [String: String] {
    let nsError = error as NSError
    return [
      "error": Self.trimMetadata(error.localizedDescription),
      "error_code": "\(nsError.code)",
      "error_domain": nsError.domain,
      "is_cancelled": isNavigationCancellation(error) ? "true" : "false",
      "is_loading": webView.isLoading ? "true" : "false",
    ]
  }

  private func isNavigationCancellation(_ error: Error) -> Bool {
    let nsError = error as NSError
    if nsError.domain == NSURLErrorDomain && nsError.code == NSURLErrorCancelled {
      return true
    }
    return nsError.domain == "WebKitErrorDomain" && nsError.code == 102
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
