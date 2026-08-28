import AppKit
import UniformTypeIdentifiers
import WebKit

final class DesktopBridgeHandler: NSObject, WKScriptMessageHandler {
  private weak var webView: WKWebView?
  private let runtime: SidecarRuntimeConfig
  private let startupTimeline: DesktopStartupTimeline?
  private let openRoute: (DesktopWebRoute) -> Void
  private let globalShortcutStatusProvider: () -> [String: Any]
  private let globalShortcutEnabledUpdater: (Bool) -> [String: Any]
  private let globalShortcutAcceleratorUpdater: (String) -> [String: Any]
  private let globalShortcutAcceleratorResetter: () -> [String: Any]
  private let updateStarter: () -> String

  init(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline?,
    openRoute: @escaping (DesktopWebRoute) -> Void,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any],
    updateStarter: @escaping () -> String
  ) {
    self.runtime = runtime
    self.startupTimeline = startupTimeline
    self.openRoute = openRoute
    self.globalShortcutStatusProvider = globalShortcutStatusProvider
    self.globalShortcutEnabledUpdater = globalShortcutEnabledUpdater
    self.globalShortcutAcceleratorUpdater = globalShortcutAcceleratorUpdater
    self.globalShortcutAcceleratorResetter = globalShortcutAcceleratorResetter
    self.updateStarter = updateStarter
  }

  func attach(webView: WKWebView) {
    self.webView = webView
  }

  func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
    if let reason = DesktopWebOriginPolicy.rejectionReason(message: message, runtime: runtime) {
      var metadata = DesktopWebOriginPolicy.metadata(message: message, runtime: runtime)
      metadata["reason"] = reason
      startupTimeline?.mark("desktop_bridge.rejected", metadata: metadata)
      reject(requestID: requestID(from: message.body), message: "Desktop bridge origin is not allowed")
      return
    }
    guard let request = DesktopBridgeRequest(body: message.body) else {
      reject(requestID: requestID(from: message.body), message: "Desktop bridge request is invalid")
      return
    }

    do {
      let payload = try handle(request)
      resolve(requestID: request.requestID, payload: payload)
    } catch {
      reject(requestID: request.requestID, message: error.localizedDescription)
    }
  }

  private func handle(_ request: DesktopBridgeRequest) throws -> [String: Any] {
    switch request.kind {
    case "app.get_app_version":
      return [
        "app_mode": runtime.appMode,
        "app_version": runtime.appVersion,
        "build_number": runtime.buildNumber,
        "platform": runtime.platform,
      ]
    case "app.get_state_root":
      return DesktopStateRootStore.statusPayload()
    case "app.choose_state_root":
      return chooseStateRoot(
        initialPath: request.stringPayload("initial_path"),
        title: request.stringPayload("title"),
        prompt: request.stringPayload("prompt")
      )
    case "app.relocate_state_root":
      let target = try DesktopStateRootMigration.scheduleMigration(
        to: request.stringPayload("path")
      )
      DispatchQueue.main.async {
        NSApp.terminate(nil)
      }
      return [
        "restarting": true,
        "target_path": target.path,
      ]
    case "app.open_external_url":
      let rawURL = request.stringPayload("url")
      try openExternalURL(rawURL)
      return ["opened": true]
    case "app.start_browser_extension_setup":
      return try startBrowserExtensionSetup()
    case "app.get_workspace_file_applications":
      return try DesktopWorkspaceFileActions.applicationsPayload(
        for: request.stringPayload("path")
      )
    case "app.open_workspace_file":
      return try DesktopWorkspaceFileActions.openPayload(
        path: request.stringPayload("path"),
        target: request.stringPayload("target"),
        applicationPath: request.stringPayload("application_path")
      )
    case "app.export_logs":
      return try exportLogs()
    case "app.open_route":
      let rawRoute = request.stringPayload("route")
      guard let route = DesktopWebRoute.appRoute(rawRoute) else {
        throw DesktopBridgeError.invalidRoute
      }
      DispatchQueue.main.async {
        self.openRoute(route)
      }
      return ["opened": true]
    case "app.start_update":
      return ["status": updateStarter()]
    case "app.get_persistent_state":
      let key = request.stringPayload("key")
      let value = try DesktopPersistentStateStore.get(key)
      return [
        "key": key,
        "value": value ?? NSNull(),
      ]
    case "app.set_persistent_state":
      try DesktopPersistentStateStore.set(request.stringPayload("value"), forKey: request.stringPayload("key"))
      return ["saved": true]
    case "app.remove_persistent_state":
      try DesktopPersistentStateStore.remove(request.stringPayload("key"))
      return ["removed": true]
    case "app.get_global_shortcut_status":
      return globalShortcutStatusProvider()
    case "app.set_global_shortcut_enabled":
      return globalShortcutEnabledUpdater(request.boolPayload("enabled"))
    case "app.set_global_shortcut_accelerator":
      return globalShortcutAcceleratorUpdater(request.stringPayload("accelerator"))
    case "app.reset_global_shortcut_accelerator":
      return globalShortcutAcceleratorResetter()
    default:
      throw DesktopBridgeError.unsupportedKind(request.kind)
    }
  }

  private func openExternalURL(_ rawURL: String) throws {
    guard let url = URL(string: rawURL) else {
      throw DesktopBridgeError.invalidURL
    }
    try DesktopExternalURLPolicy.open(url)
  }

  private func startBrowserExtensionSetup() throws -> [String: Any] {
    let root = URL(fileURLWithPath: runtime.appRootPath, isDirectory: true)
    let extensionCandidates = [
      root.appendingPathComponent("Nexus Browser Extension", isDirectory: true),
      root.appendingPathComponent("desktop/browser-extension", isDirectory: true),
    ]
    guard let extensionURL = extensionCandidates.first(where: {
      FileManager.default.fileExists(atPath: $0.appendingPathComponent("manifest.json").path)
    }) else {
      throw DesktopBridgeError.browserExtensionUnavailable
    }
    let browserCandidates = [
      (kind: "chrome", name: "Google Chrome", bundleID: "com.google.Chrome", extensionsURL: "chrome://extensions"),
      (kind: "edge", name: "Microsoft Edge", bundleID: "com.microsoft.edgemac", extensionsURL: "edge://extensions"),
    ]
    var browser: (kind: String, name: String, applicationURL: URL, extensionsURL: URL)?
    for candidate in browserCandidates {
      guard let applicationURL = NSWorkspace.shared.urlForApplication(
              withBundleIdentifier: candidate.bundleID
            ),
            let extensionsURL = URL(string: candidate.extensionsURL) else {
        continue
      }
      browser = (candidate.kind, candidate.name, applicationURL, extensionsURL)
      break
    }
    guard let browser else {
      throw DesktopBridgeError.browserExtensionsPageUnavailable
    }
    let configuration = NSWorkspace.OpenConfiguration()
    configuration.activates = true
    NSWorkspace.shared.open(
      [browser.extensionsURL],
      withApplicationAt: browser.applicationURL,
      configuration: configuration
    ) { _, _ in
      NSWorkspace.shared.activateFileViewerSelecting([extensionURL])
    }
    return [
      "browser": browser.kind,
      "browser_name": browser.name,
      "opened": true,
    ]
  }

  private func chooseStateRoot(initialPath: String, title: String, prompt: String) -> [String: Any] {
    let panel = NSOpenPanel()
    panel.title = title.isEmpty ? "选择新的 Nexus 数据目录" : title
    panel.prompt = prompt.isEmpty ? "选择目录" : prompt
    panel.canChooseFiles = false
    panel.canChooseDirectories = true
    panel.allowsMultipleSelection = false
    panel.canCreateDirectories = true
    panel.resolvesAliases = true
    panel.treatsFilePackagesAsDirectories = false
    panel.directoryURL = existingDirectoryURL(for: initialPath)

    guard panel.runModal() == .OK, let destination = panel.url else {
      return ["cancelled": true]
    }
    return [
      "cancelled": false,
      "path": destination.standardizedFileURL.path,
    ]
  }

  private func existingDirectoryURL(for rawPath: String) -> URL? {
    let trimmedPath = rawPath.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmedPath.isEmpty else {
      return nil
    }

    let fileManager = FileManager.default
    var candidate = URL(fileURLWithPath: trimmedPath, isDirectory: true).standardizedFileURL
    while true {
      var isDirectory: ObjCBool = false
      if fileManager.fileExists(atPath: candidate.path, isDirectory: &isDirectory),
         isDirectory.boolValue {
        return candidate
      }
      let parent = candidate.deletingLastPathComponent()
      guard parent.path != candidate.path else {
        return nil
      }
      candidate = parent
    }
  }

  private func exportLogs() throws -> [String: Any] {
    let savePanel = NSSavePanel()
    savePanel.title = "导出 Nexus 日志"
    savePanel.nameFieldStringValue = "nexus-logs-\(timestampString()).zip"
    savePanel.allowedContentTypes = [.zip]
    savePanel.canCreateDirectories = true

    guard savePanel.runModal() == .OK, let destination = savePanel.url else {
      return ["cancelled": true]
    }

    let archiveURL = try buildLogsArchive()
    if FileManager.default.fileExists(atPath: destination.path) {
      try FileManager.default.removeItem(at: destination)
    }
    try FileManager.default.moveItem(at: archiveURL, to: destination)
    return [
      "cancelled": false,
      "path": destination.path,
    ]
  }

  private func buildLogsArchive() throws -> URL {
    let fileManager = FileManager.default
    let tempRoot = fileManager.temporaryDirectory
      .appendingPathComponent("nexus-log-export-\(UUID().uuidString)", isDirectory: true)
    let staging = tempRoot.appendingPathComponent("NexusLogs", isDirectory: true)
    try fileManager.createDirectory(at: staging, withIntermediateDirectories: true)

    let logsDirectory = DesktopDiagnosticsReport.logsDirectory()
    if fileManager.fileExists(atPath: logsDirectory.path) {
      try fileManager.copyItem(at: logsDirectory, to: staging.appendingPathComponent("Logs", isDirectory: true))
    }
    let debugDirectory = DesktopPaths.debugDirectory
    if fileManager.fileExists(atPath: debugDirectory.path) {
      try fileManager.copyItem(at: debugDirectory, to: staging.appendingPathComponent("Debug", isDirectory: true))
    }
    try DesktopDiagnosticsReport.make(
      runtime: runtime,
      reason: "manual_log_export",
      startupTimeline: startupTimeline
    ).write(
      to: staging.appendingPathComponent("diagnostics.json"),
      atomically: true,
      encoding: .utf8
    )

    let archiveURL = tempRoot.appendingPathComponent("NexusLogs.zip")
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/ditto")
    process.arguments = ["-c", "-k", "--sequesterRsrc", "--keepParent", staging.lastPathComponent, archiveURL.path]
    process.currentDirectoryURL = tempRoot
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0 else {
      throw DesktopBridgeError.archiveFailed
    }
    return archiveURL
  }

  private func resolve(requestID: String, payload: [String: Any]) {
    guard !requestID.isEmpty else {
      return
    }
    do {
      let json = try jsonLiteral(payload)
      evaluate("window.__NEXUS_DESKTOP_BRIDGE__?.resolve(\(try jsonStringLiteral(requestID)), \(json));")
    } catch {
      NSLog("[Nexus DesktopBridge] resolve failed: \(error.localizedDescription)")
    }
  }

  private func reject(requestID: String, message: String) {
    guard !requestID.isEmpty else {
      return
    }
    do {
      evaluate("window.__NEXUS_DESKTOP_BRIDGE__?.reject(\(try jsonStringLiteral(requestID)), \(try jsonStringLiteral(message)));")
    } catch {
      NSLog("[Nexus DesktopBridge] reject failed: \(error.localizedDescription)")
    }
  }

  private func evaluate(_ script: String) {
    webView?.evaluateJavaScript(script) { _, error in
      if let error {
        NSLog("[Nexus DesktopBridge] callback failed: \(error.localizedDescription)")
      }
    }
  }

  private func timestampString() -> String {
    let formatter = DateFormatter()
    formatter.dateFormat = "yyyyMMdd-HHmmss"
    return formatter.string(from: Date())
  }

  private func jsonLiteral(_ value: Any) throws -> String {
    let data = try JSONSerialization.data(withJSONObject: value, options: [.fragmentsAllowed])
    guard let text = String(data: data, encoding: .utf8) else {
      throw DesktopBridgeError.invalidResponse
    }
    return text
  }

  private func jsonStringLiteral(_ value: String) throws -> String {
    try jsonLiteral(value)
  }

  private func requestID(from body: Any) -> String {
    guard let record = body as? [String: Any] else {
      return ""
    }
    return (record["request_id"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }
}

private struct DesktopBridgeRequest {
  let requestID: String
  let kind: String
  let payload: [String: Any]

  init?(body: Any) {
    guard let record = body as? [String: Any] else {
      return nil
    }
    let schemaVersion = record["schema_version"] as? Int
    guard schemaVersion == 1 else {
      return nil
    }
    guard let requestID = record["request_id"] as? String,
          let kind = record["kind"] as? String else {
      return nil
    }
    self.requestID = requestID.trimmingCharacters(in: .whitespacesAndNewlines)
    self.kind = kind.trimmingCharacters(in: .whitespacesAndNewlines)
    self.payload = record["payload"] as? [String: Any] ?? [:]
    if self.requestID.isEmpty || self.kind.isEmpty {
      return nil
    }
  }

  func stringPayload(_ key: String) -> String {
    (payload[key] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  func boolPayload(_ key: String) -> Bool {
    payload[key] as? Bool ?? false
  }
}

private enum DesktopBridgeError: LocalizedError {
  case unsupportedKind(String)
  case invalidURL
  case invalidRoute
  case archiveFailed
  case invalidResponse
  case browserExtensionUnavailable
  case browserExtensionsPageUnavailable

  var errorDescription: String? {
    switch self {
    case .unsupportedKind(let kind):
      return "不支持的桌面桥接请求：\(kind)"
    case .invalidURL:
      return "外部链接无效。"
    case .invalidRoute:
      return "桌面路由无效。"
    case .archiveFailed:
      return "日志归档失败。"
    case .invalidResponse:
      return "桌面桥接响应生成失败。"
    case .browserExtensionUnavailable:
      return "未找到 Nexus 浏览器扩展，请重新安装或更新 Nexus。"
    case .browserExtensionsPageUnavailable:
      return "无法打开浏览器扩展程序页面，请确认已安装 Google Chrome 或 Microsoft Edge。"
    }
  }
}
