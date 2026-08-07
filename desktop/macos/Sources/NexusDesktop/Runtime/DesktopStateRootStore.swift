import Foundation

enum DesktopStateRootStore {
  private static let stateKey = "stateRoot.bootstrap"
  private static let preferencesSuiteEnvironmentName = "NEXUS_DESKTOP_PREFERENCES_SUITE"
  private static let initialRootEnvironmentName = "NEXUS_DESKTOP_STATE_ROOT"

  private static var defaults: UserDefaults {
    if let suite = ProcessInfo.processInfo.environment[preferencesSuiteEnvironmentName],
       !suite.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
       let isolatedDefaults = UserDefaults(suiteName: suite) {
      return isolatedDefaults
    }
    return .standard
  }

  static var defaultRootDirectory: URL {
    URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".nexus", isDirectory: true)
  }

  static var bootstrapLocation: String {
    if let suite = ProcessInfo.processInfo.environment[preferencesSuiteEnvironmentName],
       !suite.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      return "UserDefaults:\(suite)"
    }
    return "UserDefaults.standard"
  }

  static var activeRootDirectory: URL {
    let storedPath = loadState()["active_path"]?.trimmingCharacters(in: .whitespacesAndNewlines)
    let activePath = storedPath?.isEmpty == false ? storedPath : nil
    let candidate = normalizedURL(activePath ?? initialRootDirectory.path)
    do {
      try validateManagedRoot(candidate)
      return candidate
    } catch {
      NSLog("[Nexus State Root] active root is invalid: \(error.localizedDescription)")
      return initialRootDirectory
    }
  }

  static var previousRootDirectory: URL? {
    guard let path = loadState()["previous_path"],
          !path.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
      return nil
    }
    let candidate = normalizedURL(path)
    do {
      try validateManagedRoot(candidate)
      return candidate
    } catch {
      NSLog("[Nexus State Root] previous root is invalid: \(error.localizedDescription)")
      return nil
    }
  }

  static func statusPayload() -> [String: Any] {
    let state = loadState()
    var payload: [String: Any] = [
      "current_path": activeRootDirectory.path,
      "default_path": defaultRootDirectory.path,
    ]
    if let message = state["migration_error"],
       !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      payload["migration_error"] = message
    }
    return payload
  }

  static func activateMigration(source: URL, target: URL) {
    saveState(active: target, previous: source, error: nil)
  }

  static func completeMigration() throws -> URL? {
    guard let previousRoot = previousRootDirectory else {
      return nil
    }
    let activeRoot = activeRootDirectory
    try validateManagedRoot(previousRoot)
    try validateManagedRoot(activeRoot)
    guard !sameManagedRoot(previousRoot, activeRoot),
          !managedRootContains(previousRoot, activeRoot),
          !managedRootContains(activeRoot, previousRoot) else {
      throw DesktopShellError.invalidStateRootBootstrap
    }
    saveState(active: activeRoot, previous: nil, error: nil)
    return previousRoot
  }

  static func rollbackMigration(message: String) -> URL? {
    guard let previousRoot = previousRootDirectory else {
      return nil
    }
    saveState(active: previousRoot, previous: nil, error: message)
    return previousRoot
  }

  static func recordMigrationFailure(source: URL, message: String) {
    saveState(active: source, previous: nil, error: message)
  }

  static func normalizedURL(_ path: String) -> URL {
    let expanded = NSString(string: path.trimmingCharacters(in: .whitespacesAndNewlines)).expandingTildeInPath
    return URL(fileURLWithPath: expanded).standardizedFileURL
  }

  static func validateManagedRoot(_ root: URL) throws {
    let normalized = canonicalURL(root)
    let home = canonicalURL(URL(fileURLWithPath: NSHomeDirectory()))
    if normalized.path == "/" || normalized.path == home.path || normalized.pathComponents.count < 3 {
      throw DesktopShellError.invalidStateRootBootstrap
    }
    let preferences = home.appendingPathComponent("Library/Preferences", isDirectory: true)
    if managedRootContains(normalized, preferences) {
      throw DesktopShellError.invalidStateRootBootstrap
    }
  }

  static func sameManagedRoot(_ left: URL, _ right: URL) -> Bool {
    canonicalComponents(left) == canonicalComponents(right)
  }

  static func managedRootContains(_ root: URL, _ candidate: URL) -> Bool {
    let rootComponents = canonicalComponents(root)
    let candidateComponents = canonicalComponents(candidate)
    return candidateComponents.count >= rootComponents.count &&
      Array(candidateComponents.prefix(rootComponents.count)) == rootComponents
  }

  private static var initialRootDirectory: URL {
    let candidate: URL
    if let override = ProcessInfo.processInfo.environment[initialRootEnvironmentName],
       !override.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      candidate = normalizedURL(override)
    } else {
      candidate = defaultRootDirectory
    }
    do {
      try validateManagedRoot(candidate)
      return candidate
    } catch {
      NSLog("[Nexus State Root] initial root is invalid: \(error.localizedDescription)")
      return defaultRootDirectory
    }
  }

  private static func loadState() -> [String: String] {
    defaults.dictionary(forKey: stateKey) as? [String: String] ?? [:]
  }

  private static func saveState(active: URL, previous: URL?, error: String?) {
    var state = ["active_path": active.standardizedFileURL.path]
    state["previous_path"] = previous?.standardizedFileURL.path
    state["migration_error"] = error
    defaults.set(state, forKey: stateKey)
    defaults.synchronize()
  }

  private static func canonicalURL(_ url: URL) -> URL {
    let fileManager = FileManager.default
    let normalized = url.standardizedFileURL
    var existingAncestor = normalized
    var missingComponents: [String] = []
    while existingAncestor.path != "/" &&
          !fileManager.fileExists(atPath: existingAncestor.path) {
      missingComponents.insert(existingAncestor.lastPathComponent, at: 0)
      existingAncestor.deleteLastPathComponent()
    }
    var resolved = existingAncestor.resolvingSymlinksInPath().standardizedFileURL
    for component in missingComponents {
      resolved.appendPathComponent(component)
    }
    return resolved.standardizedFileURL
  }

  private static func canonicalComponents(_ url: URL) -> [String] {
    // macOS 常见卷大小写不敏感；保守折叠可避免同一目录的大小写别名绕过删除保护。
    canonicalURL(url).pathComponents.map { $0.lowercased() }
  }
}
