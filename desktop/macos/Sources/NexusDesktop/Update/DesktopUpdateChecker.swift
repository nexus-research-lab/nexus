// INPUT: Update-check reasons, release metadata, download progress, and verified package facts.
// OUTPUT: Stage-specific update behavior and safe native feedback with raw causes kept in diagnostics.
// POS: macOS update coordinator; check, download/verification, and installation preparation stay distinct.

import AppKit
import CryptoKit
import Foundation

@MainActor
final class DesktopUpdateChecker {
  private enum CheckReason: String {
    case startup
    case periodic
    case manual
  }

  private enum DefaultsKey {
    static let lastAutomaticCheckAt = "NexusUpdateChecker.lastAutomaticCheckAt"
    static let lastResult = "NexusUpdateChecker.lastResult"
    static let lastLatestVersion = "NexusUpdateChecker.lastLatestVersion"
    static let lastLatestBuildNumber = "NexusUpdateChecker.lastLatestBuildNumber"
    static let lastErrorMessage = "NexusUpdateChecker.lastErrorMessage"
  }

  private static let automaticCheckInterval: TimeInterval = 4 * 60 * 60
  private static let downloadTimeout: TimeInterval = 10 * 60
  private static let sha256ByteCount = 64
  private static let latestReleaseURL = URL(string: "https://api.github.com/repos/nexus-research-lab/nexus/releases/latest")!
  private static let fallbackReleasePageURL = URL(string: "https://github.com/nexus-research-lab/nexus/releases/latest")!

  private let currentVersion: DesktopAppVersion
  private let startupTimeline: DesktopStartupTimeline
  private let defaults: UserDefaults
  private let session: URLSession
  private let isDisabled: Bool
  private var hasPerformedStartupCheck = false
  private var checkTask: Task<Void, Never>?
  private var automaticCheckTask: Task<Void, Never>?
  private var availableRelease: DesktopReleaseInfo?
  private var updateTask: Task<Void, Never>?

  init(
    startupTimeline: DesktopStartupTimeline,
    defaults: UserDefaults = .standard,
    session: URLSession = .shared
  ) {
    self.currentVersion = DesktopAppVersion.fromBundle()
    self.startupTimeline = startupTimeline
    self.defaults = defaults
    self.session = session
    self.isDisabled = ProcessInfo.processInfo.environment["NEXUS_DESKTOP_DISABLE_UPDATE_CHECK"] == "1"
  }

  func checkOnLaunchIfNeeded() {
    guard !isDisabled else {
      startupTimeline.mark("update_check.skipped", metadata: ["reason": "disabled"])
      return
    }

    guard !hasPerformedStartupCheck else {
      return
    }
    hasPerformedStartupCheck = true

    runCheck(reason: .startup, showsUpToDateAlert: false)
    startAutomaticChecks()
  }

  func checkNowFromMenu() {
    guard !isDisabled else {
      startupTimeline.mark("update_check.skipped", metadata: ["reason": "disabled"])
      return
    }

    runCheck(reason: .manual, showsUpToDateAlert: true)
  }

  func startAvailableUpdate() -> String {
    startUpdateOperation(preferredRelease: nil)
  }

  func clearStaleUpdateCacheIfNeeded() async {
    await DesktopUpdateCacheCleaner.clearStaleCachesIfNeeded(
      currentVersion: currentVersion,
      startupTimeline: startupTimeline,
      defaults: defaults
    )
  }

  private func runCheck(reason: CheckReason, showsUpToDateAlert: Bool) {
    checkTask?.cancel()
    checkTask = Task { [weak self] in
      guard let self else {
        return
      }
      await self.performCheck(reason: reason, showsUpToDateAlert: showsUpToDateAlert)
    }
  }

  private func startAutomaticChecks() {
    automaticCheckTask?.cancel()
    automaticCheckTask = Task { [weak self] in
      while !Task.isCancelled {
        do {
          try await Task.sleep(for: .seconds(Self.automaticCheckInterval))
        } catch {
          return
        }
        guard let self else {
          return
        }
        await self.performCheck(reason: .periodic, showsUpToDateAlert: false)
      }
    }
  }

