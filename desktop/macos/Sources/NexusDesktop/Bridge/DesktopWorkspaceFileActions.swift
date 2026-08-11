import AppKit
import Foundation

enum DesktopWorkspaceFileActions {
  static func applicationsPayload(for rawPath: String) throws -> [String: Any] {
    let fileURL = try validatedWorkspaceFileURL(rawPath)
    let workspace = NSWorkspace.shared
    let defaultApplication = workspace.urlForApplication(toOpen: fileURL)
      .flatMap(applicationPayload)
    var seenPaths = Set<String>()
    let applications = workspace.urlsForApplications(toOpen: fileURL)
      .compactMap(applicationPayload)
      .filter { seenPaths.insert($0["path"] ?? "").inserted }
      .sorted {
        ($0["name"] ?? "").localizedStandardCompare($1["name"] ?? "") == .orderedAscending
      }

    return [
      "default_application": defaultApplication ?? NSNull(),
      "applications": applications,
    ]
  }

  static func openPayload(
    path rawPath: String,
    target: String,
    applicationPath: String
  ) throws -> [String: Any] {
    let fileURL = try validatedWorkspaceFileURL(rawPath)
    let workspace = NSWorkspace.shared

    switch target {
    case "default":
      guard workspace.open(fileURL) else {
        throw DesktopWorkspaceFileError.openFailed
      }
    case "file_manager":
      workspace.activateFileViewerSelecting([fileURL])
    case "terminal":
      guard let terminalURL = workspace.urlForApplication(
        withBundleIdentifier: "com.apple.Terminal"
      ) else {
        throw DesktopWorkspaceFileError.applicationUnavailable
      }
      open(
        fileURL.deletingLastPathComponent(),
        withApplicationAt: terminalURL,
        workspace: workspace
      )
    case "application":
      let requestedPath = URL(fileURLWithPath: applicationPath)
        .standardizedFileURL
        .resolvingSymlinksInPath()
        .path
      guard let applicationURL = workspace.urlsForApplications(toOpen: fileURL).first(where: {
        $0.standardizedFileURL.resolvingSymlinksInPath().path == requestedPath
      }) else {
        throw DesktopWorkspaceFileError.applicationUnavailable
      }
      open(fileURL, withApplicationAt: applicationURL, workspace: workspace)
    default:
      throw DesktopWorkspaceFileError.unsupportedTarget
    }
    return ["opened": true]
  }

  static func isFileURL(_ fileURL: URL, inside rootURL: URL) -> Bool {
    let resolvedFile = fileURL.standardizedFileURL.resolvingSymlinksInPath()
    let resolvedRoot = rootURL.standardizedFileURL.resolvingSymlinksInPath()
    let rootPrefix = resolvedRoot.path.hasSuffix("/")
      ? resolvedRoot.path
      : resolvedRoot.path + "/"
    return resolvedFile.path.hasPrefix(rootPrefix)
  }

  private static func validatedWorkspaceFileURL(_ rawPath: String) throws -> URL {
    let trimmedPath = rawPath.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmedPath.isEmpty else {
      throw DesktopWorkspaceFileError.invalidPath
    }
    let fileURL = URL(fileURLWithPath: trimmedPath).standardizedFileURL
    var isDirectory: ObjCBool = false
    guard isFileURL(fileURL, inside: DesktopPaths.usersDirectory),
          FileManager.default.fileExists(atPath: fileURL.path, isDirectory: &isDirectory),
          !isDirectory.boolValue else {
      throw DesktopWorkspaceFileError.invalidPath
    }
    return fileURL.resolvingSymlinksInPath()
  }

  private static func applicationPayload(_ applicationURL: URL) -> [String: String]? {
    let resolvedURL = applicationURL.standardizedFileURL.resolvingSymlinksInPath()
    guard resolvedURL.pathExtension.lowercased() == "app" else {
      return nil
    }
    let displayName = FileManager.default.displayName(atPath: resolvedURL.path)
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let applicationName = displayName.lowercased().hasSuffix(".app")
      ? String(displayName.dropLast(4))
      : displayName
    return [
      "name": !applicationName.isEmpty
        ? applicationName
        : resolvedURL.deletingPathExtension().lastPathComponent,
      "path": resolvedURL.path,
    ]
  }

  private static func open(
    _ fileURL: URL,
    withApplicationAt applicationURL: URL,
    workspace: NSWorkspace
  ) {
    let configuration = NSWorkspace.OpenConfiguration()
    configuration.activates = true
    workspace.open(
      [fileURL],
      withApplicationAt: applicationURL,
      configuration: configuration
    )
  }
}

private enum DesktopWorkspaceFileError: LocalizedError {
  case applicationUnavailable
  case invalidPath
  case openFailed
  case unsupportedTarget

  var errorDescription: String? {
    switch self {
    case .applicationUnavailable:
      return "所选应用当前不可用。"
    case .invalidPath:
      return "工作区文件路径无效。"
    case .openFailed:
      return "无法打开工作区文件。"
    case .unsupportedTarget:
      return "不支持该文件打开方式。"
    }
  }
}
