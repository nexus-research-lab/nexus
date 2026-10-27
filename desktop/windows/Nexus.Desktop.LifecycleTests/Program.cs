// Native lifecycle acceptance: no mock of Windows Jobs, inherited pipes or process termination.
using System.Diagnostics;
using System.Reflection;
using System.Net;
using System.Net.Sockets;
using Nexus.Desktop.Sidecar;

static ProcessStartInfo Self(params string[] args)
{
    string executable = Environment.ProcessPath!;
    var arguments = new List<string>();
    if (Path.GetFileNameWithoutExtension(executable).Equals("dotnet", StringComparison.OrdinalIgnoreCase))
        arguments.Add(Assembly.GetExecutingAssembly().Location);
    arguments.AddRange(args);
    return new ProcessStartInfo(executable, string.Join(" ", arguments.Select(arg => "\"" + arg + "\"")))
    {
        UseShellExecute = false,
        WorkingDirectory = Environment.CurrentDirectory,
    };
}

static async Task<int> WaitForPid(string file)
{
    using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
    while (true)
    {
        if (File.Exists(file) && int.TryParse(await File.ReadAllTextAsync(file), out int pid)) return pid;
        await Task.Delay(25, timeout.Token);
    }
}

static async Task ExpectExited(Process process)
{
    using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(10));
    await process.WaitForExitAsync(timeout.Token);
}

if (args.Length == 0 || args[0] == "identity-only")
{
    string oldExecutable = Path.Combine(Path.GetTempPath(), "old package", "nexus-server.exe");
    string stateRoot = Path.Combine(Path.GetTempPath(), "nexus state");
    var receipt = new SidecarProcessRecord(123, oldExecutable, 456, stateRoot);
    if (!receipt.Matches(123, 456, oldExecutable, stateRoot)) throw new Exception("Original process identity should survive repackaging");
    if (receipt.Matches(124, 456, oldExecutable, stateRoot) ||
        receipt.Matches(123, 457, oldExecutable, stateRoot) ||
        receipt.Matches(123, 456, oldExecutable + ".other", stateRoot) ||
        receipt.Matches(123, 456, oldExecutable, stateRoot + ".other") ||
        new SidecarProcessRecord(123, oldExecutable).Matches(123, 456, oldExecutable, stateRoot))
        throw new Exception("PID reuse, different paths/roots and legacy identities must not authorize termination");
    Console.WriteLine("PASS recovery identity (platform-neutral)");
    if (args.Length > 0) return;
}

if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException("Native lifecycle acceptance requires Windows; do not count a skip as success.");
if (args.Length > 0)
{
    switch (args[0])
    {
        case "echo":
            Console.WriteLine(Environment.GetEnvironmentVariable("NEXUS_LIFECYCLE_TEST"));
            Console.Error.WriteLine("stderr works");
            return;
        case "leaf":
            await Task.Delay(Timeout.Infinite);
            return;
        case "tree":
            using (var listener = new TcpListener(IPAddress.Loopback, args.Length > 2 ? int.Parse(args[2]) : 0))
            using (Process leaf = Process.Start(Self("leaf"))!)
            {
                listener.Start();
                await File.WriteAllTextAsync(args[1], leaf.Id.ToString());
                await Task.Delay(Timeout.Infinite);
            }
            return;
        case "owner":
            using (SidecarProcessLifetime owned = SidecarProcessLifetime.Start(Self("tree", args[1])))
                await Task.Delay(Timeout.Infinite);
            return;
    }
    throw new ArgumentException("Unknown fixture mode");
}

string root = Path.Combine(Path.GetTempPath(), "Nexus lifecycle " + Guid.NewGuid().ToString("N"));
Directory.CreateDirectory(root);
try
{
    var launch = Self("echo");
    launch.Environment["NEXUS_LIFECYCLE_TEST"] = "unicode 环境 and spaces";
    using (var owned = SidecarProcessLifetime.Start(launch))
    {
        var output = new List<string>();
        var error = new List<string>();
        Task capture = owned.CaptureOutputAsync(output.Add, error.Add);
        await ExpectExited(owned.Process);
        await capture.WaitAsync(TimeSpan.FromSeconds(5));
        if (!output.Contains("unicode 环境 and spaces") || !error.Contains("stderr works")) throw new Exception("Inherited output/environment round trip failed");
        await owned.StopAsync(TimeSpan.FromSeconds(5));
    }
    Console.WriteLine("PASS output/environment/normal exit");

    int port;
    using (var reservation = new TcpListener(IPAddress.Loopback, 0))
    {
        reservation.Start();
        port = ((IPEndPoint)reservation.LocalEndpoint).Port;
    }
    for (int attempt = 0; attempt < 2; attempt++)
    {
        string treePid = Path.Combine(root, $"tree-{attempt}.pid");
        using var owned = SidecarProcessLifetime.Start(Self("tree", treePid, port.ToString()));
        using Process descendant = Process.GetProcessById(await WaitForPid(treePid));
        _ = descendant.Handle;
        await owned.StopAsync(TimeSpan.FromSeconds(5));
        await ExpectExited(descendant);
        using var releasedPort = new TcpListener(IPAddress.Loopback, port);
        releasedPort.Start();
    }
    Console.WriteLine("PASS stop waits for all descendants and permits immediate same-port relaunch");

    string crashPid = Path.Combine(root, "crash.pid");
    using (Process owner = Process.Start(Self("owner", crashPid))!)
    {
        try
        {
            using Process descendant = Process.GetProcessById(await WaitForPid(crashPid));
            _ = descendant.Handle;
            owner.Kill(); // Kill only the owner: kernel Job close must terminate its descendants.
            await ExpectExited(owner);
            await ExpectExited(descendant);
        }
        finally { if (!owner.HasExited) owner.Kill(); }
    }
    Console.WriteLine("PASS owner crash closes Job");
}
finally { Directory.Delete(root, recursive: true); }