  private func performCheck(reason: CheckReason, showsUpToDateAlert: Bool) async {
    startupTimeline.mark("update_check.started", metadata: [
      "reason": reason.rawValue,
      "current_version": currentVersion.version,
      "current_build": currentVersion.buildNumber,
      "current_architecture": DesktopArchitecture.current,
    ])

    do {
      let latest = try await fetchLatestRelease()
      if reason == .startup || reason == .periodic {
        defaults.set(Date(), forKey: DefaultsKey.lastAutomaticCheckAt)
      }
      defaults.set(latest.version, forKey: DefaultsKey.lastLatestVersion)
      if let buildNumber = latest.buildNumber {
        defaults.set(buildNumber, forKey: DefaultsKey.lastLatestBuildNumber)
      } else {
        defaults.removeObject(forKey: DefaultsKey.lastLatestBuildNumber)
      }
      defaults.removeObject(forKey: DefaultsKey.lastErrorMessage)

      let hasUpdate = latest.isNewer(than: currentVersion)
      availableRelease = hasUpdate ? latest : nil
      defaults.set(hasUpdate ? "update_available" : "up_to_date", forKey: DefaultsKey.lastResult)
      startupTimeline.mark("update_check.result", metadata: [
        "reason": reason.rawValue,
        "status": hasUpdate ? "update_available" : "up_to_date",
        "current_version": currentVersion.version,
        "current_build": currentVersion.buildNumber,
        "latest_version": latest.version,
        "latest_build": latest.buildNumber ?? "",
        "source": latest.source,
        "package_asset": latest.packageFileName ?? "",
        "sha256_asset": latest.packageSHA256FileName ?? "",
      ])

      if hasUpdate {
        try? DesktopPersistentStateStore.set(latest.version, forKey: "desktop.update.available")
        if reason != .periodic {
          showUpdateAvailableAlert(latest)
        }
      } else if showsUpToDateAlert {
        try? DesktopPersistentStateStore.remove("desktop.update.available")
        showUpToDateAlert(latest)
      } else {
        try? DesktopPersistentStateStore.remove("desktop.update.available")
      }
    } catch {
      defaults.set("failed", forKey: DefaultsKey.lastResult)
      defaults.set(DesktopFailureCopy.updateCheckMessage, forKey: DefaultsKey.lastErrorMessage)
      startupTimeline.mark("update_check.failed", metadata: [
        "reason": reason.rawValue,
        "error": error.localizedDescription,
      ])
      if showsUpToDateAlert {
        showCheckFailedAlert()
      }
    }
  }

  private func fetchLatestRelease() async throws -> DesktopReleaseInfo {
    let release: GitHubRelease = try await fetchJSON(Self.latestReleaseURL)
    let currentArchitecture = DesktopArchitecture.current
    let metadataAsset = DesktopReleaseAssetSelector.macOSMetadataAsset(
      in: release.assets,
      architecture: currentArchitecture
    )
    let packageAsset = DesktopReleaseAssetSelector.macOSPackageAsset(
      in: release.assets,
      architecture: currentArchitecture
    )
    let packageSHA256Asset = DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
      in: release.assets,
      packageAsset: packageAsset,
      architecture: currentArchitecture
    )

    if let metadataURL = metadataAsset?.browserDownloadURL {
      do {
        let metadata: DesktopPackageMetadata = try await fetchJSON(metadataURL)
        if let packageArchitecture = metadata.architecture,
           !DesktopArchitecture.matches(packageArchitecture, expected: currentArchitecture) {
          throw DesktopUpdateError.packageArchitectureMismatch(
            expected: currentArchitecture,
            actual: packageArchitecture
          )
        }
        return DesktopReleaseInfo(
          version: metadata.version,
          buildNumber: metadata.buildNumber,
          releaseName: release.name,
          releasePageURL: release.htmlURL ?? Self.fallbackReleasePageURL,
          packageFileName: packageAsset?.name,
          packageDownloadURL: packageAsset?.browserDownloadURL,
          packageSHA256FileName: packageSHA256Asset?.name,
          packageSHA256URL: packageSHA256Asset?.browserDownloadURL,
          releaseNotes: release.body,
          publishedAt: release.publishedAt,
          isPrerelease: release.prerelease,
          source: "github_release_metadata",
          packageSigning: metadata.signing
        )
      } catch {
        startupTimeline.mark("update_check.metadata_failed", metadata: [
          "error": error.localizedDescription,
        ])
      }
    }

