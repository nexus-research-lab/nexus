import Foundation

enum DesktopPaths {
  static var rootDirectory: URL {
    URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".nexus", isDirectory: true)
  }

  static var dataDirectory: URL {
    rootDirectory.appendingPathComponent("data", isDirectory: true)
  }

  static var configDirectory: URL {
    rootDirectory.appendingPathComponent("config", isDirectory: true)
  }

  static var workspaceDirectory: URL {
    rootDirectory.appendingPathComponent("workspace", isDirectory: true)
  }

  static var cacheDirectory: URL {
    rootDirectory.appendingPathComponent("cache", isDirectory: true)
  }

  static var logsDirectory: URL {
    rootDirectory.appendingPathComponent("logs", isDirectory: true)
  }

  static var sidecarPIDFileURL: URL {
    rootDirectory.appendingPathComponent("NexusSidecar.pid.json")
  }

  static var connectorCredentialsFallbackKeyURL: URL {
    configDirectory.appendingPathComponent("connector-credentials.key")
  }

  static func createRuntimeDirectories() throws {
    for directory in [rootDirectory, dataDirectory, configDirectory, workspaceDirectory, cacheDirectory, logsDirectory] {
      try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }
  }
}
