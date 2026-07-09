import AppKit

@MainActor
final class WindowManager: NSObject, NSWindowDelegate {
  private static let preferredWindowSize = NSSize(width: 1280, height: 820)
  private static let preferredMinimumWindowSize = NSSize(width: 1120, height: 640)
  private static let compactMinimumWindowSize = NSSize(width: 720, height: 520)
  private static let screenPadding: CGFloat = 48

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
  private var mainWindowHealthProbeTask: Task<Void, Never>?

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
    showMainWindow(route: DesktopWebRoute(path: "/launcher", entry: .app))
  }

  func reopenMainWindow() {
    showMainWindow(route: mainWindow == nil ? defaultMainRoute() : nil)
  }

  func showLauncher() {
    showMainWindow(route: DesktopWebRoute(path: "/launcher", entry: .app))
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
    if window.occlusionState.contains(.visible) {
      recoverMainWebViewIfNeeded(reason: "occlusion_visible")
    }
  }

  func windowDidBecomeKey(_ notification: Notification) {
    guard let window = notification.object as? NSWindow,
          let surface = surfaceName(for: window) else {
      return
    }
    startupTimeline.mark("\(surface)_window.became_key")
    recoverMainWebViewIfNeeded(reason: "became_key")
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
    recoverMainWebViewIfNeeded(reason: "deminiaturized")
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
      } else {
        recoverMainWebViewIfNeeded(reason: "show_existing")
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
      let windowSizing = Self.initialWindowSizing()
      let window = NSWindow(
        contentRect: windowSizing.frame,
        styleMask: [.titled, .closable, .miniaturizable, .resizable],
        backing: .buffered,
        defer: false
      )
      window.title = "Nexus"
      window.titleVisibility = .hidden
      window.titlebarAppearsTransparent = true
      window.styleMask.insert(.fullSizeContentView)
      window.minSize = windowSizing.minimumSize
      window.isReleasedWhenClosed = false
      window.delegate = self
      window.backgroundColor = .clear
      window.isOpaque = false
      window.isMovableByWindowBackground = true
      window.alphaValue = 0
      window.contentView = DesktopWindowSurface(
        webContentView: host.webView,
        material: .windowBackground
      )
      window.makeKeyAndOrderFront(nil)
      NSApp.activate()
      startupTimeline.mark("main_window.created", metadata: [
        "height": Self.metadataDimension(windowSizing.frame.height),
        "material": "windowBackground",
        "min_height": Self.metadataDimension(windowSizing.minimumSize.height),
        "min_width": Self.metadataDimension(windowSizing.minimumSize.width),
        "width": Self.metadataDimension(windowSizing.frame.width),
      ])
      host.load((route ?? defaultMainRoute()).url(runtime: runtime))

      mainWebViewHost = host
      mainWindow = window
      installInitialRevealFallback()
      installMainWindowHealthProbe()
    } catch {
      let alert = NSAlert(error: error)
      alert.runModal()
    }
  }

  private func defaultMainRoute() -> DesktopWebRoute {
    DesktopWebRoute(path: "/launcher", entry: .app)
  }

  private static func initialWindowSizing(screen: NSScreen? = NSScreen.main) -> (frame: NSRect, minimumSize: NSSize) {
    guard let visibleFrame = screen?.visibleFrame else {
      return (
        NSRect(origin: .zero, size: preferredWindowSize),
        preferredMinimumWindowSize
      )
    }

    let maxWidth = max(320, visibleFrame.width - screenPadding)
    let maxHeight = max(320, visibleFrame.height - screenPadding)
    let width = min(preferredWindowSize.width, maxWidth)
    let height = min(preferredWindowSize.height, maxHeight)
    let frame = NSRect(
      x: visibleFrame.minX + (visibleFrame.width - width) / 2,
      y: visibleFrame.minY + (visibleFrame.height - height) / 2,
      width: width,
      height: height
    )
    return (
      frame,
      NSSize(
        width: adaptiveMinimum(preferred: preferredMinimumWindowSize.width, compact: compactMinimumWindowSize.width, current: width),
        height: adaptiveMinimum(preferred: preferredMinimumWindowSize.height, compact: compactMinimumWindowSize.height, current: height)
      )
    )
  }

  private static func adaptiveMinimum(preferred: CGFloat, compact: CGFloat, current: CGFloat) -> CGFloat {
    if current >= preferred {
      return preferred
    }
    return min(current, compact)
  }

  private static func metadataDimension(_ value: CGFloat) -> String {
    String(Int(value.rounded()))
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

  private func recoverMainWebViewIfNeeded(reason: String) {
    guard mainWindowRevealed else {
      return
    }
    mainWebViewHost?.recoverAfterWindowShown(reason: reason)
  }

  private func installInitialRevealFallback() {
    Task { @MainActor in
      try? await Task.sleep(nanoseconds: 3_000_000_000)
      revealMainWindowIfNeeded(source: "fallback_timeout")
    }
  }

  private func installMainWindowHealthProbe() {
    guard mainWindowHealthProbeTask == nil else {
      return
    }
    mainWindowHealthProbeTask = Task { @MainActor [weak self] in
      while !Task.isCancelled {
        try? await Task.sleep(nanoseconds: 60_000_000_000)
        guard let self,
              self.mainWindowRevealed,
              let window = self.mainWindow,
              window.isVisible,
              window.occlusionState.contains(.visible) else {
          continue
        }
        self.recoverMainWebViewIfNeeded(reason: "periodic_visible")
      }
    }
  }
}