    return DesktopReleaseInfo(
      version: GitHubReleaseVersionNormalizer.version(from: release.tagName),
      buildNumber: nil,
      releaseName: release.name,
      releasePageURL: release.htmlURL ?? Self.fallbackReleasePageURL,
      packageFileName: packageAsset?.name,
      packageDownloadURL: packageAsset?.browserDownloadURL,
      packageSHA256FileName: packageSHA256Asset?.name,
      packageSHA256URL: packageSHA256Asset?.browserDownloadURL,
      releaseNotes: release.body,
      publishedAt: release.publishedAt,
      isPrerelease: release.prerelease,
      source: "github_release",
      packageSigning: nil
    )
  }

  private func fetchJSON<T: Decodable>(_ url: URL) async throws -> T {
    var request = URLRequest(url: url)
    request.timeoutInterval = 15
    request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
    request.setValue("Nexus-macOS/\(currentVersion.version)", forHTTPHeaderField: "User-Agent")

    let (data, response) = try await session.data(for: request)
    guard let httpResponse = response as? HTTPURLResponse else {
      throw DesktopUpdateError.invalidResponse
    }
    guard (200..<300).contains(httpResponse.statusCode) else {
      throw DesktopUpdateError.badStatusCode(httpResponse.statusCode)
    }

    let decoder = JSONDecoder()
    return try decoder.decode(T.self, from: data)
  }

  private func showUpdateAvailableAlert(_ latest: DesktopReleaseInfo) {
    let canInstall = latest.canAutoInstallPackage && currentInstallTargetURL() != nil
    startupTimeline.mark("update_check.prompt_shown", metadata: [
      "latest_version": latest.version,
      "latest_build": latest.buildNumber ?? "",
      "can_download_package": latest.canDownloadPackage ? "true" : "false",
      "can_auto_install_package": latest.canAutoInstallPackage ? "true" : "false",
      "can_install_in_place": canInstall ? "true" : "false",
      "package_signing": latest.packageSigning?.kind ?? "unknown",
      "package_developer_id": latest.packageSigning.map { $0.developerID ? "true" : "false" } ?? "unknown",
      "package_notarized": latest.packageSigning.map { $0.notarized ? "true" : "false" } ?? "unknown",
    ])

    let alert = NSAlert()
    alert.messageText = "发现 Nexus 新版本"
    alert.informativeText = updateAvailableMessage(latest)
    alert.alertStyle = .informational
    alert.accessoryView = Self.releaseNotesAccessoryView(latest.releaseNotes)
    if canInstall {
      alert.addButton(withTitle: "下载并更新")
      alert.addButton(withTitle: "打开下载页")
      alert.addButton(withTitle: "稍后")
    } else {
      alert.addButton(withTitle: "打开下载页")
      alert.addButton(withTitle: "稍后")
    }

    let response = alert.runModal()
    if canInstall {
      switch response {
      case .alertFirstButtonReturn:
        _ = startUpdateOperation(preferredRelease: latest)
      case .alertSecondButtonReturn:
        openReleasePage(latest, reason: "prompt")
      default:
        break
      }
    } else if response == .alertFirstButtonReturn {
      openReleasePage(latest, reason: "prompt")
    }
  }

  private func startUpdateOperation(preferredRelease: DesktopReleaseInfo?) -> String {
    guard !isDisabled else {
      return "disabled"
    }
    guard updateTask == nil else {
      return "in_progress"
    }

    startupTimeline.mark("update_check.update_requested", metadata: [
      "source": preferredRelease == nil ? "sidebar" : "prompt",
    ])
    updateTask = Task { [weak self] in
      guard let self else {
        return
      }
      await self.performUpdateOperation(preferredRelease: preferredRelease)
      self.updateTask = nil
    }
    return "started"
  }

  private func performUpdateOperation(preferredRelease: DesktopReleaseInfo?) async {
    do {
      let latest = try await resolveAvailableRelease(preferredRelease)
      guard latest.isNewer(than: currentVersion) else {
        availableRelease = nil
        try? DesktopPersistentStateStore.remove("desktop.update.available")
        showUpToDateAlert(latest)
        return
      }
      availableRelease = latest
      try? DesktopPersistentStateStore.set(latest.version, forKey: "desktop.update.available")
      await downloadAndInstallUpdate(latest)
    } catch {
      startupTimeline.mark("update_check.update_request_failed", metadata: [
        "error": error.localizedDescription,
      ])
      showCheckFailedAlert()
    }
  }

  private func resolveAvailableRelease(
    _ preferredRelease: DesktopReleaseInfo?
  ) async throws -> DesktopReleaseInfo {
    if let preferredRelease {
      return preferredRelease
    }
    if let availableRelease {
      return availableRelease
    }
    return try await fetchLatestRelease()
  }

  private func showUpToDateAlert(_ latest: DesktopReleaseInfo) {
    let alert = NSAlert()
    alert.messageText = "Nexus 已是最新版本"
    alert.informativeText = """
    当前版本：\(currentVersion.displayText)
    最新版本：\(latest.displayText)
    """
    alert.alertStyle = .informational
    alert.addButton(withTitle: "好")
    alert.runModal()
  }

  private func showCheckFailedAlert() {
    let alert = NSAlert()
    alert.messageText = DesktopFailureCopy.updateCheckTitle
    alert.informativeText = DesktopFailureCopy.updateCheckMessage
    alert.alertStyle = .warning
    alert.addButton(withTitle: "好")
    alert.runModal()
  }

  private func downloadAndInstallUpdate(_ latest: DesktopReleaseInfo) async {
    let canInstallInPlace = currentInstallTargetURL() != nil
    guard latest.canAutoInstallPackage && canInstallInPlace else {
      startupTimeline.mark("update_check.download_unavailable", metadata: [
        "latest_version": latest.version,
        "has_package": (latest.packageDownloadURL != nil) ? "true" : "false",
        "has_sha256": (latest.packageSHA256URL != nil) ? "true" : "false",
        "reason": latest.automaticInstallUnavailableReason
          ?? (canInstallInPlace ? "unknown" : "unsupported_install_location"),
      ])
      showManualDownloadOnlyAlert(latest)
      return
    }

    startupTimeline.mark("update_check.download_started", metadata: [
      "latest_version": latest.version,
      "latest_build": latest.buildNumber ?? "",
      "package_asset": latest.packageFileName ?? "",
    ])

    do {
      let progressWindow = DesktopDownloadProgressWindow(release: latest)
      progressWindow.show()
      let downloadedUpdate: DesktopDownloadedUpdate
      do {
        downloadedUpdate = try await downloadAndVerifyUpdate(
          latest,
          reportProgress: progressWindow.report
        )
      } catch {
        progressWindow.close()
        throw error
      }
      progressWindow.close()
      startupTimeline.mark("update_check.download_verified", metadata: [
        "latest_version": latest.version,
        "package_asset": latest.packageFileName ?? "",
        "sha256": downloadedUpdate.sha256Hash,
      ])

      let stagedAppURL = try await prepareAppBundle(from: downloadedUpdate)
      startupTimeline.mark("update_check.package_prepared", metadata: [
        "latest_version": latest.version,
        "staged_app": stagedAppURL.path,
      ])

      try await verifyStagedAppTrust(stagedAppURL)
      startupTimeline.mark("update_check.package_trust_verified", metadata: [
        "latest_version": latest.version,
        "staged_app": stagedAppURL.path,
      ])

      guard promptInstall(latest, downloadedUpdate: downloadedUpdate) else {
        return
      }

      try launchUpdaterAndQuit(stagedAppURL: stagedAppURL, latest: latest)
    } catch {
      startupTimeline.mark("update_check.download_failed", metadata: [
        "latest_version": latest.version,
        "error": error.localizedDescription,
      ])
      showDownloadFailedAlert(latest)
    }
  }

  private func downloadAndVerifyUpdate(
    _ latest: DesktopReleaseInfo,
    reportProgress: @escaping (Int64, Int64?) -> Void
  ) async throws -> DesktopDownloadedUpdate {
    guard let packageFileName = latest.packageFileName, !packageFileName.isEmpty else {
      throw DesktopUpdateError.missingPackage
    }
    guard let packageDownloadURL = latest.packageDownloadURL else {
      throw DesktopUpdateError.missingPackage
    }
    guard let packageSHA256URL = latest.packageSHA256URL else {
      throw DesktopUpdateError.missingSHA256
    }

    let updateDirectory = try updateDirectory(for: latest)
    let packageURL = updateDirectory.appendingPathComponent(Self.safePathSegment(packageFileName))
    let sha256FileName = latest.packageSHA256FileName?.isEmpty == false
      ? latest.packageSHA256FileName!
      : "\(packageFileName).sha256"
    let sha256URL = updateDirectory.appendingPathComponent(Self.safePathSegment(sha256FileName))

    try await downloadFile(from: packageDownloadURL, to: packageURL, reportProgress: reportProgress)
    try await downloadFile(from: packageSHA256URL, to: sha256URL)

    let expectedHash = try Self.readExpectedSHA256(from: sha256URL, packageFileName: packageFileName)
    let actualHash = try await Task.detached(priority: .utility) {
      try Self.computeSHA256(packageURL)
    }.value
    guard expectedHash.caseInsensitiveCompare(actualHash) == .orderedSame else {
      try? FileManager.default.removeItem(at: packageURL)
      throw DesktopUpdateError.sha256Mismatch
    }

    return DesktopDownloadedUpdate(
      packageURL: packageURL,
      sha256URL: sha256URL,
      packageFileName: packageFileName,
      sha256Hash: actualHash.lowercased()
    )
  }

  private func downloadFile(
    from url: URL,
    to destinationURL: URL,
    reportProgress: ((Int64, Int64?) -> Void)? = nil
  ) async throws {
    try FileManager.default.createDirectory(
      at: destinationURL.deletingLastPathComponent(),
      withIntermediateDirectories: true
    )

    let temporaryURL = destinationURL.appendingPathExtension("download")
    try? FileManager.default.removeItem(at: temporaryURL)
    try? FileManager.default.removeItem(at: destinationURL)

    var request = URLRequest(url: url)
    request.timeoutInterval = Self.downloadTimeout
    request.setValue("Nexus-macOS/\(currentVersion.version)", forHTTPHeaderField: "User-Agent")

    let (bytes, response) = try await session.bytes(for: request)
    guard let httpResponse = response as? HTTPURLResponse else {
      throw DesktopUpdateError.invalidResponse
    }
    guard (200..<300).contains(httpResponse.statusCode) else {
      throw DesktopUpdateError.badStatusCode(httpResponse.statusCode)
    }

    FileManager.default.createFile(atPath: temporaryURL.path, contents: nil)
    let fileHandle = try FileHandle(forWritingTo: temporaryURL)
    defer {
      try? fileHandle.close()
    }
    let totalBytes = response.expectedContentLength > 0 ? response.expectedContentLength : nil
    var receivedBytes: Int64 = 0
    var buffer = Data()
    buffer.reserveCapacity(64 * 1_024)
    for try await byte in bytes {
      buffer.append(byte)
      if buffer.count >= 64 * 1_024 {
        try fileHandle.write(contentsOf: buffer)
        receivedBytes += Int64(buffer.count)
        buffer.removeAll(keepingCapacity: true)
        reportProgress?(receivedBytes, totalBytes)
      }
    }
    if !buffer.isEmpty {
      try fileHandle.write(contentsOf: buffer)
      receivedBytes += Int64(buffer.count)
      reportProgress?(receivedBytes, totalBytes)
    }
    try FileManager.default.moveItem(at: temporaryURL, to: destinationURL)
  }

  private func prepareAppBundle(from downloadedUpdate: DesktopDownloadedUpdate) async throws -> URL {
    let extractionRoot = downloadedUpdate.packageURL
      .deletingLastPathComponent()
      .appendingPathComponent("extracted", isDirectory: true)
    try? FileManager.default.removeItem(at: extractionRoot)
    try FileManager.default.createDirectory(at: extractionRoot, withIntermediateDirectories: true)

    switch downloadedUpdate.packageURL.pathExtension.lowercased() {
    case "zip":
      try await Self.runProcess(
        executablePath: "/usr/bin/ditto",
        arguments: ["-x", "-k", downloadedUpdate.packageURL.path, extractionRoot.path]
      )
      return try await stageAppBundle(from: extractionRoot, in: extractionRoot)
    case "dmg":
      let sourceRoot = try await mountedDMGRoot(from: downloadedUpdate.packageURL, in: extractionRoot)
      do {
        let stagedAppURL = try await stageAppBundle(from: sourceRoot, in: extractionRoot)
        _ = try? await Self.runProcess(
          executablePath: "/usr/bin/hdiutil",
          arguments: ["detach", sourceRoot.path, "-quiet"]
        )
        return stagedAppURL
      } catch {
        _ = try? await Self.runProcess(
          executablePath: "/usr/bin/hdiutil",
          arguments: ["detach", sourceRoot.path, "-quiet"]
        )
        throw error
      }
    default:
      throw DesktopUpdateError.unsupportedPackageFormat(downloadedUpdate.packageURL.pathExtension)
    }
  }

  private func stageAppBundle(from sourceRoot: URL, in extractionRoot: URL) async throws -> URL {
    let extractedAppURL = try Self.findAppBundle(in: sourceRoot)
    let stagedAppURL = extractionRoot.appendingPathComponent("staged/Nexus.app", isDirectory: true)
    try? FileManager.default.removeItem(at: stagedAppURL)
    try FileManager.default.createDirectory(
      at: stagedAppURL.deletingLastPathComponent(),
      withIntermediateDirectories: true
    )
    try await Self.runProcess(
      executablePath: "/usr/bin/ditto",
      arguments: [extractedAppURL.path, stagedAppURL.path]
    )
    return stagedAppURL
  }

  private func mountedDMGRoot(from packageURL: URL, in extractionRoot: URL) async throws -> URL {
    let mountPoint = extractionRoot.appendingPathComponent("mount", isDirectory: true)
    try FileManager.default.createDirectory(at: mountPoint, withIntermediateDirectories: true)
    do {
      try await Self.runProcess(
        executablePath: "/usr/bin/hdiutil",
        arguments: ["attach", packageURL.path, "-nobrowse", "-readonly", "-mountpoint", mountPoint.path]
      )
      return mountPoint
    } catch {
      _ = try? await Self.runProcess(
        executablePath: "/usr/bin/hdiutil",
        arguments: ["detach", mountPoint.path, "-quiet"]
      )
      throw error
    }
  }

  private func promptInstall(_ latest: DesktopReleaseInfo, downloadedUpdate: DesktopDownloadedUpdate) -> Bool {
    let alert = NSAlert()
    alert.messageText = "Nexus \(latest.version) 更新已就绪"
    alert.informativeText = "更新已完成安全校验。继续后应用会短暂退出，并自动重新打开。"
    alert.alertStyle = .informational
    alert.addButton(withTitle: "立即更新")
    alert.addButton(withTitle: "稍后")

    startupTimeline.mark("update_check.install_prompt_shown", metadata: [
      "latest_version": latest.version,
      "latest_build": latest.buildNumber ?? "",
      "package_path": downloadedUpdate.packageURL.path,
    ])
    return alert.runModal() == .alertFirstButtonReturn
  }

  private func launchUpdaterAndQuit(stagedAppURL: URL, latest: DesktopReleaseInfo) throws {
    guard let targetAppURL = currentInstallTargetURL() else {
      throw DesktopUpdateError.unsupportedInstallLocation
    }

    let updateDirectory = try updateDirectory(for: latest)
    let scriptDirectory = updateDirectory.appendingPathComponent("installer", isDirectory: true)
    try FileManager.default.createDirectory(at: scriptDirectory, withIntermediateDirectories: true)

    let scriptURL = scriptDirectory.appendingPathComponent("install-nexus-update.zsh")
    let logURL = DesktopPaths.logsDirectory.appendingPathComponent("update-install-\(Self.timestamp()).log")
    try FileManager.default.createDirectory(at: DesktopPaths.logsDirectory, withIntermediateDirectories: true)
    try Self.installScript.write(to: scriptURL, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: scriptURL.path)

    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/zsh")
    process.arguments = [
      scriptURL.path,
      String(ProcessInfo.processInfo.processIdentifier),
      stagedAppURL.path,
      targetAppURL.path,
      logURL.path,
      updateDirectory.path,
    ]
    try process.run()

    startupTimeline.mark("update_check.installer_started", metadata: [
      "latest_version": latest.version,
      "target_app": targetAppURL.path,
      "staged_app": stagedAppURL.path,
      "log": logURL.path,
    ])
    NSApp.terminate(nil)
  }

  private func showManualDownloadOnlyAlert(_ latest: DesktopReleaseInfo) {
    let alert = NSAlert()
    alert.messageText = DesktopFailureCopy.automaticUpdateUnavailableTitle
    alert.informativeText = DesktopFailureCopy.automaticUpdateUnavailableMessage
    alert.alertStyle = .informational
    alert.addButton(withTitle: "打开下载页")
    alert.addButton(withTitle: "稍后")
    if alert.runModal() == .alertFirstButtonReturn {
      openReleasePage(latest, reason: "download_unavailable")
    }
  }

  private func showDownloadFailedAlert(_ latest: DesktopReleaseInfo) {
    let alert = NSAlert()
    alert.messageText = DesktopFailureCopy.updateIncompleteTitle
    alert.informativeText = DesktopFailureCopy.updateIncompleteMessage
    alert.alertStyle = .warning
    alert.addButton(withTitle: "打开下载页")
    alert.addButton(withTitle: "稍后")
    if alert.runModal() == .alertFirstButtonReturn {
      openReleasePage(latest, reason: "download_failed")
    }
  }

  private func openReleasePage(_ latest: DesktopReleaseInfo, reason: String) {
    startupTimeline.mark("update_check.release_page_opened", metadata: [
      "latest_version": latest.version,
      "reason": reason,
    ])
    NSWorkspace.shared.open(latest.releasePageURL)
  }

  private func updateAvailableMessage(_ latest: DesktopReleaseInfo) -> String {
    var lines = [
      "当前版本：\(currentVersion.displayText)",
      "最新版本：\(latest.displayText)",
    ]
    if let publishedAt = latest.publishedAt, !publishedAt.isEmpty {
      lines.append("发布时间：\(publishedAt)")
    }
    if latest.isPrerelease {
      lines.append("这是一个预发布版本。")
    }
    if Self.formatReleaseNotes(latest.releaseNotes) != nil {
      lines.append("更新内容显示在下方，完整内容可打开官方下载页查看。")
    }
    lines.append("")
    if latest.canAutoInstallPackage && currentInstallTargetURL() != nil {
      lines.append("选择“下载并更新”后，Nexus 会先验证更新包安全性，再询问是否退出并安装。")
    } else if latest.automaticInstallUnavailableReason != nil {
      lines.append("这个版本暂时不能通过应用内更新。可以打开官方下载页手动安装。")
    } else if latest.canDownloadPackage {
      lines.append("当前安装位置不支持应用内更新。可以打开官方下载页手动安装。")
    } else {
      lines.append("这个版本暂时不能通过应用内更新。可以打开官方下载页手动安装。")
    }
    return lines.joined(separator: "\n")
  }

  private func updateDirectory(for latest: DesktopReleaseInfo) throws -> URL {
    let versionSegment = Self.safePathSegment("\(latest.version)-\(latest.buildNumber ?? "unknown")")
    let directory = DesktopPaths.cacheDirectory
      .appendingPathComponent("updates", isDirectory: true)
      .appendingPathComponent(versionSegment, isDirectory: true)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    return directory
  }

  private func currentInstallTargetURL() -> URL? {
    let appURL = Bundle.main.bundleURL.standardizedFileURL
    guard appURL.pathExtension == "app" else {
      return nil
    }

    let targetPath = appURL.path
    guard !targetPath.hasPrefix("/Volumes/") else {
      return nil
    }

    let parentPath = appURL.deletingLastPathComponent().path
    guard FileManager.default.isWritableFile(atPath: parentPath) else {
      return nil
    }
    return appURL
  }

  private func verifyStagedAppTrust(_ stagedAppURL: URL) async throws {
    guard let expectedBundleIdentifier = Bundle.main.bundleIdentifier,
          !expectedBundleIdentifier.isEmpty else {
      throw DesktopUpdateError.appBundleIdentityUnavailable
    }
    guard let stagedBundle = Bundle(url: stagedAppURL),
          let actualBundleIdentifier = stagedBundle.bundleIdentifier,
          !actualBundleIdentifier.isEmpty else {
      throw DesktopUpdateError.appBundleIdentityUnavailable
    }
    guard actualBundleIdentifier == expectedBundleIdentifier else {
      throw DesktopUpdateError.appBundleIdentityMismatch(
        expected: expectedBundleIdentifier,
        actual: actualBundleIdentifier
      )
    }

    try await Self.runProcess(
      executablePath: "/usr/bin/codesign",
      arguments: ["--verify", "--deep", "--strict", stagedAppURL.path]
    )
    try await Self.runProcess(
      executablePath: "/usr/sbin/spctl",
      arguments: ["--assess", "--type", "execute", stagedAppURL.path]
    )
  }
}

