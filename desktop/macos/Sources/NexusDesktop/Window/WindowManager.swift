import AppKit

@MainActor
final class WindowManager: NSObject, NSWindowDelegate {
  private let runtime: SidecarRuntimeConfig
  private let startupTimeline: DesktopStartupTimeline
  private let globalShortcutStatusProvider: () -> [String: Any]
  private let globalShortcutEnabledUpdater: (Bool) -> [String: Any]
  private let globalShortcutAcceleratorUpdater: (String) -> [String: Any]
  private let globalShortcutAcceleratorResetter: () -> [String: Any]
  private let onMainWindowRevealed: () -> Void
  private var mainWindow: NSWindow?
  private var mainWebViewHost: WebViewHost?
  private var mainWindowRevealed = false

  init(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline,
    globalShortcutStatusProvider: @escaping () -> [String: Any],
    globalShortcutEnabledUpdater: @escaping (Bool) -> [String: Any],
    globalShortcutAcceleratorUpdater: @escaping (String) -> [String: Any],
    globalShortcutAcceleratorResetter: @escaping () -> [String: Any],
    onMainWindowRevealed: @escaping () -> Void
  ) {
    self.runtime = runtime
    self.startupTimeline = startupTimeline
    self.globalShortcutStatusProvider = globalShortcutStatusProvider
    self.globalShortcutEnabledUpdater = globalShortcutEnabledUpdater
    self.globalShortcutAcceleratorUpdater = globalShortcutAcceleratorUpdater
    self.globalShortcutAcceleratorResetter = globalShortcutAcceleratorResetter
    self.onMainWindowRevealed = onMainWindowRevealed
    super.init()
  }

  func showMainWindow() {
    showMainWindow(route: DesktopWebRoute(path: "/", entry: .app))
  }

  func reopenMainWindow() {
    showMainWindow(route: mainWindow == nil ? defaultMainRoute() : nil)
  }

  func showLauncher() {
    showMainWindow(route: DesktopWebRoute(path: "/", entry: .app))
  }

  func showSettings() {
    showMainWindow(route: DesktopWebRoute(path: "/settings"))
  }

  func reloadMainWindow() {
    mainWebViewHost?.reload()
  }

  func handleApplicationURL(_ url: URL) -> Bool {
    guard let route = DesktopURLRouter.webRoute(for: url) else {
      return false
    }
    startupTimeline.mark("app.url_route", metadata: [
      "host": url.host?.lowercased() ?? "",
      "path": url.path,
      "route_path": route.path,
    ])
    open(route: route)
    return true
  }

  func windowShouldClose(_ sender: NSWindow) -> Bool {
    sender.orderOut(nil)
    return false
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
    showMainWindow(route: route)
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

  private func defaultMainRoute() -> DesktopWebRoute {
    DesktopWebRoute(path: "/", entry: .app)
  }

  private func surfaceName(for window: NSWindow) -> String? {
    if window === mainWindow {
      return "main"
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
    onMainWindowRevealed()
  }

  private func installInitialRevealFallback() {
    Task { @MainActor in
      try? await Task.sleep(nanoseconds: 3_000_000_000)
      revealMainWindowIfNeeded(source: "fallback_timeout")
    }
  }
}
