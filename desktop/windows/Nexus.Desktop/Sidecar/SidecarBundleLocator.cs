namespace Nexus.Desktop.Sidecar;

internal sealed record SidecarBundle(
    string AppRoot,
    string WebDistDirectory,
    string Command,
    string Arguments,
    string WorkingDirectory,
    bool IsDevelopment);

internal static class SidecarBundleLocator
{
    public static SidecarBundle Resolve()
    {
        SidecarBundle? bundled = ResolveBundled();
        if (bundled is not null)
        {
            return bundled;
        }

        return ResolveDevelopment();
    }

    private static SidecarBundle? ResolveBundled()
    {
        string baseDirectory = AppContext.BaseDirectory;
        string webDist = Path.Combine(baseDirectory, "Resources", "Web");
        string sidecar = Path.Combine(baseDirectory, "Resources", "nexus-server.exe");
        if (File.Exists(Path.Combine(webDist, "app.html")) && File.Exists(sidecar))
        {
            return new SidecarBundle(
                AppRoot: Path.Combine(baseDirectory, "Resources"),
                WebDistDirectory: webDist,
                Command: sidecar,
                Arguments: string.Empty,
                WorkingDirectory: Path.Combine(baseDirectory, "Resources"),
                IsDevelopment: false);
        }

        return null;
    }

    private static SidecarBundle ResolveDevelopment()
    {
        string? root = FindProjectRoot();
        if (root is null)
        {
            throw new InvalidOperationException("未找到 Nexus 仓库根目录。");
        }

        string webDist = Path.Combine(root, "web", "dist");
        if (!File.Exists(Path.Combine(webDist, "app.html")))
        {
            throw new InvalidOperationException($"未找到 web/dist/app.html：{webDist}");
        }

        return new SidecarBundle(
            AppRoot: root,
            WebDistDirectory: webDist,
            Command: "go",
            Arguments: "run ./cmd/nexus-server",
            WorkingDirectory: root,
            IsDevelopment: true);
    }

    private static string? FindProjectRoot()
    {
        string current = Directory.GetCurrentDirectory();
        while (!string.IsNullOrWhiteSpace(current))
        {
            if (File.Exists(Path.Combine(current, "go.mod")) && File.Exists(Path.Combine(current, "web", "index.html")))
            {
                return current;
            }

            DirectoryInfo? parent = Directory.GetParent(current);
            if (parent is null)
            {
                return null;
            }
            current = parent.FullName;
        }

        return null;
    }
}