private extension DesktopUpdateChecker {
  static let releaseNotesAccessoryWidth: CGFloat = 620
  static let releaseNotesAccessoryHeight: CGFloat = 280
  static let releaseNotesTitleHeight: CGFloat = 22
  static let releaseNotesSpacing: CGFloat = 8
  static let releaseNotesMaxCharacters = 20000

  static let installScript = """
  #!/bin/zsh
  set -euo pipefail

  APP_PID="$1"
  SOURCE_APP="$2"
  TARGET_APP="$3"
  LOG_PATH="$4"
  UPDATE_DIRECTORY="$5"

  {
    echo "Nexus update installer started at $(/bin/date -u '+%Y-%m-%dT%H:%M:%SZ')"
    echo "source: ${SOURCE_APP}"
    echo "target: ${TARGET_APP}"

    for _ in {1..120}; do
      if ! /bin/kill -0 "${APP_PID}" 2>/dev/null; then
        break
      fi
      /bin/sleep 0.5
    done

    if /bin/kill -0 "${APP_PID}" 2>/dev/null; then
      echo "target process still running; terminating ${APP_PID}"
      /bin/kill "${APP_PID}" 2>/dev/null || true
      /bin/sleep 2
    fi

    if /bin/kill -0 "${APP_PID}" 2>/dev/null; then
      echo "target process did not exit"
      exit 1
    fi

    if [[ ! -d "${SOURCE_APP}" ]]; then
      echo "missing staged app: ${SOURCE_APP}"
      exit 1
    fi

    TARGET_PARENT="$(/usr/bin/dirname "${TARGET_APP}")"
    /bin/mkdir -p "${TARGET_PARENT}"
    BACKUP_APP="${TARGET_APP}.previous-update-$(/bin/date +%s)"

    if [[ -d "${TARGET_APP}" ]]; then
      /bin/mv "${TARGET_APP}" "${BACKUP_APP}"
    fi

    if ! /usr/bin/ditto "${SOURCE_APP}" "${TARGET_APP}"; then
      echo "copy failed; restoring previous app"
      if [[ -d "${BACKUP_APP}" && ! -d "${TARGET_APP}" ]]; then
        /bin/mv "${BACKUP_APP}" "${TARGET_APP}"
      fi
      exit 1
    fi

    if [[ -d "${BACKUP_APP}" ]]; then
      /bin/rm -rf "${BACKUP_APP}"
    fi

    if [[ -d "${UPDATE_DIRECTORY}" ]]; then
      if ! /bin/rm -rf "${UPDATE_DIRECTORY}"; then
        echo "update cache cleanup failed: ${UPDATE_DIRECTORY}"
      fi
    fi
    /usr/bin/open "${TARGET_APP}"
    echo "Nexus update installer finished"
  } >> "${LOG_PATH}" 2>&1
  """

