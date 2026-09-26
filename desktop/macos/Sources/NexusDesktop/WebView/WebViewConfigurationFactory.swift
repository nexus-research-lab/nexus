import WebKit

enum WebViewConfigurationFactory {
  // 隔离测试的所有窗口共用本次进程的临时存储，不读取或清除正式登录 Cookie。
  static let websiteDataStore = makeWebsiteDataStore(
    preferencesSuite: ProcessInfo.processInfo.environment["NEXUS_DESKTOP_PREFERENCES_SUITE"]
  )

  static func makeWebsiteDataStore(preferencesSuite: String?) -> WKWebsiteDataStore {
    let isolated = !(preferencesSuite?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true)
    return isolated ? .nonPersistent() : .default()
  }

  static func make(
    runtime: SidecarRuntimeConfig,
    windowCloseButtonCenter: CGPoint,
    windowControlsLeadingInset: CGFloat,
    bridgeHandler: DesktopBridgeHandler,
    lifecycleHandler: DesktopLifecycleHandler,
    windowInteractionHandler: DesktopWindowInteractionHandler
  ) throws -> WKWebViewConfiguration {
    let configuration = WKWebViewConfiguration()
    configuration.websiteDataStore = websiteDataStore
    configuration.defaultWebpagePreferences.allowsContentJavaScript = true
    configuration.preferences.javaScriptCanOpenWindowsAutomatically = true

    let runtimeScript = try DesktopRuntimeScript.make(
      runtime: runtime,
      windowCloseButtonCenter: windowCloseButtonCenter,
      windowControlsLeadingInset: windowControlsLeadingInset
    )
    let userScript = WKUserScript(
      source: runtimeScript,
      injectionTime: .atDocumentStart,
      forMainFrameOnly: true
    )
    configuration.userContentController.addUserScript(userScript)

    let bridgeScript = WKUserScript(
      source: DesktopBridgeScript.make(),
      injectionTime: .atDocumentStart,
      forMainFrameOnly: true
    )
    configuration.userContentController.addUserScript(bridgeScript)

    let windowInteractionScript = WKUserScript(
      source: DesktopWindowInteractionScript.make(),
      injectionTime: .atDocumentStart,
      forMainFrameOnly: true
    )
    configuration.userContentController.addUserScript(windowInteractionScript)
    configuration.userContentController.add(bridgeHandler, name: "nexusDesktop")
    configuration.userContentController.add(lifecycleHandler, name: "nexusDesktopLifecycle")
    configuration.userContentController.add(
      windowInteractionHandler,
      name: "nexusDesktopWindow"
    )
    return configuration
  }
}
