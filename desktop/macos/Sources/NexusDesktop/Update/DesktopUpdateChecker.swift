import AppKit
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
    let metadataAsset = release.assets.first { asset in
      let name = asset.name.lowercased()
      return name.contains("macos") && name.hasSuffix(".metadata.json")
    }
    let downloadAsset = release.assets.first { asset in
      let name = asset.name.lowercased()
      return name.contains("macos") && (name.hasSuffix(".dmg") || name.hasSuffix(".zip"))
    }

    if let metadataURL = metadataAsset?.browserDownloadURL {
      do {
        let metadata: DesktopPackageMetadata = try await fetchJSON(metadataURL)
        return DesktopReleaseInfo(
          version: metadata.version,
          buildNumber: metadata.buildNumber,
          releaseName: release.name,
          releasePageURL: release.htmlURL ?? Self.fallbackReleasePageURL,
          downloadURL: downloadAsset?.browserDownloadURL,
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
      downloadURL: downloadAsset?.browserDownloadURL,
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
    startupTimeline.mark("update_check.prompt_shown", metadata: [
      "latest_version": latest.version,
      "latest_build": latest.buildNumber ?? "",
    ])

    let alert = NSAlert()
    alert.messageText = "发现 Nexus 新版本"
    alert.informativeText = updateAvailableMessage(latest)
    alert.alertStyle = .informational
    alert.addButton(withTitle: "打开下载页")
    alert.addButton(withTitle: "稍后")

    if alert.runModal() == .alertFirstButtonReturn {
      startupTimeline.mark("update_check.release_page_opened", metadata: [
        "latest_version": latest.version,
      ])
      NSWorkspace.shared.open(latest.releasePageURL)
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
    lines.append("当前阶段不会自动安装更新，打开下载页后请校验对应的 sha256 文件。")
    return lines.joined(separator: "\n")
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
  let downloadURL: URL?
  let publishedAt: String?
  let isPrerelease: Bool
  let source: String

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
    let base = normalized.split(separator: "-", maxSplits: 1).first ?? Substring(normalized)
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
  let prerelease: Bool
  let publishedAt: String?
  let assets: [GitHubReleaseAsset]

  private enum CodingKeys: String, CodingKey {
    case tagName = "tag_name"
    case name
    case htmlURL = "html_url"
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

private enum DesktopUpdateError: LocalizedError {
  case invalidResponse
  case badStatusCode(Int)

  var errorDescription: String? {
    switch self {
    case .invalidResponse:
      return "更新服务返回了无效响应。"
    case let .badStatusCode(statusCode):
      return "更新服务返回 HTTP \(statusCode)。"
    }
  }
}
