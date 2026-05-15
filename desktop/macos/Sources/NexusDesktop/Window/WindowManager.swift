import AppKit

@MainActor
final class WindowManager: NSObject, NSWindowDelegate {
  private let runtime: SidecarRuntimeConfig
  private let startupTimeline: DesktopStartupTimeline
  private let globalShortcutStatusProvider: () -> [String: Any]
  private let globalShortcutEnabledUpdater: (Bool) -> [String: Any]
  private let globalShortcutAcceleratorUpdater: (String) -> [String: Any]
  private let globalShortcutAcceleratorResetter: () -> [String: Any]
  private var mainWindow: NSWindow?
  private var mainWebViewHost: WebViewHost?
  private var mainWindowRevealed = false
  private var launcherWindow: NSPanel?
  private var launcherWebViewHost: WebViewHost?
  private var launcherWindowRevealed = false
  private var launcherKeyDownMonitor: Any?

  init(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any]
  ) {
    self.runtime = runtime
    self.startupTimeline = startupTimeline
    self.globalShortcutStatusProvider = globalShortcutStatusProvider
    self.globalShortcutEnabledUpdater = globalShortcutEnabledUpdater
    self.globalShortcutAcceleratorUpdater = globalShortcutAcceleratorUpdater
    self.globalShortcutAcceleratorResetter = globalShortcutAcceleratorResetter
    super.init()
  }

  func showMainWindow() {
    showMainWindow(route: nil)
  }

  func showLauncher() {
    showLauncherWindow()
  }

  func closeLauncher() {
    launcherWindow?.orderOut(nil)
  }

  func showSettings() {
    showMainWindow(route: DesktopWebRoute(path: "/settings"))
  }

  func reloadMainWindow() {
    if launcherWindow?.isKeyWindow == true {
      launcherWebViewHost?.reload()
      return
    }
    mainWebViewHost?.reload()
  }

  func handleApplicationURL(_ url: URL) -> Bool {
    guard let route = DesktopURLRouter.webRoute(for: url) else {
      return false
    }
    open(route: route)
    return true
  }

  func windowShouldClose(_ sender: NSWindow) -> Bool {
    if sender === launcherWindow {
      closeLauncher()
      return false
    }
    sender.orderOut(nil)
    return false
  }

  func windowDidResignKey(_ notification: Notification) {
    guard let window = notification.object as? NSWindow,
          window === launcherWindow else {
      return
    }
    closeLauncher()
  }

  func windowDidChangeOcclusionState(_ notification: Notification) {
    guard let window = notification.object as? NSWindow,
          let surface = surfaceName(for: window) else {
      return
    }
    startupTimeline.mark("\(surface)_window.occlusion_changed", metadata: [
      "visible": window.occlusionState.contains(.visible) ? "true" : "false",
    ])
  }

  func windowDidMiniaturize(_ notification: Notification) {
    guard let window = notification.object as? NSWindow,
          let surface = surfaceName(for: window) else {
      return
    }
    startupTimeline.mark("\(surface)_window.miniaturized")
  }

  func windowDidDeminiaturize(_ notification: Notification) {
    guard let window = notification.object as? NSWindow,
          let surface = surfaceName(for: window) else {
      return
    }
    startupTimeline.mark("\(surface)_window.deminiaturized")
  }

  private func open(route: DesktopWebRoute) {
    switch route.presentation {
    case .main:
      showMainWindow(route: route)
      closeLauncher()
    case .launcher:
      showLauncherWindow()
    }
  }

  private func showMainWindow(route: DesktopWebRoute?) {
    if let mainWindow {
      mainWindow.makeKeyAndOrderFront(nil)
      NSApp.activate()
      if let route {
        startupTimeline.mark("main_window.route_load", metadata: ["path": route.path])
        mainWebViewHost?.load(route.url(runtime: runtime))
      }
      return
    }

    do {
      startupTimeline.mark("main_window.create_begin")
      mainWindowRevealed = false
      let host = try WebViewHost(
        runtime: runtime,
        surfaceName: "main",
        startupTimeline: startupTimeline,
        onWebReady: { [weak self] in
          self?.revealMainWindowIfNeeded(source: "web.ready")
        },
        openRoute: { [weak self] route in
          self?.open(route: route)
        },
        closeLauncher: { [weak self] in
          self?.closeLauncher()
        },
        globalShortcutStatusProvider: globalShortcutStatusProvider,
        globalShortcutEnabledUpdater: globalShortcutEnabledUpdater,
        globalShortcutAcceleratorUpdater: globalShortcutAcceleratorUpdater,
        globalShortcutAcceleratorResetter: globalShortcutAcceleratorResetter
      )
      let window = NSWindow(
        contentRect: NSRect(x: 0, y: 0, width: 1280, height: 820),
        styleMask: [.titled, .closable, .miniaturizable, .resizable],
        backing: .buffered,
        defer: false
      )
      window.title = "Nexus"
      window.minSize = NSSize(width: 960, height: 640)
      window.isReleasedWhenClosed = false
      window.delegate = self
      window.backgroundColor = .clear
      window.isOpaque = false
      window.alphaValue = 0
      window.center()
      window.contentView = DesktopWindowSurface(
        webContentView: host.webView,
        material: .windowBackground
      )
      window.makeKeyAndOrderFront(nil)
      NSApp.activate()
      startupTimeline.mark("main_window.created", metadata: [
        "material": "windowBackground",
      ])
      host.load((route ?? defaultMainRoute()).url(runtime: runtime))

      mainWebViewHost = host
      mainWindow = window
      installInitialRevealFallback()
    } catch {
      let alert = NSAlert(error: error)
      alert.runModal()
    }
  }

  private func showLauncherWindow() {
    let route = DesktopWebRoute(
      path: "/",
      percentEncodedQuery: "desktop_surface=launcher",
      presentation: .launcher,
      entry: .launcher
    )
    if let launcherWindow {
      launcherWebViewHost?.load(route.url(runtime: runtime))
      launcherWindow.center()
      launcherWindow.makeKeyAndOrderFront(nil)
      NSApp.activate()
      return
    }

    do {
      startupTimeline.mark("launcher_window.create_begin")
      launcherWindowRevealed = false
      let host = try WebViewHost(
        runtime: runtime,
        surfaceName: "launcher",
        startupTimeline: startupTimeline,
        onWebReady: { [weak self] in
          self?.revealLauncherWindowIfNeeded(source: "web.ready")
        },
        openRoute: { [weak self] route in
          self?.open(route: route)
        },
        closeLauncher: { [weak self] in
          self?.closeLauncher()
        },
        globalShortcutStatusProvider: globalShortcutStatusProvider,
        globalShortcutEnabledUpdater: globalShortcutEnabledUpdater,
        globalShortcutAcceleratorUpdater: globalShortcutAcceleratorUpdater,
        globalShortcutAcceleratorResetter: globalShortcutAcceleratorResetter
      )
      let window = NSPanel(
        contentRect: NSRect(x: 0, y: 0, width: 900, height: 620),
        styleMask: [.titled, .closable, .fullSizeContentView],
        backing: .buffered,
        defer: false
      )
      window.title = "Nexus Launcher"
      window.titleVisibility = .hidden
      window.titlebarAppearsTransparent = true
      window.isMovableByWindowBackground = true
      window.isReleasedWhenClosed = false
      window.delegate = self
      window.level = .floating
      window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
      window.backgroundColor = .clear
      window.isOpaque = false
      window.hasShadow = true
      window.alphaValue = 0
      window.standardWindowButton(.closeButton)?.isHidden = true
      window.standardWindowButton(.miniaturizeButton)?.isHidden = true
      window.standardWindowButton(.zoomButton)?.isHidden = true
      window.center()
      window.contentView = DesktopWindowSurface(
        webContentView: host.webView,
        material: .popover,
        cornerRadius: 18
      )
      window.makeKeyAndOrderFront(nil)
      NSApp.activate()
      startupTimeline.mark("launcher_window.created", metadata: [
        "material": "popover",
      ])
      host.load(route.url(runtime: runtime))

      launcherWebViewHost = host
      launcherWindow = window
      installLauncherKeyDownMonitor()
      installLauncherRevealFallback()
    } catch {
      let alert = NSAlert(error: error)
      alert.runModal()
    }
  }

  private func defaultMainRoute() -> DesktopWebRoute {
    DesktopWebRoute(path: "/app", entry: .app)
  }

  private func surfaceName(for window: NSWindow) -> String? {
    if window === mainWindow {
      return "main"
    }
    if window === launcherWindow {
      return "launcher"
    }
    return nil
  }

  private func revealMainWindowIfNeeded(source: String) {
    guard !mainWindowRevealed, let mainWindow else {
      return
    }
    mainWindowRevealed = true
    startupTimeline.mark("main_window.revealed", metadata: ["source": source])
    NSAnimationContext.runAnimationGroup { context in
      context.duration = 0.12
      mainWindow.animator().alphaValue = 1
    }
  }

  private func revealLauncherWindowIfNeeded(source: String) {
    guard !launcherWindowRevealed, let launcherWindow else {
      return
    }
    launcherWindowRevealed = true
    startupTimeline.mark("launcher_window.revealed", metadata: ["source": source])
    NSAnimationContext.runAnimationGroup { context in
      context.duration = 0.10
      launcherWindow.animator().alphaValue = 1
    }
  }

  private func installInitialRevealFallback() {
    Task { @MainActor in
      try? await Task.sleep(nanoseconds: 3_000_000_000)
      revealMainWindowIfNeeded(source: "fallback_timeout")
    }
  }

  private func installLauncherRevealFallback() {
    Task { @MainActor in
      try? await Task.sleep(nanoseconds: 2_000_000_000)
      revealLauncherWindowIfNeeded(source: "fallback_timeout")
    }
  }

  private func installLauncherKeyDownMonitor() {
    guard launcherKeyDownMonitor == nil else {
      return
    }
    launcherKeyDownMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
      guard let self else {
        return event
      }
      if self.launcherWindow?.isKeyWindow == true, event.keyCode == 53 {
        self.closeLauncher()
        return nil
      }
      return event
    }
  }
}