  static func formatReleaseNotes(_ rawNotes: String?) -> String? {
    guard let rawNotes else {
      return nil
    }

    let normalized = rawNotes
      .replacingOccurrences(of: "\r\n", with: "\n")
      .replacingOccurrences(of: "\r", with: "\n")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    guard !normalized.isEmpty else {
      return nil
    }

    guard normalized.count > releaseNotesMaxCharacters else {
      return normalized
    }

    let endIndex = normalized.index(normalized.startIndex, offsetBy: releaseNotesMaxCharacters)
    let clipped = String(normalized[..<endIndex])
      .trimmingCharacters(in: .whitespacesAndNewlines)
    return "\(clipped)\n\n...\n完整更新内容可在官方下载页查看。"
  }

  static func releaseNotesAccessoryView(_ rawNotes: String?) -> NSView? {
    guard let releaseNotes = formatReleaseNotes(rawNotes) else {
      return nil
    }

    let container = NSView(frame: NSRect(
      x: 0,
      y: 0,
      width: releaseNotesAccessoryWidth,
      height: releaseNotesAccessoryHeight
    ))
    container.setContentHuggingPriority(.required, for: .vertical)
    container.setContentCompressionResistancePriority(.required, for: .vertical)

    let title = NSTextField(labelWithString: "更新内容")
    title.font = NSFont.systemFont(ofSize: 13, weight: .semibold)
    title.textColor = .labelColor
    title.translatesAutoresizingMaskIntoConstraints = false

    let scrollHeight = releaseNotesAccessoryHeight - releaseNotesTitleHeight - releaseNotesSpacing
    let scrollView = NSScrollView(frame: NSRect(
      x: 0,
      y: 0,
      width: releaseNotesAccessoryWidth,
      height: scrollHeight
    ))
    scrollView.borderType = .bezelBorder
    scrollView.hasVerticalScroller = true
    scrollView.hasHorizontalScroller = false
    scrollView.autohidesScrollers = false
    scrollView.translatesAutoresizingMaskIntoConstraints = false
    scrollView.drawsBackground = true
    scrollView.backgroundColor = .textBackgroundColor

    let textView = NSTextView(frame: NSRect(
      x: 0,
      y: 0,
      width: releaseNotesAccessoryWidth - 2,
      height: scrollHeight
    ))
    textView.textStorage?.setAttributedString(DesktopReleaseNotesRenderer.render(releaseNotes))
    textView.isEditable = false
    textView.isSelectable = true
    textView.drawsBackground = false
    textView.allowsUndo = false
    textView.isAutomaticLinkDetectionEnabled = true
    textView.textContainerInset = NSSize(width: 8, height: 8)
    textView.isHorizontallyResizable = false
    textView.isVerticallyResizable = true
    textView.autoresizingMask = [.width]
    textView.textContainer?.lineFragmentPadding = 0
    textView.textContainer?.containerSize = NSSize(
      width: releaseNotesAccessoryWidth - 18,
      height: CGFloat.greatestFiniteMagnitude
    )
    textView.textContainer?.widthTracksTextView = true
    if let textContainer = textView.textContainer,
       let layoutManager = textView.layoutManager
    {
      layoutManager.ensureLayout(for: textContainer)
      let usedHeight = layoutManager.usedRect(for: textContainer).height + textView.textContainerInset.height * 2
      var textFrame = textView.frame
      textFrame.size.height = max(scrollHeight, usedHeight)
      textView.frame = textFrame
    }

    scrollView.documentView = textView
    container.addSubview(title)
    container.addSubview(scrollView)
    NSLayoutConstraint.activate([
      title.leadingAnchor.constraint(equalTo: container.leadingAnchor),
      title.trailingAnchor.constraint(equalTo: container.trailingAnchor),
      title.topAnchor.constraint(equalTo: container.topAnchor),
      title.heightAnchor.constraint(equalToConstant: releaseNotesTitleHeight),
      scrollView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
      scrollView.trailingAnchor.constraint(equalTo: container.trailingAnchor),
      scrollView.topAnchor.constraint(equalTo: title.bottomAnchor, constant: releaseNotesSpacing),
      scrollView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
    ])
    return container
  }

