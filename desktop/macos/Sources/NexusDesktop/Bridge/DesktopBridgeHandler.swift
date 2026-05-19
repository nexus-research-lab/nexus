import AppKit
import UniformTypeIdentifiers
import WebKit

final class DesktopBridgeHandler: NSObject, WKScriptMessageHandler {
  private weak var webView: WKWebView?
  private let runtime: SidecarRuntimeConfig
  private let startupTimeline: DesktopStartupTimeline?
  private let openRoute: (DesktopWebRoute) -> Void
  private let closeLauncher: () -> Void
  private let globalShortcutStatusProvider: () -> [String: Any]
  private let globalShortcutEnabledUpdater: (Bool) -> [String: Any]
  private let globalShortcutAcceleratorUpdater: (String) -> [String: Any]
  private let globalShortcutAcceleratorResetter: () -> [String: Any]

  init(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline?,
    openRoute: @escaping (DesktopWebRoute) -> Void,
    closeLauncher: @escaping () -> Void,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any]
  ) {
    self.runtime = runtime
    self.startupTimeline = startupTimeline
    self.openRoute = openRoute
    self.closeLauncher = closeLauncher
    self.globalShortcutStatusProvider = globalShortcutStatusProvider
    self.globalShortcutEnabledUpdater = globalShortcutEnabledUpdater
    self.globalShortcutAcceleratorUpdater = globalShortcutAcceleratorUpdater
    self.globalShortcutAcceleratorResetter = globalShortcutAcceleratorResetter
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
    case "app.open_external_url":
      let rawURL = request.stringPayload("url")
      try openExternalURL(rawURL)
      return ["opened": true]
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
    case "app.close_launcher":
      DispatchQueue.main.async {
        self.closeLauncher()
      }
      return ["closed": true]
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
    }
  }
}
