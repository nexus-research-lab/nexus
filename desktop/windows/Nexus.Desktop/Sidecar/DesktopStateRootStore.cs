using System.Diagnostics;
using System.IO;
using System.Text.Json;
using Microsoft.Win32;

namespace Nexus.Desktop.Sidecar;

internal static class DesktopStateRootStore
{
    private const string RegistryKeyEnvironmentName = "NEXUS_DESKTOP_REGISTRY_KEY";
    private const string InitialRootEnvironmentName = "NEXUS_DESKTOP_STATE_ROOT";
    private const string StateValueName = "StateRootBootstrap";

    private sealed record BootstrapState(
        string ActiveRoot,
        string? PreviousRoot = null,
        string? MigrationError = null);

    public static string DefaultRootDirectory => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        ".nexus");

    public static string BootstrapLocation => $"HKCU\\{RegistryKeyPath}";

    public static string ActiveRootDirectory
    {
        get
        {
            try
            {
                return NormalizePath(Load().ActiveRoot);
            }
            catch (Exception exception)
            {
                Trace.WriteLine($"[Nexus State Root] bootstrap read failed: {exception.Message}");
                return InitialRootDirectory;
            }
        }
    }

    public static string? PreviousRootDirectory
    {
        get
        {
            try
            {
                string? path = Load().PreviousRoot;
                return string.IsNullOrWhiteSpace(path) ? null : NormalizePath(path);
            }
            catch (Exception exception)
            {
                Trace.WriteLine($"[Nexus State Root] previous root read failed: {exception.Message}");
                return null;
            }
        }
    }

    public static Dictionary<string, object?> StatusPayload()
    {
        BootstrapState? state = TryLoad();
        return new Dictionary<string, object?>
        {
            ["current_path"] = ActiveRootDirectory,
            ["default_path"] = DefaultRootDirectory,
            ["migration_error"] = string.IsNullOrWhiteSpace(state?.MigrationError) ? null : state.MigrationError,
        };
    }

    public static void ActivateMigration(string source, string target)
    {
        Save(new BootstrapState(NormalizePath(target), NormalizePath(source)));
    }

    public static string? CompleteMigration()
    {
        BootstrapState state = Load();
        if (string.IsNullOrWhiteSpace(state.PreviousRoot))
        {
            return null;
        }
        string previousRoot = NormalizePath(state.PreviousRoot);
        string activeRoot = NormalizePath(state.ActiveRoot);
        ValidateManagedRoot(previousRoot);
        ValidateManagedRoot(activeRoot);
        if (SameManagedRoot(previousRoot, activeRoot)
            || ManagedRootContains(previousRoot, activeRoot)
            || ManagedRootContains(activeRoot, previousRoot))
        {
            throw new InvalidDataException("Nexus 桌面启动指针中的新旧目录重叠。");
        }
        Save(new BootstrapState(activeRoot));
        return previousRoot;
    }

    public static string? RollbackMigration(string message)
    {
        BootstrapState state = Load();
        if (string.IsNullOrWhiteSpace(state.PreviousRoot))
        {
            return null;
        }
        string previousRoot = NormalizePath(state.PreviousRoot);
        Save(new BootstrapState(previousRoot, MigrationError: message));
        return previousRoot;
    }

    public static void RecordMigrationFailure(string source, string message)
    {
        try
        {
            Save(new BootstrapState(NormalizePath(source), MigrationError: message));
        }
        catch (Exception exception)
        {
            Trace.WriteLine($"[Nexus State Root] failed to persist migration error: {exception.Message}");
        }
    }

    public static string NormalizePath(string path)
    {
        string value = Environment.ExpandEnvironmentVariables(path.Trim());
        if (value == "~" || value.StartsWith("~\\", StringComparison.Ordinal) || value.StartsWith("~/", StringComparison.Ordinal))
        {
            string suffix = value.Length == 1 ? string.Empty : value[2..];
            value = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), suffix);
        }
        return Path.GetFullPath(value);
    }

    public static void ValidateManagedRoot(string root)
    {
        string normalized = CanonicalPath(root).TrimEnd(Path.DirectorySeparatorChar);
        string fileSystemRoot = Path.GetPathRoot(normalized)?.TrimEnd(Path.DirectorySeparatorChar) ?? string.Empty;
        string userProfile = CanonicalPath(
            Environment.GetFolderPath(Environment.SpecialFolder.UserProfile)
        ).TrimEnd(Path.DirectorySeparatorChar);
        if (string.IsNullOrWhiteSpace(normalized)
            || string.Equals(normalized, fileSystemRoot, StringComparison.OrdinalIgnoreCase)
            || string.Equals(normalized, userProfile, StringComparison.OrdinalIgnoreCase))
        {
            throw new InvalidDataException("Nexus 数据目录不能是磁盘根目录或用户主目录。");
        }
    }

    public static bool SameManagedRoot(string left, string right)
    {
        return string.Equals(
            CanonicalPath(left).TrimEnd(Path.DirectorySeparatorChar),
            CanonicalPath(right).TrimEnd(Path.DirectorySeparatorChar),
            StringComparison.OrdinalIgnoreCase);
    }

    public static bool ManagedRootContains(string root, string candidate)
    {
        string relative = Path.GetRelativePath(CanonicalPath(root), CanonicalPath(candidate));
        return relative != "."
            && relative != ".."
            && !relative.StartsWith(".." + Path.DirectorySeparatorChar, StringComparison.Ordinal)
            && !Path.IsPathFullyQualified(relative);
    }

    private static string RegistryKeyPath
    {
        get
        {
            string? overridePath = Environment.GetEnvironmentVariable(RegistryKeyEnvironmentName);
            return string.IsNullOrWhiteSpace(overridePath)
                ? @"Software\NexusDesktop"
                : overridePath.Trim().TrimStart('\\');
        }
    }

    private static string InitialRootDirectory
    {
        get
        {
            try
            {
                string? overridePath = Environment.GetEnvironmentVariable(InitialRootEnvironmentName);
                string candidate = string.IsNullOrWhiteSpace(overridePath)
                    ? DefaultRootDirectory
                    : NormalizePath(overridePath);
                ValidateManagedRoot(candidate);
                return candidate;
            }
            catch (Exception exception)
            {
                Trace.WriteLine($"[Nexus State Root] initial root is invalid: {exception.Message}");
                return DefaultRootDirectory;
            }
        }
    }

    private static BootstrapState? TryLoad()
    {
        try
        {
            return Load();
        }
        catch (Exception exception)
        {
            Trace.WriteLine($"[Nexus State Root] status read failed: {exception.Message}");
            return null;
        }
    }

    private static BootstrapState Load()
    {
        using RegistryKey? key = Registry.CurrentUser.OpenSubKey(RegistryKeyPath);
        string? json = key?.GetValue(StateValueName) as string;
        if (string.IsNullOrWhiteSpace(json))
        {
            return new BootstrapState(InitialRootDirectory);
        }
        BootstrapState state = JsonSerializer.Deserialize<BootstrapState>(json)
            ?? throw new InvalidDataException("Nexus 桌面启动指针为空。");
        if (string.IsNullOrWhiteSpace(state.ActiveRoot))
        {
            throw new InvalidDataException("Nexus 桌面启动指针格式不正确。");
        }
        ValidateManagedRoot(state.ActiveRoot);
        if (!string.IsNullOrWhiteSpace(state.PreviousRoot))
        {
            ValidateManagedRoot(state.PreviousRoot);
        }
        return state;
    }

    private static void Save(BootstrapState state)
    {
        using RegistryKey key = Registry.CurrentUser.CreateSubKey(RegistryKeyPath, writable: true)
            ?? throw new InvalidOperationException("无法写入 Nexus 桌面启动指针。");
        key.SetValue(StateValueName, JsonSerializer.Serialize(state), RegistryValueKind.String);
        key.Flush();
    }

    private static string CanonicalPath(string path)
    {
        string fullPath = NormalizePath(path);
        string root = Path.GetPathRoot(fullPath)
            ?? throw new InvalidDataException("Nexus 数据目录缺少文件系统根。");
        string relative = Path.GetRelativePath(root, fullPath);
        if (relative == ".")
        {
            return NormalizePath(root);
        }

        string current = root;
        foreach (string segment in relative.Split(
            new[] { Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar },
            StringSplitOptions.RemoveEmptyEntries))
        {
            string candidate = Path.Combine(current, segment);
            FileSystemInfo? entry = Directory.Exists(candidate)
                ? new DirectoryInfo(candidate)
                : File.Exists(candidate) ? new FileInfo(candidate) : null;
            if (entry is not null && entry.Attributes.HasFlag(FileAttributes.ReparsePoint))
            {
                FileSystemInfo? resolved = entry.ResolveLinkTarget(returnFinalTarget: true);
                if (resolved is not null)
                {
                    current = NormalizePath(resolved.FullName);
                    continue;
                }
            }
            current = candidate;
        }
        return NormalizePath(current);
    }
}
