import Foundation

enum DesktopWebEntry {
  case app
  case settings
  case oauthCallback

  var path: String {
    switch self {
    case .app:
      return "/app.html"
    case .settings:
      return "/settings.html"
    case .oauthCallback:
      return "/oauth-callback.html"
    }
  }
}

struct DesktopWebRoute {
  let path: String
  let percentEncodedQuery: String?
  let percentEncodedFragment: String?
  let entry: DesktopWebEntry

  init(
    path: String,
    percentEncodedQuery: String? = nil,
    percentEncodedFragment: String? = nil,
    entry: DesktopWebEntry? = nil
  ) {
    self.path = path
    self.percentEncodedQuery = percentEncodedQuery
    self.percentEncodedFragment = percentEncodedFragment
    self.entry = entry ?? Self.defaultEntry(path: path)
  }

  func url(runtime: SidecarRuntimeConfig) -> URL {
    guard var components = URLComponents(url: runtime.webURL, resolvingAgainstBaseURL: false) else {
      return runtime.webURL
    }
    components.path = entry.path
    components.queryItems = [
      URLQueryItem(name: "desktop_route", value: routeString()),
    ]
    return components.url ?? runtime.webURL
  }

  static func appRoute(_ raw: String) -> DesktopWebRoute? {
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
      return nil
    }
    if trimmed.hasPrefix("/") {
      return routeFromComponents(URLComponents(string: trimmed))
    }
    guard let components = URLComponents(string: trimmed) else {
      return nil
    }
    return routeFromComponents(components)
  }

  private static func routeFromComponents(_ components: URLComponents?) -> DesktopWebRoute? {
    guard let components else {
      return nil
    }
    let path = components.path.trimmingCharacters(in: .whitespacesAndNewlines)
    guard path.hasPrefix("/") else {
      return nil
    }
    return DesktopWebRoute(
      path: path,
      percentEncodedQuery: components.percentEncodedQuery,
      percentEncodedFragment: components.percentEncodedFragment
    )
  }

  private func routeString() -> String {
    var value = path
    if let percentEncodedQuery, !percentEncodedQuery.isEmpty {
      value += "?\(percentEncodedQuery)"
    }
    if let percentEncodedFragment, !percentEncodedFragment.isEmpty {
      value += "#\(percentEncodedFragment)"
    }
    return value
  }

  private static func defaultEntry(path: String) -> DesktopWebEntry {
    switch path {
    case "/settings":
      return .settings
    case "/capability/connectors/oauth/callback":
      return .oauthCallback
    default:
      return .app
    }
  }
}

enum DesktopURLRouter {
  static func webRoute(for url: URL) -> DesktopWebRoute? {
    guard url.scheme?.lowercased() == "nexus" else {
      return nil
    }

    if isSettingsURL(url) {
      return DesktopWebRoute(path: "/settings", entry: .settings)
    }
    if isLauncherURL(url) {
      return DesktopWebRoute(path: "/launcher", entry: .app)
    }
    if isConnectorOAuthCallbackURL(url) {
      let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
      return DesktopWebRoute(
        path: "/capability/connectors/oauth/callback",
        percentEncodedQuery: components?.percentEncodedQuery,
        percentEncodedFragment: components?.percentEncodedFragment,
        entry: .oauthCallback
      )
    }
    if url.host?.lowercased() == "open" || url.host == nil && url.path.isEmpty {
      return DesktopWebRoute(path: "/launcher", entry: .app)
    }
    return nil
  }

  private static func isSettingsURL(_ url: URL) -> Bool {
    url.host?.lowercased() == "settings" || normalizedPath(url) == "/settings"
  }

  private static func isLauncherURL(_ url: URL) -> Bool {
    url.host?.lowercased() == "launcher" || normalizedPath(url) == "/launcher"
  }

  private static func isConnectorOAuthCallbackURL(_ url: URL) -> Bool {
    let host = url.host?.lowercased()
    let path = normalizedPath(url)
    return host == "connectors" && path == "/oauth/callback" || path == "/connectors/oauth/callback"
  }

  private static func normalizedPath(_ url: URL) -> String {
    let path = url.path.trimmingCharacters(in: .whitespacesAndNewlines)
    if path.isEmpty {
      return ""
    }
    return path.hasPrefix("/") ? path : "/\(path)"
  }
}
