using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Net.Http.Headers;
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

        state.LastAutomaticCheckAt = now;
        SaveState(state);
        _ = RunStartupCheckAsync(owner);
    }

    private async Task RunStartupCheckAsync(System.Windows.Window owner)
    {
        startupTimeline.Mark("update_check.started", new Dictionary<string, string>
        {
            ["reason"] = "startup",
            ["current_version"] = currentVersion.Version,
            ["current_build"] = currentVersion.BuildNumber,
        });

        try
        {
            DesktopReleaseInfo latest = await FetchLatestReleaseAsync();
            bool hasUpdate = latest.IsNewerThan(currentVersion);
            SaveState(new UpdateCheckState
            {
                LastAutomaticCheckAt = DateTimeOffset.UtcNow,
                LastResult = hasUpdate ? "update_available" : "up_to_date",
                LastLatestVersion = latest.Version,
                LastLatestBuildNumber = latest.BuildNumber,
                LastErrorMessage = null,
            });

            startupTimeline.Mark("update_check.result", new Dictionary<string, string>
            {
                ["reason"] = "startup",
                ["status"] = hasUpdate ? "update_available" : "up_to_date",
                ["current_version"] = currentVersion.Version,
                ["current_build"] = currentVersion.BuildNumber,
                ["latest_version"] = latest.Version,
                ["latest_build"] = latest.BuildNumber ?? string.Empty,
                ["source"] = latest.Source,
            });

            if (hasUpdate)
            {
                await ShowUpdateAvailableAsync(owner, latest);
            }
        }
        catch (Exception exception)
        {
            SaveState(new UpdateCheckState
            {
                LastAutomaticCheckAt = DateTimeOffset.UtcNow,
                LastResult = "failed",
                LastErrorMessage = exception.Message,
            });
            startupTimeline.Mark("update_check.failed", new Dictionary<string, string>
            {
                ["reason"] = "startup",
                ["error"] = exception.Message,
            });
        }
    }

    private async Task<DesktopReleaseInfo> FetchLatestReleaseAsync()
    {
        GitHubRelease release = await FetchJsonAsync<GitHubRelease>(LatestReleaseUrl);
        GitHubReleaseAsset? metadataAsset = release.Assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.Contains("windows", StringComparison.Ordinal) && name.EndsWith(".metadata.json", StringComparison.Ordinal);
        });
        GitHubReleaseAsset? downloadAsset = release.Assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.StartsWith("nexussetup-", StringComparison.Ordinal) && name.EndsWith(".exe", StringComparison.Ordinal);
        }) ?? release.Assets.FirstOrDefault(asset =>
        {
            string name = asset.Name.ToLowerInvariant();
            return name.Contains("windows", StringComparison.Ordinal) && name.EndsWith(".zip", StringComparison.Ordinal);
        });

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
                    downloadAsset?.BrowserDownloadUrl,
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
            downloadAsset?.BrowserDownloadUrl,
            release.PublishedAt,
            release.Prerelease,
            "github_release");
    }

    private async Task<T> FetchJsonAsync<T>(Uri url)
    {
        using HttpRequestMessage request = new(HttpMethod.Get, url);
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/vnd.github+json"));
        request.Headers.UserAgent.ParseAdd($"Nexus-Windows/{currentVersion.Version}");

        using CancellationTokenSource timeout = new(TimeSpan.FromSeconds(15));
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

        await owner.Dispatcher.InvokeAsync(() =>
        {
            startupTimeline.Mark("update_check.prompt_shown", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
                ["latest_build"] = latest.BuildNumber ?? string.Empty,
            });

            MessageBoxResult result = MessageBox.Show(
                owner,
                UpdateAvailableMessage(latest),
                "发现 Nexus 新版本",
                MessageBoxButton.YesNo,
                MessageBoxImage.Information);

            if (result != MessageBoxResult.Yes)
            {
                return;
            }

            startupTimeline.Mark("update_check.release_page_opened", new Dictionary<string, string>
            {
                ["latest_version"] = latest.Version,
            });
            Process.Start(new ProcessStartInfo
            {
                FileName = latest.ReleasePageUrl.ToString(),
                UseShellExecute = true,
            });
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
        lines.Add("当前阶段不会自动安装更新。选择“是”打开下载页后，请校验对应的 sha256 文件。");
        return string.Join(Environment.NewLine, lines);
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
}

internal sealed class UpdateCheckState
{
    public DateTimeOffset? LastAutomaticCheckAt { get; set; }

    public string? LastResult { get; set; }

    public string? LastLatestVersion { get; set; }

    public string? LastLatestBuildNumber { get; set; }

    public string? LastErrorMessage { get; set; }
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
    Uri? DownloadUrl,
    string? PublishedAt,
    bool IsPrerelease,
    string Source)
{
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
