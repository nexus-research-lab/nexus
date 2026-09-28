// INPUT: Persisted sidecar record and boot-bound kernel process identity.
// OUTPUT: Exact-identity termination or retained unknown record with startup refusal.
// POS: Reads legacy records without treating PID/path as signal authority.
import Darwin
import Foundation

struct SidecarProcessRecord: Codable, Equatable {
  let pid: Int32
  let executablePath: String
  let version: Int?
  let identity: SidecarProcessIdentity?
}

final class SidecarOrphanReaper {
  private let fileManager: FileManager
  private let pidFileURL: URL
  private let expectedExecutablePath: String
  private let control: SidecarProcessControlling
  private let pause: () -> Void
  private var expectedRecord: SidecarProcessRecord?
  private var ownedIdentity: SidecarProcessIdentity?

  init(pidFileURL: URL, expectedExecutablePath: String, fileManager: FileManager = .default,
       control: SidecarProcessControlling = NativeSidecarProcessControl(),
       pause: @escaping () -> Void = { usleep(100_000) }) {
    self.fileManager = fileManager
    self.pidFileURL = pidFileURL
    self.expectedExecutablePath = expectedExecutablePath
    self.control = control
    self.pause = pause
  }

  func reapIfNeeded() throws {
    guard let record = try readRecord() else { return }
    expectedRecord = record
    guard record.pid > 1, record.pid != getpid(), record.executablePath.hasPrefix("/") else {
      throw SidecarRecoveryError(detail: "invalid saved sidecar identity")
    }
    guard let live = try control.inspect(pid: record.pid) else {
      removeRecord()
      return
    }
    guard record.version == 2, let original = record.identity,
          original.pid == record.pid, original.executablePath == record.executablePath,
          original.auditToken.count == 8, original.auditToken[5] == UInt32(record.pid),
          original.auditToken[7] != 0, original.auditToken[1] == geteuid(),
          UUID(uuidString: original.bootSession) != nil else {
      throw SidecarRecoveryError(detail: "legacy live process has no exact identity")
    }
    if !live.identifiesSameProcess(as: original) {
      // The saved exact process is gone; never signal the replacement PID.
      removeRecord()
      return
    }
    guard live.executablePath == original.executablePath else {
      throw SidecarRecoveryError(detail: "saved process changed executable")
    }
    try control.signal(original, SIGTERM)
    if try waitUntilGone(original) { removeRecord(); return }
    try control.signal(original, SIGKILL)
    guard try waitUntilGone(original) else {
      throw SidecarRecoveryError(detail: "sidecar termination remains unknown")
    }
    removeRecord()
  }

  func write(pid: Int32) throws {
    let identity: SidecarProcessIdentity
    do {
      guard let observed = try control.inspect(pid: pid), observed.executablePath == expectedExecutablePath else {
        throw SidecarRecoveryError(detail: "new sidecar identity unavailable")
      }
      identity = observed
    } catch {
      // Preserve a legacy-format pending record if identity capture fails after launch.
      let pending = SidecarProcessRecord(pid: pid, executablePath: expectedExecutablePath, version: 1, identity: nil)
      try fileManager.createDirectory(at: pidFileURL.deletingLastPathComponent(), withIntermediateDirectories: true)
      try JSONEncoder().encode(pending).write(to: pidFileURL, options: .atomic)
      expectedRecord = pending
      throw error
    }
    ownedIdentity = identity
    let record = SidecarProcessRecord(pid: pid, executablePath: expectedExecutablePath,
                                      version: 2, identity: identity)
    try fileManager.createDirectory(at: pidFileURL.deletingLastPathComponent(), withIntermediateDirectories: true)
    try JSONEncoder().encode(record).write(to: pidFileURL, options: .atomic)
    expectedRecord = record
  }

  // The live Process object is only a wait source; signals use the captured kernel identity.
  func signalOwned(pid: Int32, signal: Int32) throws {
    guard let identity = ownedIdentity, identity.pid == pid else {
      throw SidecarRecoveryError(detail: "owned sidecar identity unavailable")
    }
    try control.signal(identity, signal)
  }

  func removeRecord() {
    guard let expected = expectedRecord else { return }
    do {
      guard try readRecord() == expected else { return }
      try fileManager.removeItem(at: pidFileURL)
      expectedRecord = nil
    } catch {
      NSLog("[Nexus Sidecar] retained process record: \(error.localizedDescription)")
    }
  }

  private func readRecord() throws -> SidecarProcessRecord? {
    let fd = open(pidFileURL.path, O_RDONLY | O_NOFOLLOW | O_CLOEXEC)
    if fd < 0 {
      if errno == ENOENT { return nil }
      throw SidecarRecoveryError(detail: "cannot open sidecar record: \(errno)")
    }
    defer { close(fd) }
    var info = stat()
    guard fstat(fd, &info) == 0, (info.st_mode & S_IFMT) == S_IFREG,
          info.st_nlink == 1, info.st_uid == geteuid(), info.st_size <= 4096 else {
      throw SidecarRecoveryError(detail: "invalid sidecar record file")
    }
    var bytes = [UInt8](repeating: 0, count: 4097)
    let size = read(fd, &bytes, bytes.count)
    guard size > 0, size <= 4096 else { throw SidecarRecoveryError(detail: "invalid sidecar record length") }
    do { return try JSONDecoder().decode(SidecarProcessRecord.self, from: Data(bytes.prefix(size))) }
    catch { throw SidecarRecoveryError(detail: "invalid sidecar record format") }
  }

  private func waitUntilGone(_ identity: SidecarProcessIdentity) throws -> Bool {
    var lastObservationError: Error?
    for _ in 0..<20 {
      do {
        guard let current = try control.inspect(pid: identity.pid) else { return true }
        if !current.identifiesSameProcess(as: identity) { return true }
        lastObservationError = nil
      } catch { lastObservationError = error }
      pause()
    }
    if let error = lastObservationError { throw error }
    return false
  }
}
