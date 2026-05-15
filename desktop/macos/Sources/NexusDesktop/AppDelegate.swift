import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
  private static let showMainWindowNotification = Notification.Name("com.leemysw.nexus.showMainWindow")

  private let startupTimeline = DesktopStartupTimeline()
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
    windowManager?.showMainWindow()
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
        }
      )
      windowManager = manager
      startupTimeline.mark("window_manager.ready")
      applyGlobalShortcutPreference()
      drainPendingStartupActions(manager: manager)
    } catch {
      showStartupError(error)
    }
  }

  private func drainPendingStartupActions(manager: WindowManager) {
    if shouldShowSettingsAfterStart {
      shouldShowSettingsAfterStart = false
      manager.showSettings()
    } else {
      manager.showMainWindow()
    }

    guard !pendingApplicationURLs.isEmpty else {
      return
    }
    let urls = pendingApplicationURLs
    pendingApplicationURLs.removeAll()
    handleApplicationURLs(urls)
  }

  private func handleApplicationURLs(_ urls: [URL]) {
    guard let windowManager else {
      pendingApplicationURLs.append(contentsOf: urls)
      return
    }

    for url in urls {
      if !windowManager.handleApplicationURL(url) {
        NSLog("[Nexus App] unsupported application URL: \(url.absoluteString)")
      }
    }
  }

  private func applyGlobalShortcutPreference() {
    globalShortcutMonitor?.stop()
    globalShortcutMonitor = nil
    globalShortcutLastError = nil

    guard GlobalShortcutPreferences.launcherEnabled else {
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
    GlobalShortcutPreferences.launcherEnabled = true
    applyGlobalShortcutPreference()
    return globalShortcutStatus()
  }

  @objc
  private func showMainWindowFromDistributedNotification(_ notification: Notification) {
    windowManager?.showMainWindow()
  }

  private func notifyRunningInstance() {
    DistributedNotificationCenter.default().postNotificationName(
      Self.showMainWindowNotification,
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
