import Foundation

enum DesktopDiagnosticsReport {
  static func make(
    runtime: SidecarRuntimeConfig?,
    reason: String? = nil,
    startupTimeline: DesktopStartupTimeline? = nil,
    details: [String: Any] = [:]
  ) throws -> String {
    var payload: [String: Any] = [
      "generated_at": ISO8601DateFormatter().string(from: Date()),
      "app": appPayload(),
      "process": processPayload(),
      "system": systemPayload(),
      "paths": pathsPayload(),
      "checks": checksPayload(),
    ]
    if let runtime {
      payload["runtime"] = runtimePayload(runtime)
    }
    if let reason, !reason.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      payload["reason"] = reason
    }
    if let startupTimeline {
      payload["startup_timeline"] = startupTimeline.snapshot()
    }
    if !details.isEmpty {
      payload["details"] = details
    }
    return try jsonString(payload)
  }

  static func writeStartupFailure(error: Error, startupTimeline: DesktopStartupTimeline? = nil) -> URL? {
    do {
      let directory = logsDirectory()
      try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
      let fileURL = directory.appendingPathComponent("startup-failure-\(timestampString()).json")
      let text = try make(runtime: nil, reason: error.localizedDescription, startupTimeline: startupTimeline)
      try text.write(to: fileURL, atomically: true, encoding: .utf8)
      return fileURL
    } catch {
      NSLog("[Nexus Diagnostics] failed to write startup diagnostics: \(error.localizedDescription)")
      return nil
    }
  }

  static func writeRuntimeIssue(
    prefix: String,
    reason: String,
    runtime: SidecarRuntimeConfig?,
    startupTimeline: DesktopStartupTimeline? = nil,
    details: [String: Any] = [:]
  ) -> URL? {
    do {
      let directory = logsDirectory()
      try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
      let safePrefix = sanitizedFilePrefix(prefix)
      let fileURL = directory.appendingPathComponent("\(safePrefix)-\(timestampString()).json")
      let text = try make(
        runtime: runtime,
        reason: reason,
        startupTimeline: startupTimeline,
        details: details
      )
      try text.write(to: fileURL, atomically: true, encoding: .utf8)
      return fileURL
    } catch {
      NSLog("[Nexus Diagnostics] failed to write runtime diagnostics: \(error.localizedDescription)")
      return nil
    }
  }

  static func logsDirectory() -> URL {
    let base = FileManager.default.urls(for: .libraryDirectory, in: .userDomainMask).first
      ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library")
    return base.appendingPathComponent("Logs/Nexus", isDirectory: true)
  }

  private static func appPayload() -> [String: Any] {
    let bundle = Bundle.main
    return [
      "name": bundle.object(forInfoDictionaryKey: "CFBundleName") as? String ?? "Nexus",
      "bundle_identifier": bundle.bundleIdentifier ?? "",
      "version": bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0.0.0",
      "build_number": bundle.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "dev",
      "bundle_path": bundle.bundleURL.path,
      "resource_path": bundle.resourceURL?.path ?? "",
      "executable_path": bundle.executableURL?.path ?? "",
      "url_schemes": urlSchemesPayload(bundle: bundle),
    ]
  }

  private static func processPayload() -> [String: Any] {
    let info = ProcessInfo.processInfo
    return [
      "pid": info.processIdentifier,
      "process_name": info.processName,
      "host_name": info.hostName,
      "current_directory": FileManager.default.currentDirectoryPath,
    ]
  }

  private static func systemPayload() -> [String: Any] {
    let version = ProcessInfo.processInfo.operatingSystemVersion
    return [
      "platform": "macos",
      "os_version": "\(version.majorVersion).\(version.minorVersion).\(version.patchVersion)",
      "active_processor_count": ProcessInfo.processInfo.activeProcessorCount,
      "physical_memory": ProcessInfo.processInfo.physicalMemory,
    ]
  }

  private static func runtimePayload(_ runtime: SidecarRuntimeConfig) -> [String: Any] {
    [
      "app_mode": runtime.appMode,
      "app_version": runtime.appVersion,
      "build_number": runtime.buildNumber,
      "platform": runtime.platform,
      "web_url": runtime.webURL.absoluteString,
      "api_base_url": runtime.apiBaseURL.absoluteString,
      "websocket_url": runtime.webSocketURL.absoluteString,
      "health_url": runtime.healthURL.absoluteString,
    ]
  }

  private static func pathsPayload() -> [String: Any] {
    let appSupport = applicationSupportDirectory()
    let config = appSupport.appendingPathComponent("config", isDirectory: true)
    return [
      "application_support_dir": appSupport.path,
      "logs_dir": logsDirectory().path,
      "config_dir": config.path,
      "workspace_dir": appSupport.appendingPathComponent("workspace", isDirectory: true).path,
      "cache_dir": appSupport.appendingPathComponent("cache", isDirectory: true).path,
      "sidecar_pid_record": appSupport.appendingPathComponent("NexusSidecar.pid.json").path,
      "connector_credentials_fallback_key": config.appendingPathComponent("connector-credentials.key").path,
    ]
  }

  private static func checksPayload() -> [String: Any] {
    let fileManager = FileManager.default
    let appSupport = applicationSupportDirectory()
    let config = appSupport.appendingPathComponent("config", isDirectory: true)
    return [
      "application_support_exists": fileManager.fileExists(atPath: appSupport.path),
      "logs_dir_exists": fileManager.fileExists(atPath: logsDirectory().path),
      "sidecar_pid_record_exists": fileManager.fileExists(
        atPath: appSupport.appendingPathComponent("NexusSidecar.pid.json").path
      ),
      "connector_credentials_fallback_key_exists": fileManager.fileExists(
        atPath: config.appendingPathComponent("connector-credentials.key").path
      ),
      "bundled_web_index_exists": bundledResourceExists(relativePath: "Web/index.html"),
      "bundled_sidecar_exists": bundledExecutableExists(name: "nexus-server"),
      "nexus_url_scheme_declared": declaredURLSchemes(bundle: Bundle.main).contains("nexus"),
    ]
  }

  private static func bundledResourceExists(relativePath: String) -> Bool {
    guard let resourceURL = Bundle.main.resourceURL else {
      return false
    }
    return FileManager.default.fileExists(atPath: resourceURL.appendingPathComponent(relativePath).path)
  }

  private static func bundledExecutableExists(name: String) -> Bool {
    guard let executableDir = Bundle.main.executableURL?.deletingLastPathComponent() else {
      return false
    }
    return FileManager.default.isExecutableFile(atPath: executableDir.appendingPathComponent(name).path)
  }

  private static func applicationSupportDirectory() -> URL {
    let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
      ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support")
    return base.appendingPathComponent("Nexus", isDirectory: true)
  }

  private static func timestampString() -> String {
    let formatter = DateFormatter()
    formatter.dateFormat = "yyyyMMdd-HHmmss"
    return formatter.string(from: Date())
  }

  private static func urlSchemesPayload(bundle: Bundle) -> [String] {
    declaredURLSchemes(bundle: bundle)
  }

  private static func declaredURLSchemes(bundle: Bundle) -> [String] {
    guard let urlTypes = bundle.object(forInfoDictionaryKey: "CFBundleURLTypes") as? [[String: Any]] else {
      return []
    }
    return urlTypes
      .flatMap { $0["CFBundleURLSchemes"] as? [String] ?? [] }
      .map { $0.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() }
      .filter { !$0.isEmpty }
      .sorted()
  }

  private static func sanitizedFilePrefix(_ prefix: String) -> String {
    let allowed = CharacterSet(charactersIn: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")
    let value = prefix
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .unicodeScalars
      .map { allowed.contains($0) ? Character($0) : "-" }
    let text = String(value).trimmingCharacters(in: CharacterSet(charactersIn: "-_"))
    return text.isEmpty ? "runtime-issue" : text
  }

  private static func jsonString(_ value: Any) throws -> String {
    let data = try JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys])
    guard let text = String(data: data, encoding: .utf8) else {
      throw DesktopDiagnosticsError.invalidJSON
    }
    return text
  }
}

private enum DesktopDiagnosticsError: LocalizedError {
  case invalidJSON

  var errorDescription: String? {
    switch self {
    case .invalidJSON:
      return "桌面诊断报告生成失败。"
    }
  }
}
