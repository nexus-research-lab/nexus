import WebKit

enum WebViewConfigurationFactory {
  static func make(
    runtime: SidecarRuntimeConfig,
    bridgeHandler: DesktopBridgeHandler,
    lifecycleHandler: DesktopLifecycleHandler
  ) throws -> WKWebViewConfiguration {
    let configuration = WKWebViewConfiguration()
    configuration.defaultWebpagePreferences.allowsContentJavaScript = true
    configuration.preferences.javaScriptCanOpenWindowsAutomatically = true

    let runtimeScript = try DesktopRuntimeScript.make(runtime: runtime)
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
    configuration.userContentController.add(bridgeHandler, name: "nexusDesktop")
    configuration.userContentController.add(lifecycleHandler, name: "nexusDesktopLifecycle")
    return configuration
  }
}
