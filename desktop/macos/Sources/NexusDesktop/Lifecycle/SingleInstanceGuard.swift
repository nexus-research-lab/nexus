import Darwin
import Foundation

final class SingleInstanceGuard {
  private let fileDescriptor: Int32

  private init(fileDescriptor: Int32) {
    self.fileDescriptor = fileDescriptor
  }

  deinit {
    Darwin.lockf(fileDescriptor, F_ULOCK, 0)
    Darwin.close(fileDescriptor)
  }

  static func acquire() throws -> SingleInstanceGuard {
    let fileManager = FileManager.default
    let directory = applicationSupportDirectory()
    try fileManager.createDirectory(at: directory, withIntermediateDirectories: true)

    let lockPath = directory.appendingPathComponent("NexusDesktop.lock").path
    let descriptor = Darwin.open(lockPath, O_CREAT | O_RDWR, S_IRUSR | S_IWUSR)
    guard descriptor >= 0 else {
      throw DesktopShellError.singleInstanceLockUnavailable
    }

    guard Darwin.lockf(descriptor, F_TLOCK, 0) == 0 else {
      Darwin.close(descriptor)
      throw DesktopShellError.appAlreadyRunning
    }

    Darwin.ftruncate(descriptor, 0)
    let processID = "\(ProcessInfo.processInfo.processIdentifier)\n"
    processID.withCString { pointer in
      _ = Darwin.write(descriptor, pointer, strlen(pointer))
    }
    return SingleInstanceGuard(fileDescriptor: descriptor)
  }

  private static func applicationSupportDirectory() -> URL {
    let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
      ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support")
    return base.appendingPathComponent("Nexus", isDirectory: true)
  }
}
