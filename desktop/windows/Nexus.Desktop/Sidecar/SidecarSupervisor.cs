// INPUT: Trusted desktop runtime configuration and durable sidecar identity.
// OUTPUT: Job-owned backend startup, bounded asynchronous shutdown and conservative recovery.
// POS: Windows sidecar lifecycle; native process primitives live in SidecarProcessLifetime.
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;

namespace Nexus.Desktop.Sidecar;

internal sealed class SidecarSupervisor : IDisposable
{
    private const int OutputTailLineLimit = 200;
    private const int StopTimeoutMilliseconds = 5000;
    private const int ReapTimeoutMilliseconds = 3000;

    private readonly DesktopStartupTimeline startupTimeline;
    private readonly SidecarBundle locator;
    private readonly SidecarRuntimeConfig runtime;
    private readonly object outputSync = new();
    private readonly List<string> stdoutTail = [];
    private readonly List<string> stderrTail = [];
    private Process? process;
    private SidecarProcessLifetime? lifetime;
    private Task? stopTask;
    private Task? outputTask;

    public SidecarSupervisor(DesktopStartupTimeline startupTimeline, CancellationToken cancellationToken = default)
    {
        this.startupTimeline = startupTimeline;
        locator = SidecarBundleLocator.Resolve();
        startupTimeline.Mark("sidecar.reap_begin");
        ReapOrphanedSidecars();
        int port = SidecarPortAllocator.Allocate(startupTimeline, cancellationToken);
        runtime = new SidecarRuntimeConfig(
            Port: port,
            SessionToken: DesktopSessionToken.Generate(),
            AppVersion: AppVersionInfo.Version,
            BuildNumber: AppVersionInfo.BuildNumber,
            Platform: "windows",
            AppRoot: locator.AppRoot);
        startupTimeline.Mark("sidecar.config_resolved", new Dictionary<string, string>
        {
            ["mode"] = locator.IsDevelopment ? "development" : "bundle",
            ["port"] = port.ToString(),
        });
    }

    public async Task<SidecarRuntimeConfig> StartAsync(CancellationToken cancellationToken = default)
    {
        startupTimeline.Mark("sidecar.launch_begin");
        ProcessStartInfo startInfo = BuildStartInfo();
        cancellationToken.ThrowIfCancellationRequested();
        lifetime = SidecarProcessLifetime.Start(startInfo);
        process = lifetime.Process;
        outputTask = lifetime.CaptureOutputAsync(
            line => RecordOutputTail(stdoutTail, line),
            line => RecordOutputTail(stderrTail, line));
        startupTimeline.Mark("sidecar.process_started", new Dictionary<string, string>
        {
            ["pid"] = process.Id.ToString(),
        });
        WriteProcessRecord(process);

        await WaitUntilHealthyAsync(cancellationToken);
        startupTimeline.Mark("sidecar.health_ready");
        return runtime;
    }

    public Task StopAsync() => stopTask ??= StopOwnedProcessAsync();

    private async Task StopOwnedProcessAsync()
    {
        if (lifetime is null) return;
        startupTimeline.Mark("sidecar.stop_begin", new Dictionary<string, string> { ["pid"] = lifetime.Process.Id.ToString() });
        try
        {
            await lifetime.StopAsync(TimeSpan.FromMilliseconds(StopTimeoutMilliseconds));
            RemoveProcessRecord();
            startupTimeline.Mark("sidecar.stop_finished");
        }
        catch (Exception exception)
        {
            // Preserve the identity receipt until termination can be confirmed on the next launch.
            startupTimeline.Mark("sidecar.stop_failed", new Dictionary<string, string> { ["error"] = exception.Message });
            throw;
        }
        finally
        {
            Dispose();
        }
    }

    public void Dispose()
    {
        lifetime?.Dispose();
        lifetime = null;
    }

