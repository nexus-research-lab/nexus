using System.Diagnostics;
using System.IO;
using System.IO.Compression;
using System.Text.Json;
using System.Windows;
using Microsoft.Web.WebView2.Core;
using Microsoft.Win32;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Lifecycle;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Bridge;

internal sealed class DesktopBridgeHandler
{
    private readonly CoreWebView2 webView;
    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly Func<string, Task> openRoute;
    private readonly Func<string> updateStarter;

    public DesktopBridgeHandler(
        CoreWebView2 webView,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        Func<string, Task> openRoute,
        Func<string> updateStarter)
    {
        this.webView = webView;
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.openRoute = openRoute;
        this.updateStarter = updateStarter;
    }

    public async Task HandleAsync(JsonElement payload)
    {
        string requestID = payload.TryGetProperty("request_id", out JsonElement requestIDElement)
            ? requestIDElement.GetString() ?? string.Empty
            : string.Empty;
        string kind = payload.TryGetProperty("kind", out JsonElement kindElement)
            ? kindElement.GetString() ?? string.Empty
            : string.Empty;

        try
        {
            object result = kind switch
            {
                "app.get_app_version" => new
                {
                    app_mode = runtime.AppMode,
                    app_version = runtime.AppVersion,
                    build_number = runtime.BuildNumber,
                    platform = runtime.Platform,
                },
                "app.get_state_root" => DesktopStateRootStore.StatusPayload(),
                "app.choose_state_root" => ChooseStateRoot(payload),
                "app.relocate_state_root" => RelocateStateRoot(payload),
                "app.open_external_url" => OpenExternalUrl(payload),
                "app.start_browser_extension_setup" => StartBrowserExtensionSetup(),
                "app.get_workspace_file_applications" => GetWorkspaceFileApplications(payload),
                "app.open_workspace_file" => OpenWorkspaceFile(payload),
                "app.export_logs" => ExportLogs(),
                "app.open_route" => await OpenRouteAsync(payload),
                "app.start_update" => new { status = updateStarter() },
                "app.get_persistent_state" => GetPersistentState(payload),
                "app.set_persistent_state" => SetPersistentState(payload),
                "app.remove_persistent_state" => RemovePersistentState(payload),
                "app.get_global_shortcut_status" => new
                {
                    enabled = false,
                    registered = false,
                    accelerator = "",
                    default_accelerator = "",
                    is_default = false,
                },
                "app.set_global_shortcut_enabled" => new
                {
                    enabled = false,
                    registered = false,
                    accelerator = "",
                    default_accelerator = "",
                    is_default = false,
                },
                "app.set_global_shortcut_accelerator" => new
                {
                    enabled = false,
                    registered = false,
                    accelerator = "",
                    default_accelerator = "",
                    is_default = false,
                },
                "app.reset_global_shortcut_accelerator" => new
                {
                    enabled = false,
                    registered = false,
                    accelerator = "",
                    default_accelerator = "",
                    is_default = false,
                },
                _ => throw new NotSupportedException($"不支持的桌面桥接请求：{kind}"),
            };
            await ResolveAsync(requestID, result);
            if (string.Equals(kind, "app.relocate_state_root", StringComparison.Ordinal))
            {
                _ = System.Windows.Application.Current.Dispatcher.InvokeAsync(
                    () => App.RequestApplicationExit(0));
            }
        }
        catch (Exception exception)
        {
            await RejectAsync(requestID, exception.Message);
        }
    }

    private static object OpenExternalUrl(JsonElement payload)
    {
        string? rawUrl = payload.TryGetProperty("payload", out JsonElement payloadElement)
            && payloadElement.TryGetProperty("url", out JsonElement urlElement)
            ? urlElement.GetString()
            : null;
        if (string.IsNullOrWhiteSpace(rawUrl))
        {
            throw new ArgumentException("外部链接无效。");
        }

        Process.Start(new ProcessStartInfo
        {
            FileName = rawUrl,
            UseShellExecute = true,
        });
        return new { opened = true };
    }

    private object StartBrowserExtensionSetup()
    {
        string[] candidates =
        [
            Path.Combine(runtime.AppRoot, "Nexus Browser Extension"),
            Path.Combine(runtime.AppRoot, "desktop", "browser-extension"),
        ];
        string? extensionDirectory = candidates.FirstOrDefault(
            path => File.Exists(Path.Combine(path, "manifest.json")));
        if (extensionDirectory is null)
        {
            throw new DirectoryNotFoundException("未找到 Nexus 浏览器扩展，请重新安装或更新 Nexus。");
        }

