import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
  private static let showMainWindowNotification = Notification.Name("com.leemysw.nexus.showMainWindow")
  private static let showLauncherNotification = Notification.Name("com.leemysw.nexus.showLauncher")

  private let startupTimeline = DesktopStartupTimeline()
  private lazy var updateChecker = DesktopUpdateChecker(startupTimeline: startupTimeline)
  private var singleInstanceGuard: SingleInstanceGuard?
  private var sidecar: SidecarSupervisor?
  private var windowManager: WindowManager?
  private var globalShortcutMonitor: GlobalShortcutMonitor?
  private var globalShortcutLastError: String?
  private var pendingApplicationURLs: [URL] = []
  private var shouldShowSettingsAfterStart = false

  func applicationDidFinishLaunching(_ notification: Notification) {
    startupTimeline.mark("app.did_finish_launching")
    NSApp.setActivationPolicy(.regular)
    ApplicationMenuBuilder.install(target: self)

    do {
      singleInstanceGuard = try SingleInstanceGuard.acquire()
    } catch DesktopShellError.appAlreadyRunning {
      notifyRunningInstance()
      NSApp.terminate(nil)
      return
    } catch {
      showStartupError(error)
      return
    }
    startupTimeline.mark("single_instance.acquired")

    DistributedNotificationCenter.default().addObserver(
      self,
      selector: #selector(showMainWindowFromDistributedNotification(_:)),
      name: Self.showMainWindowNotification,
      object: nil
    )
    DistributedNotificationCenter.default().addObserver(
      self,
      selector: #selector(showLauncherFromDistributedNotification(_:)),
      name: Self.showLauncherNotification,
      object: nil
    )

    Task {
      await start()
    }
  }

  func applicationWillTerminate(_ notification: Notification) {
    DistributedNotificationCenter.default().removeObserver(self)
    globalShortcutMonitor?.stop()
    sidecar?.stop()
    singleInstanceGuard = nil
  }

  func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    false
  }

  func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
    windowManager?.reopenMainWindow()
    return true
  }

  func application(_ application: NSApplication, open urls: [URL]) {
    handleApplicationURLs(urls)
  }

  @objc
  func showPreferences(_ sender: Any?) {
    guard let windowManager else {
      shouldShowSettingsAfterStart = true
      return
    }
    windowManager.showSettings()
  }

  @objc
  func showLauncher(_ sender: Any?) {
    windowManager?.showLauncher()
  }

  @objc
  func reloadMainWindow(_ sender: Any?) {
    windowManager?.reloadMainWindow()
  }

  @objc
  func checkForUpdates(_ sender: Any?) {
    updateChecker.checkNowFromMenu()
  }

  private func start() async {
    do {
      startupTimeline.mark("desktop.start_begin")
      let supervisor = try SidecarSupervisor(startupTimeline: startupTimeline)
      sidecar = supervisor
      let runtime = try await supervisor.start()
      let manager = WindowManager(
        runtime: runtime,
        startupTimeline: startupTimeline,
        globalShortcutStatusProvider: { [weak self] in
          self?.globalShortcutStatus() ?? [:]
        },
        globalShortcutEnabledUpdater: { [weak self] enabled in
          self?.setGlobalShortcutEnabled(enabled) ?? [:]
        },
        globalShortcutAcceleratorUpdater: { [weak self] accelerator in
          self?.setGlobalShortcutAccelerator(accelerator) ?? [:]
        },
        globalShortcutAcceleratorResetter: { [weak self] in
          self?.resetGlobalShortcutAccelerator() ?? [:]
        },
        onMainWindowRevealed: { [weak self] in
          self?.updateChecker.checkOnLaunchIfNeeded()
        }
      )
      windowManager = manager
      startupTimeline.mark("window_manager.ready")
      GlobalShortcutPreferences.disableDefaultLauncherShortcut()
      applyGlobalShortcutPreference()
      drainPendingStartupActions(manager: manager)
    } catch {
      showStartupError(error)
    }
  }

  private func drainPendingStartupActions(manager: WindowManager) {
    let showSettings = shouldShowSettingsAfterStart
    shouldShowSettingsAfterStart = false

    let urls = pendingApplicationURLs
    pendingApplicationURLs.removeAll()

    if showSettings {
      manager.showSettings()
    }

    let handledURL = handleApplicationURLs(urls)
    if !showSettings && !handledURL {
      manager.showLauncher()
    }
  }

  @discardableResult
  private func handleApplicationURLs(_ urls: [URL]) -> Bool {
    guard let windowManager else {
      pendingApplicationURLs.append(contentsOf: urls)
      return false
    }

    var handled = false
    for url in urls {
      if windowManager.handleApplicationURL(url) {
        handled = true
      } else {
        NSLog("[Nexus App] unsupported application URL: \(url.absoluteString)")
      }
    }
    return handled
  }

  private func applyGlobalShortcutPreference() {
    globalShortcutMonitor?.stop()
    globalShortcutMonitor = nil
    globalShortcutLastError = nil

    guard GlobalShortcutPreferences.launcherEnabled else {
      return
    }

    guard GlobalShortcutPreferences.launcherAccelerator != GlobalShortcutPreferences.defaultLauncherAccelerator else {
      GlobalShortcutPreferences.launcherEnabled = false
      return
    }

    let definition: GlobalShortcutDefinition
    do {
      definition = try GlobalShortcutDefinition.parse(GlobalShortcutPreferences.launcherAccelerator)
      GlobalShortcutPreferences.launcherAccelerator = definition.accelerator
    } catch {
      globalShortcutLastError = error.localizedDescription
      NSLog("[Nexus App] global shortcut invalid: \(error.localizedDescription)")
      return
    }

    let monitor = GlobalShortcutMonitor { [weak self] in
      self?.windowManager?.showLauncher()
    }
    do {
      try monitor.start(definition: definition)
      globalShortcutMonitor = monitor
    } catch {
      globalShortcutLastError = error.localizedDescription
      NSLog("[Nexus App] global shortcut unavailable: \(error.localizedDescription)")
    }
  }

  private func globalShortcutStatus() -> [String: Any] {
    var payload: [String: Any] = [
      "enabled": GlobalShortcutPreferences.launcherEnabled,
      "registered": globalShortcutMonitor != nil,
      "accelerator": GlobalShortcutPreferences.launcherAccelerator,
      "default_accelerator": GlobalShortcutPreferences.defaultLauncherAccelerator,
      "is_default": GlobalShortcutPreferences.launcherAccelerator == GlobalShortcutPreferences.defaultLauncherAccelerator,
    ]
    if let globalShortcutLastError {
      payload["error_message"] = globalShortcutLastError
    }
    return payload
  }

  private func setGlobalShortcutEnabled(_ enabled: Bool) -> [String: Any] {
    GlobalShortcutPreferences.launcherEnabled = enabled
    applyGlobalShortcutPreference()
    return globalShortcutStatus()
  }

  private func setGlobalShortcutAccelerator(_ accelerator: String) -> [String: Any] {
    do {
      let definition = try GlobalShortcutDefinition.parse(accelerator)
      GlobalShortcutPreferences.launcherAccelerator = definition.accelerator
      GlobalShortcutPreferences.launcherEnabled = true
      applyGlobalShortcutPreference()
    } catch {
      globalShortcutLastError = error.localizedDescription
    }
    return globalShortcutStatus()
  }

  private func resetGlobalShortcutAccelerator() -> [String: Any] {
    GlobalShortcutPreferences.resetLauncherAccelerator()
    GlobalShortcutPreferences.launcherEnabled = false
    applyGlobalShortcutPreference()
    return globalShortcutStatus()
  }

  @objc
  private func showMainWindowFromDistributedNotification(_ notification: Notification) {
    windowManager?.showMainWindow()
  }

  @objc
  private func showLauncherFromDistributedNotification(_ notification: Notification) {
    windowManager?.showLauncher()
  }

  private func notifyRunningInstance() {
    DistributedNotificationCenter.default().postNotificationName(
      Self.showLauncherNotification,
      object: nil,
      userInfo: nil,
      deliverImmediately: true
    )

    guard let bundleIdentifier = Bundle.main.bundleIdentifier else {
      return
    }
    let currentProcessID = ProcessInfo.processInfo.processIdentifier
    NSRunningApplication.runningApplications(withBundleIdentifier: bundleIdentifier)
      .first { $0.processIdentifier != currentProcessID }?
      .activate(options: [.activateAllWindows])
  }

  private func showStartupError(_ error: Error) {
    startupTimeline.mark("startup.failed", metadata: ["error": error.localizedDescription])
    let diagnosticsURL = DesktopDiagnosticsReport.writeStartupFailure(error: error, startupTimeline: startupTimeline)
    let alert = NSAlert()
    alert.messageText = "Nexus 启动失败"
    if let diagnosticsURL {
      alert.informativeText = "\(error.localizedDescription)\n\n诊断报告已写入：\(diagnosticsURL.path)"
    } else {
      alert.informativeText = error.localizedDescription
    }
    alert.alertStyle = .critical
    alert.runModal()
    NSApp.terminate(nil)
  }
}
