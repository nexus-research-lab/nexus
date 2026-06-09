import Foundation

final class SidecarSupervisor {
  private let locator: SidecarBundleLocator
  private let port: Int
  private let runtimeConfig: SidecarRuntimeConfig
  private let orphanReaper: SidecarOrphanReaper
  private let startupTimeline: DesktopStartupTimeline?
  private let stdoutPipe = SidecarLogPipe()
  private let stderrPipe = SidecarLogPipe(label: "sidecar stderr")
  private var process: Process?

  init(startupTimeline: DesktopStartupTimeline? = nil) throws {
    self.startupTimeline = startupTimeline
    locator = try SidecarBundleLocator.resolve()
    port = try SidecarPortAllocator.allocate()
    runtimeConfig = SidecarRuntimeConfig(port: port, sessionToken: try DesktopSessionToken.generate())
    orphanReaper = SidecarOrphanReaper(
      pidFileURL: DesktopPaths.sidecarPIDFileURL,
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
    try DesktopPaths.createRuntimeDirectories()

    environment["NEXUS_APP_MODE"] = "desktop"
    environment["NEXUS_APP_ROOT"] = locator.appRootURL.path
    environment["NEXUS_CONFIG_DIR"] = DesktopPaths.rootDirectory.path
    environment["CLAUDE_CONFIG_DIR"] = DesktopPaths.rootDirectory.path
    environment["HOST"] = "127.0.0.1"
    environment["PORT"] = "\(port)"
    environment["NEXUS_DESKTOP_SESSION_TOKEN"] = runtimeConfig.sessionToken
    environment["WEB_DIST_DIR"] = locator.webDistURL.path
    environment["DATABASE_DRIVER"] = "sqlite"
    environment["DATABASE_URL"] = DesktopPaths.dataDirectory.appendingPathComponent("nexus.db").path
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
    environment["WORKSPACE_PATH"] = DesktopPaths.workspaceDirectory.path
    environment["CACHE_FILE_DIR"] = DesktopPaths.cacheDirectory.path
    environment["LOG_PATH"] = DesktopPaths.logsDirectory.appendingPathComponent("sidecar.log").path
    environment["LOG_STDOUT"] = "true"
    environment["LOG_FILE_ENABLED"] = "true"
    environment["DISCORD_ENABLED"] = "false"
    environment["TELEGRAM_ENABLED"] = "false"
    environment["CONNECTOR_OAUTH_REDIRECT_URI"] = "nexus://connectors/oauth/callback"
    applyPackagedConnectorConfig(to: &environment)
    applyBundledNXSRuntime(to: &environment)
    let webOrigin = runtimeConfig.webURL.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    environment["CONNECTOR_OAUTH_ALLOWED_ORIGINS"] = "\(webOrigin),nexus://connectors"
    return environment
  }

  private func applyPackagedConnectorConfig(to environment: inout [String: String]) {
    let configURL = locator.appRootURL.appendingPathComponent("desktop.env")
    guard let data = try? String(contentsOf: configURL, encoding: .utf8) else {
      return
    }
    for rawLine in data.split(whereSeparator: \.isNewline) {
      let line = rawLine.trimmingCharacters(in: .whitespacesAndNewlines)
      if line.isEmpty || line.hasPrefix("#") {
        continue
      }
      let parts = line.split(separator: "=", maxSplits: 1, omittingEmptySubsequences: false)
      guard parts.count == 2 else {
        continue
      }
      let key = parts[0].trimmingCharacters(in: .whitespacesAndNewlines)
      let value = parts[1].trimmingCharacters(in: .whitespacesAndNewlines)
      if key == "CONNECTOR_GITHUB_CLIENT_ID" && !value.isEmpty {
        environment[key] = value
      }
    }
  }

  private func applyBundledNXSRuntime(to environment: inout [String: String]) {
    guard locator.projectRoot == nil else {
      startupTimeline?.mark("sidecar.nxs_runtime", metadata: [
        "source": "development",
      ])
      return
    }
    if let override = environment["NEXUS_NXS_COMMAND_PATH"], !override.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      startupTimeline?.mark("sidecar.nxs_runtime", metadata: [
        "source": "override",
        "path": override,
      ])
      return
    }
    let nxsURL = locator.appRootURL.appendingPathComponent("bin/nxs")
    if FileManager.default.isExecutableFile(atPath: nxsURL.path) {
      environment["NEXUS_NXS_COMMAND_PATH"] = nxsURL.path
      startupTimeline?.mark("sidecar.nxs_runtime", metadata: [
        "source": "bundled",
        "path": nxsURL.path,
      ])
      return
    }
    startupTimeline?.mark("sidecar.nxs_runtime", metadata: [
      "source": "missing",
    ])
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

}
