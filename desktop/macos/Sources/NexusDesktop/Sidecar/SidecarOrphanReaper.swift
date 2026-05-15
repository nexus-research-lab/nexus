import Darwin
import Foundation

struct SidecarProcessRecord: Codable {
  let pid: Int32
  let executablePath: String
}

final class SidecarOrphanReaper {
  private static let pidPathBufferSize = 4096

  private let fileManager: FileManager
  private let pidFileURL: URL
  private let expectedExecutablePath: String

  init(pidFileURL: URL, expectedExecutablePath: String, fileManager: FileManager = .default) {
    self.fileManager = fileManager
    self.pidFileURL = pidFileURL
    self.expectedExecutablePath = expectedExecutablePath
  }

  func reapIfNeeded() {
    guard let record = readRecord() else {
      return
    }

    guard isReapable(record: record) else {
      removeRecord()
      return
    }

    NSLog("[Nexus Sidecar] found orphaned sidecar pid \(record.pid), terminating")
    terminate(pid: record.pid)
    removeRecord()
  }

  func write(pid: Int32) {
    let record = SidecarProcessRecord(pid: pid, executablePath: expectedExecutablePath)
    do {
      try fileManager.createDirectory(
        at: pidFileURL.deletingLastPathComponent(),
        withIntermediateDirectories: true
      )
      let data = try JSONEncoder().encode(record)
      try data.write(to: pidFileURL, options: .atomic)
    } catch {
      NSLog("[Nexus Sidecar] failed to write pid record: \(error.localizedDescription)")
    }
  }

  func removeRecord() {
    guard fileManager.fileExists(atPath: pidFileURL.path) else {
      return
    }
    do {
      try fileManager.removeItem(at: pidFileURL)
    } catch {
      NSLog("[Nexus Sidecar] failed to remove pid record: \(error.localizedDescription)")
    }
  }

  private func readRecord() -> SidecarProcessRecord? {
    do {
      let data = try Data(contentsOf: pidFileURL)
      return try JSONDecoder().decode(SidecarProcessRecord.self, from: data)
    } catch {
      if fileManager.fileExists(atPath: pidFileURL.path) {
        NSLog("[Nexus Sidecar] removing invalid pid record: \(error.localizedDescription)")
        removeRecord()
      }
      return nil
    }
  }

  private func isReapable(record: SidecarProcessRecord) -> Bool {
    guard record.pid > 0 else {
      return false
    }
    guard record.pid != ProcessInfo.processInfo.processIdentifier else {
      return false
    }
    guard record.executablePath == expectedExecutablePath else {
      return false
    }
    guard let liveExecutablePath = executablePath(pid: record.pid) else {
      return false
    }
    return liveExecutablePath == expectedExecutablePath
  }

  private func terminate(pid: Int32) {
    guard kill(pid, SIGTERM) == 0 else {
      return
    }

    // Go sidecar 正常会响应 SIGTERM；短暂等待后仍存活再强制清理，避免遗留进程继续占用本地资源。
    for _ in 0..<20 {
      if kill(pid, 0) != 0 {
        return
      }
      usleep(100_000)
    }
    _ = kill(pid, SIGKILL)
  }

  private func executablePath(pid: Int32) -> String? {
    var buffer = [CChar](repeating: 0, count: Self.pidPathBufferSize)
    let result = proc_pidpath(pid, &buffer, UInt32(buffer.count))
    guard result > 0 else {
      return nil
    }
    return String(cString: buffer)
  }
}
