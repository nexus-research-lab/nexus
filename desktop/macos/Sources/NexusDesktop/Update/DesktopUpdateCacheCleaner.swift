import Foundation

enum DesktopUpdateCacheCleaner {
  private static let lastCleanupVersionKey = "com.leemysw.nexus.desktop.lastUpdateCacheCleanupVersion"

  @MainActor
  static func clearStaleCachesIfNeeded(
    currentVersion: DesktopAppVersion,
    startupTimeline: DesktopStartupTimeline,
    defaults: UserDefaults = .standard
  ) async {
    let versionMarker = "\(currentVersion.version)-\(currentVersion.buildNumber)"
    let previousMarker = defaults.string(forKey: lastCleanupVersionKey)
    guard previousMarker != versionMarker else {
      startupTimeline.mark("update_cache.cleanup_reuse", metadata: [
        "version": versionMarker,
      ])
      return
    }

    let updatesDirectory = DesktopPaths.cacheDirectory
      .appendingPathComponent("updates", isDirectory: true)
    startupTimeline.mark("update_cache.cleanup_begin", metadata: [
      "current": versionMarker,
      "previous": previousMarker ?? "",
    ])

    let result = await Task.detached(priority: .utility) {
      Self.removeEntries(at: updatesDirectory)
    }.value
    guard result.failedEntries == 0 else {
      startupTimeline.mark("update_cache.cleanup_failed", metadata: [
        "current": versionMarker,
        "failed_entries": String(result.failedEntries),
        "removed_entries": String(result.removedEntries),
      ])
      return
    }

    defaults.set(versionMarker, forKey: lastCleanupVersionKey)
    startupTimeline.mark("update_cache.cleanup_finished", metadata: [
      "current": versionMarker,
      "previous": previousMarker ?? "",
      "removed_entries": String(result.removedEntries),
    ])
  }

  private static func removeEntries(at updatesDirectory: URL) -> CleanupResult {
    let fileManager = FileManager.default
    guard fileManager.fileExists(atPath: updatesDirectory.path) else {
      return CleanupResult()
    }

    let entries: [URL]
    do {
      entries = try fileManager.contentsOfDirectory(
        at: updatesDirectory,
        includingPropertiesForKeys: nil,
        options: []
      )
    } catch {
      return CleanupResult(failedEntries: 1)
    }

    var removedEntries = 0
    var failedEntries = 0
    for entry in entries {
      do {
        try fileManager.removeItem(at: entry)
        removedEntries += 1
      } catch {
        failedEntries += 1
      }
    }

    if failedEntries == 0, fileManager.fileExists(atPath: updatesDirectory.path) {
      do {
        try fileManager.removeItem(at: updatesDirectory)
      } catch {
        failedEntries += 1
      }
    }

    return CleanupResult(
      removedEntries: removedEntries,
      failedEntries: failedEntries
    )
  }
}

private extension DesktopUpdateCacheCleaner {
  struct CleanupResult: Sendable {
    let removedEntries: Int
    let failedEntries: Int

    init(removedEntries: Int = 0, failedEntries: Int = 0) {
      self.removedEntries = removedEntries
      self.failedEntries = failedEntries
    }
  }
}