        Process.Start(new ProcessStartInfo
        {
            FileName = "chrome.exe",
            Arguments = "chrome://extensions",
            UseShellExecute = true,
        });
        Process.Start(ExplorerSelection(extensionDirectory));
        return new { opened = true };
    }

    private static object GetWorkspaceFileApplications(JsonElement payload)
    {
        _ = ValidatedWorkspaceFilePath(StringPayload(payload, "path"));
        return new
        {
            default_application = (object?)null,
            applications = Array.Empty<object>(),
        };
    }

    private static object OpenWorkspaceFile(JsonElement payload)
    {
        string filePath = ValidatedWorkspaceFilePath(StringPayload(payload, "path"));
        string target = StringPayload(payload, "target");
        ProcessStartInfo startInfo = target switch
        {
            "default" => new ProcessStartInfo
            {
                FileName = filePath,
                UseShellExecute = true,
            },
            "file_manager" => ExplorerSelection(filePath),
            _ => throw new NotSupportedException("当前平台不支持该文件打开方式。"),
        };
        Process.Start(startInfo);
        return new { opened = true };
    }

    private static ProcessStartInfo ExplorerSelection(string filePath)
    {
        ProcessStartInfo startInfo = new("explorer.exe")
        {
            UseShellExecute = false,
        };
        startInfo.ArgumentList.Add("/select," + filePath);
        return startInfo;
    }

