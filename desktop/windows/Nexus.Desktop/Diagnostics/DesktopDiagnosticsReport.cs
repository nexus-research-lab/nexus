using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text.Json;
using Microsoft.Win32;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Diagnostics;

internal static class DesktopDiagnosticsReport
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
    };

    public static string Make(
        SidecarRuntimeConfig? runtime,
        string? reason = null,
        DesktopStartupTimeline? startupTimeline = null,
        IReadOnlyDictionary<string, object?>? details = null)
    {
        var payload = new Dictionary<string, object?>
        {
            ["generated_at"] = DateTimeOffset.UtcNow.ToString("O"),
            ["app"] = AppPayload(),
            ["process"] = ProcessPayload(),
            ["system"] = SystemPayload(),
            ["paths"] = PathsPayload(),
            ["checks"] = ChecksPayload(),
        };

        if (runtime is not null)
        {
            payload["runtime"] = RuntimePayload(runtime);
        }
        if (!string.IsNullOrWhiteSpace(reason))
        {
            payload["reason"] = reason;
        }
        if (startupTimeline is not null)
        {
            payload["startup_timeline"] = startupTimeline.Snapshot().Select(TimelinePayload).ToList();
        }
        if (details is not null && details.Count > 0)
        {
            payload["details"] = details;
        }

        return JsonSerializer.Serialize(payload, JsonOptions);
    }

    public static string? WriteStartupFailure(Exception exception, DesktopStartupTimeline? startupTimeline = null)
    {
        return WriteReport(
            prefix: "startup-failure",
            reason: exception.Message,
            runtime: null,
            startupTimeline: startupTimeline,
            details: new Dictionary<string, object?>
            {
                ["exception_type"] = exception.GetType().FullName,
            });
    }

    public static string? WriteRuntimeIssue(
        string prefix,
        string reason,
        SidecarRuntimeConfig? runtime,
        DesktopStartupTimeline? startupTimeline = null,
        IReadOnlyDictionary<string, object?>? details = null)
    {
        return WriteReport(prefix, reason, runtime, startupTimeline, details);
    }

    private static string? WriteReport(
        string prefix,
        string reason,
        SidecarRuntimeConfig? runtime,
        DesktopStartupTimeline? startupTimeline,
        IReadOnlyDictionary<string, object?>? details)
    {
        try
        {
            Directory.CreateDirectory(DesktopPaths.LogsDirectory);
            string filePath = Path.Combine(DesktopPaths.LogsDirectory, $"{SanitizeFilePrefix(prefix)}-{DateTimeOffset.Now:yyyyMMdd-HHmmss}.json");
            string text = Make(runtime, reason, startupTimeline, details);
            File.WriteAllText(filePath, text);
            return filePath;
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            Trace.WriteLine($"[Nexus Diagnostics] failed to write diagnostics report: {exception.Message}");
            return null;
        }
    }

    private static Dictionary<string, object?> AppPayload()
    {
        return new Dictionary<string, object?>
        {
            ["name"] = "Nexus",
            ["version"] = AppVersionInfo.Version,
            ["build_number"] = AppVersionInfo.BuildNumber,
            ["base_directory"] = AppContext.BaseDirectory,
            ["executable_path"] = Environment.ProcessPath ?? string.Empty,
        };
    }

    private static Dictionary<string, object?> ProcessPayload()
    {
        using Process process = Process.GetCurrentProcess();
        return new Dictionary<string, object?>
        {
            ["pid"] = process.Id,
            ["process_name"] = process.ProcessName,
            ["current_directory"] = Environment.CurrentDirectory,
            ["user_interactive"] = Environment.UserInteractive,
        };
    }

    private static Dictionary<string, object?> SystemPayload()
    {
        return new Dictionary<string, object?>
        {
            ["platform"] = "windows",
            ["os_version"] = RuntimeInformation.OSDescription,
            ["architecture"] = RuntimeInformation.OSArchitecture.ToString(),
            ["process_architecture"] = RuntimeInformation.ProcessArchitecture.ToString(),
            ["machine_name"] = Environment.MachineName,
            ["processor_count"] = Environment.ProcessorCount,
            ["is_64bit_process"] = Environment.Is64BitProcess,
        };
    }

    private static Dictionary<string, object?> RuntimePayload(SidecarRuntimeConfig runtime)
    {
        return new Dictionary<string, object?>
        {
            ["app_mode"] = runtime.AppMode,
            ["app_version"] = runtime.AppVersion,
            ["build_number"] = runtime.BuildNumber,
            ["platform"] = runtime.Platform,
            ["web_url"] = runtime.WebBaseUrl,
            ["api_base_url"] = runtime.ApiBaseUrl,
            ["websocket_url"] = runtime.WebSocketUrl,
            ["health_url"] = runtime.HealthUrl,
        };
    }

    private static Dictionary<string, object?> PathsPayload()
    {
        return new Dictionary<string, object?>
        {
            ["application_data_dir"] = DesktopPaths.ApplicationDataDirectory,
            ["config_root"] = DesktopPaths.RootDirectory,
            ["config_dir"] = DesktopPaths.ConfigDirectory,
            ["transcript_projects_dir"] = DesktopPaths.ProjectsDirectory,
            ["workspace_dir"] = DesktopPaths.WorkspaceDirectory,
            ["cache_dir"] = DesktopPaths.CacheDirectory,
            ["logs_dir"] = DesktopPaths.LogsDirectory,
            ["debug_dir"] = DesktopPaths.DebugDirectory,
            ["connector_credentials_dpapi"] = Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.dpapi"),
            ["connector_credentials_fallback_key"] = Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.key"),
        };
    }

    private static Dictionary<string, object?> ChecksPayload()
    {
        string resourceDirectory = Path.Combine(AppContext.BaseDirectory, "Resources");
        return new Dictionary<string, object?>
        {
            ["application_data_exists"] = Directory.Exists(DesktopPaths.ApplicationDataDirectory),
            ["logs_dir_exists"] = Directory.Exists(DesktopPaths.LogsDirectory),
            ["debug_dir_exists"] = Directory.Exists(DesktopPaths.DebugDirectory),
            ["transcript_projects_dir_exists"] = Directory.Exists(DesktopPaths.ProjectsDirectory),
            ["bundled_web_app_exists"] = File.Exists(Path.Combine(resourceDirectory, "Web", "app.html")),
            ["bundled_web_settings_exists"] = File.Exists(Path.Combine(resourceDirectory, "Web", "settings.html")),
            ["bundled_web_oauth_callback_exists"] = File.Exists(Path.Combine(resourceDirectory, "Web", "oauth-callback.html")),
            ["bundled_sidecar_exists"] = File.Exists(Path.Combine(resourceDirectory, "nexus-server.exe")),
            ["bundled_nexusctl_exists"] = File.Exists(Path.Combine(resourceDirectory, "bin", "nexusctl.exe")),
            ["bundled_nxs_exists"] = File.Exists(Path.Combine(resourceDirectory, "bin", "nxs.exe")),
            ["connector_credentials_dpapi_exists"] = File.Exists(Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.dpapi")),
            ["connector_credentials_fallback_key_exists"] = File.Exists(Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.key")),
            ["nexus_url_scheme_registered"] = NexusUrlSchemeRegistered(),
        };
    }

    private static Dictionary<string, object?> TimelinePayload(DesktopStartupEvent item)
    {
        return new Dictionary<string, object?>
        {
            ["name"] = item.Name,
            ["elapsed_ms"] = item.ElapsedMilliseconds,
            ["delta_ms"] = item.DeltaMilliseconds,
            ["metadata"] = item.Metadata,
        };
    }

    private static bool NexusUrlSchemeRegistered()
    {
        try
        {
            using RegistryKey? key = Registry.CurrentUser.OpenSubKey(@"Software\Classes\nexus");
            return key?.GetValue("URL Protocol") is not null;
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            return false;
        }
    }

    private static string SanitizeFilePrefix(string prefix)
    {
        char[] chars = prefix
            .Trim()
            .Select(item => char.IsLetterOrDigit(item) || item is '-' or '_' ? item : '-')
            .ToArray();
        string value = new string(chars).Trim('-', '_');
        return string.IsNullOrWhiteSpace(value) ? "runtime-issue" : value;
    }
}
