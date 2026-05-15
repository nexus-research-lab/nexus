import Foundation

final class SidecarSupervisor {
  private let locator: SidecarBundleLocator
  private let port: Int
  private let runtimeConfig: SidecarRuntimeConfig
  private let orphanReaper: SidecarOrphanReaper
  private let startupTimeline: DesktopStartupTimeline?
  private let stdoutPipe = SidecarLogPipe(label: "stdout")
  private let stderrPipe = SidecarLogPipe(label: "stderr")
  private var process: Process?

  init(startupTimeline: DesktopStartupTimeline? = nil) throws {
    self.startupTimeline = startupTimeline
    locator = try SidecarBundleLocator.resolve()
    port = try SidecarPortAllocator.allocate()
    runtimeConfig = SidecarRuntimeConfig(port: port, sessionToken: try DesktopSessionToken.generate())
    orphanReaper = SidecarOrphanReaper(
      pidFileURL: Self.sidecarPIDFileURL(),
      expectedExecutablePath: locator.command
    )
    startupTimeline?.mark("sidecar.config_resolved", metadata: [
      "mode": locator.projectRoot == nil ? "bundle" : "development",
      "port": "\(port)",
    ])
  }

  func start() async throws -> SidecarRuntimeConfig {
    startupTimeline?.mark("sidecar.reap_begin")
    orphanReaper.reapIfNeeded()
    startupTimeline?.mark("sidecar.launch_begin")

    let sidecarProcess = Process()
    sidecarProcess.executableURL = URL(fileURLWithPath: locator.command)
    sidecarProcess.arguments = locator.arguments
    sidecarProcess.currentDirectoryURL = locator.workingDirectory
    sidecarProcess.environment = try buildEnvironment()
    sidecarProcess.standardOutput = stdoutPipe.fileHandleForWriting
    sidecarProcess.standardError = stderrPipe.fileHandleForWriting

    try sidecarProcess.run()
    process = sidecarProcess
    startupTimeline?.mark("sidecar.process_started", metadata: [
      "pid": "\(sidecarProcess.processIdentifier)",
    ])
    orphanReaper.write(pid: sidecarProcess.processIdentifier)
    do {
      try await waitUntilHealthy()
    } catch {
      stop()
      throw error
    }
    startupTimeline?.mark("sidecar.health_ready")
    return runtimeConfig
  }

  func stop() {
    defer {
      orphanReaper.removeRecord()
      stdoutPipe.close()
      stderrPipe.close()
    }

    guard let process, process.isRunning else {
      return
    }
    process.terminate()
    process.waitUntilExit()
  }

  private func buildEnvironment() throws -> [String: String] {
    var environment = ProcessInfo.processInfo.environment
    let dataDir = applicationSupportDirectory()
    let logDir = logsDirectory()
    let configDir = dataDir.appendingPathComponent("config", isDirectory: true)
    let workspaceDir = dataDir.appendingPathComponent("workspace", isDirectory: true)
    let cacheDir = dataDir.appendingPathComponent("cache", isDirectory: true)

    for directory in [dataDir, logDir, configDir, workspaceDir, cacheDir] {
      try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    environment["NEXUS_APP_MODE"] = "desktop"
    environment["NEXUS_APP_ROOT"] = locator.appRootURL.path
    environment["NEXUS_CONFIG_DIR"] = configDir.path
    environment["HOST"] = "127.0.0.1"
    environment["PORT"] = "\(port)"
    environment["NEXUS_DESKTOP_SESSION_TOKEN"] = runtimeConfig.sessionToken
    environment["WEB_DIST_DIR"] = locator.webDistURL.path
    environment["DATABASE_DRIVER"] = "sqlite"
    environment["DATABASE_URL"] = dataDir.appendingPathComponent("nexus.db").path
    let credentialsKeyMode = connectorCredentialsKeyMode(environment: environment)
    startupTimeline?.mark("sidecar.credentials_key_begin", metadata: [
      "mode": credentialsKeyMode.rawValue,
    ])
    let credentialsKey = try DesktopKeychainStore.connectorCredentialsKey(mode: credentialsKeyMode)
    environment["CONNECTOR_CREDENTIALS_KEY"] = credentialsKey.value
    startupTimeline?.mark("sidecar.credentials_key_ready", metadata: [
      "mode": credentialsKeyMode.rawValue,
      "reason": credentialsKey.reason,
      "storage": credentialsKey.storage,
    ])
    environment["WORKSPACE_PATH"] = workspaceDir.path
    environment["CACHE_FILE_DIR"] = cacheDir.path
    environment["LOG_PATH"] = logDir.appendingPathComponent("sidecar.log").path
    environment["LOG_STDOUT"] = "true"
    environment["LOG_FILE_ENABLED"] = "true"
    environment["DISCORD_ENABLED"] = "false"
    environment["TELEGRAM_ENABLED"] = "false"
    environment["CONNECTOR_OAUTH_REDIRECT_URI"] = "nexus://connectors/oauth/callback"
    let webOrigin = runtimeConfig.webURL.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    environment["CONNECTOR_OAUTH_ALLOWED_ORIGINS"] = "\(webOrigin),nexus://connectors"
    return environment
  }

  private func connectorCredentialsKeyMode(environment: [String: String]) -> DesktopKeychainMode {
    if let mode = DesktopKeychainMode(environmentValue: environment["NEXUS_DESKTOP_KEYCHAIN_MODE"]) {
      return mode
    }
    if locator.projectRoot != nil {
      return .file
    }
    return .auto
  }

  private func waitUntilHealthy() async throws {
    let deadline = Date().addingTimeInterval(45)
    while Date() < deadline {
      if let process, !process.isRunning {
        throw DesktopShellError.sidecarExited
      }
      if await isHealthy() {
        return
      }
      try await Task.sleep(nanoseconds: 300_000_000)
    }
    throw DesktopShellError.sidecarExited
  }

  private func isHealthy() async -> Bool {
    do {
      let (_, response) = try await URLSession.shared.data(from: runtimeConfig.healthURL)
      guard let httpResponse = response as? HTTPURLResponse else {
        return false
      }
      return (200..<300).contains(httpResponse.statusCode)
    } catch {
      return false
    }
  }

  private func applicationSupportDirectory() -> URL {
    Self.applicationSupportDirectory()
  }

  private static func sidecarPIDFileURL() -> URL {
    applicationSupportDirectory().appendingPathComponent("NexusSidecar.pid.json")
  }

  private static func applicationSupportDirectory() -> URL {
    let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
      ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support")
    return base.appendingPathComponent("Nexus", isDirectory: true)
  }

  private func logsDirectory() -> URL {
    let base = FileManager.default.urls(for: .libraryDirectory, in: .userDomainMask).first
      ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library")
    return base.appendingPathComponent("Logs/Nexus", isDirectory: true)
  }
}
