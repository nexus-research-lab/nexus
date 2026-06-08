import AppKit
import CryptoKit
import Foundation

@MainActor
final class DesktopUpdateChecker {
  private enum CheckReason: String {
    case startup
    case manual
  }

  private enum DefaultsKey {
    static let lastAutomaticCheckAt = "NexusUpdateChecker.lastAutomaticCheckAt"
    static let lastResult = "NexusUpdateChecker.lastResult"
    static let lastLatestVersion = "NexusUpdateChecker.lastLatestVersion"
    static let lastLatestBuildNumber = "NexusUpdateChecker.lastLatestBuildNumber"
    static let lastErrorMessage = "NexusUpdateChecker.lastErrorMessage"
  }

  private static let automaticCheckInterval: TimeInterval = 24 * 60 * 60
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

    if let lastCheckAt = defaults.object(forKey: DefaultsKey.lastAutomaticCheckAt) as? Date {
      let elapsed = Date().timeIntervalSince(lastCheckAt)
      guard elapsed >= Self.automaticCheckInterval else {
        startupTimeline.mark("update_check.skipped", metadata: [
          "reason": "recent",
          "elapsed_minutes": String(Int(elapsed / 60)),
        ])
        return
      }
    }

    runCheck(reason: .startup, showsUpToDateAlert: false)
  }

  func checkNowFromMenu() {
    guard !isDisabled else {
      startupTimeline.mark("update_check.skipped", metadata: ["reason": "disabled"])
      return
    }

    runCheck(reason: .manual, showsUpToDateAlert: true)
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

  private func performCheck(reason: CheckReason, showsUpToDateAlert: Bool) async {
    startupTimeline.mark("update_check.started", metadata: [
      "reason": reason.rawValue,
      "current_version": currentVersion.version,
      "current_build": currentVersion.buildNumber,
    ])

    do {
      let latest = try await fetchLatestRelease()
      if reason == .startup {
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
        showUpdateAvailableAlert(latest)
      } else if showsUpToDateAlert {
        showUpToDateAlert(latest)
      }
    } catch {
      defaults.set("failed", forKey: DefaultsKey.lastResult)
      defaults.set(error.localizedDescription, forKey: DefaultsKey.lastErrorMessage)
      startupTimeline.mark("update_check.failed", metadata: [
        "reason": reason.rawValue,
        "error": error.localizedDescription,
      ])
      if showsUpToDateAlert {
        showCheckFailedAlert(error)
      }
    }
  }

  private func fetchLatestRelease() async throws -> DesktopReleaseInfo {
    let release: GitHubRelease = try await fetchJSON(Self.latestReleaseURL)
    let metadataAsset = Self.findMacOSMetadataAsset(release.assets)
    let packageAsset = Self.findMacOSPackageAsset(release.assets)
    let packageSHA256Asset = Self.findMacOSPackageSHA256Asset(release.assets, packageAsset: packageAsset)

    if let metadataURL = metadataAsset?.browserDownloadURL {
      do {
        let metadata: DesktopPackageMetadata = try await fetchJSON(metadataURL)
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
          source: "github_release_metadata"
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
      source: "github_release"
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
    let canInstall = latest.canDownloadPackage && currentInstallTargetURL() != nil
    startupTimeline.mark("update_check.prompt_shown", metadata: [
      "latest_version": latest.version,
      "latest_build": latest.buildNumber ?? "",
      "can_download_package": latest.canDownloadPackage ? "true" : "false",
      "can_install_in_place": canInstall ? "true" : "false",
    ])

    let alert = NSAlert()
    alert.messageText = "发现 Nexus 新版本"
    alert.informativeText = updateAvailableMessage(latest)
    alert.alertStyle = .informational
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
        Task {
          await self.downloadAndInstallUpdate(latest)
        }
      case .alertSecondButtonReturn:
        openReleasePage(latest, reason: "prompt")
      default:
        break
      }
    } else if response == .alertFirstButtonReturn {
      openReleasePage(latest, reason: "prompt")
    }
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

  private func showCheckFailedAlert(_ error: Error) {
    let alert = NSAlert()
    alert.messageText = "检查更新失败"
    alert.informativeText = error.localizedDescription
    alert.alertStyle = .warning
    alert.addButton(withTitle: "好")
    alert.runModal()
  }

  private func downloadAndInstallUpdate(_ latest: DesktopReleaseInfo) async {
    guard latest.canDownloadPackage else {
      startupTimeline.mark("update_check.download_unavailable", metadata: [
        "latest_version": latest.version,
        "has_package": (latest.packageDownloadURL != nil) ? "true" : "false",
        "has_sha256": (latest.packageSHA256URL != nil) ? "true" : "false",
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
      let downloadedUpdate = try await downloadAndVerifyUpdate(latest)
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
      showDownloadFailedAlert(latest, error: error)
    }
  }

  private func downloadAndVerifyUpdate(_ latest: DesktopReleaseInfo) async throws -> DesktopDownloadedUpdate {
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

    try await downloadFile(from: packageDownloadURL, to: packageURL)
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

  private func downloadFile(from url: URL, to destinationURL: URL) async throws {
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

    let (downloadedURL, response) = try await session.download(for: request)
    guard let httpResponse = response as? HTTPURLResponse else {
      throw DesktopUpdateError.invalidResponse
    }
    guard (200..<300).contains(httpResponse.statusCode) else {
      throw DesktopUpdateError.badStatusCode(httpResponse.statusCode)
    }

    try FileManager.default.moveItem(at: downloadedURL, to: temporaryURL)
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
    alert.messageText = "Nexus 更新已就绪"
    alert.informativeText = """
    Nexus \(latest.displayText) 已下载并通过 sha256 校验。
    更新包：\(downloadedUpdate.packageURL.lastPathComponent)
    sha256：\(downloadedUpdate.sha256Hash)

    更新包已通过 macOS 签名与 Gatekeeper 信任评估。继续后 Nexus 会退出，替换当前 App，并自动重新打开。
    """
    alert.alertStyle = .informational
    alert.addButton(withTitle: "退出并更新")
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

    let scriptDirectory = try updateDirectory(for: latest)
      .appendingPathComponent("installer", isDirectory: true)
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
    alert.messageText = "Nexus 更新暂不可自动安装"
    alert.informativeText = """
    当前 Release 缺少可自动校验的 macOS 安装包或 sha256 文件，或者当前 App 所在位置不可替换。
    是否打开下载页手动处理？
    """
    alert.alertStyle = .informational
    alert.addButton(withTitle: "打开下载页")
    alert.addButton(withTitle: "稍后")
    if alert.runModal() == .alertFirstButtonReturn {
      openReleasePage(latest, reason: "download_unavailable")
    }
  }

  private func showDownloadFailedAlert(_ latest: DesktopReleaseInfo, error: Error) {
    let alert = NSAlert()
    alert.messageText = "Nexus 更新下载失败"
    alert.informativeText = """
    更新下载、校验或准备安装失败：\(error.localizedDescription)

    是否打开 Release 页面手动下载？
    """
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
    lines.append("")
    if let releaseNotes = Self.formatReleaseNotes(latest.releaseNotes) {
      lines.append("更新内容：")
      lines.append(releaseNotes)
      lines.append("")
    }
    if latest.canDownloadPackage && currentInstallTargetURL() != nil {
      lines.append("选择“下载并更新”会下载安装包和 sha256 文件，通过 macOS 本地信任校验后再询问是否退出并替换当前 App。")
    } else {
      lines.append("当前 Release 缺少可自动校验的 macOS 安装包或 sha256 文件，或者当前 App 所在位置不可替换。")
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
  static let releaseNotesMaxCharacters = 1800
  static let releaseNotesMaxLines = 24

  static let installScript = """
  #!/bin/zsh
  set -euo pipefail

  APP_PID="$1"
  SOURCE_APP="$2"
  TARGET_APP="$3"
  LOG_PATH="$4"

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

    /usr/bin/open "${TARGET_APP}"
    echo "Nexus update installer finished"
  } >> "${LOG_PATH}" 2>&1
  """

  static func findMacOSMetadataAsset(_ assets: [GitHubReleaseAsset]) -> GitHubReleaseAsset? {
    assets.first { asset in
      let name = asset.name.lowercased()
      return name.contains("macos") && name.hasSuffix(".metadata.json")
    }
  }

  static func findMacOSPackageAsset(_ assets: [GitHubReleaseAsset]) -> GitHubReleaseAsset? {
    assets.first { asset in
      let name = asset.name.lowercased()
      return name.contains("macos") && (name.hasSuffix(".dmg") || name.hasSuffix(".zip"))
    }
  }

  static func findMacOSPackageSHA256Asset(
    _ assets: [GitHubReleaseAsset],
    packageAsset: GitHubReleaseAsset?
  ) -> GitHubReleaseAsset? {
    if let packageAsset {
      let exactMatch = assets.first { asset in
        asset.name.caseInsensitiveCompare("\(packageAsset.name).sha256") == .orderedSame
      }
      if let exactMatch {
        return exactMatch
      }
    }

    return assets.first { asset in
      let name = asset.name.lowercased()
      return name.contains("macos") &&
        (name.hasSuffix(".dmg.sha256") || name.hasSuffix(".zip.sha256"))
    }
  }

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

    let lines = normalized.components(separatedBy: "\n")
    var clipped = lines
      .prefix(releaseNotesMaxLines)
      .joined(separator: "\n")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    var wasTruncated = normalized.count > releaseNotesMaxCharacters || lines.count > releaseNotesMaxLines
    if clipped.count > releaseNotesMaxCharacters {
      let endIndex = clipped.index(clipped.startIndex, offsetBy: releaseNotesMaxCharacters)
      clipped = String(clipped[..<endIndex]).trimmingCharacters(in: .whitespacesAndNewlines)
      wasTruncated = true
    }

    if wasTruncated {
      clipped.append("\n...\n完整更新内容请打开 Release 页面查看。")
    }
    return clipped
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

private struct DesktopAppVersion {
  let version: String
  let buildNumber: String

  static func fromBundle(_ bundle: Bundle = .main) -> DesktopAppVersion {
    DesktopAppVersion(
      version: bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0.0.0",
      buildNumber: bundle.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "dev"
    )
  }

  var displayText: String {
    "版本 \(version)，构建 \(buildNumber)"
  }
}

private struct DesktopReleaseInfo {
  let version: String
  let buildNumber: String?
  let releaseName: String?
  let releasePageURL: URL
  let packageFileName: String?
  let packageDownloadURL: URL?
  let packageSHA256FileName: String?
  let packageSHA256URL: URL?
  let releaseNotes: String?
  let publishedAt: String?
  let isPrerelease: Bool
  let source: String

  var canDownloadPackage: Bool {
    guard let packageFileName, !packageFileName.isEmpty else {
      return false
    }
    return packageDownloadURL != nil && packageSHA256URL != nil
  }

  var displayText: String {
    if let buildNumber, !buildNumber.isEmpty {
      return "版本 \(version)，构建 \(buildNumber)"
    }
    return "版本 \(version)"
  }

  func isNewer(than current: DesktopAppVersion) -> Bool {
    let latestVersion = ComparableVersion(version)
    let currentVersion = ComparableVersion(current.version)

    if latestVersion > currentVersion {
      return true
    }
    if latestVersion < currentVersion {
      return false
    }

    guard let latestBuild = buildNumber.flatMap(Int.init),
          let currentBuild = Int(current.buildNumber) else {
      return false
    }
    return latestBuild > currentBuild
  }
}

private struct ComparableVersion: Comparable {
  private let parts: [Int]

  init(_ rawValue: String) {
    let normalized = GitHubReleaseVersionNormalizer.version(from: rawValue)
    let base = normalized.split { character in
      character == "-" || character == "+"
    }
    .first ?? Substring(normalized)
    parts = base.split(separator: ".").map { Int($0) ?? 0 }
  }

  static func < (lhs: ComparableVersion, rhs: ComparableVersion) -> Bool {
    let count = max(lhs.parts.count, rhs.parts.count)
    for index in 0..<count {
      let left = index < lhs.parts.count ? lhs.parts[index] : 0
      let right = index < rhs.parts.count ? rhs.parts[index] : 0
      if left != right {
        return left < right
      }
    }
    return false
  }
}

private enum GitHubReleaseVersionNormalizer {
  static func version(from rawValue: String) -> String {
    let trimmed = rawValue.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.lowercased().hasPrefix("v") {
      return String(trimmed.dropFirst())
    }
    return trimmed
  }
}

private struct GitHubRelease: Decodable {
  let tagName: String
  let name: String?
  let htmlURL: URL?
  let body: String?
  let prerelease: Bool
  let publishedAt: String?
  let assets: [GitHubReleaseAsset]

  private enum CodingKeys: String, CodingKey {
    case tagName = "tag_name"
    case name
    case htmlURL = "html_url"
    case body
    case prerelease
    case publishedAt = "published_at"
    case assets
  }
}

private struct GitHubReleaseAsset: Decodable {
  let name: String
  let browserDownloadURL: URL?

  private enum CodingKeys: String, CodingKey {
    case name
    case browserDownloadURL = "browser_download_url"
  }
}

private struct DesktopPackageMetadata: Decodable {
  let version: String
  let buildNumber: String

  private enum CodingKeys: String, CodingKey {
    case version
    case buildNumber = "build_number"
  }
}

private struct DesktopDownloadedUpdate {
  let packageURL: URL
  let sha256URL: URL
  let packageFileName: String
  let sha256Hash: String
}

private struct DesktopProcessOutput {
  let standardOutput: String
  let standardError: String
}

private enum DesktopUpdateError: LocalizedError {
  case invalidResponse
  case badStatusCode(Int)
  case missingPackage
  case missingSHA256
  case invalidSHA256File
  case sha256Mismatch
  case unsupportedPackageFormat(String)
  case appBundleNotFound
  case unsupportedInstallLocation
  case appBundleIdentityUnavailable
  case appBundleIdentityMismatch(expected: String, actual: String)
  case processFailed(String, Int32, String)

  var errorDescription: String? {
    switch self {
    case .invalidResponse:
      return "更新服务返回了无效响应。"
    case let .badStatusCode(statusCode):
      return "更新服务返回 HTTP \(statusCode)。"
    case .missingPackage:
      return "当前 Release 缺少 macOS 安装包。"
    case .missingSHA256:
      return "当前 Release 缺少 macOS 安装包 sha256 文件。"
    case .invalidSHA256File:
      return "sha256 文件中没有找到有效的 SHA256 值。"
    case .sha256Mismatch:
      return "下载的 macOS 安装包 sha256 校验未通过，已丢弃本地文件。"
    case let .unsupportedPackageFormat(fileExtension):
      return "不支持的 macOS 更新包格式：\(fileExtension)。"
    case .appBundleNotFound:
      return "更新包中没有找到可替换的 Nexus.app。"
    case .unsupportedInstallLocation:
      return "当前 Nexus.app 所在位置不可自动替换。"
    case .appBundleIdentityUnavailable:
      return "更新包缺少可验证的 App 标识，无法自动安装。"
    case let .appBundleIdentityMismatch(expected, actual):
      return "更新包 App 标识不匹配，期望 \(expected)，实际 \(actual)。"
    case let .processFailed(executablePath, statusCode, output):
      let detail = output.trimmingCharacters(in: .whitespacesAndNewlines)
      if detail.isEmpty {
        return "\(executablePath) 执行失败，退出码 \(statusCode)。"
      }
      return "\(executablePath) 执行失败，退出码 \(statusCode)：\(detail)"
    }
  }
}