  static func readExpectedSHA256(from sha256URL: URL, packageFileName: String) throws -> String {
    let content = try String(contentsOf: sha256URL, encoding: .utf8)
    var fallbackHash: String?
    for line in content.components(separatedBy: .newlines) {
      let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
      guard !trimmed.isEmpty else {
        continue
      }

      let parts = trimmed
        .split { character in character == " " || character == "\t" }
        .map(String.init)
      guard let rawHash = parts.first else {
        continue
      }

      let hash = rawHash.trimmingCharacters(in: CharacterSet(charactersIn: "\u{feff}"))
      guard isSHA256Hash(hash) else {
        continue
      }

      if parts.count == 1 {
        return hash
      }

      let publishedFileName = parts
        .dropFirst()
        .joined(separator: " ")
        .trimmingCharacters(in: .whitespacesAndNewlines)
        .trimmingCharacters(in: CharacterSet(charactersIn: "*"))
      if URL(fileURLWithPath: publishedFileName)
        .lastPathComponent
        .caseInsensitiveCompare(packageFileName) == .orderedSame {
        return hash
      }

      if fallbackHash == nil {
        fallbackHash = hash
      }
    }

    guard let fallbackHash else {
      throw DesktopUpdateError.invalidSHA256File
    }
    return fallbackHash
  }

