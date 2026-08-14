import Foundation

// INPUT: macOS bundle resources, or a development checkout with Web build inputs and web/dist.
// OUTPUT: A sidecar launch configuration whose Web assets exist and, in development, are current.
// POS: The single boundary that distinguishes immutable release resources from mutable checkout assets.
struct SidecarBundleLocator {
  private static let developmentWebInputDirectories = ["src", "public"]
  private static let developmentWebInputFiles = [
    "app.html",
    "index.html",
    "oauth-callback.html",
    "package.json",
    "pnpm-lock.yaml",
    "pnpm-workspace.yaml",
    "postcss.config.mjs",
    "settings.html",
    "tsconfig.json",
    "vite.config.ts",
  ]

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
    return try resolveDevelopment(projectRoot: root)
  }

  static func resolveDevelopment(
    projectRoot root: URL,
    fileManager: FileManager = .default
  ) throws -> SidecarBundleLocator {
    let webDistURL = root.appendingPathComponent("web/dist", isDirectory: true)
    let distIndexURL = webDistURL.appendingPathComponent("index.html")
    guard fileManager.fileExists(atPath: distIndexURL.path) else {
      throw DesktopShellError.webDistNotFound(webDistURL.path)
    }
    try validateDevelopmentWebDist(
      webRootURL: root.appendingPathComponent("web", isDirectory: true),
      distIndexURL: distIndexURL,
      fileManager: fileManager
    )
    return SidecarBundleLocator(
      projectRoot: root,
      webDistURL: webDistURL,
      appRootURL: root,
      command: "/usr/bin/env",
      arguments: ["go", "run", "./cmd/nexus-server"],
      workingDirectory: root
    )
  }

  private static func validateDevelopmentWebDist(
    webRootURL: URL,
    distIndexURL: URL,
    fileManager: FileManager
  ) throws {
    guard let distModifiedAt = modificationDate(of: distIndexURL, fileManager: fileManager),
          let newestInput = newestDevelopmentWebInput(in: webRootURL, fileManager: fileManager),
          newestInput.modifiedAt > distModifiedAt else {
      return
    }
    throw DesktopShellError.webDistStale(
      webDistPath: distIndexURL.deletingLastPathComponent().path,
      newerInputPath: newestInput.url.path
    )
  }

  private static func newestDevelopmentWebInput(
    in webRootURL: URL,
    fileManager: FileManager
  ) -> (url: URL, modifiedAt: Date)? {
    var candidates = developmentWebInputFiles.map { webRootURL.appendingPathComponent($0) }
    candidates.append(contentsOf: developmentEnvironmentFiles(in: webRootURL, fileManager: fileManager))

    for directory in developmentWebInputDirectories {
      let directoryURL = webRootURL.appendingPathComponent(directory, isDirectory: true)
      guard let enumerator = fileManager.enumerator(
        at: directoryURL,
        includingPropertiesForKeys: [.isRegularFileKey, .contentModificationDateKey],
        options: [.skipsHiddenFiles]
      ) else {
        continue
      }
      for case let fileURL as URL in enumerator {
        candidates.append(fileURL)
      }
    }

    var newest: (url: URL, modifiedAt: Date)?
    for candidate in candidates {
      guard let modifiedAt = modificationDate(of: candidate, fileManager: fileManager) else {
        continue
      }
      if let current = newest, current.modifiedAt >= modifiedAt {
        continue
      } else {
        newest = (candidate, modifiedAt)
      }
    }
    return newest
  }

  private static func developmentEnvironmentFiles(
    in webRootURL: URL,
    fileManager: FileManager
  ) -> [URL] {
    guard let entries = try? fileManager.contentsOfDirectory(
      at: webRootURL,
      includingPropertiesForKeys: [.isRegularFileKey],
      options: []
    ) else {
      return []
    }
    return entries.filter { $0.lastPathComponent.hasPrefix(".env") }
  }

  private static func modificationDate(
    of fileURL: URL,
    fileManager: FileManager
  ) -> Date? {
    guard fileManager.fileExists(atPath: fileURL.path),
          let attributes = try? fileManager.attributesOfItem(atPath: fileURL.path),
          let fileType = attributes[.type] as? FileAttributeType,
          fileType == .typeRegular else {
      return nil
    }
    return attributes[.modificationDate] as? Date
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