    private ProcessStartInfo BuildStartInfo()
    {
        PrepareDirectories();
        DesktopCredentialsKey credentialsKey = DesktopCredentialsKeyStore.ConnectorCredentialsKey();
        startupTimeline.Mark("sidecar.credentials_key_ready", new Dictionary<string, string>
        {
            ["storage"] = credentialsKey.Storage,
            ["reason"] = credentialsKey.Reason,
        });

        var startInfo = new ProcessStartInfo
        {
            FileName = locator.Command,
            Arguments = locator.Arguments,
            WorkingDirectory = locator.WorkingDirectory,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
            CreateNoWindow = true,
        };

        startInfo.Environment["NEXUS_APP_MODE"] = "desktop";
        startInfo.Environment["NEXUS_APP_ROOT"] = locator.AppRoot;
        startInfo.Environment["NEXUS_STATE_ROOT"] = DesktopPaths.RootDirectory;
        if (DesktopStateRootStore.PreviousRootDirectory is string previousRoot)
        {
            startInfo.Environment["NEXUS_PREVIOUS_STATE_ROOT"] = previousRoot;
        }
        else
        {
            startInfo.Environment.Remove("NEXUS_PREVIOUS_STATE_ROOT");
        }
        startInfo.Environment["NEXUS_CONFIG_DIR"] = DesktopPaths.RootDirectory;
        startInfo.Environment["CLAUDE_CONFIG_DIR"] = DesktopPaths.RootDirectory;
        startInfo.Environment["HOST"] = "127.0.0.1";
        startInfo.Environment["PORT"] = runtime.Port.ToString();
        startInfo.Environment["NEXUS_DESKTOP_SESSION_TOKEN"] = runtime.SessionToken;
        startInfo.Environment["WEB_DIST_DIR"] = locator.WebDistDirectory;
        startInfo.Environment["DATABASE_DRIVER"] = "sqlite";
        startInfo.Environment["DATABASE_URL"] = Path.Combine(DesktopPaths.DataDirectory, "nexus.db");
        startInfo.Environment["CONNECTOR_CREDENTIALS_KEY"] = credentialsKey.Value;
        startInfo.Environment["WORKSPACE_PATH"] = DesktopPaths.WorkspaceDirectory;
        startInfo.Environment["CACHE_FILE_DIR"] = DesktopPaths.CacheDirectory;
        startInfo.Environment["LOG_PATH"] = Path.Combine(DesktopPaths.LogsDirectory, "sidecar.log");
        startInfo.Environment["LOG_STDOUT"] = "true";
        startInfo.Environment["LOG_FILE_ENABLED"] = "true";
        startInfo.Environment["DISCORD_ENABLED"] = "false";
        startInfo.Environment["TELEGRAM_ENABLED"] = "false";
        startInfo.Environment["CONNECTOR_OAUTH_REDIRECT_URI"] = runtime.OAuthRedirectUri;
        ApplyPackagedConnectorConfig(startInfo);
        ApplyBundledControlCommands(startInfo);
        ApplyBundledNXSRuntime(startInfo);
        startInfo.Environment["CONNECTOR_OAUTH_ALLOWED_ORIGINS"] = runtime.WebBaseUrl.TrimEnd('/');
        return startInfo;
    }

    private void ApplyPackagedConnectorConfig(ProcessStartInfo startInfo)
    {
        string configPath = Path.Combine(locator.AppRoot, "desktop.env");
        if (!File.Exists(configPath))
        {
            return;
        }

        foreach (string rawLine in File.ReadLines(configPath))
        {
            string line = rawLine.Trim();
            if (string.IsNullOrWhiteSpace(line) || line.StartsWith("#", StringComparison.Ordinal))
            {
                continue;
            }

            int separator = line.IndexOf('=');
            if (separator <= 0)
            {
                continue;
            }

            string key = line[..separator].Trim().TrimStart('\uFEFF');
            string value = line[(separator + 1)..].Trim();
            if (string.Equals(key, "CONNECTOR_GITHUB_CLIENT_ID", StringComparison.Ordinal) && !string.IsNullOrWhiteSpace(value))
            {
                startInfo.Environment[key] = value;
            }
        }
    }

    private void ApplyBundledControlCommands(ProcessStartInfo startInfo)
    {
        foreach ((string name, string environmentKey) in new[]
        {
            ("nexusctl", "NEXUSCTL_COMMAND_PATH"),
            ("nexuscfg", "NEXUSCFG_COMMAND_PATH"),
        })
        {
            string timelineKey = $"sidecar.{name}_command";
            if (startInfo.Environment.TryGetValue(environmentKey, out string? overridePath) &&
                !string.IsNullOrWhiteSpace(overridePath))
            {
                startupTimeline.Mark(timelineKey, new Dictionary<string, string>
                {
                    ["source"] = "override",
                    ["path"] = overridePath,
                });
                continue;
            }
            if (locator.IsDevelopment)
            {
                startupTimeline.Mark(timelineKey, new Dictionary<string, string>
                {
                    ["source"] = "development",
                });
                continue;
            }
            string commandPath = Path.Combine(locator.AppRoot, "bin", $"{name}.exe");
            if (File.Exists(commandPath))
            {
                startInfo.Environment[environmentKey] = commandPath;
                startupTimeline.Mark(timelineKey, new Dictionary<string, string>
                {
                    ["source"] = "bundled",
                    ["path"] = commandPath,
                });
                continue;
            }
            startupTimeline.Mark(timelineKey, new Dictionary<string, string>
            {
                ["source"] = "missing",
            });
        }
    }

