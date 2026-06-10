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
      startupTimeline?.mark("web.lifecycle_rejected", metadata: metadata)
      return
    }
    guard let record = message.body as? [String: Any],
          let kind = (record["kind"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines),
          !kind.isEmpty else {
      startupTimeline?.mark("web.lifecycle_ignored", metadata: [
        "body_type": String(describing: type(of: message.body)),
        "surface": surfaceName,
      ])
      return
    }

    switch kind {
    case "web.ready":
      startupTimeline?.mark("web.ready", metadata: readyMetadata(record: record))
      Task { @MainActor in
        onWebReady()
      }
    case "web.fatal":
      handleFatalLifecycle(record: record)
    case "web.health":
      handleHealthLifecycle(record: record)
    default:
      startupTimeline?.mark("web.lifecycle_ignored", metadata: [
        "kind": trimMetadata(kind),
        "surface": surfaceName,
      ])
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

  private func handleFatalLifecycle(record: [String: Any]) {
    var metadata = lifecycleMetadata(record: record)
    let reason = metadata["message"] ?? "web.fatal"
    if let diagnosticsURL = DesktopDiagnosticsReport.writeRuntimeIssue(
      prefix: "web-fatal",
      reason: reason,
      runtime: runtime,
      startupTimeline: startupTimeline,
      details: metadata
    ) {
      metadata["diagnostics_path"] = diagnosticsURL.path
    }
    startupTimeline?.mark("web.fatal", metadata: metadata)
  }

  private func handleHealthLifecycle(record: [String: Any]) {
    let metadata = lifecycleMetadata(record: record)
    let status = metadata["status"] ?? "unknown"
    let event = status == "ready" ? "web.health" : "web.health_unhealthy"
    startupTimeline?.mark(event, metadata: metadata)
  }

  private func lifecycleMetadata(record: [String: Any]) -> [String: String] {
    var metadata: [String: String] = ["surface": surfaceName]
    for key in ["kind", "source", "status", "message", "name", "stack", "component_stack"] {
      if let value = record[key] {
        metadata[key] = trimMetadata(stringValue(value))
      }
    }
    if let location = record["location"] as? String {
      for (key, value) in sanitizedLocationMetadata(location) {
        metadata[key] = value
      }
    }
    if let snapshot = record["snapshot"] as? [String: Any] {
      for key in ["path", "ready_state", "title", "has_root", "root_children", "body_children", "body_text_length"] {
        if let value = snapshot[key] {
          metadata["snapshot_\(key)"] = trimMetadata(stringValue(value))
        }
      }
    }
    if let performance = record["performance"] as? [String: Any],
       let readyMS = performance["ready_ms"] {
      metadata["web_ready_ms"] = stringValue(readyMS)
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

  private func trimMetadata(_ value: String) -> String {
    let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
    let maxLength = 240
    if normalized.count <= maxLength {
      return normalized
    }
    let endIndex = normalized.index(normalized.startIndex, offsetBy: maxLength)
    return String(normalized[..<endIndex]) + "..."
  }
}
