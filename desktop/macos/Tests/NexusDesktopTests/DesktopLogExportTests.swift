import Foundation
import XCTest
@testable import NexusDesktop

final class DesktopLogExportTests: XCTestCase {
  func testExportIncludesRuntimeDiagnosticsAndRotatedLogsWithoutTruncation() throws {
    let fm = FileManager.default
    let root = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? fm.removeItem(at: root) }
    let state = root.appendingPathComponent("state")
    let staging = root.appendingPathComponent("export")
    try fm.createDirectory(at: staging, withIntermediateDirectories: true)
    let fixtures = [
      ("app/logs/sidecar-2026-09-14.log", "Logs/sidecar-2026-09-14.log"),
      ("app/debug/host.jsonl", "Debug/host.jsonl"),
      ("users/__system__/runtime/logs/runtime-2026-09-13.log.gz", "RuntimeLogs/runtime-2026-09-13.log.gz"),
      ("users/__system__/runtime/logs/debug/session.jsonl", "RuntimeLogs/debug/session.jsonl"),
      ("users/__system__/runtime/debug/session.txt", "RuntimeDebug/session.txt"),
    ]
    let content = Data((String(repeating: "diagnostic event\n", count: 10000) + "last event\n").utf8)
    for (source, _) in fixtures {
      let file = state.appendingPathComponent(source)
      try fm.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
      try content.write(to: file)
    }
    // User workspaces and other owners are outside the diagnostic export scope.
    for excluded in ["users/__system__/workspace/private.txt", "users/another/runtime/logs/private.log"] {
      let file = state.appendingPathComponent(excluded)
      try fm.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
      try Data("private".utf8).write(to: file)
    }

    try DesktopBridgeHandler.stageLogDirectories(stateRoot: state, staging: staging)

    for (_, destination) in fixtures {
      XCTAssertEqual(try Data(contentsOf: staging.appendingPathComponent(destination)), content)
    }
    XCTAssertEqual(Set(try fm.contentsOfDirectory(atPath: staging.path)), Set(["Logs", "Debug", "RuntimeLogs", "RuntimeDebug"]))
  }

  func testExportAllowsMissingDiagnosticDirectories() throws {
    let fm = FileManager.default
    let root = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? fm.removeItem(at: root) }
    try fm.createDirectory(at: root, withIntermediateDirectories: true)

    XCTAssertNoThrow(try DesktopBridgeHandler.stageLogDirectories(
      stateRoot: root.appendingPathComponent("missing"), staging: root
    ))
    XCTAssertTrue(try fm.contentsOfDirectory(atPath: root.path).isEmpty)
  }
}
