using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

namespace Nexus.Desktop.Update;

// 通过系统 WinVerifyTrust 校验文件的 Authenticode 签名，并读取签名者主题。
internal static class Authenticode
{
    // 签名有效时返回签名者证书主题；未签名、签名无效或已被篡改时返回 null。
    public static string? Signer(string path)
    {
        if (!Verify(path)) return null;
        try
        {
#pragma warning disable SYSLIB0057
            using var certificate = X509Certificate.CreateFromSignedFile(path);
#pragma warning restore SYSLIB0057
            return certificate.Subject;
        }
        catch (CryptographicException) { return null; }
    }

    private static bool Verify(string path)
    {
        IntPtr pathPointer = Marshal.StringToCoTaskMemUni(path);
        var file = new FileInfo { Size = (uint)Marshal.SizeOf<FileInfo>(), Path = pathPointer };
        IntPtr filePointer = Marshal.AllocCoTaskMem(Marshal.SizeOf<FileInfo>());
        try
        {
            Marshal.StructureToPtr(file, filePointer, false);
            var data = new TrustData
            {
                Size = (uint)Marshal.SizeOf<TrustData>(),
                UiChoice = 2, // WTD_UI_NONE
                RevocationChecks = 0, // WTD_REVOKE_NONE：离线时也能完成校验
                UnionChoice = 1, // WTD_CHOICE_FILE
                File = filePointer,
                StateAction = 0, // WTD_STATEACTION_IGNORE
                ProviderFlags = 0x00000010, // WTD_REVOCATION_CHECK_NONE
            };
            var action = new Guid("00AAC56B-CD44-11d0-8CC2-00C04FC295EE"); // WINTRUST_ACTION_GENERIC_VERIFY_V2
            return WinVerifyTrust(new IntPtr(-1), ref action, ref data) == 0;
        }
        finally
        {
            Marshal.FreeCoTaskMem(filePointer);
            Marshal.FreeCoTaskMem(pathPointer);
        }
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct FileInfo
    {
        public uint Size;
        public IntPtr Path;
        public IntPtr Handle;
        public IntPtr KnownSubject;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct TrustData
    {
        public uint Size;
        public IntPtr PolicyCallbackData;
        public IntPtr SipClientData;
        public uint UiChoice;
        public uint RevocationChecks;
        public uint UnionChoice;
        public IntPtr File;
        public uint StateAction;
        public IntPtr StateData;
        public IntPtr UrlReference;
        public uint ProviderFlags;
        public uint UiContext;
        public IntPtr SignatureSettings;
    }

    [DllImport("wintrust.dll", ExactSpelling = true, CharSet = CharSet.Unicode)]
    private static extern int WinVerifyTrust(IntPtr window, ref Guid action, ref TrustData data);
}