  static func findAppBundle(in directory: URL) throws -> URL {
    let resourceKeys: [URLResourceKey] = [.isDirectoryKey]
    guard let enumerator = FileManager.default.enumerator(
      at: directory,
      includingPropertiesForKeys: resourceKeys,
      options: [.skipsHiddenFiles]
    ) else {
      throw DesktopUpdateError.appBundleNotFound
    }

    var fallbackAppURL: URL?
    for case let appURL as URL in enumerator {
      guard appURL.pathExtension == "app" else {
        continue
      }
      if appURL.lastPathComponent == "Nexus.app" {
        return appURL
      }
      if fallbackAppURL == nil {
        fallbackAppURL = appURL
      }
      enumerator.skipDescendants()
    }

    guard let fallbackAppURL else {
      throw DesktopUpdateError.appBundleNotFound
    }
    return fallbackAppURL
  }

  @discardableResult
  static func runProcess(executablePath: String, arguments: [String]) async throws -> DesktopProcessOutput {
    try await withCheckedThrowingContinuation { continuation in
      let process = Process()
      let standardOutput = Pipe()
      let standardError = Pipe()

      process.executableURL = URL(fileURLWithPath: executablePath)
      process.arguments = arguments
      process.standardOutput = standardOutput
      process.standardError = standardError
      process.terminationHandler = { process in
        let outputData = standardOutput.fileHandleForReading.readDataToEndOfFile()
        let errorData = standardError.fileHandleForReading.readDataToEndOfFile()
        let output = String(data: outputData, encoding: .utf8) ?? ""
        let error = String(data: errorData, encoding: .utf8) ?? ""
        guard process.terminationStatus == 0 else {
          continuation.resume(throwing: DesktopUpdateError.processFailed(
            executablePath,
            process.terminationStatus,
            error.isEmpty ? output : error
          ))
          return
        }

        continuation.resume(returning: DesktopProcessOutput(
          standardOutput: output,
          standardError: error
        ))
      }

      do {
        try process.run()
      } catch {
        continuation.resume(throwing: error)
      }
    }
  }

  static func safePathSegment(_ value: String) -> String {
    let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "._-"))
    let sanitized = String(value.unicodeScalars.map { scalar in
      allowed.contains(scalar) ? Character(scalar) : "_"
    })
      .trimmingCharacters(in: CharacterSet(charactersIn: "._-"))
    return sanitized.isEmpty ? "latest" : sanitized
  }

  static func timestamp() -> String {
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(secondsFromGMT: 0)
    formatter.dateFormat = "yyyyMMdd-HHmmss"
    return formatter.string(from: Date())
  }

  nonisolated static func computeSHA256(_ fileURL: URL) throws -> String {
    let fileHandle = try FileHandle(forReadingFrom: fileURL)
    defer {
      try? fileHandle.close()
    }

    var hasher = SHA256()
    while true {
      let chunk = try fileHandle.read(upToCount: 1024 * 1024) ?? Data()
      if chunk.isEmpty {
        break
      }
      hasher.update(data: chunk)
    }

    return hasher.finalize()
      .map { String(format: "%02x", $0) }
      .joined()
  }

  private static func isSHA256Hash(_ value: String) -> Bool {
    value.count == sha256ByteCount && value.allSatisfy { character in
      character.isHexDigit
    }
  }
}