    private void ApplyBundledNXSRuntime(ProcessStartInfo startInfo)
    {
        if (locator.IsDevelopment)
        {
            startupTimeline.Mark("sidecar.nxs_runtime", new Dictionary<string, string>
            {
                ["source"] = "development",
            });
            return;
        }

        if (startInfo.Environment.TryGetValue("NEXUS_NXS_COMMAND_PATH", out string? overridePath) &&
            !string.IsNullOrWhiteSpace(overridePath))
        {
            startupTimeline.Mark("sidecar.nxs_runtime", new Dictionary<string, string>
            {
                ["source"] = "override",
                ["path"] = overridePath,
            });
            return;
        }

        string nxsPath = Path.Combine(locator.AppRoot, "bin", "nxs.exe");
        if (File.Exists(nxsPath))
        {
            startInfo.Environment["NEXUS_NXS_COMMAND_PATH"] = nxsPath;
            startupTimeline.Mark("sidecar.nxs_runtime", new Dictionary<string, string>
            {
                ["source"] = "bundled",
                ["path"] = nxsPath,
            });
            return;
        }

        startupTimeline.Mark("sidecar.nxs_runtime", new Dictionary<string, string>
        {
            ["source"] = "missing",
        });
    }


    private static void PrepareDirectories()
    {
        Directory.CreateDirectory(DesktopPaths.RootDirectory);
        Directory.CreateDirectory(DesktopPaths.ApplicationDataDirectory);
        Directory.CreateDirectory(DesktopPaths.UsersDirectory);
        Directory.CreateDirectory(DesktopPaths.SystemRuntimeDirectory);
        Directory.CreateDirectory(DesktopPaths.SystemRuntimeHomeDirectory);
        Directory.CreateDirectory(DesktopPaths.SystemRuntimeCacheDirectory);
        Directory.CreateDirectory(DesktopPaths.SystemRuntimeLogsDirectory);
        Directory.CreateDirectory(DesktopPaths.SystemRuntimeTempDirectory);
        Directory.CreateDirectory(DesktopPaths.DataDirectory);
        Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
        Directory.CreateDirectory(DesktopPaths.WorkspaceDirectory);
        Directory.CreateDirectory(DesktopPaths.ProjectsDirectory);
        Directory.CreateDirectory(DesktopPaths.CacheDirectory);
        Directory.CreateDirectory(DesktopPaths.LogsDirectory);
        Directory.CreateDirectory(DesktopPaths.DebugDirectory);
    }

    private void ReapOrphanedSidecars()
    {
        SidecarProcessRecord? record = ReadProcessRecord();
        if (record is null) return;
        if (record.Pid <= 0) throw new InvalidOperationException("旧后端进程记录无效，无法安全回收。");
        Process orphan;
        try { orphan = Process.GetProcessById(record.Pid); }
        catch (ArgumentException) { RemoveProcessRecord(); return; }
        using (orphan)
        {
            // Hold the process handle before checking identity; PID reuse must never select another process.
            _ = orphan.Handle;
            if (orphan.HasExited) { RemoveProcessRecord(); return; }
            if (record.StartTimeUtcTicks == 0 || !SidecarProcessRecord.SamePath(record.StateRoot, DesktopPaths.RootDirectory))
                throw new InvalidOperationException("旧后端的进程身份记录不完整，请退出旧版本 Nexus 后重试；未自动结束不明进程。");
            if (orphan.StartTime.ToUniversalTime().Ticks != record.StartTimeUtcTicks)
            {
                RemoveProcessRecord(); // The recorded process is gone; this PID now belongs to someone else.
                return;
            }
            if (!record.Matches(orphan.Id, orphan.StartTime.ToUniversalTime().Ticks, orphan.MainModule?.FileName, DesktopPaths.RootDirectory))
                throw new InvalidOperationException("旧后端的可执行路径与身份记录不符，无法安全回收。");
            startupTimeline.Mark("sidecar.reap_recorded", new Dictionary<string, string> { ["pid"] = orphan.Id.ToString() });
            // The held process handle identifies the old backend even when the new package lives elsewhere.
            // Its own bridge Jobs close with it. Never enumerate and kill unrelated same-name processes.
            orphan.Kill();
            if (!orphan.WaitForExit(ReapTimeoutMilliseconds))
                throw new TimeoutException("旧 Nexus 后端未在回收期限内退出，请检查 shell.log 后重试。");
            RemoveProcessRecord();
        }
    }

