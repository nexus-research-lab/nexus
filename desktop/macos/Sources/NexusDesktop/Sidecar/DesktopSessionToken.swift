import Foundation
import Security

enum DesktopSessionToken {
  static func generate() throws -> String {
    var bytes = [UInt8](repeating: 0, count: 32)
    let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
    guard status == errSecSuccess else {
      throw DesktopShellError.sessionTokenUnavailable
    }
    return bytes.map { String(format: "%02x", $0) }.joined()
  }
}
