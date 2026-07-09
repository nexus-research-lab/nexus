using System.IO;
using Microsoft.Web.WebView2.Core;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.WebView;

internal static class DesktopWebCacheInvalidator
{
    private const string LastRuntimeVersionFileName = "last-runtime-version.txt";

    public static async Task ClearCachesIfNeededAsync(
        CoreWebView2 core,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline)
    {
        string currentVersion = $"{runtime.Platform}:{runtime.AppVersion}-{runtime.BuildNumber}";
        string markerPath = Path.Combine(DesktopPaths.CacheDirectory, "WebView2", LastRuntimeVersionFileName);
        string previousVersion = await ReadPreviousVersionAsync(markerPath);
        if (string.Equals(previousVersion, currentVersion, StringComparison.Ordinal))
        {
            startupTimeline.Mark("webview.cache_reuse", new Dictionary<string, string>
            {
                ["version"] = currentVersion,
            });
            return;
        }

        startupTimeline.Mark("webview.cache_clear_begin", new Dictionary<string, string>
        {
            ["current"] = currentVersion,
            ["previous"] = previousVersion,
        });
        try
        {
            await core.Profile.ClearBrowsingDataAsync(
                CoreWebView2BrowsingDataKinds.DiskCache |
                CoreWebView2BrowsingDataKinds.CacheStorage);
            Directory.CreateDirectory(Path.GetDirectoryName(markerPath)!);
            await File.WriteAllTextAsync(markerPath, currentVersion);
            startupTimeline.Mark("webview.cache_clear_finished", new Dictionary<string, string>
            {
                ["current"] = currentVersion,
                ["previous"] = previousVersion,
            });
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("webview.cache_clear_failed", new Dictionary<string, string>
            {
                ["current"] = currentVersion,
                ["error"] = TrimMetadata(exception.Message),
                ["previous"] = previousVersion,
            });
        }
    }

    public static async Task ClearCachesManuallyAsync(
        CoreWebView2 core,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        string reason)
    {
        string currentVersion = $"{runtime.Platform}:{runtime.AppVersion}-{runtime.BuildNumber}";
        startupTimeline.Mark("webview.cache_clear_manual_begin", new Dictionary<string, string>
        {
            ["current"] = currentVersion,
            ["reason"] = reason,
        });
        try
        {
            await core.Profile.ClearBrowsingDataAsync(
                CoreWebView2BrowsingDataKinds.DiskCache |
                CoreWebView2BrowsingDataKinds.CacheStorage);
            startupTimeline.Mark("webview.cache_clear_manual_finished", new Dictionary<string, string>
            {
                ["current"] = currentVersion,
                ["reason"] = reason,
            });
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("webview.cache_clear_manual_failed", new Dictionary<string, string>
            {
                ["current"] = currentVersion,
                ["error"] = TrimMetadata(exception.Message),
                ["reason"] = reason,
            });
        }
    }

    private static async Task<string> ReadPreviousVersionAsync(string markerPath)
    {
        try
        {
            if (!File.Exists(markerPath))
            {
                return string.Empty;
            }
            return (await File.ReadAllTextAsync(markerPath)).Trim();
        }
        catch
        {
            return string.Empty;
        }
    }

    private static string TrimMetadata(string value)
    {
        string normalized = value.Trim();
        const int maxLength = 240;
        return normalized.Length <= maxLength ? normalized : normalized[..maxLength] + "...";
    }
}
