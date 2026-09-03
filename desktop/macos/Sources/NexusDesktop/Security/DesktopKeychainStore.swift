// INPUT: macOS 签名模式、Keychain 条目及 canonical/legacy fallback key 文件。
// OUTPUT: 对同一状态根稳定复用的 active Connector credentials key、显式 legacy keys 与来源诊断。
// POS: 桌面宿主启动 sidecar 前唯一的 Connector active/legacy 加密密钥选择边界。
import Foundation
import LocalAuthentication
import Security

enum DesktopKeychainMode: String {
  case auto
  case file
  case keychain

  init?(environmentValue: String?) {
    guard let value = environmentValue?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased(),
          !value.isEmpty else {
      return nil
    }
    self.init(rawValue: value)
  }
}

struct DesktopCredentialsKey {
  let value: String
  let legacyValues: [String]
  let storage: String
  let reason: String
}

enum DesktopKeychainStore {
  private static let service = "com.leemysw.nexus.desktop"
  private static let connectorCredentialsKeyAccount = "connector-credentials-key"
  private static let adHocSignatureFlag: UInt32 = 0x0002

  static func connectorCredentialsKey(mode: DesktopKeychainMode) throws -> DesktopCredentialsKey {
    switch mode {
    case .file:
      return resolvedCredentialsKey(
        value: try localFallbackKey(),
        storage: "file",
        reason: "forced_or_development"
      )
    case .keychain:
      return resolvedCredentialsKey(
        value: try keychainConnectorCredentialsKey(),
        storage: "keychain",
        reason: "forced"
      )
    case .auto:
      if isCurrentCodeAdHocSigned() {
        NSLog("[Nexus Keychain] ad-hoc signature detected, using local protected key as active.")
        return resolvedCredentialsKey(
          value: try localFallbackKey(),
          storage: "file",
          reason: "ad_hoc_signature"
        )
      }
      do {
        return resolvedCredentialsKey(
          value: try keychainConnectorCredentialsKey(),
          storage: "keychain",
          reason: "signed_auto"
        )
      } catch {
        NSLog("[Nexus Keychain] unavailable, using local protected fallback: \(error.localizedDescription)")
        return resolvedCredentialsKey(
          value: try localFallbackKey(),
          storage: "file",
          reason: "keychain_unavailable"
        )
      }
    }
  }

  private static func resolvedCredentialsKey(
    value: String,
    storage: String,
    reason: String
  ) -> DesktopCredentialsKey {
    var candidates = existingFallbackKeys()
    if storage != "keychain",
       let keychain = try? readWithTimeout(account: connectorCredentialsKeyAccount),
       !keychain.isEmpty {
      candidates.insert(keychain, at: 0)
    }
    var seen = Set<String>([value])
    let legacyValues = candidates.filter { candidate in
      guard !seen.contains(candidate) else { return false }
      seen.insert(candidate)
      return true
    }
    return DesktopCredentialsKey(
      value: value,
      legacyValues: legacyValues,
      storage: storage,
      reason: reason
    )
  }

  private static func keychainConnectorCredentialsKey() throws -> String {
    do {
      if let existing = try readWithTimeout(account: connectorCredentialsKeyAccount) {
        return existing
      }
      if let fallback = existingFallbackKey() {
        try write(fallback, account: connectorCredentialsKeyAccount)
        return fallback
      }
      let generated = try generateBase64Key()
      try write(generated, account: connectorCredentialsKeyAccount)
      return generated
    } catch {
      throw error
    }
  }

  // ad-hoc 重签名后的 legacy Keychain ACL 可能阻塞授权等待，启动路径必须有硬超时。
  private static func readWithTimeout(account: String) throws -> String? {
    let semaphore = DispatchSemaphore(value: 0)
    let resultLock = NSLock()
    var result: Result<String?, Error>?

    DispatchQueue.global(qos: .utility).async {
      let nextResult = Result {
        try read(account: account)
      }
      resultLock.lock()
      result = nextResult
      resultLock.unlock()
      semaphore.signal()
    }

    guard semaphore.wait(timeout: .now() + 1.0) == .success else {
      throw DesktopShellError.keychainReadTimedOut
    }
    resultLock.lock()
    let finalResult = result
    resultLock.unlock()
    return try finalResult?.get()
  }

