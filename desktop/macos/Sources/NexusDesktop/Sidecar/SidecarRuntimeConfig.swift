import Foundation

struct SidecarRuntimeConfig {
  let port: Int
  let sessionToken: String
  let appMode: String
  let appVersion: String
  let buildNumber: String
  let platform: String

  init(
    port: Int,
    sessionToken: String,
    appMode: String = "desktop",
    appVersion: String = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0.0.0",
    buildNumber: String = Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "dev",
    platform: String = "macos"
  ) {
    self.port = port
    self.sessionToken = sessionToken
    self.appMode = appMode
    self.appVersion = appVersion
    self.buildNumber = buildNumber
    self.platform = platform
  }

  var webURL: URL {
    URL(string: "http://127.0.0.1:\(port)/")!
  }

  var apiBaseURL: URL {
    URL(string: "http://127.0.0.1:\(port)/nexus/v1")!
  }

  var webSocketURL: URL {
    URL(string: "ws://127.0.0.1:\(port)/nexus/v1/chat/ws")!
  }

  var healthURL: URL {
    URL(string: "http://127.0.0.1:\(port)/nexus/v1/health")!
  }
}
