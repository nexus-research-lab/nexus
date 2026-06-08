import Foundation

final class SidecarLogPipe {
  private let pipe = Pipe()
  private let label: String?

  init(label: String? = nil) {
    self.label = label
    pipe.fileHandleForReading.readabilityHandler = { [label] handle in
      let data = handle.availableData
      guard !data.isEmpty, let text = String(data: data, encoding: .utf8) else {
        return
      }
      for line in text.split(separator: "\n", omittingEmptySubsequences: false) {
        let value = line.trimmingCharacters(in: .whitespacesAndNewlines)
        if !value.isEmpty {
          let output = stripSidecarPrettyTimestamp(value)
          if let label {
            NSLog("[\(label)] %@", output)
          } else {
            NSLog("%@", output)
          }
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

private func stripSidecarPrettyTimestamp(_ value: String) -> String {
  guard value.first == "[", let closeIndex = value.firstIndex(of: "]") else {
    return value
  }
  let token = value[value.index(after: value.startIndex)..<closeIndex]
    .trimmingCharacters(in: .whitespaces)
  guard isSidecarPrettyTimestampToken(token) else {
    return value
  }
  let restIndex = value.index(after: closeIndex)
  return value[restIndex...].trimmingCharacters(in: .whitespaces)
}

private func isSidecarPrettyTimestampToken(_ value: String) -> Bool {
  let scalars = Array(value.unicodeScalars)
  guard scalars.count == 12 else {
    return false
  }
  for (index, scalar) in scalars.enumerated() {
    switch index {
    case 2, 5:
      guard scalar.value == 58 else {
        return false
      }
    case 8:
      guard scalar.value == 46 else {
        return false
      }
    default:
      guard scalar.value >= 48 && scalar.value <= 57 else {
        return false
      }
    }
  }
  return true
}
