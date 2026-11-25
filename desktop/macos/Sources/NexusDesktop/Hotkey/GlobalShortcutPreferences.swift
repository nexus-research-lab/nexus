import Foundation

enum GlobalShortcutPreferences {
  static let defaultAccelerator = "Option + Space"

  static var accelerator: String {
    get {
      UserDefaults.standard.string(forKey: acceleratorKey) ?? defaultAccelerator
    }
    set {
      UserDefaults.standard.set(newValue, forKey: acceleratorKey)
    }
  }

  static var enabled: Bool {
    get {
      guard UserDefaults.standard.object(forKey: enabledKey) != nil else {
        return false
      }
      return UserDefaults.standard.bool(forKey: enabledKey)
    }
    set {
      UserDefaults.standard.set(newValue, forKey: enabledKey)
    }
  }

  static func resetAccelerator() {
    accelerator = defaultAccelerator
  }

  static func disableDefaultShortcut() {
    if accelerator == defaultAccelerator {
      enabled = false
    }
  }

  // 键名沿用旧值，升级后保留用户已设置的快捷键。
  private static let enabledKey = "launcher.globalShortcut.enabled"
  private static let acceleratorKey = "launcher.globalShortcut.accelerator"
}
