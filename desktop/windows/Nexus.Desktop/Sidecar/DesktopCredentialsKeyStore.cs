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

        string generated = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
        File.WriteAllText(keyPath, generated);
        return new DesktopCredentialsKey(generated, "file", reason);
    }

    private static void PersistDpapiKey(string path, string value)
    {
        byte[] plainBytes = Encoding.UTF8.GetBytes(value);
        byte[] protectedBytes = ProtectedData.Protect(plainBytes, optionalEntropy: null, DataProtectionScope.CurrentUser);
        File.WriteAllBytes(path, protectedBytes);
    }
}
