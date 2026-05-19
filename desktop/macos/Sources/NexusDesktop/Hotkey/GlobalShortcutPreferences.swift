import Foundation

enum GlobalShortcutPreferences {
  static let defaultLauncherAccelerator = "Option + Space"

  static var launcherAccelerator: String {
    get {
      UserDefaults.standard.string(forKey: launcherAcceleratorKey) ?? defaultLauncherAccelerator
    }
    set {
      UserDefaults.standard.set(newValue, forKey: launcherAcceleratorKey)
    }
  }

  static var launcherEnabled: Bool {
    get {
      guard UserDefaults.standard.object(forKey: launcherEnabledKey) != nil else {
        return false
      }
      return UserDefaults.standard.bool(forKey: launcherEnabledKey)
    }
    set {
      UserDefaults.standard.set(newValue, forKey: launcherEnabledKey)
    }
  }

  static func resetLauncherAccelerator() {
    launcherAccelerator = defaultLauncherAccelerator
  }

  static func disableDefaultLauncherShortcut() {
    if launcherAccelerator == defaultLauncherAccelerator {
      launcherEnabled = false
    }
  }

  private static let launcherEnabledKey = "launcher.globalShortcut.enabled"
  private static let launcherAcceleratorKey = "launcher.globalShortcut.accelerator"
}
