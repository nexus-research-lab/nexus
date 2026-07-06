import Foundation

struct DesktopAppVersion {
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

struct DesktopReleaseInfo {
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
  let packageSigning: DesktopPackageSigning?

  var canDownloadPackage: Bool {
    guard let packageFileName, !packageFileName.isEmpty else {
      return false
    }
    return packageDownloadURL != nil && packageSHA256URL != nil
  }

  var canAutoInstallPackage: Bool {
    canDownloadPackage && automaticInstallUnavailableReason == nil
  }

  var automaticInstallUnavailableReason: String? {
    guard canDownloadPackage else {
      return "当前 Release 缺少可自动校验的 macOS 安装包或 sha256 文件。"
    }
    guard let packageSigning else {
      return nil
    }

    var missingRequirements: [String] = []
    if !packageSigning.developerID {
      missingRequirements.append("Developer ID 签名")
    }
    if !packageSigning.notarized {
      missingRequirements.append("公证")
    }
    guard !missingRequirements.isEmpty else {
      return nil
    }

    let signingKind = packageSigning.kind.isEmpty ? "未知" : packageSigning.kind
    let missingText = missingRequirements.joined(separator: "和")
    return "当前 Release 的 macOS 安装包为 \(signingKind) 签名，缺少 \(missingText)，无法通过 macOS Gatekeeper 自动安装。"
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

struct ComparableVersion: Comparable {
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

enum GitHubReleaseVersionNormalizer {
  static func version(from rawValue: String) -> String {
    let trimmed = rawValue.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.lowercased().hasPrefix("v") {
      return String(trimmed.dropFirst())
    }
    return trimmed
  }
}

struct GitHubRelease: Decodable {
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

struct GitHubReleaseAsset: Decodable {
  let name: String
  let browserDownloadURL: URL?

  private enum CodingKeys: String, CodingKey {
    case name
    case browserDownloadURL = "browser_download_url"
  }
}

struct DesktopPackageMetadata: Decodable {
  let version: String
  let buildNumber: String
  let signing: DesktopPackageSigning?

  private enum CodingKeys: String, CodingKey {
    case version
    case buildNumber = "build_number"
    case signing
  }
}

struct DesktopPackageSigning: Decodable {
  let kind: String
  let developerID: Bool
  let notarized: Bool

  private enum CodingKeys: String, CodingKey {
    case kind
    case developerID = "developer_id"
    case notarized
  }

  init(from decoder: Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    kind = try container.decodeIfPresent(String.self, forKey: .kind) ?? "unknown"
    developerID = try container.decodeIfPresent(Bool.self, forKey: .developerID) ?? false
    notarized = try container.decodeIfPresent(Bool.self, forKey: .notarized) ?? false
  }
}

struct DesktopDownloadedUpdate {
  let packageURL: URL
  let sha256URL: URL
  let packageFileName: String
  let sha256Hash: String
}

struct DesktopProcessOutput {
  let standardOutput: String
  let standardError: String
}

enum DesktopUpdateError: LocalizedError {
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
