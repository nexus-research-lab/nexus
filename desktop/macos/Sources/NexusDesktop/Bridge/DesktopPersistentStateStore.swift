import Foundation

enum DesktopPersistentStateStore {
  private static let queue = DispatchQueue(label: "com.leemysw.nexus.desktop-persistent-state")
  private static var stateURL: URL {
    DesktopPaths.configDirectory.appendingPathComponent("desktop-state.json")
  }

  static func get(_ key: String) throws -> String? {
    let normalizedKey = try normalizeKey(key)
    return try queue.sync {
      let values = try readAll()
      return values[normalizedKey]
    }
  }

  static func set(_ value: String, forKey key: String) throws {
    let normalizedKey = try normalizeKey(key)
    try queue.sync {
      var values = try readAll()
      values[normalizedKey] = value
      try writeAll(values)
    }
  }

  static func remove(_ key: String) throws {
    let normalizedKey = try normalizeKey(key)
    try queue.sync {
      var values = try readAll()
      values.removeValue(forKey: normalizedKey)
      try writeAll(values)
    }
  }

  private static func normalizeKey(_ key: String) throws -> String {
    let normalized = key.trimmingCharacters(in: .whitespacesAndNewlines)
    let allowed = CharacterSet(charactersIn: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")
    guard !normalized.isEmpty,
          normalized.count <= 128,
          normalized.rangeOfCharacter(from: allowed.inverted) == nil else {
      throw DesktopPersistentStateError.invalidKey
    }
    return normalized
  }

  private static func readAll() throws -> [String: String] {
    let fileManager = FileManager.default
    guard fileManager.fileExists(atPath: stateURL.path) else {
      return [:]
    }
    do {
      let data = try Data(contentsOf: stateURL)
      let raw = try JSONSerialization.jsonObject(with: data) as? [String: String]
      return raw ?? [:]
    } catch {
      return [:]
    }
  }

  private static func writeAll(_ values: [String: String]) throws {
    let fileManager = FileManager.default
    try fileManager.createDirectory(at: DesktopPaths.configDirectory, withIntermediateDirectories: true)
    let data = try JSONSerialization.data(withJSONObject: values, options: [.sortedKeys])
    let tempURL = stateURL.appendingPathExtension("tmp")
    try data.write(to: tempURL, options: [.atomic])
    if fileManager.fileExists(atPath: stateURL.path) {
      _ = try fileManager.replaceItemAt(stateURL, withItemAt: tempURL)
    } else {
      try fileManager.moveItem(at: tempURL, to: stateURL)
    }
  }
}

private enum DesktopPersistentStateError: LocalizedError {
  case invalidKey

  var errorDescription: String? {
    "Persistent state key is invalid."
  }
}
