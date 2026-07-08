import Foundation
import WebKit

enum DesktopWebCacheInvalidator {
  private static let lastRuntimeVersionKey = "com.leemysw.nexus.desktop.lastWebRuntimeVersion"

  @MainActor
  static func clearCachesIfNeeded(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline
  ) async {
    let currentVersion = "\(runtime.platform):\(runtime.appVersion)-\(runtime.buildNumber)"
    let defaults = UserDefaults.standard
    let previousVersion = defaults.string(forKey: lastRuntimeVersionKey)
    guard previousVersion != currentVersion else {
      startupTimeline.mark("webview.cache_reuse", metadata: ["version": currentVersion])
      return
    }

    startupTimeline.mark("webview.cache_clear_begin", metadata: [
      "current": currentVersion,
      "previous": previousVersion ?? "",
    ])
    await clearCacheData()
    defaults.set(currentVersion, forKey: lastRuntimeVersionKey)
    startupTimeline.mark("webview.cache_clear_finished", metadata: [
      "current": currentVersion,
      "previous": previousVersion ?? "",
    ])
  }

  @MainActor
  static func clearCachesManually(startupTimeline: DesktopStartupTimeline) async {
    startupTimeline.mark("webview.cache_clear_manual_begin")
    await clearCacheData()
    startupTimeline.mark("webview.cache_clear_manual_finished")
  }

  @MainActor
  private static func clearCacheData() async {
    let dataTypes = Set(WKWebsiteDataStore.allWebsiteDataTypes().filter { dataType in
      dataType.range(of: "cache", options: [.caseInsensitive]) != nil
    })
    guard !dataTypes.isEmpty else {
      return
    }

    await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
      WKWebsiteDataStore.default().removeData(
        ofTypes: dataTypes,
        modifiedSince: .distantPast
      ) {
        continuation.resume()
      }
    }
  }
}