  private static func read(account: String) throws -> String? {
    let context = LAContext()
    context.interactionNotAllowed = true
    var query = baseQuery(account: account)
    query[kSecReturnData as String] = true
    query[kSecMatchLimit as String] = kSecMatchLimitOne
    query[kSecUseAuthenticationContext as String] = context

    var result: CFTypeRef?
    let status = SecItemCopyMatching(query as CFDictionary, &result)
    if status == errSecItemNotFound {
      return nil
    }
    guard status == errSecSuccess else {
      throw DesktopShellError.keychainReadFailed(status)
    }
    guard let data = result as? Data,
          let value = String(data: data, encoding: .utf8),
          !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
      throw DesktopShellError.keychainPayloadInvalid
    }
    return value.trimmingCharacters(in: .whitespacesAndNewlines)
  }

  private static func write(_ value: String, account: String) throws {
    guard let data = value.data(using: .utf8) else {
      throw DesktopShellError.keychainPayloadInvalid
    }
    var item = baseQuery(account: account)
    item[kSecValueData as String] = data
    item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly

    let status = SecItemAdd(item as CFDictionary, nil)
    if status == errSecDuplicateItem {
      let updateStatus = SecItemUpdate(baseQuery(account: account) as CFDictionary, [
        kSecValueData as String: data,
      ] as CFDictionary)
      guard updateStatus == errSecSuccess else {
        throw DesktopShellError.keychainWriteFailed(updateStatus)
      }
      return
    }
    guard status == errSecSuccess else {
      throw DesktopShellError.keychainWriteFailed(status)
    }
  }

  private static func generateBase64Key() throws -> String {
    var bytes = [UInt8](repeating: 0, count: 32)
    let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
    guard status == errSecSuccess else {
      throw DesktopShellError.keychainWriteFailed(status)
    }
    return Data(bytes).base64EncodedString()
  }

  private static func baseQuery(account: String) -> [String: Any] {
    [
      kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: service,
      kSecAttrAccount as String: account,
      kSecAttrSynchronizable as String: kCFBooleanFalse as Any,
    ]
  }

  private static func localFallbackKey() throws -> String {
    let fileURL = localFallbackKeyURL()
    let directoryURL = fileURL.deletingLastPathComponent()
    try FileManager.default.createDirectory(at: directoryURL, withIntermediateDirectories: true)
    try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: directoryURL.path)

    if let existing = readFallbackKey(at: fileURL) {
      return existing
    }

    if let legacy = readFallbackKey(at: legacyFallbackKeyURL()) {
      try legacy.write(to: fileURL, atomically: true, encoding: .utf8)
      try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: fileURL.path)
      return legacy
    }

    let generated = try generateBase64Key()
    try generated.write(to: fileURL, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: fileURL.path)
    return generated
  }

  private static func existingFallbackKey() -> String? {
    existingFallbackKeys().first
  }

  private static func existingFallbackKeys() -> [String] {
    var seen = Set<String>()
    return [localFallbackKeyURL(), legacyFallbackKeyURL()].compactMap { fileURL in
      guard let value = readFallbackKey(at: fileURL), !seen.contains(value) else {
        return nil
      }
      seen.insert(value)
      return value
    }
  }

  private static func readFallbackKey(at fileURL: URL) -> String? {
    guard let value = try? String(contentsOf: fileURL, encoding: .utf8) else {
      return nil
    }
    let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
    return trimmed.isEmpty ? nil : trimmed
  }

  private static func legacyFallbackKeyURL() -> URL {
    DesktopPaths.rootDirectory
      .appendingPathComponent("config", isDirectory: true)
      .appendingPathComponent("connector-credentials.key", isDirectory: false)
  }

  private static func localFallbackKeyURL() -> URL {
    DesktopPaths.connectorCredentialsFallbackKeyURL
  }

  private static func isCurrentCodeAdHocSigned() -> Bool {
    guard let flags = currentCodeSignatureFlags() else {
      return true
    }
    return flags & adHocSignatureFlag != 0
  }

  private static func currentCodeSignatureFlags() -> UInt32? {
    var code: SecCode?
    let codeStatus = SecCodeCopySelf(SecCSFlags(), &code)
    guard codeStatus == errSecSuccess, let code else {
      NSLog("[Nexus Keychain] unable to inspect current code signature: \(codeStatus)")
      return nil
    }

    var staticCode: SecStaticCode?
    let staticStatus = SecCodeCopyStaticCode(code, SecCSFlags(), &staticCode)
    guard staticStatus == errSecSuccess, let staticCode else {
      NSLog("[Nexus Keychain] unable to inspect static code signature: \(staticStatus)")
      return nil
    }

    var information: CFDictionary?
    let infoStatus = SecCodeCopySigningInformation(staticCode, SecCSFlags(rawValue: kSecCSSigningInformation), &information)
    guard infoStatus == errSecSuccess,
          let signingInfo = information as? [String: Any],
          let flags = signingInfo[kSecCodeInfoFlags as String] as? NSNumber else {
      NSLog("[Nexus Keychain] unable to read code signature flags: \(infoStatus)")
      return nil
    }
    return flags.uint32Value
  }
}
