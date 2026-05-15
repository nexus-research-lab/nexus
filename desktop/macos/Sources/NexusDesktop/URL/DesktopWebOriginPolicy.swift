import Foundation
import WebKit

enum DesktopWebOriginPolicy {
  static func isAllowed(message: WKScriptMessage, runtime: SidecarRuntimeConfig) -> Bool {
    rejectionReason(message: message, runtime: runtime) == nil
  }

  static func rejectionReason(message: WKScriptMessage, runtime: SidecarRuntimeConfig) -> String? {
    let origin = message.frameInfo.securityOrigin
    let expectedScheme = (runtime.webURL.scheme ?? "http").lowercased()
    let originScheme = origin.protocol.lowercased()
    if originScheme != expectedScheme {
      return "scheme_mismatch"
    }

    let expectedHost = runtime.webURL.host ?? "127.0.0.1"
    if !isHostAllowed(origin.host, expectedHost: expectedHost) {
      return "host_mismatch"
    }

    let originPort = normalizedPort(scheme: originScheme, port: Int(origin.port))
    if originPort != normalizedPort(url: runtime.webURL) {
      return "port_mismatch"
    }
    return nil
  }

  static func metadata(message: WKScriptMessage, runtime: SidecarRuntimeConfig) -> [String: String] {
    let origin = message.frameInfo.securityOrigin
    return [
      "expected_host": runtime.webURL.host ?? "unknown",
      "expected_port": String(normalizedPort(url: runtime.webURL)),
      "expected_scheme": runtime.webURL.scheme ?? "unknown",
      "origin_host": origin.host.isEmpty ? "empty" : origin.host,
      "origin_port": String(origin.port),
      "origin_scheme": origin.protocol.isEmpty ? "empty" : origin.protocol,
    ]
  }

  static func isURLAllowed(_ url: URL, runtime: SidecarRuntimeConfig) -> Bool {
    guard let scheme = url.scheme?.lowercased(),
          let expectedScheme = runtime.webURL.scheme?.lowercased(),
          scheme == expectedScheme else {
      return false
    }
    let expectedHost = runtime.webURL.host ?? "127.0.0.1"
    guard isHostAllowed(url.host ?? "", expectedHost: expectedHost) else {
      return false
    }
    return normalizedPort(url: url) == normalizedPort(url: runtime.webURL)
  }

  static func normalizedPort(url: URL) -> Int {
    normalizedPort(scheme: url.scheme, port: url.port)
  }

  private static func isHostAllowed(_ host: String, expectedHost: String) -> Bool {
    let normalizedHost = canonicalHost(host)
    let normalizedExpectedHost = canonicalHost(expectedHost)
    if normalizedHost == normalizedExpectedHost {
      return true
    }
    return isLoopbackHost(normalizedHost) && isLoopbackHost(normalizedExpectedHost)
  }

  private static func canonicalHost(_ host: String) -> String {
    let trimmed = host.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    if trimmed.hasPrefix("[") && trimmed.hasSuffix("]") {
      return String(trimmed.dropFirst().dropLast())
    }
    return trimmed
  }

  private static func isLoopbackHost(_ host: String) -> Bool {
    ["127.0.0.1", "localhost", "::1", "0:0:0:0:0:0:0:1"].contains(host)
  }

  private static func normalizedPort(scheme: String?, port: Int?) -> Int {
    if let port, port > 0 {
      return port
    }
    switch scheme?.lowercased() {
    case "https":
      return 443
    case "http":
      return 80
    default:
      return -1
    }
  }
}
