import Foundation
import WebKit

final class DesktopLifecycleHandler: NSObject, WKScriptMessageHandler {
  private let runtime: SidecarRuntimeConfig
  private let surfaceName: String
  private let startupTimeline: DesktopStartupTimeline?
  private let onWebReady: @MainActor () -> Void

  init(
    runtime: SidecarRuntimeConfig,
    surfaceName: String,
    startupTimeline: DesktopStartupTimeline?,
    onWebReady: @escaping @MainActor () -> Void
  ) {
    self.runtime = runtime
    self.surfaceName = surfaceName
    self.startupTimeline = startupTimeline
    self.onWebReady = onWebReady
  }

  func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
    if let reason = DesktopWebOriginPolicy.rejectionReason(message: message, runtime: runtime) {
      var metadata = DesktopWebOriginPolicy.metadata(message: message, runtime: runtime)
      metadata["reason"] = reason
      metadata["surface"] = surfaceName
      startupTimeline?.mark("web.ready_rejected", metadata: metadata)
      return
    }
    guard let record = message.body as? [String: Any],
          (record["kind"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) == "web.ready" else {
      startupTimeline?.mark("web.lifecycle_ignored", metadata: [
        "body_type": String(describing: type(of: message.body)),
        "surface": surfaceName,
      ])
      return
    }
    startupTimeline?.mark("web.ready", metadata: readyMetadata(record: record))
    Task { @MainActor in
      onWebReady()
    }
  }

  private func readyMetadata(record: [String: Any]) -> [String: String] {
    var metadata: [String: String] = ["surface": surfaceName]
    if let source = record["source"] as? String {
      metadata["source"] = source.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? "unknown" : source
    }
    if let location = record["location"] as? String {
      for (key, value) in sanitizedLocationMetadata(location) {
        metadata[key] = value
      }
    }
    if let performance = record["performance"] as? [String: Any] {
      for key in [
        "ready_ms",
        "response_end_ms",
        "dom_content_loaded_ms",
        "load_event_end_ms",
        "first_contentful_paint_ms",
      ] {
        if let value = performance[key] {
          metadata["web_\(key)"] = stringValue(value)
        }
      }
    }
    return metadata
  }

  private func stringValue(_ value: Any) -> String {
    if let number = value as? NSNumber {
      return String(format: "%.1f", number.doubleValue)
    }
    return "\(value)"
  }

  private func sanitizedLocationMetadata(_ location: String) -> [String: String] {
    guard let components = URLComponents(string: location) else {
      return ["location_path": location]
    }
    var metadata: [String: String] = [
      "location_path": components.path.isEmpty ? "/" : components.path,
    ]
    if let queryItems = components.queryItems, !queryItems.isEmpty {
      let keys = queryItems.map(\.name).filter { !$0.isEmpty }
      metadata["location_query_keys"] = keys.isEmpty ? "unknown" : Array(Set(keys)).sorted().joined(separator: ",")
    }
    return metadata
  }
}
