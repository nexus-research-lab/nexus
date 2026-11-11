using System.IO;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Update;

internal static class DesktopUpdateCacheCleaner
{
    private const string LastCleanupVersionFileName = "last-update-cache-cleanup-version.txt";

    public static async Task ClearStaleCachesIfNeededAsync(
        DesktopAppVersion currentVersion,
        DesktopStartupTimeline startupTimeline)
    {
        string versionMarker = $"{currentVersion.Version}-{currentVersion.BuildNumber}";
        string markerPath = Path.Combine(DesktopPaths.ConfigDirectory, LastCleanupVersionFileName);
        string previousMarker = await ReadMarkerAsync(markerPath);
        if (string.Equals(previousMarker, versionMarker, StringComparison.Ordinal))
        {
            startupTimeline.Mark("update_cache.cleanup_reuse", new Dictionary<string, string>
            {
                ["version"] = versionMarker,
            });
            return;
        }

        string updatesDirectory = Path.Combine(DesktopPaths.CacheDirectory, "updates");
        startupTimeline.Mark("update_cache.cleanup_begin", new Dictionary<string, string>
        {
            ["current"] = versionMarker,
            ["previous"] = previousMarker,
        });

        CleanupResult result;
        try
        {
            result = await Task.Run(() => RemoveEntries(updatesDirectory));
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("update_cache.cleanup_failed", new Dictionary<string, string>
            {
                ["current"] = versionMarker,
                ["error"] = exception.Message,
            });
            return;
        }
        if (result.FailedEntries > 0)
        {
            startupTimeline.Mark("update_cache.cleanup_failed", new Dictionary<string, string>
            {
                ["current"] = versionMarker,
                ["failed_entries"] = result.FailedEntries.ToString(),
                ["removed_entries"] = result.RemovedEntries.ToString(),
            });
            return;
        }

        try
        {
            Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
            await File.WriteAllTextAsync(markerPath, versionMarker);
            startupTimeline.Mark("update_cache.cleanup_finished", new Dictionary<string, string>
            {
                ["current"] = versionMarker,
                ["previous"] = previousMarker,
                ["removed_entries"] = result.RemovedEntries.ToString(),
            });
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            startupTimeline.Mark("update_cache.cleanup_failed", new Dictionary<string, string>
            {
                ["current"] = versionMarker,
                ["error"] = exception.Message,
                ["removed_entries"] = result.RemovedEntries.ToString(),
            });
        }
    }

    private static async Task<string> ReadMarkerAsync(string markerPath)
    {
        try
        {
            if (!File.Exists(markerPath))
            {
                return string.Empty;
            }

            return (await File.ReadAllTextAsync(markerPath)).Trim();
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            return string.Empty;
        }
    }

    private static CleanupResult RemoveEntries(string updatesDirectory)
    {
        if (!Directory.Exists(updatesDirectory))
        {
            return new CleanupResult();
        }

        string[] entries;
        try
        {
            entries = Directory.GetFileSystemEntries(updatesDirectory);
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            return new CleanupResult(FailedEntries: 1);
        }

        int removedEntries = 0;
        int failedEntries = 0;
        foreach (string entry in entries)
        {
            try
            {
                if (Directory.Exists(entry))
                {
                    Directory.Delete(entry, recursive: true);
                    removedEntries++;
                }
                else if (File.Exists(entry))
                {
                    File.Delete(entry);
                    removedEntries++;
                }
            }
            catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
            {
                failedEntries++;
            }
        }

        if (failedEntries == 0 && Directory.Exists(updatesDirectory))
        {
            try
            {
                Directory.Delete(updatesDirectory);
            }
            catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
            {
                failedEntries++;
            }
        }

        return new CleanupResult(removedEntries, failedEntries);
    }

    private readonly record struct CleanupResult(int RemovedEntries = 0, int FailedEntries = 0);
}