    private static string ValidatedWorkspaceFilePath(string rawPath)
    {
        string filePath = Path.GetFullPath(rawPath.Trim());
        string usersRoot = Path.GetFullPath(DesktopPaths.UsersDirectory)
            .TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar)
            + Path.DirectorySeparatorChar;
        if (!filePath.StartsWith(usersRoot, StringComparison.OrdinalIgnoreCase)
            || !File.Exists(filePath))
        {
            throw new ArgumentException("工作区文件路径无效。");
        }
        return filePath;
    }

    private static object RelocateStateRoot(JsonElement payload)
    {
        string targetPath = DesktopStateRootMigration.ScheduleMigration(StringPayload(payload, "path"));
        return new
        {
            restarting = true,
            target_path = targetPath,
        };
    }

    private static object ChooseStateRoot(JsonElement payload)
    {
        OpenFolderDialog dialog = new()
        {
            Title = StringPayload(payload, "title"),
            InitialDirectory = ExistingDirectory(StringPayload(payload, "initial_path")),
            Multiselect = false,
        };
        bool? accepted = System.Windows.Application.Current?.MainWindow is System.Windows.Window owner
            ? dialog.ShowDialog(owner)
            : dialog.ShowDialog();
        if (accepted == true)
        {
            return new { cancelled = false, path = dialog.FolderName };
        }
        return new { cancelled = true };
    }

    private static string ExistingDirectory(string rawPath)
    {
        string fallback = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        if (string.IsNullOrWhiteSpace(rawPath))
        {
            return fallback;
        }
        try
        {
            string candidate = Path.GetFullPath(rawPath.Trim());
            while (!Directory.Exists(candidate))
            {
                string? parent = Path.GetDirectoryName(candidate);
                if (string.IsNullOrWhiteSpace(parent)
                    || string.Equals(parent, candidate, StringComparison.OrdinalIgnoreCase))
                {
                    return fallback;
                }
                candidate = parent;
            }
            return candidate;
        }
        catch (Exception exception) when (exception is ArgumentException or IOException or NotSupportedException)
        {
            return fallback;
        }
    }

    private object ExportLogs()
    {
        Directory.CreateDirectory(DesktopPaths.LogsDirectory);
        string exportsDirectory = Path.Combine(DesktopPaths.ApplicationDataDirectory, "exports");
        Directory.CreateDirectory(exportsDirectory);

        string fileName = $"nexus-logs-{DateTimeOffset.Now:yyyyMMdd-HHmmss}.zip";
        string zipPath = Path.Combine(exportsDirectory, fileName);
        if (File.Exists(zipPath))
        {
            File.Delete(zipPath);
        }
        using ZipArchive archive = ZipFile.Open(zipPath, ZipArchiveMode.Create);
        AddDirectoryToArchive(archive, DesktopPaths.LogsDirectory, "logs");
        AddDirectoryToArchive(archive, DesktopPaths.DebugDirectory, "debug");
        ZipArchiveEntry runtimeEntry = archive.CreateEntry("desktop-runtime.txt");
        using (StreamWriter writer = new(runtimeEntry.Open()))
        {
            writer.WriteLine($"exported_at={DateTimeOffset.Now:O}");
            writer.WriteLine("platform=windows");
        }
        AddTextEntry(
            archive,
            "diagnostics.json",
            DesktopDiagnosticsReport.Make(runtime, "manual_export", startupTimeline));

        return new { cancelled = false, path = zipPath };
    }

    private static object GetPersistentState(JsonElement payload)
    {
        string key = StringPayload(payload, "key");
        return new { key, value = DesktopPersistentStateStore.Get(key) };
    }

    private static object SetPersistentState(JsonElement payload)
    {
        string key = StringPayload(payload, "key");
        string value = StringPayload(payload, "value");
        DesktopPersistentStateStore.Set(key, value);
        return new { saved = true };
    }

    private static object RemovePersistentState(JsonElement payload)
    {
        string key = StringPayload(payload, "key");
        DesktopPersistentStateStore.Remove(key);
        return new { removed = true };
    }

    private async Task<object> OpenRouteAsync(JsonElement payload)
    {
        string? route = StringPayload(payload, "route");
        if (string.IsNullOrWhiteSpace(route))
        {
            throw new ArgumentException("路由无效。");
        }

        await openRoute(route);
        return new { opened = true };
    }

    private static string StringPayload(JsonElement payload, string name)
    {
        return payload.TryGetProperty("payload", out JsonElement payloadElement)
            && payloadElement.TryGetProperty(name, out JsonElement valueElement)
            ? valueElement.GetString() ?? string.Empty
            : string.Empty;
    }

    private static void AddDirectoryToArchive(ZipArchive archive, string directory, string prefix)
    {
        if (!Directory.Exists(directory))
        {
            return;
        }

        foreach (string file in Directory.EnumerateFiles(directory, "*", SearchOption.AllDirectories))
        {
            string relativePath = Path.GetRelativePath(directory, file).Replace('\\', '/');
            string entryPath = $"{prefix}/{relativePath}";
            try
            {
                AddFileEntry(archive, file, entryPath);
            }
            catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
            {
                // 日志导出不能被单个仍在写入的文件阻断，保留失败说明便于排查。
                AddTextEntry(archive, $"{entryPath}.export-error.txt", exception.Message);
            }
        }
    }

    private static void AddFileEntry(ZipArchive archive, string sourcePath, string entryPath)
    {
        ZipArchiveEntry entry = archive.CreateEntry(entryPath);
        using Stream source = new FileStream(
            sourcePath,
            FileMode.Open,
            FileAccess.Read,
            FileShare.ReadWrite | FileShare.Delete);
        using Stream target = entry.Open();
        source.CopyTo(target);
    }

    private static void AddTextEntry(ZipArchive archive, string path, string content)
    {
        ZipArchiveEntry entry = archive.CreateEntry(path);
        using StreamWriter writer = new(entry.Open());
        writer.WriteLine(content);
    }

    private Task ResolveAsync(string requestID, object payload)
    {
        if (string.IsNullOrWhiteSpace(requestID))
        {
            return Task.CompletedTask;
        }

        string payloadJson = JsonSerializer.Serialize(payload);
        string requestIDJson = JsonSerializer.Serialize(requestID);
        return webView.ExecuteScriptAsync($"window.__NEXUS_DESKTOP_BRIDGE__?.resolve({requestIDJson}, {payloadJson});");
    }

    private Task RejectAsync(string requestID, string message)
    {
        if (string.IsNullOrWhiteSpace(requestID))
        {
            return Task.CompletedTask;
        }

        string requestIDJson = JsonSerializer.Serialize(requestID);
        string messageJson = JsonSerializer.Serialize(message);
        return webView.ExecuteScriptAsync($"window.__NEXUS_DESKTOP_BRIDGE__?.reject({requestIDJson}, {messageJson});");
    }
}
