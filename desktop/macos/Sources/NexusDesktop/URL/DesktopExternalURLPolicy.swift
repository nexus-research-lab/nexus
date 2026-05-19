import AppKit
import Foundation

enum DesktopExternalURLPolicy {
  private static let allowedSchemes: Set<String> = ["http", "https", "mailto"]

  static func canOpen(_ url: URL) -> Bool {
    guard let scheme = url.scheme?.lowercased() else {
      return false
    }
    return allowedSchemes.contains(scheme)
  }

  static func open(_ url: URL) throws {
    guard canOpen(url) else {
      throw DesktopExternalURLError.unsupportedURLScheme
    }
    guard NSWorkspace.shared.open(url) else {
      throw DesktopExternalURLError.openFailed
    }
  }
}

enum DesktopExternalURLError: LocalizedError {
  case unsupportedURLScheme
  case openFailed

  var errorDescription: String? {
    switch self {
    case .unsupportedURLScheme:
      return "不支持打开该类型的外部链接。"
    case .openFailed:
      return "无法打开外部链接。"
    }
  }
}
