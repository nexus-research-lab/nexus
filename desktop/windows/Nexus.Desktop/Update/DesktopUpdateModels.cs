using System.Text.Json.Serialization;
using Nexus.Desktop.Runtime;

namespace Nexus.Desktop.Update;

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