    private void WriteProcessRecord(Process startedProcess)
    {
        try
        {
            Directory.CreateDirectory(DesktopPaths.RootDirectory);
            var record = new SidecarProcessRecord(startedProcess.Id, startedProcess.MainModule?.FileName ?? locator.Command, startedProcess.StartTime.ToUniversalTime().Ticks, DesktopPaths.RootDirectory);
            File.WriteAllText(DesktopPaths.SidecarPIDFilePath, JsonSerializer.Serialize(record));
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            throw new InvalidOperationException("无法保存后端身份记录，停止启动以避免无法恢复的残留。", exception);
        }
    }

    private static SidecarProcessRecord? ReadProcessRecord()
    {
        try
        {
            if (!File.Exists(DesktopPaths.SidecarPIDFilePath))
            {
                return null;
            }
            string text = File.ReadAllText(DesktopPaths.SidecarPIDFilePath);
            return JsonSerializer.Deserialize<SidecarProcessRecord>(text);
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            throw new InvalidOperationException("无法读取旧后端身份记录；已保留原记录，请检查后重试。", exception);
        }
    }

    private static void RemoveProcessRecord()
    {
        try
        {
            if (File.Exists(DesktopPaths.SidecarPIDFilePath))
            {
                File.Delete(DesktopPaths.SidecarPIDFilePath);
            }
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            Trace.WriteLine($"[Nexus Sidecar] failed to remove pid record: {exception.Message}");
        }
    }

    private async Task WaitUntilHealthyAsync(CancellationToken cancellationToken)
    {
        using HttpClient client = new() { Timeout = TimeSpan.FromSeconds(2) };
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        deadline.CancelAfter(TimeSpan.FromSeconds(45));

        while (!deadline.IsCancellationRequested)
        {
            if (process is { HasExited: true })
            {
                if (outputTask is not null)
                {
                    try { await outputTask.WaitAsync(TimeSpan.FromMilliseconds(250), cancellationToken); }
                    catch (TimeoutException) { /* A descendant may still own a pipe; shutdown will close the Job. */ }
                }
                startupTimeline.Mark("sidecar.process_exited", ProcessExitMetadata(process));
                throw new InvalidOperationException("nexus-server 在启动完成前退出。");
            }

            try
            {
                using HttpResponseMessage response = await client.GetAsync(runtime.HealthUrl, deadline.Token);
                if (response.IsSuccessStatusCode)
                {
                    return;
                }
            }
            catch (HttpRequestException)
            {
                // sidecar 尚未监听端口，继续等待。
            }

            catch (OperationCanceledException)
            {
                cancellationToken.ThrowIfCancellationRequested();
                if (deadline.IsCancellationRequested) break;
            }

            try { await Task.Delay(300, deadline.Token); }
            catch (OperationCanceledException) { cancellationToken.ThrowIfCancellationRequested(); break; }
        }

        cancellationToken.ThrowIfCancellationRequested();
        startupTimeline.Mark("sidecar.health_timeout", OutputMetadata());
        throw new TimeoutException("等待 nexus-server 健康检查超时。");
    }

    private Dictionary<string, string> ProcessExitMetadata(Process exitedProcess)
    {
        Dictionary<string, string> metadata = new()
        {
            ["exit_code"] = exitedProcess.ExitCode.ToString(),
        };
        foreach (KeyValuePair<string, string> entry in OutputMetadata())
        {
            metadata[entry.Key] = entry.Value;
        }
        return metadata;
    }

    private Dictionary<string, string> OutputMetadata()
    {
        Dictionary<string, string> metadata = new();
        string stdout = OutputTail(stdoutTail);
        if (!string.IsNullOrWhiteSpace(stdout))
        {
            metadata["stdout_tail"] = stdout;
        }
        string stderr = OutputTail(stderrTail);
        if (!string.IsNullOrWhiteSpace(stderr))
        {
            metadata["stderr_tail"] = stderr;
        }
        return metadata;
    }

    private void RecordOutputTail(List<string> target, string line)
    {
        lock (outputSync)
        {
            target.Add(line);
            if (target.Count > OutputTailLineLimit)
            {
                target.RemoveAt(0);
            }
        }
    }

    private string OutputTail(List<string> target)
    {
        lock (outputSync)
        {
            return string.Join("\n", target);
        }
    }
}
