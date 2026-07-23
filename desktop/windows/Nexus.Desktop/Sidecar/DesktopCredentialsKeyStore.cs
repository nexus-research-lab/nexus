using System.IO;
using System.Security.Cryptography;
using System.Text;

namespace Nexus.Desktop.Sidecar;

internal sealed record DesktopCredentialsKey(string Value, string Storage, string Reason);

internal static class DesktopCredentialsKeyStore
{
    public static DesktopCredentialsKey ConnectorCredentialsKey()
    {
        Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
        try
        {
            return ConnectorCredentialsKeyFromDpapi();
        }
        catch (CryptographicException exception)
        {
            return ConnectorCredentialsKeyFromPlainFile($"dpapi_failed:{exception.GetType().Name}");
        }
        catch (IOException exception)
        {
            return ConnectorCredentialsKeyFromPlainFile($"dpapi_io_failed:{exception.GetType().Name}");
        }
        catch (UnauthorizedAccessException exception)
        {
            return ConnectorCredentialsKeyFromPlainFile($"dpapi_access_failed:{exception.GetType().Name}");
        }
    }

    private static DesktopCredentialsKey ConnectorCredentialsKeyFromDpapi()
    {
        string dpapiPath = Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.dpapi");
        string plainPath = Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.key");
        if (File.Exists(dpapiPath))
        {
            byte[] protectedBytes = File.ReadAllBytes(dpapiPath);
            byte[] plainBytes = ProtectedData.Unprotect(protectedBytes, optionalEntropy: null, DataProtectionScope.CurrentUser);
            string value = Encoding.UTF8.GetString(plainBytes).Trim();
            if (!string.IsNullOrWhiteSpace(value))
            {
                return new DesktopCredentialsKey(value, "dpapi", "current_user");
            }
        }

        if (File.Exists(plainPath))
        {
            string existing = File.ReadAllText(plainPath).Trim();
            if (!string.IsNullOrWhiteSpace(existing))
            {
                PersistDpapiKey(dpapiPath, existing);
                return new DesktopCredentialsKey(existing, "dpapi", "migrated_plain_file");
            }
        }

        string? legacy = ReadLegacyDpapiKey() ?? ReadLegacyPlainKey();
        if (!string.IsNullOrWhiteSpace(legacy))
        {
            PersistDpapiKey(dpapiPath, legacy);
            return new DesktopCredentialsKey(legacy, "dpapi", "migrated_legacy_file");
        }

        string generated = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
        PersistDpapiKey(dpapiPath, generated);
        return new DesktopCredentialsKey(generated, "dpapi", "generated");
    }

    private static DesktopCredentialsKey ConnectorCredentialsKeyFromPlainFile(string reason)
    {
        string keyPath = Path.Combine(DesktopPaths.ConfigDirectory, "connector-credentials.key");
        if (File.Exists(keyPath))
        {
            string existing = File.ReadAllText(keyPath).Trim();
            if (!string.IsNullOrWhiteSpace(existing))
            {
                return new DesktopCredentialsKey(existing, "file", reason);
            }
        }

        string? legacy = ReadLegacyPlainKey();
        if (!string.IsNullOrWhiteSpace(legacy))
        {
            File.WriteAllText(keyPath, legacy);
            return new DesktopCredentialsKey(legacy, "file", $"{reason}:migrated_legacy_file");
        }

        string generated = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
        File.WriteAllText(keyPath, generated);
        return new DesktopCredentialsKey(generated, "file", reason);
    }

    private static string? ReadLegacyPlainKey()
    {
        string legacyPath = Path.Combine(
            DesktopPaths.RootDirectory,
            "config",
            "connector-credentials.key");
        if (!File.Exists(legacyPath))
        {
            return null;
        }

        string value = File.ReadAllText(legacyPath).Trim();
        return string.IsNullOrWhiteSpace(value) ? null : value;
    }

    private static string? ReadLegacyDpapiKey()
    {
        string legacyPath = Path.Combine(
            DesktopPaths.RootDirectory,
            "config",
            "connector-credentials.dpapi");
        if (!File.Exists(legacyPath))
        {
            return null;
        }

        try
        {
            byte[] protectedBytes = File.ReadAllBytes(legacyPath);
            byte[] plainBytes = ProtectedData.Unprotect(
                protectedBytes,
                optionalEntropy: null,
                DataProtectionScope.CurrentUser);
            string value = Encoding.UTF8.GetString(plainBytes).Trim();
            return string.IsNullOrWhiteSpace(value) ? null : value;
        }
        catch (CryptographicException)
        {
            return null;
        }
        catch (IOException)
        {
            return null;
        }
        catch (UnauthorizedAccessException)
        {
            return null;
        }
    }

    private static void PersistDpapiKey(string path, string value)
    {
        byte[] plainBytes = Encoding.UTF8.GetBytes(value);
        byte[] protectedBytes = ProtectedData.Protect(plainBytes, optionalEntropy: null, DataProtectionScope.CurrentUser);
        File.WriteAllBytes(path, protectedBytes);
    }
}
