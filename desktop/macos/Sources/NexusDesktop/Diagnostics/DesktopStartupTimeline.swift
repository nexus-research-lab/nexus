import Foundation

final class DesktopStartupTimeline {
  private struct Mark {
    let event: String
    let elapsedMilliseconds: Double
    let deltaMilliseconds: Double
    let metadata: [String: String]
  }

  private let startNanoseconds = DispatchTime.now().uptimeNanoseconds
  private let lock = NSLock()
  private var lastNanoseconds: UInt64
  private var marks: [Mark] = []

  init() {
    lastNanoseconds = startNanoseconds
  }

  @discardableResult
  func mark(_ event: String, metadata: [String: String] = [:]) -> [String: Any] {
    let now = DispatchTime.now().uptimeNanoseconds
    let mark: Mark

    lock.lock()
    mark = Mark(
      event: event,
      elapsedMilliseconds: milliseconds(from: startNanoseconds, to: now),
      deltaMilliseconds: milliseconds(from: lastNanoseconds, to: now),
      metadata: metadata
    )
    lastNanoseconds = now
    marks.append(mark)
    lock.unlock()

    log(mark)
    return dictionary(for: mark)
  }

  func snapshot() -> [[String: Any]] {
    lock.lock()
    let currentMarks = marks
    lock.unlock()
    return currentMarks.map { dictionary(for: $0) }
  }

  private func log(_ mark: Mark) {
    let metadata = mark.metadata
      .sorted { $0.key < $1.key }
      .map { "\($0.key)=\($0.value)" }
      .joined(separator: " ")
    let suffix = metadata.isEmpty ? "" : " \(metadata)"
    NSLog(
      "[Nexus Startup] event=\(mark.event) elapsed_ms=\(format(mark.elapsedMilliseconds)) delta_ms=\(format(mark.deltaMilliseconds))\(suffix)"
    )
  }

  private func dictionary(for mark: Mark) -> [String: Any] {
    [
      "event": mark.event,
      "elapsed_ms": rounded(mark.elapsedMilliseconds),
      "delta_ms": rounded(mark.deltaMilliseconds),
      "metadata": mark.metadata,
    ]
  }

  private func milliseconds(from start: UInt64, to end: UInt64) -> Double {
    Double(end - start) / 1_000_000
  }

  private func format(_ value: Double) -> String {
    String(format: "%.1f", value)
  }

  private func rounded(_ value: Double) -> Double {
    (value * 10).rounded() / 10
  }
}
