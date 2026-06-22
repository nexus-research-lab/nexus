using System.Diagnostics;
using System.IO;
using System.Net.Http;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;

namespace Nexus.Desktop.Sidecar;

internal sealed class SidecarSupervisor : IDisposable
{
    private const int OutputTailLineLimit = 200;

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
        int port = SidecarPortAllocator.Allocate();
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

        await WaitUntilHealthyAsync();
        startupTimeline.Mark("sidecar.health_ready");
        return runtime;
    }

    public void Dispose()
    {
        if (process is { HasExited: false })
        {
            process.Kill(entireProcessTree: true);
            process.WaitForExit(3000);
        }
        process?.Dispose();
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
            CreateNoWindow = true,
        };

        startInfo.Environment["NEXUS_APP_MODE"] = "desktop";
        startInfo.Environment["NEXUS_APP_ROOT"] = locator.AppRoot;
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
        ApplyBundledNexusctlCommand(startInfo);
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

    private void ApplyBundledNexusctlCommand(ProcessStartInfo startInfo)
    {
        if (startInfo.Environment.TryGetValue("NEXUSCTL_COMMAND_PATH", out string? overridePath) &&
            !string.IsNullOrWhiteSpace(overridePath))
        {
            startupTimeline.Mark("sidecar.nexusctl_command", new Dictionary<string, string>
            {
                ["source"] = "override",
                ["path"] = overridePath,
            });
            return;
        }

        if (locator.IsDevelopment)
        {
            startupTimeline.Mark("sidecar.nexusctl_command", new Dictionary<string, string>
            {
                ["source"] = "development",
            });
            return;
        }

        string nexusctlPath = Path.Combine(locator.AppRoot, "bin", "nexusctl.exe");
        if (File.Exists(nexusctlPath))
        {
            startInfo.Environment["NEXUSCTL_COMMAND_PATH"] = nexusctlPath;
            startupTimeline.Mark("sidecar.nexusctl_command", new Dictionary<string, string>
            {
                ["source"] = "bundled",
                ["path"] = nexusctlPath,
            });
            return;
        }

        startupTimeline.Mark("sidecar.nexusctl_command", new Dictionary<string, string>
        {
            ["source"] = "missing",
        });
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
        Directory.CreateDirectory(DesktopPaths.ApplicationDataDirectory);
        Directory.CreateDirectory(DesktopPaths.DataDirectory);
        Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
        Directory.CreateDirectory(DesktopPaths.WorkspaceDirectory);
        Directory.CreateDirectory(DesktopPaths.ProjectsDirectory);
        Directory.CreateDirectory(DesktopPaths.CacheDirectory);
        Directory.CreateDirectory(DesktopPaths.LogsDirectory);
        Directory.CreateDirectory(DesktopPaths.DebugDirectory);
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
