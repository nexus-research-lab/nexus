import Darwin
import Foundation

private enum DesktopStateRootMigrationError: LocalizedError {
  case executableUnavailable
  case invalidArguments
  case invalidDestination(String)
  case parentDidNotExit

  var errorDescription: String? {
    switch self {
    case .executableUnavailable:
      return "无法定位 Nexus 桌面应用可执行文件。"
    case .invalidArguments:
      return "状态根迁移参数无效。"
    case .invalidDestination(let message):
      return message
    case .parentDidNotExit:
      return "Nexus 主进程未能及时退出，迁移已取消。"
    }
  }
}

enum DesktopStateRootMigration {
  private static let migrateArgument = "--nexus-state-root-migrate"
  private static let relaunchArgument = "--nexus-state-root-relaunch"
  private static let parentPIDArgument = "--parent-pid"
  private static let sourceArgument = "--source"
  private static let targetArgument = "--target"
  private static let parentExitTimeout: TimeInterval = 45
  private static let scheduleLock = NSLock()
  private static var migrationScheduled = false

  static func runHelperIfRequested() -> Bool {
    let arguments = ProcessInfo.processInfo.arguments
    guard arguments.contains(migrateArgument) || arguments.contains(relaunchArgument) else {
      return false
    }
    let source = value(after: sourceArgument, in: arguments).map(DesktopStateRootStore.normalizedURL)
    do {
      guard let parentValue = value(after: parentPIDArgument, in: arguments),
            let parentPID = Int32(parentValue) else {
        throw DesktopStateRootMigrationError.invalidArguments
      }
      try waitForParentExit(parentPID)
      if arguments.contains(migrateArgument) {
        guard let source,
              let targetValue = value(after: targetArgument, in: arguments) else {
          throw DesktopStateRootMigrationError.invalidArguments
        }
        let target = DesktopStateRootStore.normalizedURL(targetValue)
        try validate(source: source, target: target)
        try copyStateRoot(source: source, target: target)
        DesktopStateRootStore.activateMigration(source: source, target: target)
      }
    } catch {
      if let source {
        DesktopStateRootStore.recordMigrationFailure(
          source: source,
          message: error.localizedDescription
        )
      }
      NSLog("[Nexus State Root] migration helper failed: \(error.localizedDescription)")
    }
    do {
      try relaunchApplication()
    } catch {
      NSLog("[Nexus State Root] relaunch failed: \(error.localizedDescription)")
    }
    return true
  }

  static func scheduleMigration(to rawPath: String) throws -> URL {
    scheduleLock.lock()
    defer { scheduleLock.unlock() }
    if migrationScheduled {
      throw DesktopStateRootMigrationError.invalidDestination("Nexus 数据目录迁移已经开始。")
    }
    let source = DesktopPaths.rootDirectory
    let target = rawPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
      ? DesktopStateRootStore.defaultRootDirectory
      : DesktopStateRootStore.normalizedURL(rawPath)
    try validate(source: source, target: target)
    try startHelper(arguments: [
      migrateArgument,
      parentPIDArgument,
      "\(ProcessInfo.processInfo.processIdentifier)",
      sourceArgument,
      source.path,
      targetArgument,
      target.path,
    ])
    migrationScheduled = true
    return target
  }

  static func scheduleRelaunchAfterExit(source: URL) throws {
    try startHelper(arguments: [
      relaunchArgument,
      parentPIDArgument,
      "\(ProcessInfo.processInfo.processIdentifier)",
      sourceArgument,
      source.path,
    ])
  }

  private static func startHelper(arguments: [String]) throws {
    guard let executableURL = Bundle.main.executableURL else {
      throw DesktopStateRootMigrationError.executableUnavailable
    }
    let helper = Process()
    helper.executableURL = executableURL
    helper.arguments = arguments
    helper.environment = ProcessInfo.processInfo.environment
    try helper.run()
  }

  private static func validate(source: URL, target: URL) throws {
    guard source.path.hasPrefix("/"), target.path.hasPrefix("/") else {
      throw DesktopStateRootMigrationError.invalidDestination("Nexus 数据目录必须使用绝对路径。")
    }
    try DesktopStateRootStore.validateManagedRoot(source)
    try DesktopStateRootStore.validateManagedRoot(target)
    if DesktopStateRootStore.sameManagedRoot(source, target) {
      throw DesktopStateRootMigrationError.invalidDestination("新目录与当前 Nexus 数据目录相同。")
    }
    if DesktopStateRootStore.managedRootContains(source, target) ||
       DesktopStateRootStore.managedRootContains(target, source) {
      throw DesktopStateRootMigrationError.invalidDestination("新旧 Nexus 数据目录不能互相包含。")
    }
    guard FileManager.default.fileExists(atPath: source.path) else {
      throw DesktopStateRootMigrationError.invalidDestination("当前 Nexus 数据目录不存在。")
    }
    try requireEmptyOrMissingDirectory(target)
  }

  private static func requireEmptyOrMissingDirectory(_ target: URL) throws {
    var isDirectory: ObjCBool = false
    guard FileManager.default.fileExists(atPath: target.path, isDirectory: &isDirectory) else {
      return
    }
    guard isDirectory.boolValue else {
      throw DesktopStateRootMigrationError.invalidDestination("目标路径已存在且不是目录。")
    }
    let entries = try FileManager.default.contentsOfDirectory(atPath: target.path)
    if !entries.isEmpty {
      throw DesktopStateRootMigrationError.invalidDestination("目标目录必须为空。")
    }
  }

  private static func copyStateRoot(source: URL, target: URL) throws {
    let fileManager = FileManager.default
    let parent = target.deletingLastPathComponent()
    try fileManager.createDirectory(at: parent, withIntermediateDirectories: true)
    let staging = parent.appendingPathComponent(
      ".\(target.lastPathComponent).nexus-migration-\(UUID().uuidString)",
      isDirectory: true
    )
    do {
      try fileManager.copyItem(at: source, to: staging)
      if fileManager.fileExists(atPath: target.path) {
        try fileManager.removeItem(at: target)
      }
      try fileManager.moveItem(at: staging, to: target)
    } catch {
      try? fileManager.removeItem(at: staging)
      throw error
    }
  }

  private static func waitForParentExit(_ parentPID: Int32) throws {
    let deadline = Date().addingTimeInterval(parentExitTimeout)
    while Date() < deadline {
      if kill(parentPID, 0) != 0 && errno == ESRCH {
        return
      }
      Thread.sleep(forTimeInterval: 0.1)
    }
    throw DesktopStateRootMigrationError.parentDidNotExit
  }

  private static func relaunchApplication() throws {
    guard let executableURL = Bundle.main.executableURL else {
      throw DesktopStateRootMigrationError.executableUnavailable
    }
    let process = Process()
    process.executableURL = executableURL
    process.environment = ProcessInfo.processInfo.environment
    try process.run()
  }

  private static func value(after key: String, in arguments: [String]) -> String? {
    guard let index = arguments.firstIndex(of: key), arguments.indices.contains(index + 1) else {
      return nil
    }
    return arguments[index + 1]
  }

}
