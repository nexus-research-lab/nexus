using System.Diagnostics;
using System.IO;
using System.Net.Http;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;

namespace Nexus.Desktop.Sidecar;

internal sealed class SidecarSupervisor : IDisposable
{
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly SidecarBundle locator;
    private readonly SidecarRuntimeConfig runtime;
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
                Trace.WriteLine($"[Nexus Sidecar stdout] {args.Data}");
            }
        };
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrWhiteSpace(args.Data))
            {
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
        startInfo.Environment["NEXUS_CONFIG_DIR"] = DesktopPaths.ConfigDirectory;
        startInfo.Environment["HOST"] = "127.0.0.1";
        startInfo.Environment["PORT"] = runtime.Port.ToString();
        startInfo.Environment["NEXUS_DESKTOP_SESSION_TOKEN"] = runtime.SessionToken;
        startInfo.Environment["WEB_DIST_DIR"] = locator.WebDistDirectory;
        startInfo.Environment["DATABASE_DRIVER"] = "sqlite";
        startInfo.Environment["DATABASE_URL"] = Path.Combine(DesktopPaths.ApplicationDataDirectory, "nexus.db");
        startInfo.Environment["CONNECTOR_CREDENTIALS_KEY"] = credentialsKey.Value;
        startInfo.Environment["WORKSPACE_PATH"] = DesktopPaths.WorkspaceDirectory;
        startInfo.Environment["CACHE_FILE_DIR"] = DesktopPaths.CacheDirectory;
        startInfo.Environment["LOG_PATH"] = Path.Combine(DesktopPaths.LogsDirectory, "sidecar.log");
        startInfo.Environment["LOG_STDOUT"] = "true";
        startInfo.Environment["LOG_FILE_ENABLED"] = "true";
        startInfo.Environment["DISCORD_ENABLED"] = "false";
        startInfo.Environment["TELEGRAM_ENABLED"] = "false";
        startInfo.Environment["CONNECTOR_OAUTH_REDIRECT_URI"] = "nexus://connectors/oauth/callback";
        startInfo.Environment["CONNECTOR_OAUTH_ALLOWED_ORIGINS"] = $"{runtime.WebBaseUrl.TrimEnd('/')},nexus://connectors";
        return startInfo;
    }

    private static void PrepareDirectories()
    {
        Directory.CreateDirectory(DesktopPaths.ApplicationDataDirectory);
        Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
        Directory.CreateDirectory(DesktopPaths.WorkspaceDirectory);
        Directory.CreateDirectory(DesktopPaths.CacheDirectory);
        Directory.CreateDirectory(DesktopPaths.LogsDirectory);
    }

    private async Task WaitUntilHealthyAsync()
    {
        using HttpClient client = new();
        DateTimeOffset deadline = DateTimeOffset.UtcNow.AddSeconds(45);

        while (DateTimeOffset.UtcNow < deadline)
        {
            if (process is { HasExited: true })
            {
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

        throw new TimeoutException("等待 nexus-server 健康检查超时。");
    }
}
