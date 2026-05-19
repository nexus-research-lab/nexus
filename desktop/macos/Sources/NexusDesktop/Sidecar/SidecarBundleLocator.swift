import Foundation

struct SidecarBundleLocator {
  let projectRoot: URL?
  let webDistURL: URL
  let appRootURL: URL
  let command: String
  let arguments: [String]
  let workingDirectory: URL

  static func resolve() throws -> SidecarBundleLocator {
    if let bundled = try resolveBundled() {
      return bundled
    }
    return try resolveDevelopment()
  }

  private static func resolveBundled() throws -> SidecarBundleLocator? {
    guard let resourceURL = Bundle.main.resourceURL else {
      return nil
    }
    let webDistURL = resourceURL.appendingPathComponent("Web", isDirectory: true)
    let serverURL = resourceURL
      .deletingLastPathComponent()
      .appendingPathComponent("MacOS", isDirectory: true)
      .appendingPathComponent("nexus-server")
    guard FileManager.default.fileExists(atPath: webDistURL.appendingPathComponent("index.html").path) else {
      return nil
    }
    guard FileManager.default.isExecutableFile(atPath: serverURL.path) else {
      return nil
    }
    return SidecarBundleLocator(
      projectRoot: nil,
      webDistURL: webDistURL,
      appRootURL: resourceURL,
      command: serverURL.path,
      arguments: [],
      workingDirectory: resourceURL
    )
  }

  private static func resolveDevelopment() throws -> SidecarBundleLocator {
    guard let root = findProjectRoot() else {
      throw DesktopShellError.projectRootNotFound
    }
    let webDistURL = root.appendingPathComponent("web/dist", isDirectory: true)
    guard FileManager.default.fileExists(atPath: webDistURL.appendingPathComponent("index.html").path) else {
      throw DesktopShellError.webDistNotFound(webDistURL.path)
    }
    return SidecarBundleLocator(
      projectRoot: root,
      webDistURL: webDistURL,
      appRootURL: root,
      command: "/usr/bin/env",
      arguments: ["go", "run", "./cmd/nexus-server"],
      workingDirectory: root
    )
  }

  private static func findProjectRoot() -> URL? {
    let candidates = [
      URL(fileURLWithPath: FileManager.default.currentDirectoryPath),
      URL(fileURLWithPath: #filePath),
    ]
    for candidate in candidates {
      var current = candidate.hasDirectoryPath ? candidate : candidate.deletingLastPathComponent()
      while current.path != current.deletingLastPathComponent().path {
        let goMod = current.appendingPathComponent("go.mod")
        let webIndex = current.appendingPathComponent("web/index.html")
        if FileManager.default.fileExists(atPath: goMod.path),
           FileManager.default.fileExists(atPath: webIndex.path) {
          return current
        }
        current.deleteLastPathComponent()
      }
    }
    return nil
  }
}
