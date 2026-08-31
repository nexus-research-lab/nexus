// INPUT: macOS lifecycle events, sidecar startup, state-root recovery, and native menu actions.
// OUTPUT: A running Nexus shell or a safe startup failure with internal diagnostics preserved.
// POS: macOS application lifecycle boundary; raw startup causes never enter user-facing alerts.

import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
  private static let showMainWindowNotification = Notification.Name("com.leemysw.nexus.showMainWindow")
  private static let showLauncherNotification = Notification.Name("com.leemysw.nexus.showLauncher")
  private static let exitNotification = Notification.Name("com.leemysw.nexus.exit")
  private static let exitCommandArgument = "--nexus-desktop-exit"

  private let startupTimeline = DesktopStartupTimeline()
  private lazy var updateChecker = DesktopUpdateChecker(startupTimeline: startupTimeline)
  private var singleInstanceGuard: SingleInstanceGuard?
  private var sidecar: SidecarSupervisor?
  private var windowManager: WindowManager?
  private var globalShortcutMonitor: GlobalShortcutMonitor?
  private var globalShortcutLastError: String?
  private var pendingApplicationURLs: [URL] = []
  private var shouldShowSettingsAfterStart = false
  private var terminationRequested = false
  private var runtimeShutdownCompleted = false

  func applicationDidFinishLaunching(_ notification: Notification) {
    startupTimeline.mark("app.did_finish_launching")
    NSApp.setActivationPolicy(.regular)
    ApplicationMenuBuilder.install(target: self)

    if Self.isExitCommandRequested {
      startupTimeline.mark("app.exit_activation")
      notifyRunningInstanceToExit()
      NSApp.terminate(nil)
      return
    }

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
    DistributedNotificationCenter.default().addObserver(
      self,
      selector: #selector(exitFromDistributedNotification(_:)),
      name: Self.exitNotification,
      object: nil
    )

    Task {
      await start()
    }
  }

  func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
    if runtimeShutdownCompleted {
      return .terminateNow
    }
    terminationRequested = true
    shutdownRuntime(reason: "application_should_terminate")
    return .terminateNow
  }

  func applicationWillTerminate(_ notification: Notification) {
    shutdownRuntime(reason: "application_will_terminate")
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
  func clearWebCache(_ sender: Any?) {
    Task { @MainActor [weak self] in
      guard let self else {
        return
      }
      await DesktopWebCacheInvalidator.clearCachesManually(startupTimeline: startupTimeline)
      windowManager?.reloadMainWindow()
    }
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
      guard !terminationRequested else {
        supervisor.stop()
        return
      }
      completePendingStateRootMigration()
      await DesktopWebCacheInvalidator.clearCachesIfNeeded(
        runtime: runtime,
        startupTimeline: startupTimeline
      )
      await updateChecker.clearStaleUpdateCacheIfNeeded()
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
        updateStarter: { [weak self] in
          self?.updateChecker.startAvailableUpdate() ?? "unavailable"
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
      guard !terminationRequested else {
        return
      }
      if rollbackPendingStateRootMigration(error: error) {
        return
      }
      showStartupError(error)
    }
  }

  private func completePendingStateRootMigration() {
    do {
      guard let previousRoot = try DesktopStateRootStore.completeMigration() else {
        return
      }
      startupTimeline.mark("state_root.migration_committed", metadata: [
        "previous_root": previousRoot.path,
        "state_root": DesktopPaths.rootDirectory.path,
      ])
      Task.detached {
        do {
          try FileManager.default.removeItem(at: previousRoot)
        } catch {
          NSLog("[Nexus State Root] old root cleanup failed: \(error.localizedDescription)")
        }
      }
    } catch {
      startupTimeline.mark("state_root.migration_commit_failed", metadata: [
        "error": error.localizedDescription,
      ])
    }
  }

  private func rollbackPendingStateRootMigration(error: Error) -> Bool {
    guard let previousRoot = DesktopStateRootStore.previousRootDirectory else {
      return false
    }
    do {
      _ = DesktopStateRootStore.rollbackMigration(message: error.localizedDescription)
      try DesktopStateRootMigration.scheduleRelaunchAfterExit(source: previousRoot)
      terminationRequested = true
      startupTimeline.mark("state_root.migration_rolled_back", metadata: [
        "error": error.localizedDescription,
        "state_root": previousRoot.path,
      ])
      NSApp.terminate(nil)
      return true
    } catch {
      startupTimeline.mark("state_root.migration_rollback_failed", metadata: [
        "error": error.localizedDescription,
      ])
      return false
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
      globalShortcutLastError = DesktopFailureCopy.globalShortcutMessage
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
      globalShortcutLastError = DesktopFailureCopy.globalShortcutMessage
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
      globalShortcutLastError = DesktopFailureCopy.globalShortcutMessage
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

  @objc
  private func exitFromDistributedNotification(_ notification: Notification) {
    Task { @MainActor [weak self] in
      self?.startupTimeline.mark("app.exit_requested", metadata: [
        "source": "distributed_notification",
      ])
      NSApp.terminate(nil)
    }
  }

  private static var isExitCommandRequested: Bool {
    ProcessInfo.processInfo.arguments.contains { argument in
      argument.caseInsensitiveCompare(exitCommandArgument) == .orderedSame
    }
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

  private func notifyRunningInstanceToExit() {
    DistributedNotificationCenter.default().postNotificationName(
      Self.exitNotification,
      object: nil,
      userInfo: nil,
      deliverImmediately: true
    )
  }

  private func showStartupError(_ error: Error) {
    startupTimeline.mark("startup.failed", metadata: ["error": error.localizedDescription])
    let diagnosticsURL = DesktopDiagnosticsReport.writeStartupFailure(error: error, startupTimeline: startupTimeline)
    let alert = NSAlert()
    alert.messageText = DesktopFailureCopy.startupTitle
    alert.informativeText = DesktopFailureCopy.startupMessage(
      diagnosticsAvailable: diagnosticsURL != nil
    )
    alert.alertStyle = .critical
    alert.runModal()
    NSApp.terminate(nil)
  }

  private func shutdownRuntime(reason: String) {
    guard !runtimeShutdownCompleted else {
      return
    }
    runtimeShutdownCompleted = true
    startupTimeline.mark("desktop.shutdown_begin", metadata: [
      "reason": reason,
    ])
    DistributedNotificationCenter.default().removeObserver(self)
    globalShortcutMonitor?.stop()
    globalShortcutMonitor = nil
    sidecar?.stop()
    sidecar = nil
    singleInstanceGuard = nil
    startupTimeline.mark("desktop.shutdown_finished", metadata: [
      "reason": reason,
    ])
  }
}
