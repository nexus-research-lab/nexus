// INPUT: An isolated test runtime, JSON commands on stdin and the production native source files.
// OUTPUT: A real WindowManager/WKWebView process, native input events, layout observations and snapshots.
// POS: UI verification executable only; never compiled into the product target or started through AppDelegate/sidecar.

import AppKit
import WebKit

@main
enum FrontendHarnessMain {
  @MainActor static func main() {
    let application = NSApplication.shared
    let delegate = FrontendHarnessDelegate()
    application.delegate = delegate
    application.setActivationPolicy(.regular)
    application.run()
  }
}

@MainActor
private final class FrontendHarnessDelegate: NSObject, NSApplicationDelegate {
  private var manager: WindowManager?
  private let timeline = DesktopStartupTimeline()
  private var outputRoot = URL(fileURLWithPath: "/tmp")

  func applicationDidFinishLaunching(_ notification: Notification) {
    Task { await start() }
  }

  private func start() async {
    do {
      let environment = ProcessInfo.processInfo.environment
      guard let rawPort = environment["NEXUS_UI_TEST_PORT"], let port = Int(rawPort),
            (1024...65535).contains(port), port != 34343,
            let root = environment["NEXUS_DESKTOP_STATE_ROOT"],
            let output = environment["NEXUS_UI_TEST_OUTPUT"],
            let suite = environment["NEXUS_DESKTOP_PREFERENCES_SUITE"],
            suite.hasPrefix("com.nexus.frontend-qa."),
            suite == "\(Bundle.main.bundleIdentifier ?? "missing").bootstrap",
            DesktopStateRootStore.sameManagedRoot(DesktopPaths.rootDirectory, URL(fileURLWithPath: root)) else {
        throw HarnessError("Independent application, state root and port are required")
      }
      outputRoot = URL(fileURLWithPath: output)
      // Match the browser fixture: chrome uses local fonts. A remote CSS @import
      // must not delay all following rules or make native evidence network-dependent.
      let fontRule = try await WKContentRuleListStore.default().compileContentRuleList(
        forIdentifier: "nexus-ui-local-fonts",
        encodedContentRuleList: #"[{"trigger":{"url-filter":"^https://fontsapi\\.zeoseven\\.com/309/main/result\\.css"},"action":{"type":"block"}}]"#)
      let runtime = SidecarRuntimeConfig(port: port, sessionToken: "ui-fixture-only", appVersion: "ui-verification", appRootPath: root)
      manager = WindowManager(runtime: runtime, startupTimeline: timeline,
        globalShortcutStatusProvider: { [:] }, globalShortcutEnabledUpdater: { _ in [:] },
        globalShortcutAcceleratorUpdater: { _ in [:] }, globalShortcutAcceleratorResetter: { [:] },
        updateStarter: { "disabled-in-ui-fixture" }, onMainWindowRevealed: {})
      manager?.showMainWindow()
      guard let fontRule, let webView = findWebView(window?.contentView) else { throw HarnessError("Native fixture configuration is unavailable") }
      webView.configuration.userContentController.add(fontRule)
      reply(0, result: ["event": "started", "bundle_id": Bundle.main.bundleIdentifier ?? "", "preferences_suite": suite,
                        "state_root": DesktopPaths.rootDirectory.path])
      DispatchQueue.global().async { [weak self] in
        while let line = readLine() {
          guard let data = line.data(using: .utf8),
                let command = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { continue }
          Task { @MainActor in await self?.run(command) }
        }
        Task { @MainActor in NSApp.terminate(nil) }
      }
    } catch {
      reply(0, error: error)
      NSApp.terminate(nil)
    }
  }

  private var window: NSWindow? { NSApp.windows.first { $0 is DesktopWindow } }

  private func findWebView(_ view: NSView?) -> WKWebView? {
    guard let view else { return nil }
    if let webView = view as? WKWebView { return webView }
    return view.subviews.lazy.compactMap { self.findWebView($0) }.first
  }

  private func run(_ command: [String: Any]) async {
    let id = command["id"] as? Int ?? -1
    do {
      guard let window, let webView = findWebView(window.contentView) else { throw HarnessError("Window is unavailable") }
      switch command["op"] as? String {
      case "status":
        let center = DesktopWindowMetrics.windowCloseButtonCenter(in: window)
        reply(id, result: ["visible": window.isVisible, "key": window.isKeyWindow,
          "occluded": !window.occlusionState.contains(.visible), "window": [window.frame.width, window.frame.height],
          "webview": [webView.bounds.width, webView.bounds.height], "controls_center": [center.x, center.y],
          "controls_inset": DesktopWindowMetrics.windowControlsLeadingInset(in: window), "url": webView.url?.absoluteString ?? ""])
      case "evaluate":
        guard let script = command["script"] as? String else { throw HarnessError("Missing script") }
        reply(id, result: try await webView.evaluateJavaScript(script) ?? NSNull())
      case "route":
        guard let query = command["query"] as? String,
              let url = URL(string: "nexus://launcher?\(query)"), manager?.handleApplicationURL(url) == true else {
          throw HarnessError("Invalid fixture route")
        }
        reply(id, result: true)
      case "resize":
        guard let width = command["width"] as? Double, let height = command["height"] as? Double,
              (360...1600).contains(width), (520...1200).contains(height) else { throw HarnessError("Invalid test window size") }
        window.setContentSize(NSSize(width: width, height: height))
        reply(id, result: true)
      case "click":
        guard window.isKeyWindow, NSApp.isActive else { throw HarnessError("Activate the QA application before native input") }
        guard let x = command["x"] as? Double, let y = command["y"] as? Double else { throw HarnessError("Missing click coordinates") }
        let point = webView.convert(NSPoint(x: x, y: webView.isFlipped ? y : webView.bounds.height - y), to: nil)
        for type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
          guard let event = NSEvent.mouseEvent(with: type, location: point, modifierFlags: [],
            timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber, context: nil,
            eventNumber: 0, clickCount: 1, pressure: 1) else { throw HarnessError("Cannot create native pointer event") }
          window.sendEvent(event)
        }
        reply(id, result: true)
      case "key":
        guard window.isKeyWindow, NSApp.isActive else { throw HarnessError("Activate the QA application before native input") }
        guard let characters = command["characters"] as? String, let keyCode = command["key_code"] as? UInt16 else { throw HarnessError("Missing key") }
        let flags = (command["modifiers"] as? [String] ?? []).reduce(NSEvent.ModifierFlags()) { result, name in
          result.union(["shift": .shift, "control": .control, "option": .option, "command": .command][name] ?? [])
        }
        for type in [NSEvent.EventType.keyDown, .keyUp] {
          guard let event = NSEvent.keyEvent(with: type, location: .zero, modifierFlags: flags,
            timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber, context: nil,
            characters: characters, charactersIgnoringModifiers: characters, isARepeat: false, keyCode: keyCode) else { throw HarnessError("Cannot create native key event") }
          window.sendEvent(event)
        }
        reply(id, result: true)
      case "hide": window.orderOut(nil); reply(id, result: true)
      case "show": manager?.reopenMainWindow(); reply(id, result: true)
      case "snapshot":
        guard let name = command["name"] as? String,
              name.range(of: "^[a-zA-Z0-9_-]+$", options: .regularExpression) != nil else { throw HarnessError("Invalid snapshot name") }
        let snapshot = try await webView.takeSnapshot(configuration: nil)
        guard let tiff = snapshot.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff),
              let png = bitmap.representation(using: .png, properties: [:]) else { throw HarnessError("Cannot encode WKWebView snapshot") }
        let target = outputRoot.appendingPathComponent("\(name).png")
        try png.write(to: target)
        reply(id, result: target.path)
      case "timeline": reply(id, result: timeline.snapshot())
      case "quit": reply(id, result: true); NSApp.terminate(nil)
      default: throw HarnessError("Unknown test operation")
      }
    } catch { reply(id, error: error) }
  }

  private func reply(_ id: Int, result: Any = NSNull(), error: Error? = nil) {
    var value: [String: Any] = ["id": id, "result": result]
    if let error { value["error"] = error.localizedDescription }
    if let data = try? JSONSerialization.data(withJSONObject: value, options: [.sortedKeys]), let line = String(data: data, encoding: .utf8) {
      print("NEXUS_UI_HARNESS \(line)")
      fflush(stdout)
    }
  }
}

private struct HarnessError: LocalizedError {
  let message: String
  init(_ message: String) { self.message = message }
  var errorDescription: String? { message }
}
