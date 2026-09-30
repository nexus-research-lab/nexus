import Darwin
import Foundation



final class SidecarOrphanReaperTests {
  private final class Control: SidecarProcessControlling {
    var current: SidecarProcessIdentity?
    var denied = false
    var signals: [Int32] = []
    var signalDenied = false
    var ignoresTerm = false
    func inspect(pid: Int32) throws -> SidecarProcessIdentity? {
      if denied { throw SidecarRecoveryError(detail: "fixture access denied") }
      return current
    }
    func signal(_ identity: SidecarProcessIdentity, _ signal: Int32) throws {
      if signalDenied { throw SidecarRecoveryError(detail: "fixture signal failure") }
      signals.append(signal)
      if signal != SIGTERM || !ignoresTerm { current = nil }
    }
  }

  private func fixture() throws -> URL {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("sidecar-identity-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    return root
  }

  private func identity(version: UInt32 = 1) -> SidecarProcessIdentity {
    SidecarProcessIdentity(pid: 424242, executablePath: "/fixture/nexus-server",
                           bootSession: "12345678-1234-1234-1234-123456789abc",
                           auditToken: [0, geteuid(), 0, 0, 0, 424242, 0, version])
  }

  func testLegacyLiveAndObservationFailurePreserveRecord() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let data = Data("{\"pid\":424242,\"executablePath\":\"/fixture/nexus-server\"}".utf8)
    try data.write(to: file)
    let control = Control(); control.current = identity()
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: "/new/nexus-server", control: control, pause: {})
    XCTAssertThrowsError(try reaper.reapIfNeeded())
    XCTAssertEqual(try Data(contentsOf: file), data)
    XCTAssertTrue(control.signals.isEmpty)
    control.denied = true
    XCTAssertThrowsError(try reaper.reapIfNeeded())
    XCTAssertEqual(try Data(contentsOf: file), data)
    control.denied = false; control.current = nil
    try reaper.reapIfNeeded()
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }

  func testReusedPIDNeverReceivesSignal() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.current = identity()
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control, pause: {})
    try reaper.write(pid: identity().pid)
    control.current = identity(version: 2)
    try reaper.reapIfNeeded()
    XCTAssertTrue(control.signals.isEmpty)
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }

  func testExactRecordSurvivesDifferentInstalledPath() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.current = identity()
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control).write(pid: identity().pid)
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: "/new/app/nexus-server", control: control, pause: {}).reapIfNeeded()
    XCTAssertEqual(control.signals, [SIGTERM])
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }

  func testCorruptAndReplacedRecordsAreRetained() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.current = identity()
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control)
    try reaper.write(pid: identity().pid)
    let corrupt = Data("invalid".utf8); try corrupt.write(to: file)
    reaper.removeRecord()
    XCTAssertEqual(try Data(contentsOf: file), corrupt)
    XCTAssertThrowsError(try reaper.reapIfNeeded())
    XCTAssertTrue(control.signals.isEmpty)
  }

  func testSignalFailureAndSymlinkRetainRecord() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.current = identity()
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control, pause: {})
    try reaper.write(pid: identity().pid)
    let original = try Data(contentsOf: file)
    control.signalDenied = true
    XCTAssertThrowsError(try reaper.reapIfNeeded())
    XCTAssertEqual(try Data(contentsOf: file), original)
    let link = root.appendingPathComponent("link.json")
    try FileManager.default.createSymbolicLink(at: link, withDestinationURL: file)
    XCTAssertThrowsError(try SidecarOrphanReaper(pidFileURL: link, expectedExecutablePath: identity().executablePath, control: control).reapIfNeeded())
    XCTAssertEqual(try Data(contentsOf: file), original)
  }

  func testNativeOrphanCanBeObservedAndRetired() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let parent = Process(); let output = Pipe()
    parent.executableURL = URL(fileURLWithPath: "/bin/sh")
    parent.arguments = ["-c", "/bin/sleep 30 </dev/null >/dev/null 2>&1 & echo $!"]
    parent.standardOutput = output
    try parent.run()
    let bytes = output.fileHandleForReading.readDataToEndOfFile()
    parent.waitUntilExit()
    let pid = try XCTUnwrap(Int32(String(decoding: bytes, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)))
    let native = NativeSidecarProcessControl()
    let original = try XCTUnwrap(native.inspect(pid: pid))
    defer { try? native.signal(original, SIGKILL) }
    let file = root.appendingPathComponent("record.json")
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: original.executablePath).write(pid: pid)
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: "/updated/nexus-server").reapIfNeeded()
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }

  func testCaptureFailureLeavesPendingRecordAndNoSignalAuthority() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.denied = true
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control, pause: {})
    XCTAssertThrowsError(try reaper.write(pid: identity().pid))
    let pending = try JSONDecoder().decode(SidecarProcessRecord.self, from: Data(contentsOf: file))
    XCTAssertEqual(pending.version, 1)
    XCTAssertTrue(pending.identity == nil)
    XCTAssertThrowsError(try reaper.signalOwned(pid: identity().pid, signal: SIGTERM))
    XCTAssertTrue(control.signals.isEmpty)
  }

  func testIgnoredGracefulSignalEscalatesOnlySameIdentity() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("record.json")
    let control = Control(); control.current = identity(); control.ignoresTerm = true
    let reaper = SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: identity().executablePath, control: control, pause: {})
    try reaper.write(pid: identity().pid)
    try reaper.reapIfNeeded()
    XCTAssertEqual(control.signals, [SIGTERM, SIGKILL])
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }

  func testNativeExactTerminationAndStaleTokenDenial() throws {
    let root = try fixture(); defer { try? FileManager.default.removeItem(at: root) }
    let process = Process(); process.executableURL = URL(fileURLWithPath: "/bin/sleep"); process.arguments = ["30"]
    try process.run()
    defer { if process.isRunning { process.terminate() }; process.waitUntilExit() }
    let native = NativeSidecarProcessControl()
    let original = try XCTUnwrap(native.inspect(pid: process.processIdentifier))
    var words = original.auditToken; words[7] = words[7] == UInt32.max ? 1 : words[7] + 1
    let stale = SidecarProcessIdentity(pid: original.pid, executablePath: original.executablePath, bootSession: original.bootSession, auditToken: words)
    try native.signal(stale, SIGTERM)
    XCTAssertTrue(process.isRunning)
    let file = root.appendingPathComponent("record.json")
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: original.executablePath).write(pid: original.pid)
    try SidecarOrphanReaper(pidFileURL: file, expectedExecutablePath: "/new/location/nexus-server").reapIfNeeded()
    process.waitUntilExit()
    XCTAssertEqual(process.terminationReason, .uncaughtSignal)
    XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
  }
}
