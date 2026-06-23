import Foundation

enum DesktopRuntimeScript {
  static func make(runtime: SidecarRuntimeConfig) throws -> String {
    let payload: [String: String] = [
      "api_base_url": runtime.apiBaseURL.absoluteString,
      "ws_url": runtime.webSocketURL.absoluteString,
      "auth_token": runtime.sessionToken,
      "app_mode": runtime.appMode,
      "app_version": runtime.appVersion,
      "build_number": runtime.buildNumber,
      "platform": runtime.platform,
      "oauth_redirect_uri": runtime.oauthRedirectURL.absoluteString,
    ]
    let data = try JSONSerialization.data(withJSONObject: payload, options: [])
    guard let json = String(data: data, encoding: .utf8) else {
      throw DesktopShellError.invalidRuntimeConfig
    }
    return "window.__NEXUS_DESKTOP_RUNTIME__ = \(json);"
  }
}
