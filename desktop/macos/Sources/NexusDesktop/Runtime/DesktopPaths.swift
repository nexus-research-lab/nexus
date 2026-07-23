import Foundation

enum DesktopPaths {
  static var rootDirectory: URL {
    URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".nexus", isDirectory: true)
  }

  static var appDirectory: URL {
    rootDirectory.appendingPathComponent("app", isDirectory: true)
  }

  static var usersDirectory: URL {
    rootDirectory.appendingPathComponent("users", isDirectory: true)
  }

  static var systemRuntimeDirectory: URL {
    usersDirectory
      .appendingPathComponent("__system__", isDirectory: true)
      .appendingPathComponent("runtime", isDirectory: true)
  }

  static var dataDirectory: URL {
    appDirectory.appendingPathComponent("data", isDirectory: true)
  }

  static var configDirectory: URL {
    appDirectory.appendingPathComponent("config", isDirectory: true)
  }

  static var workspaceDirectory: URL {
    usersDirectory
  }

  static var projectsDirectory: URL {
    systemRuntimeDirectory.appendingPathComponent("projects", isDirectory: true)
  }

  static var cacheDirectory: URL {
    appDirectory.appendingPathComponent("cache", isDirectory: true)
  }

  static var logsDirectory: URL {
    appDirectory.appendingPathComponent("logs", isDirectory: true)
  }

  static var debugDirectory: URL {
    appDirectory.appendingPathComponent("debug", isDirectory: true)
  }

  static var sidecarPIDFileURL: URL {
    rootDirectory.appendingPathComponent("NexusSidecar.pid.json")
  }

  static var connectorCredentialsFallbackKeyURL: URL {
    configDirectory.appendingPathComponent("connector-credentials.key")
  }

  static func createRuntimeDirectories() throws {
    for directory in [
      rootDirectory,
      appDirectory,
      usersDirectory,
      systemRuntimeDirectory,
      systemRuntimeDirectory.appendingPathComponent("home", isDirectory: true),
      systemRuntimeDirectory.appendingPathComponent("cache", isDirectory: true),
      systemRuntimeDirectory.appendingPathComponent("logs", isDirectory: true),
      systemRuntimeDirectory.appendingPathComponent("tmp", isDirectory: true),
      dataDirectory,
      configDirectory,
      workspaceDirectory,
      projectsDirectory,
      cacheDirectory,
      logsDirectory,
      debugDirectory,
    ] {
      try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }
  }
}
