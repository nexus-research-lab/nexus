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
    private const int StopTimeoutMilliseconds = 3000;
    private const int ReapTimeoutMilliseconds = 3000;

    private readonly DesktopStartupTimeline startupTimeline;
    private readonly SidecarBundle locator;
    private readonly SidecarRuntimeConfig runtime;
    private readonly object outputSync = new();
    private readonly List<string> stdoutTail = [];
    private readonly List<string> stderrTail = [];
    private Process? process;

    public SidecarSupervisor(DesktopStartupTimeline startupTimeline)
    {
        this.startupTimeline = startupTimeline;
        locator = SidecarBundleLocator.Resolve();
        startupTimeline.Mark("sidecar.reap_begin");
        ReapOrphanedSidecars();
        int port = SidecarPortAllocator.Allocate(startupTimeline);
        runtime = new SidecarRuntimeConfig(
            Port: port,
            SessionToken: DesktopSessionToken.Generate(),
            AppVersion: AppVersionInfo.Version,
            BuildNumber: AppVersionInfo.BuildNumber,
            Platform: "windows");
        startupTimeline.Mark("sidecar.config_resolved", new Dictionary<string, string>
        {
            ["mode"] = locator.IsDevelopment ? "development" : "bundle",
            ["port"] = port.ToString(),
        });
    }

    public async Task<SidecarRuntimeConfig> StartAsync()
    {
        startupTimeline.Mark("sidecar.launch_begin");
        ProcessStartInfo startInfo = BuildStartInfo();
        process = Process.Start(startInfo) ?? throw new InvalidOperationException("无法启动 nexus-server。");
        process.OutputDataReceived += (_, args) =>
        {
            if (!string.IsNullOrWhiteSpace(args.Data))
            {
                RecordOutputTail(stdoutTail, args.Data);
                Trace.WriteLine($"[Nexus Sidecar stdout] {args.Data}");
            }
        };
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrWhiteSpace(args.Data))
            {
                RecordOutputTail(stderrTail, args.Data);
                Trace.WriteLine($"[Nexus Sidecar stderr] {args.Data}");
            }
        };
        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
        startupTimeline.Mark("sidecar.process_started", new Dictionary<string, string>
        {
            ["pid"] = process.Id.ToString(),
        });
        WriteProcessRecord(process);

        await WaitUntilHealthyAsync();
        startupTimeline.Mark("sidecar.health_ready");
        return runtime;
    }

    public void Dispose()
    {
        bool removeRecord = true;
        try
        {
            if (process is { HasExited: false })
            {
                startupTimeline.Mark("sidecar.stop_begin", new Dictionary<string, string>
                {
                    ["pid"] = process.Id.ToString(),
                });
                process.Kill(entireProcessTree: true);
                if (process.WaitForExit(StopTimeoutMilliseconds))
                {
                    startupTimeline.Mark("sidecar.stop_finished", new Dictionary<string, string>
                    {
                        ["pid"] = process.Id.ToString(),
                        ["exit_code"] = process.ExitCode.ToString(),
                    });
                }
                else
                {
                    removeRecord = false;
                    startupTimeline.Mark("sidecar.stop_failed", new Dictionary<string, string>
                    {
                        ["pid"] = process.Id.ToString(),
                    });
                }
            }
        }
        catch (InvalidOperationException)
        {
        }
        catch (Exception exception) when (exception is System.ComponentModel.Win32Exception or NotSupportedException)
        {
            removeRecord = false;
        }
        finally
        {
            if (removeRecord)
            {
                RemoveProcessRecord();
            }
            process?.Dispose();
        }
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
        if (record is not null)
        {
            ReapRecordedProcess(record);
        }
        ReapMatchingProcesses();
    }

    private void ReapRecordedProcess(SidecarProcessRecord record)
    {
        if (record.Pid <= 0 || !SamePath(record.ExecutablePath, locator.Command))
        {
            RemoveProcessRecord();
            return;
        }

        try
        {
            using Process orphan = Process.GetProcessById(record.Pid);
            if (!IsExpectedSidecar(orphan))
            {
                RemoveProcessRecord();
                return;
            }

            startupTimeline.Mark("sidecar.reap_recorded", new Dictionary<string, string>
            {
                ["pid"] = orphan.Id.ToString(),
            });
            KillProcessTree(orphan);
        }
        catch (ArgumentException)
        {
            RemoveProcessRecord();
        }
        catch (InvalidOperationException)
        {
            RemoveProcessRecord();
        }
        catch (System.ComponentModel.Win32Exception)
        {
        }
    }

    private void ReapMatchingProcesses()
    {
        if (!Path.IsPathFullyQualified(locator.Command))
        {
            return;
        }

        string processName = Path.GetFileNameWithoutExtension(locator.Command);
        if (string.IsNullOrWhiteSpace(processName))
        {
            return;
        }

        foreach (Process candidate in Process.GetProcessesByName(processName))
        {
            using (candidate)
            {
                if (candidate.Id == Environment.ProcessId || !IsExpectedSidecar(candidate))
                {
                    continue;
                }

                startupTimeline.Mark("sidecar.reap_matching", new Dictionary<string, string>
                {
                    ["pid"] = candidate.Id.ToString(),
                });
                KillProcessTree(candidate);
            }
        }
    }

    private void KillProcessTree(Process target)
    {
        int pid = target.Id;
        if (TryKillProcess(target, true, out string reason))
        {
            RemoveProcessRecord();
            return;
        }

        try
        {
            using Process retry = Process.GetProcessById(pid);
            if (!IsExpectedSidecar(retry))
            {
                RemoveProcessRecord();
                return;
            }

            startupTimeline.Mark("sidecar.reap_retry", new Dictionary<string, string>
            {
                ["pid"] = pid.ToString(),
                ["reason"] = reason,
            });

            if (TryKillProcess(retry, false, out reason))
            {
                RemoveProcessRecord();
                return;
            }
        }
        catch (Exception exception) when (exception is ArgumentException or InvalidOperationException)
        {
            RemoveProcessRecord();
            return;
        }
        catch (System.ComponentModel.Win32Exception exception)
        {
            reason = exception.GetType().Name;
        }

        startupTimeline.Mark(
            reason == "Timeout" ? "sidecar.reap_timeout" : "sidecar.reap_failed",
            new Dictionary<string, string>
            {
                ["pid"] = pid.ToString(),
                ["reason"] = reason,
            });
    }

    private static bool TryKillProcess(Process target, bool entireProcessTree, out string reason)
    {
        try
        {
            target.Kill(entireProcessTree);
            if (target.WaitForExit(ReapTimeoutMilliseconds))
            {
                reason = "";
                return true;
            }
            reason = "Timeout";
        }
        catch (Exception exception) when (exception is InvalidOperationException or
                                           System.ComponentModel.Win32Exception or
                                           NotSupportedException)
        {
            reason = exception.GetType().Name;
        }
        return false;
    }

    private bool IsExpectedSidecar(Process target)
    {
        try
        {
            string? executablePath = target.MainModule?.FileName;
            return SamePath(executablePath, locator.Command);
        }
        catch (Exception exception) when (exception is InvalidOperationException or System.ComponentModel.Win32Exception)
        {
            return false;
        }
    }

    private void WriteProcessRecord(Process startedProcess)
    {
        try
        {
            Directory.CreateDirectory(DesktopPaths.RootDirectory);
            var record = new SidecarProcessRecord(startedProcess.Id, locator.Command);
            File.WriteAllText(DesktopPaths.SidecarPIDFilePath, JsonSerializer.Serialize(record));
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            Trace.WriteLine($"[Nexus Sidecar] failed to write pid record: {exception.Message}");
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
            RemoveProcessRecord();
            Trace.WriteLine($"[Nexus Sidecar] removed invalid pid record: {exception.Message}");
            return null;
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

    private static bool SamePath(string? left, string? right)
    {
        if (string.IsNullOrWhiteSpace(left) || string.IsNullOrWhiteSpace(right))
        {
            return false;
        }
        try
        {
            return string.Equals(
                Path.GetFullPath(left),
                Path.GetFullPath(right),
                StringComparison.OrdinalIgnoreCase);
        }
        catch (Exception exception) when (exception is ArgumentException or NotSupportedException or PathTooLongException)
        {
            return false;
        }
    }

    private async Task WaitUntilHealthyAsync()
    {
        using HttpClient client = new();
        DateTimeOffset deadline = DateTimeOffset.UtcNow.AddSeconds(45);

        while (DateTimeOffset.UtcNow < deadline)
        {
            if (process is { HasExited: true })
            {
                process.WaitForExit();
                startupTimeline.Mark("sidecar.process_exited", ProcessExitMetadata(process));
                throw new InvalidOperationException("nexus-server 在启动完成前退出。");
            }

            try
            {
                using HttpResponseMessage response = await client.GetAsync(runtime.HealthUrl);
                if (response.IsSuccessStatusCode)
                {
                    return;
                }
            }
            catch (HttpRequestException)
            {
                // sidecar 尚未监听端口，继续等待。
            }

            await Task.Delay(300);
        }

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

internal sealed record SidecarProcessRecord(int Pid, string ExecutablePath);
