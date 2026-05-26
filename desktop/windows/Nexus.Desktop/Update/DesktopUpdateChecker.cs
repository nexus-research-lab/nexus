using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Windows;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Update;

internal sealed class DesktopUpdateChecker
{
    private static readonly TimeSpan AutomaticCheckInterval = TimeSpan.FromHours(24);
    private static readonly TimeSpan MetadataRequestTimeout = TimeSpan.FromSeconds(15);
    private static readonly TimeSpan DownloadRequestTimeout = TimeSpan.FromMinutes(10);
    private const int ReleaseNotesMaxCharacters = 1800;
    private const int ReleaseNotesMaxLines = 24;
    private static readonly Uri LatestReleaseUrl = new("https://api.github.com/repos/nexus-research-lab/nexus/releases/latest");
    private static readonly Uri FallbackReleasePageUrl = new("https://github.com/nexus-research-lab/nexus/releases/latest");
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        WriteIndented = true,
    };

    private readonly DesktopStartupTimeline startupTimeline;
    private readonly HttpClient httpClient;
    private readonly DesktopAppVersion currentVersion;
    private readonly string statePath;
    private readonly bool isDisabled;
    private readonly SemaphoreSlim checkLock = new(1, 1);
    private bool hasPerformedStartupCheck;

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

        UpdateCheckState state = LoadState();
        DateTimeOffset now = DateTimeOffset.UtcNow;
        if (state.LastAutomaticCheckAt is not null)
        {
            TimeSpan elapsed = now - state.LastAutomaticCheckAt.Value;
            if (elapsed < AutomaticCheckInterval)
            {
                startupTimeline.Mark("update_check.skipped", new Dictionary<string, string>
                {
                    ["reason"] = "recent",
                    ["elapsed_minutes"] = Math.Max(0, (int)elapsed.TotalMinutes).ToString(),
                });
                return;
            }
        }

        _ = RunStartupCheckAsync(owner);
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

    private async Task RunStartupCheckAsync(System.Windows.Window owner)
    {
        await CheckWithLockAsync(owner, "startup", showsUpToDateAlert: false);
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
            UpdateCheckState previousState = LoadState();
            SaveState(new UpdateCheckState
            {
                LastAutomaticCheckAt = reason == "startup"
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
                await ShowUpdateAvailableAsync(owner, latest);
            }
            else if (showsUpToDateAlert)
            {
                await ShowUpToDateAsync(owner, latest);
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
                LastErrorMessage = exception.Message,
            });
            startupTimeline.Mark("update_check.failed", new Dictionary<string, string>
            {
                ["reason"] = reason,
                ["error"] = exception.Message,
            });
            if (showsUpToDateAlert)
            {
                await ShowCheckFailedAsync(owner, exception);
            }

            return DesktopUpdateCheckResult.Failed(currentVersion, exception.Message);
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
        switch (action)
        {
            case UpdatePromptAction.DownloadAndInstall:
                await DownloadAndOfferInstallAsync(owner, latest);
                break;
            case UpdatePromptAction.OpenReleasePage:
                OpenReleasePage(latest, "prompt");
                break;
            case UpdatePromptAction.Later:
            default:
                break;
        }
    }

    private async Task ShowUpToDateAsync(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        await owner.Dispatcher.InvokeAsync(() => System.Windows.MessageBox.Show(
            owner,
            string.Join(
                Environment.NewLine,
                $"当前版本：{currentVersion.DisplayText}",
                $"最新版本：{latest.DisplayText}"),
            "Nexus 已是最新版本",
            MessageBoxButton.OK,
            MessageBoxImage.Information));
    }

    private async Task ShowCheckFailedAsync(System.Windows.Window owner, Exception exception)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        await owner.Dispatcher.InvokeAsync(() => System.Windows.MessageBox.Show(
            owner,
            exception.Message,
            "Nexus 检查更新失败",
            MessageBoxButton.OK,
            MessageBoxImage.Warning));
    }

    private UpdatePromptAction PromptForUpdate(System.Windows.Window owner, DesktopReleaseInfo latest)
    {
        startupTimeline.Mark("update_check.prompt_shown", new Dictionary<string, string>
        {
            ["latest_version"] = latest.Version,
            ["latest_build"] = latest.BuildNumber ?? string.Empty,
            ["can_download_installer"] = latest.CanDownloadInstaller.ToString(),
        });

        MessageBoxResult result = System.Windows.MessageBox.Show(
            owner,
            UpdateAvailableMessage(latest),
            "发现 Nexus 新版本",
            latest.CanDownloadInstaller ? MessageBoxButton.YesNoCancel : MessageBoxButton.YesNo,
            MessageBoxImage.Information);

        if (!latest.CanDownloadInstaller)
        {
            return result == MessageBoxResult.Yes ? UpdatePromptAction.OpenReleasePage : UpdatePromptAction.Later;
        }

        return result switch
        {
            MessageBoxResult.Yes => UpdatePromptAction.DownloadAndInstall,
            MessageBoxResult.No => UpdatePromptAction.OpenReleasePage,
            _ => UpdatePromptAction.Later,
        };
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
            DownloadedUpdate downloadedUpdate = await DownloadAndVerifyUpdateAsync(latest);
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
            await ShowDownloadFailedAsync(owner, latest, exception);
        }
    }

    private async Task<DownloadedUpdate> DownloadAndVerifyUpdateAsync(DesktopReleaseInfo latest)
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

        await DownloadFileAsync(installerUrl, installerPath);
        await DownloadFileAsync(sha256Url, sha256Path);

        string expectedHash = ReadExpectedSha256(sha256Path, installerFileName);
        string actualHash = ComputeSha256(installerPath);
        if (!string.Equals(expectedHash, actualHash, StringComparison.OrdinalIgnoreCase))
        {
            TryDeleteFile(installerPath);
            throw new InvalidOperationException("下载的安装器 sha256 校验未通过，已丢弃本地文件。");
        }

        return new DownloadedUpdate(installerPath, sha256Path, actualHash.ToLowerInvariant());
    }

    private async Task DownloadFileAsync(Uri url, string destinationPath)
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
            await source.CopyToAsync(destination, timeout.Token);
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

        MessageBoxResult result = await owner.Dispatcher.InvokeAsync(() =>
        {
            startupTimeline.Mark("update_check.install_prompt_shown", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["latest_build"] = latest.BuildNumber ?? string.Empty,
                ["installer_path"] = downloadedUpdate.InstallerPath,
            });

            return System.Windows.MessageBox.Show(
                owner,
                InstallReadyMessage(latest, downloadedUpdate),
                "Nexus 更新已就绪",
                MessageBoxButton.YesNo,
                MessageBoxImage.Question);
        });

        if (result != MessageBoxResult.Yes)
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

        MessageBoxResult result = await owner.Dispatcher.InvokeAsync(() => System.Windows.MessageBox.Show(
            owner,
            "当前 Release 缺少可自动校验的 Windows 安装器或 sha256 文件。是否打开下载页手动处理？",
            "Nexus 更新暂不可自动下载",
            MessageBoxButton.YesNo,
            MessageBoxImage.Information));
        if (result == MessageBoxResult.Yes)
        {
            OpenReleasePage(latest, "download_unavailable");
        }
    }

    private async Task ShowDownloadFailedAsync(
        System.Windows.Window owner,
        DesktopReleaseInfo latest,
        Exception exception)
    {
        if (owner.Dispatcher.HasShutdownStarted)
        {
            return;
        }

        MessageBoxResult result = await owner.Dispatcher.InvokeAsync(() => System.Windows.MessageBox.Show(
            owner,
            $"更新下载或校验失败：{exception.Message}{Environment.NewLine}{Environment.NewLine}是否打开 Release 页面手动下载？",
            "Nexus 更新下载失败",
            MessageBoxButton.YesNo,
            MessageBoxImage.Warning));
        if (result == MessageBoxResult.Yes)
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

    private string UpdateAvailableMessage(DesktopReleaseInfo latest)
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

        lines.Add(string.Empty);
        string? releaseNotes = FormatReleaseNotes(latest.ReleaseNotes);
        if (!string.IsNullOrWhiteSpace(releaseNotes))
        {
            lines.Add("更新内容：");
            lines.Add(releaseNotes);
            lines.Add(string.Empty);
        }

        if (latest.CanDownloadInstaller)
        {
            lines.Add("选择“是”将下载安装器和 sha256 文件，校验通过后再询问是否启动安装。");
            lines.Add("选择“否”将打开 Release 页面；选择“取消”稍后再说。");
        }
        else
        {
            lines.Add("当前 Release 缺少 Windows 安装器或 sha256 文件。选择“是”打开下载页手动处理。");
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

        var lines = normalized
            .Split('\n')
            .Take(ReleaseNotesMaxLines + 1)
            .ToList();
        bool wasTruncated = normalized.Length > ReleaseNotesMaxCharacters || lines.Count > ReleaseNotesMaxLines;
        string clipped = string.Join(Environment.NewLine, lines.Take(ReleaseNotesMaxLines)).Trim();
        if (clipped.Length > ReleaseNotesMaxCharacters)
        {
            clipped = clipped[..ReleaseNotesMaxCharacters].TrimEnd();
            wasTruncated = true;
        }

        if (wasTruncated)
        {
            clipped = $"{clipped}{Environment.NewLine}...{Environment.NewLine}完整更新内容请打开 Release 页面查看。";
        }
        return clipped;
    }

    private string InstallReadyMessage(DesktopReleaseInfo latest, DownloadedUpdate downloadedUpdate)
    {
        return string.Join(
            Environment.NewLine,
            $"Nexus {latest.DisplayText} 已下载并通过 sha256 校验。",
            $"安装器：{Path.GetFileName(downloadedUpdate.InstallerPath)}",
            $"sha256：{downloadedUpdate.Sha256Hash}",
            string.Empty,
            "是否现在启动安装器？Nexus 将退出，安装器会继续完成更新。");
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

internal sealed class UpdateCheckState
{
    public DateTimeOffset? LastAutomaticCheckAt { get; set; }

    public string? LastResult { get; set; }

    public string? LastLatestVersion { get; set; }

    public string? LastLatestBuildNumber { get; set; }

    public string? LastErrorMessage { get; set; }
}

internal sealed record DesktopUpdateCheckResult(
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("current_version")] string CurrentVersion,
    [property: JsonPropertyName("current_build_number")] string CurrentBuildNumber,
    [property: JsonPropertyName("latest_version")] string? LatestVersion,
    [property: JsonPropertyName("latest_build_number")] string? LatestBuildNumber,
    [property: JsonPropertyName("release_page_url")] string? ReleasePageUrl,
    [property: JsonPropertyName("can_download_installer")] bool CanDownloadInstaller,
    [property: JsonPropertyName("error_message")] string? ErrorMessage)
{
    public static DesktopUpdateCheckResult From(
        DesktopAppVersion current,
        DesktopReleaseInfo latest,
        bool hasUpdate) => new(
            hasUpdate ? "update_available" : "up_to_date",
            current.Version,
            current.BuildNumber,
            latest.Version,
            latest.BuildNumber,
            latest.ReleasePageUrl.ToString(),
            latest.CanDownloadInstaller,
            null);

    public static DesktopUpdateCheckResult Disabled(DesktopAppVersion current) => new(
        "disabled",
        current.Version,
        current.BuildNumber,
        null,
        null,
        null,
        false,
        null);

    public static DesktopUpdateCheckResult Failed(DesktopAppVersion current, string errorMessage) => new(
        "failed",
        current.Version,
        current.BuildNumber,
        null,
        null,
        null,
        false,
        errorMessage);
}

internal sealed record DesktopAppVersion(string Version, string BuildNumber)
{
    public static DesktopAppVersion Current() => new(AppVersionInfo.Version, AppVersionInfo.BuildNumber);

    public string DisplayText => $"版本 {Version}，构建 {BuildNumber}";
}

internal sealed record DesktopReleaseInfo(
    string Version,
    string? BuildNumber,
    string? ReleaseName,
    Uri ReleasePageUrl,
    string? InstallerFileName,
    Uri? InstallerDownloadUrl,
    string? InstallerSha256FileName,
    Uri? InstallerSha256Url,
    string? ReleaseNotes,
    string? PublishedAt,
    bool IsPrerelease,
    string Source)
{
    public bool CanDownloadInstaller =>
        !string.IsNullOrWhiteSpace(InstallerFileName) &&
        InstallerDownloadUrl is not null &&
        InstallerSha256Url is not null;

    public string DisplayText => string.IsNullOrWhiteSpace(BuildNumber)
        ? $"版本 {Version}"
        : $"版本 {Version}，构建 {BuildNumber}";

    public bool IsNewerThan(DesktopAppVersion current)
    {
        ComparableVersion latestVersion = new(Version);
        ComparableVersion currentVersion = new(current.Version);
        if (latestVersion > currentVersion)
        {
            return true;
        }
        if (latestVersion < currentVersion)
        {
            return false;
        }

        return int.TryParse(BuildNumber, out int latestBuild) &&
            int.TryParse(current.BuildNumber, out int currentBuild) &&
            latestBuild > currentBuild;
    }
}

internal sealed record DownloadedUpdate(string InstallerPath, string Sha256Path, string Sha256Hash);

internal enum UpdatePromptAction
{
    Later,
    OpenReleasePage,
    DownloadAndInstall,
}

internal sealed class ComparableVersion : IComparable<ComparableVersion>
{
    private readonly IReadOnlyList<int> parts;

    public ComparableVersion(string rawValue)
    {
        string normalized = GitHubReleaseVersionNormalizer.VersionFrom(rawValue);
        string baseVersion = normalized.Split(['-', '+'], 2, StringSplitOptions.TrimEntries)[0];
        parts = baseVersion
            .Split('.', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Select(part => int.TryParse(part, out int value) ? value : 0)
            .ToList();
    }

    public static bool operator >(ComparableVersion left, ComparableVersion right) => left.CompareTo(right) > 0;

    public static bool operator <(ComparableVersion left, ComparableVersion right) => left.CompareTo(right) < 0;

    public int CompareTo(ComparableVersion? other)
    {
        if (other is null)
        {
            return 1;
        }

        int count = Math.Max(parts.Count, other.parts.Count);
        for (int index = 0; index < count; index++)
        {
            int left = index < parts.Count ? parts[index] : 0;
            int right = index < other.parts.Count ? other.parts[index] : 0;
            int comparison = left.CompareTo(right);
            if (comparison != 0)
            {
                return comparison;
            }
        }
        return 0;
    }
}

internal static class GitHubReleaseVersionNormalizer
{
    public static string VersionFrom(string rawValue)
    {
        string trimmed = rawValue.Trim();
        return trimmed.StartsWith("v", StringComparison.OrdinalIgnoreCase)
            ? trimmed[1..]
            : trimmed;
    }
}

internal sealed class GitHubRelease
{
    [JsonPropertyName("tag_name")]
    public string TagName { get; set; } = string.Empty;

    public string? Name { get; set; }

    [JsonPropertyName("html_url")]
    public Uri? HtmlUrl { get; set; }

    public string? Body { get; set; }

    public bool Prerelease { get; set; }

    [JsonPropertyName("published_at")]
    public string? PublishedAt { get; set; }

    public List<GitHubReleaseAsset> Assets { get; set; } = [];
}

internal sealed class GitHubReleaseAsset
{
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("browser_download_url")]
    public Uri? BrowserDownloadUrl { get; set; }
}

internal sealed class DesktopPackageMetadata
{
    public string Version { get; set; } = string.Empty;

    [JsonPropertyName("build_number")]
    public string BuildNumber { get; set; } = string.Empty;
}
