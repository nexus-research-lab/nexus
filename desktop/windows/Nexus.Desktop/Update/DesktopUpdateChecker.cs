// INPUT: Update-check reasons, release metadata, download progress, and verified package facts.
// OUTPUT: Stage-specific update behavior and safe native feedback with raw causes kept in diagnostics.
// POS: Windows update coordinator; check, download/verification, and installer launch stay distinct.

using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text.Json;
using System.Windows;
using Nexus.Desktop.Bridge;
using Nexus.Desktop.Dialog;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Update;

internal sealed class DesktopUpdateChecker
{
    private static readonly TimeSpan AutomaticCheckInterval = TimeSpan.FromHours(4);
    private static readonly TimeSpan MetadataRequestTimeout = TimeSpan.FromSeconds(15);
    private static readonly TimeSpan DownloadRequestTimeout = TimeSpan.FromMinutes(10);
    private const int ReleaseNotesMaxCharacters = 20_000;
    private static readonly Uri LatestReleaseUrl = new("https://api.github.com/repos/nexus-research-lab/nexus/releases/latest");
    private static readonly Uri FallbackReleasePageUrl = new("https://github.com/nexus-research-lab/nexus/releases/latest");
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        WriteIndented = true,
    };
    private static readonly IReadOnlyDictionary<bool, IReadOnlyList<NexusDialogAction>>
        UpdatePromptActions = new Dictionary<bool, IReadOnlyList<NexusDialogAction>>
        {
            [true] =
            [
                new NexusDialogAction(
                    UpdatePromptAction.Later.ToString(),
                    "稍后",
                    IsCancel: true),
                new NexusDialogAction(
                    UpdatePromptAction.OpenReleasePage.ToString(),
                    "打开下载页"),
                new NexusDialogAction(
                    UpdatePromptAction.DownloadAndInstall.ToString(),
                    "下载并更新",
                    NexusDialogActionTone.Primary,
                    IsDefault: true),
            ],
            [false] =
            [
                new NexusDialogAction(
                    UpdatePromptAction.Later.ToString(),
                    "稍后",
                    IsCancel: true),
                new NexusDialogAction(
                    UpdatePromptAction.OpenReleasePage.ToString(),
                    "打开下载页",
                    NexusDialogActionTone.Primary,
                    IsDefault: true),
            ],
        };

    private readonly DesktopStartupTimeline startupTimeline;
    private readonly HttpClient httpClient;
    private readonly DesktopAppVersion currentVersion;
    private readonly string statePath;
    private readonly bool isDisabled;
    private readonly SemaphoreSlim checkLock = new(1, 1);
    private bool hasPerformedStartupCheck;
    private CancellationTokenSource? automaticCheckCancellation;
    private DesktopReleaseInfo? availableRelease;
    private int updateInProgress;

    public DesktopUpdateChecker(DesktopStartupTimeline startupTimeline, HttpClient? httpClient = null)
    {
        this.startupTimeline = startupTimeline;
        this.httpClient = httpClient ?? new HttpClient();
        currentVersion = DesktopAppVersion.Current();
        statePath = Path.Combine(DesktopPaths.ConfigDirectory, "update-check.json");
        isDisabled = string.Equals(
            Environment.GetEnvironmentVariable("NEXUS_DESKTOP_DISABLE_UPDATE_CHECK"),
            "1",
            StringComparison.Ordinal);
    }

    public void CheckOnLaunchIfNeeded(System.Windows.Window owner)
    {
        if (isDisabled)
        {
            startupTimeline.Mark("update_check.skipped", new Dictionary<string, string>
            {
                ["reason"] = "disabled",
            });
            return;
        }

        if (hasPerformedStartupCheck)
        {
            return;
        }
        hasPerformedStartupCheck = true;

        _ = RunStartupCheckAsync(owner);
        StartAutomaticChecks(owner);
    }

    public async Task<DesktopUpdateCheckResult> CheckNowAsync(System.Windows.Window owner)
    {
        if (isDisabled)
        {
            startupTimeline.Mark("update_check.skipped", new Dictionary<string, string>
            {
                ["reason"] = "disabled",
                ["source"] = "manual",
            });
            return DesktopUpdateCheckResult.Disabled(currentVersion);
        }

        return await CheckWithLockAsync(owner, "manual", showsUpToDateAlert: true);
    }

    public string StartAvailableUpdate(System.Windows.Window owner)
    {
        return StartAvailableUpdate(owner, preferredRelease: null, source: "sidebar");
    }

    private string StartAvailableUpdate(
        System.Windows.Window owner,
        DesktopReleaseInfo? preferredRelease,
        string source)
    {
        if (isDisabled)
        {
            return "disabled";
        }
        if (Interlocked.CompareExchange(ref updateInProgress, 1, 0) != 0)
        {
            return "in_progress";
        }

        startupTimeline.Mark("update_check.update_requested", new Dictionary<string, string>
        {
            ["source"] = source,
        });
        _ = RunAvailableUpdateAsync(owner, preferredRelease);
        return "started";
    }

    public Task ClearStaleUpdateCacheIfNeededAsync() =>
        DesktopUpdateCacheCleaner.ClearStaleCachesIfNeededAsync(currentVersion, startupTimeline);

    private async Task RunStartupCheckAsync(System.Windows.Window owner)
    {
        await CheckWithLockAsync(owner, "startup", showsUpToDateAlert: false);
    }

    private void StartAutomaticChecks(System.Windows.Window owner)
    {
        automaticCheckCancellation?.Cancel();
        automaticCheckCancellation = new CancellationTokenSource();
        CancellationToken cancellationToken = automaticCheckCancellation.Token;
        _ = RunAutomaticChecksAsync(owner, cancellationToken);
    }

    private async Task RunAutomaticChecksAsync(
        System.Windows.Window owner,
        CancellationToken cancellationToken)
    {
        using PeriodicTimer timer = new(AutomaticCheckInterval);
        try
        {
            while (await timer.WaitForNextTickAsync(cancellationToken))
            {
                if (owner.Dispatcher.HasShutdownStarted)
                {
                    return;
                }
                await CheckWithLockAsync(owner, "periodic", showsUpToDateAlert: false);
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            // 应用退出或重新装配检测器时，定时检查自然结束。
        }
    }

    private async Task<DesktopUpdateCheckResult> CheckWithLockAsync(
        System.Windows.Window owner,
        string reason,
        bool showsUpToDateAlert)
    {
        await checkLock.WaitAsync();
        try
        {
            return await PerformCheckAsync(owner, reason, showsUpToDateAlert);
        }
        finally
        {
            checkLock.Release();
        }
    }

    private async Task<DesktopUpdateCheckResult> PerformCheckAsync(
        System.Windows.Window owner,
        string reason,
        bool showsUpToDateAlert)
    {
        startupTimeline.Mark("update_check.started", new Dictionary<string, string>
        {
            ["reason"] = reason,
            ["current_version"] = currentVersion.Version,
            ["current_build"] = currentVersion.BuildNumber,
        });

        try
        {
            DesktopReleaseInfo latest = await FetchLatestReleaseAsync();
            bool hasUpdate = latest.IsNewerThan(currentVersion);
            availableRelease = hasUpdate ? latest : null;
            UpdateCheckState previousState = LoadState();
            SaveState(new UpdateCheckState
            {
                LastAutomaticCheckAt = reason is "startup" or "periodic"
                    ? DateTimeOffset.UtcNow
                    : previousState.LastAutomaticCheckAt,
                LastResult = hasUpdate ? "update_available" : "up_to_date",
                LastLatestVersion = latest.Version,
                LastLatestBuildNumber = latest.BuildNumber,
                LastErrorMessage = null,
            });

            startupTimeline.Mark("update_check.result", new Dictionary<string, string>
            {
                ["reason"] = reason,
                ["status"] = hasUpdate ? "update_available" : "up_to_date",
                ["current_version"] = currentVersion.Version,
                ["current_build"] = currentVersion.BuildNumber,
                ["latest_version"] = latest.Version,
                ["latest_build"] = latest.BuildNumber ?? string.Empty,
                ["source"] = latest.Source,
                ["installer_asset"] = latest.InstallerFileName ?? string.Empty,
                ["sha256_asset"] = latest.InstallerSha256FileName ?? string.Empty,
            });

            if (hasUpdate)
            {
                DesktopPersistentStateStore.Set("desktop.update.available", latest.Version);
                if (reason != "periodic")
                {
                    await ShowUpdateAvailableAsync(owner, latest);
                }
            }
            else if (showsUpToDateAlert)
            {
                DesktopPersistentStateStore.Remove("desktop.update.available");
                await ShowUpToDateAsync(owner, latest);
            }
            else
            {
                DesktopPersistentStateStore.Remove("desktop.update.available");
            }

            return DesktopUpdateCheckResult.From(currentVersion, latest, hasUpdate);
        }
        catch (Exception exception)
        {
            UpdateCheckState previousState = LoadState();
            SaveState(new UpdateCheckState
            {
                LastAutomaticCheckAt = previousState.LastAutomaticCheckAt,
                LastResult = "failed",
                LastErrorMessage = DesktopFailureCopy.UpdateCheckMessage,
            });
            startupTimeline.Mark("update_check.failed", new Dictionary<string, string>
            {
                ["reason"] = reason,
                ["error"] = exception.Message,
            });
            if (showsUpToDateAlert)
            {
                await ShowCheckFailedAsync(owner);
            }

            return DesktopUpdateCheckResult.Failed(
                currentVersion,
                DesktopFailureCopy.UpdateCheckMessage);
        }
    }

    private async Task<DesktopReleaseInfo> FetchLatestReleaseAsync()
    {
        GitHubRelease release = await FetchJsonAsync<GitHubRelease>(LatestReleaseUrl);
        GitHubReleaseAsset? metadataAsset = FindWindowsMetadataAsset(release.Assets);
        GitHubReleaseAsset? installerAsset = FindWindowsInstallerAsset(release.Assets);
        GitHubReleaseAsset? installerSha256Asset = FindWindowsInstallerSha256Asset(release.Assets, installerAsset);

        if (metadataAsset?.BrowserDownloadUrl is not null)
        {
            try
            {
                DesktopPackageMetadata metadata = await FetchJsonAsync<DesktopPackageMetadata>(metadataAsset.BrowserDownloadUrl);
                return new DesktopReleaseInfo(
                    metadata.Version,
                    metadata.BuildNumber,
                    release.Name,
                    release.HtmlUrl ?? FallbackReleasePageUrl,
                    installerAsset?.Name,
                    installerAsset?.BrowserDownloadUrl,
                    installerSha256Asset?.Name,
                    installerSha256Asset?.BrowserDownloadUrl,
                    release.Body,
                    release.PublishedAt,
                    release.Prerelease,
                    "github_release_metadata");
            }
            catch (Exception exception)
            {
                startupTimeline.Mark("update_check.metadata_failed", new Dictionary<string, string>
                {
                    ["error"] = exception.Message,
                });
            }
        }

        return new DesktopReleaseInfo(
            GitHubReleaseVersionNormalizer.VersionFrom(release.TagName),
            null,
            release.Name,
            release.HtmlUrl ?? FallbackReleasePageUrl,
            installerAsset?.Name,
            installerAsset?.BrowserDownloadUrl,
            installerSha256Asset?.Name,
            installerSha256Asset?.BrowserDownloadUrl,
            release.Body,
            release.PublishedAt,
            release.Prerelease,
            "github_release");
    }

    private async Task RunAvailableUpdateAsync(
        System.Windows.Window owner,
        DesktopReleaseInfo? preferredRelease)
    {
        try
        {
            DesktopReleaseInfo latest = await ResolveAvailableReleaseAsync(preferredRelease);
            if (!latest.IsNewerThan(currentVersion))
            {
                availableRelease = null;
                DesktopPersistentStateStore.Remove("desktop.update.available");
                await ShowUpToDateAsync(owner, latest);
                return;
            }

            availableRelease = latest;
            DesktopPersistentStateStore.Set("desktop.update.available", latest.Version);
            await DownloadAndOfferInstallAsync(owner, latest);
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("update_check.update_request_failed", new Dictionary<string, string>
            {
                ["error"] = exception.Message,
            });
            await ShowCheckFailedAsync(owner);
        }
        finally
        {
            Interlocked.Exchange(ref updateInProgress, 0);
        }
    }

    private async Task<DesktopReleaseInfo> ResolveAvailableReleaseAsync(
        DesktopReleaseInfo? preferredRelease)
    {
        if (preferredRelease is not null)
        {
            return preferredRelease;
        }
        if (availableRelease is not null)
        {
            return availableRelease;
        }

        await checkLock.WaitAsync();
        try
        {
            return availableRelease ?? await FetchLatestReleaseAsync();
        }
        finally
        {
            checkLock.Release();
        }
    }

    private async Task<T> FetchJsonAsync<T>(Uri url)
    {
        using HttpRequestMessage request = CreateGitHubRequest(HttpMethod.Get, url);
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/vnd.github+json"));

        using CancellationTokenSource timeout = new(MetadataRequestTimeout);
        using HttpResponseMessage response = await httpClient.SendAsync(request, timeout.Token);
        if (!response.IsSuccessStatusCode)
        {
            throw new InvalidOperationException($"更新服务返回 HTTP {(int)response.StatusCode}。");
        }

        await using Stream stream = await response.Content.ReadAsStreamAsync(timeout.Token);
        T? payload = await JsonSerializer.DeserializeAsync<T>(stream, JsonOptions, timeout.Token);
        return payload ?? throw new InvalidOperationException("更新服务返回了无效响应。");
    }

    private async Task ShowUpdateAvailableAsync(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        UpdatePromptAction action = await owner.Dispatcher.InvokeAsync(() => PromptForUpdate(owner, latest));
        var handlers = new Dictionary<UpdatePromptAction, Func<Task>>
        {
            [UpdatePromptAction.DownloadAndInstall] = () =>
            {
                _ = StartAvailableUpdate(owner, latest, "prompt");
                return Task.CompletedTask;
            },
            [UpdatePromptAction.OpenReleasePage] = () =>
            {
                OpenReleasePage(latest, "prompt");
                return Task.CompletedTask;
            },
            [UpdatePromptAction.Later] = static () => Task.CompletedTask,
        };
        await handlers[action]();
    }

    private async Task ShowUpToDateAsync(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        await owner.Dispatcher.InvokeAsync(() => NexusDialogWindow.ShowMessage(
            owner,
            "Nexus 已是最新版本",
            string.Join(
                Environment.NewLine,
                $"当前版本：{currentVersion.DisplayText}",
                $"最新版本：{latest.DisplayText}")));
    }

    private async Task ShowCheckFailedAsync(System.Windows.Window owner)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        await owner.Dispatcher.InvokeAsync(() => NexusDialogWindow.ShowMessage(
            owner,
            DesktopFailureCopy.UpdateCheckTitle,
            DesktopFailureCopy.UpdateCheckMessage));
    }

    private UpdatePromptAction PromptForUpdate(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        startupTimeline.Mark("update_check.prompt_shown", new Dictionary<string, string>
        {
            ["latest_version"] = latest.Version,
            ["latest_build"] = latest.BuildNumber ?? string.Empty,
            ["can_download_installer"] = latest.CanDownloadInstaller.ToString(),
        });

        string? releaseNotes = FormatReleaseNotes(latest.ReleaseNotes);
        string? actionID = NexusDialogWindow.Show(owner, new NexusDialogOptions(
            "发现 Nexus 新版本",
            UpdateAvailableMessage(latest, releaseNotes),
            UpdatePromptActions[latest.CanDownloadInstaller],
            DetailsTitle: releaseNotes is null ? null : "更新内容",
            Details: releaseNotes is null ? null : DesktopReleaseNotesRenderer.Render(releaseNotes),
            ContentWidth: 640,
            BodyMaxHeight: 500));
        return Enum.TryParse(actionID, out UpdatePromptAction action)
            ? action
            : UpdatePromptAction.Later;
    }

    private async Task DownloadAndOfferInstallAsync(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        if (!latest.CanDownloadInstaller)
        {
            startupTimeline.Mark("update_check.download_unavailable", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["has_installer"] = (latest.InstallerDownloadUrl is not null).ToString(),
                ["has_sha256"] = (latest.InstallerSha256Url is not null).ToString(),
            });
            await ShowManualDownloadOnlyAsync(owner, latest);
            return;
        }

        startupTimeline.Mark("update_check.download_started", new Dictionary<string, string>
        {
            ["latest_version"] = latest.Version,
            ["latest_build"] = latest.BuildNumber ?? string.Empty,
            ["installer_asset"] = latest.InstallerFileName ?? string.Empty,
        });

        try
        {
            DesktopDownloadProgressWindow progressWindow = await owner.Dispatcher.InvokeAsync(() =>
            {
                var window = new DesktopDownloadProgressWindow(owner, latest);
                window.Show();
                return window;
            });
            DownloadedUpdate downloadedUpdate;
            try
            {
                downloadedUpdate = await DownloadAndVerifyUpdateAsync(latest, progressWindow.Report);
            }
            finally
            {
                await owner.Dispatcher.InvokeAsync(progressWindow.Close);
            }
            startupTimeline.Mark("update_check.download_verified", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["installer_asset"] = latest.InstallerFileName ?? string.Empty,
                ["sha256"] = downloadedUpdate.Sha256Hash,
            });
            await PromptInstallAsync(owner, latest, downloadedUpdate);
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("update_check.download_failed", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["error"] = exception.Message,
            });
            await ShowDownloadFailedAsync(owner, latest);
        }
    }

    private async Task<DownloadedUpdate> DownloadAndVerifyUpdateAsync(
        DesktopReleaseInfo latest,
        Action<long, long?> reportProgress)
    {
        string installerFileName = latest.InstallerFileName
            ?? throw new InvalidOperationException("当前 Release 缺少 Windows 安装器文件名。");
        Uri installerUrl = latest.InstallerDownloadUrl
            ?? throw new InvalidOperationException("当前 Release 缺少 Windows 安装器下载地址。");
        Uri sha256Url = latest.InstallerSha256Url
            ?? throw new InvalidOperationException("当前 Release 缺少 Windows 安装器 sha256 文件。");

        string updateDir = Path.Combine(
            DesktopPaths.CacheDirectory,
            "updates",
            SafePathSegment($"{latest.Version}-{latest.BuildNumber ?? "unknown"}"));
        Directory.CreateDirectory(updateDir);

        string installerPath = Path.Combine(updateDir, SafePathSegment(installerFileName));
        string sha256FileName = string.IsNullOrWhiteSpace(latest.InstallerSha256FileName)
            ? $"{installerFileName}.sha256"
            : latest.InstallerSha256FileName;
        string sha256Path = Path.Combine(updateDir, SafePathSegment(sha256FileName));

        await DownloadFileAsync(installerUrl, installerPath, reportProgress);
        await DownloadFileAsync(sha256Url, sha256Path, static (_, _) => { });

        string expectedHash = ReadExpectedSha256(sha256Path, installerFileName);
        string actualHash = ComputeSha256(installerPath);
        if (!string.Equals(expectedHash, actualHash, StringComparison.OrdinalIgnoreCase))
        {
            TryDeleteFile(installerPath);
            throw new InvalidOperationException("下载的安装器 sha256 校验未通过，已丢弃本地文件。");
        }

        return new DownloadedUpdate(installerPath, sha256Path, actualHash.ToLowerInvariant());
    }

    private async Task DownloadFileAsync(
        Uri url,
        string destinationPath,
        Action<long, long?> reportProgress)
    {
        string temporaryPath = $"{destinationPath}.download";
        TryDeleteFile(temporaryPath);

        using HttpRequestMessage request = CreateGitHubRequest(HttpMethod.Get, url);
        using CancellationTokenSource timeout = new(DownloadRequestTimeout);
        using HttpResponseMessage response = await httpClient.SendAsync(
            request,
            HttpCompletionOption.ResponseHeadersRead,
            timeout.Token);
        if (!response.IsSuccessStatusCode)
        {
            throw new InvalidOperationException($"更新文件下载失败，HTTP {(int)response.StatusCode}。");
        }

        Directory.CreateDirectory(Path.GetDirectoryName(destinationPath)!);
        await using (Stream source = await response.Content.ReadAsStreamAsync(timeout.Token))
        await using (FileStream destination = File.Create(temporaryPath))
        {
            long receivedBytes = 0;
            long? totalBytes = response.Content.Headers.ContentLength;
            byte[] buffer = new byte[64 * 1024];
            int read;
            while ((read = await source.ReadAsync(buffer, timeout.Token)) > 0)
            {
                await destination.WriteAsync(buffer.AsMemory(0, read), timeout.Token);
                receivedBytes += read;
                reportProgress(receivedBytes, totalBytes);
            }
        }

        File.Move(temporaryPath, destinationPath, overwrite: true);
    }

    private async Task PromptInstallAsync(
        System.Windows.Window owner,
        DesktopReleaseInfo latest,
        DownloadedUpdate downloadedUpdate)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        bool shouldInstall = await owner.Dispatcher.InvokeAsync(() =>
        {
            startupTimeline.Mark("update_check.install_prompt_shown", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["latest_build"] = latest.BuildNumber ?? string.Empty,
                ["installer_path"] = downloadedUpdate.InstallerPath,
            });

            return NexusDialogWindow.Confirm(
                owner,
                $"Nexus {latest.Version} 更新已就绪",
                "更新已完成安全校验。继续后应用会退出，由安装程序完成更新。",
                "立即更新",
                "稍后");
        });

        if (!shouldInstall)
        {
            return;
        }

        Process.Start(new ProcessStartInfo
        {
            FileName = downloadedUpdate.InstallerPath,
            WorkingDirectory = Path.GetDirectoryName(downloadedUpdate.InstallerPath)!,
            UseShellExecute = true,
        });
        startupTimeline.Mark("update_check.installer_started", new Dictionary<string, string>
        {
            ["latest_version"] = latest.Version,
            ["installer_path"] = downloadedUpdate.InstallerPath,
        });

        if (!owner.Dispatcher.HasShutdownStarted)
        {
            await owner.Dispatcher.InvokeAsync(() => App.RequestApplicationExit(0));
        }
    }

    private async Task ShowManualDownloadOnlyAsync(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        bool shouldOpenRelease = await owner.Dispatcher.InvokeAsync(() => NexusDialogWindow.Confirm(
            owner,
            DesktopFailureCopy.AutomaticUpdateUnavailableTitle,
            DesktopFailureCopy.AutomaticUpdateUnavailableMessage,
            "打开下载页"));
        if (shouldOpenRelease)
        {
            OpenReleasePage(latest, "download_unavailable");
        }
    }

    private async Task ShowDownloadFailedAsync(
        System.Windows.Window owner,
        DesktopReleaseInfo latest)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        bool shouldOpenRelease = await owner.Dispatcher.InvokeAsync(() => NexusDialogWindow.Confirm(
            owner,
            DesktopFailureCopy.UpdateIncompleteTitle,
            DesktopFailureCopy.UpdateIncompleteMessage,
            "打开下载页"));
        if (shouldOpenRelease)
        {
            OpenReleasePage(latest, "download_failed");
        }
    }

    private void OpenReleasePage(DesktopReleaseInfo latest, string reason)
    {
        startupTimeline.Mark("update_check.release_page_opened", new Dictionary<string, string>
        {
            ["latest_version"] = latest.Version,
            ["reason"] = reason,
        });
        Process.Start(new ProcessStartInfo
        {
            FileName = latest.ReleasePageUrl.ToString(),
            UseShellExecute = true,
        });
    }

    private string UpdateAvailableMessage(DesktopReleaseInfo latest, string? releaseNotes)
    {
        var lines = new List<string>
        {
            $"当前版本：{currentVersion.DisplayText}",
            $"最新版本：{latest.DisplayText}",
        };
        if (!string.IsNullOrWhiteSpace(latest.PublishedAt))
        {
            lines.Add($"发布时间：{latest.PublishedAt}");
        }
        if (latest.IsPrerelease)
        {
            lines.Add("这是一个预发布版本。");
        }
        if (!string.IsNullOrWhiteSpace(releaseNotes))
        {
            lines.Add("更新内容显示在下方，完整内容可打开官方下载页查看。");
        }

        lines.Add(string.Empty);
        if (latest.CanDownloadInstaller)
        {
            lines.Add("选择“下载并更新”后，Nexus 会先验证更新包安全性，再询问是否退出并安装。");
            lines.Add("也可以打开官方下载页手动安装，或稍后再处理。");
        }
        else
        {
            lines.Add("这个版本暂时不能通过应用内更新。可以打开官方下载页手动安装。");
        }
        return string.Join(Environment.NewLine, lines);
    }

    private static string? FormatReleaseNotes(string? rawNotes)
    {
        if (string.IsNullOrWhiteSpace(rawNotes))
        {
            return null;
        }

        string normalized = rawNotes
            .Replace("\r\n", "\n", StringComparison.Ordinal)
            .Replace('\r', '\n')
            .Trim();
        if (normalized.Length == 0)
        {
            return null;
        }

        if (normalized.Length <= ReleaseNotesMaxCharacters)
        {
            return normalized;
        }

        string clipped = normalized[..ReleaseNotesMaxCharacters].TrimEnd();
        return $"{clipped}{Environment.NewLine}{Environment.NewLine}...{Environment.NewLine}完整更新内容可在官方下载页查看。";
    }

    private UpdateCheckState LoadState()
    {
        try
        {
            if (!File.Exists(statePath))
            {
                return new UpdateCheckState();
            }

            string text = File.ReadAllText(statePath);
            return JsonSerializer.Deserialize<UpdateCheckState>(text, JsonOptions) ?? new UpdateCheckState();
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            startupTimeline.Mark("update_check.state_read_failed", new Dictionary<string, string>
            {
                ["error"] = exception.Message,
            });
            return new UpdateCheckState();
        }
    }

    private void SaveState(UpdateCheckState state)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(statePath)!);
            string text = JsonSerializer.Serialize(state, JsonOptions);
            File.WriteAllText(statePath, text);
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            startupTimeline.Mark("update_check.state_write_failed", new Dictionary<string, string>
            {
                ["error"] = exception.Message,
            });
        }
    }

    private HttpRequestMessage CreateGitHubRequest(HttpMethod method, Uri url)
    {
        var request = new HttpRequestMessage(method, url);
        request.Headers.UserAgent.ParseAdd($"Nexus-Windows/{currentVersion.Version}");
        return request;
    }

    private static GitHubReleaseAsset? FindWindowsMetadataAsset(IEnumerable<GitHubReleaseAsset> assets) =>
        assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.Contains("windows", StringComparison.Ordinal) && name.EndsWith(".metadata.json", StringComparison.Ordinal);
        });

    private static GitHubReleaseAsset? FindWindowsInstallerAsset(IEnumerable<GitHubReleaseAsset> assets) =>
        assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.StartsWith("nexussetup-", StringComparison.Ordinal) && name.EndsWith(".exe", StringComparison.Ordinal);
        });

    private static GitHubReleaseAsset? FindWindowsInstallerSha256Asset(
        IEnumerable<GitHubReleaseAsset> assets,
        GitHubReleaseAsset? installerAsset)
    {
        if (installerAsset is not null)
        {
            GitHubReleaseAsset? exactMatch = assets.FirstOrDefault(asset =>
                string.Equals(asset.Name, $"{installerAsset.Name}.sha256", StringComparison.OrdinalIgnoreCase));
            if (exactMatch is not null)
            {
                return exactMatch;
            }
        }

        return assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.StartsWith("nexussetup-", StringComparison.Ordinal) && name.EndsWith(".exe.sha256", StringComparison.Ordinal);
        });
    }

    private static string ReadExpectedSha256(string sha256Path, string installerFileName)
    {
        string? fallbackHash = null;
        foreach (string line in File.ReadLines(sha256Path))
        {
            string trimmed = line.Trim();
            if (string.IsNullOrWhiteSpace(trimmed))
            {
                continue;
            }

            string[] parts = trimmed.Split([' ', '\t'], StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
            if (parts.Length == 0)
            {
                continue;
            }

            string hash = parts[0].TrimStart('\uFEFF');
            if (!IsSha256Hash(hash))
            {
                continue;
            }

            if (parts.Length == 1)
            {
                return hash;
            }

            string publishedFileName = string.Join(" ", parts.Skip(1)).Trim().TrimStart('*');
            if (string.Equals(Path.GetFileName(publishedFileName), installerFileName, StringComparison.OrdinalIgnoreCase))
            {
                return hash;
            }

            fallbackHash ??= hash;
        }

        return fallbackHash ?? throw new InvalidOperationException("sha256 文件中没有找到有效的 SHA256 值。");
    }

    private static bool IsSha256Hash(string value) =>
        value.Length == 64 && value.All(character =>
            char.IsAsciiHexDigit(character));

    private static string ComputeSha256(string filePath)
    {
        using SHA256 sha256 = SHA256.Create();
        using FileStream stream = File.OpenRead(filePath);
        return Convert.ToHexString(sha256.ComputeHash(stream)).ToLowerInvariant();
    }

    private static string SafePathSegment(string value)
    {
        string sanitized = string.Join(
            "_",
            value.Split(Path.GetInvalidFileNameChars(), StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries));
        return string.IsNullOrWhiteSpace(sanitized) ? "latest" : sanitized;
    }

    private static void TryDeleteFile(string path)
    {
        try
        {
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
        catch (IOException)
        {
        }
        catch (UnauthorizedAccessException)
        {
        }
    }
}
