import Foundation

final class SidecarLogPipe {
  private let pipe = Pipe()
  private let label: String

  init(label: String) {
    self.label = label
    pipe.fileHandleForReading.readabilityHandler = { [label] handle in
      let data = handle.availableData
      guard !data.isEmpty, let text = String(data: data, encoding: .utf8) else {
        return
      }
      for line in text.split(separator: "\n", omittingEmptySubsequences: false) {
        let value = line.trimmingCharacters(in: .whitespacesAndNewlines)
        if !value.isEmpty {
          NSLog("[Nexus Sidecar \(label)] \(value)")
        }
      }
    }
  }

  var fileHandleForWriting: FileHandle {
    pipe.fileHandleForWriting
  }

  func close() {
    pipe.fileHandleForReading.readabilityHandler = nil
  }
}
