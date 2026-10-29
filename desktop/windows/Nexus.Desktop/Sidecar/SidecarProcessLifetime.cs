// INPUT: Trusted sidecar launch configuration and explicit stop requests.
// OUTPUT: Suspended launch admitted to a kill-on-close Job before any application code runs.
// POS: Native Windows process ownership; redirected pipes and Job handles belong to this object.
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32.SafeHandles;

namespace Nexus.Desktop.Sidecar;

internal sealed class SidecarProcessLifetime : IDisposable
{
    private const uint KillOnJobClose = 0x2000;
    private const uint LaunchFlags = 0x08080404; // SUSPENDED | EXTENDED_STARTUPINFO | UNICODE_ENVIRONMENT | NO_WINDOW
    private const int HandleListAttribute = 0x20002;
    private const int JobListAttribute = 0x2000D;

    private readonly SafeFileHandle job;
    private readonly AnonymousPipeServerStream stdin;
    private readonly bool gracefulStop;
    private readonly AnonymousPipeServerStream stdout;
    private readonly AnonymousPipeServerStream stderr;
    public Process Process { get; }

    private SidecarProcessLifetime(SafeFileHandle job, Process process, AnonymousPipeServerStream stdin, AnonymousPipeServerStream stdout, AnonymousPipeServerStream stderr, bool gracefulStop)
    {
        this.job = job;
        this.stdin = stdin;
        this.gracefulStop = gracefulStop;
        Process = process;
        this.stdout = stdout;
        this.stderr = stderr;
    }

    public static SidecarProcessLifetime Start(ProcessStartInfo info)
    {
        SafeFileHandle job = CreateJobObjectW(IntPtr.Zero, null);
        var stdin = new AnonymousPipeServerStream(PipeDirection.Out, HandleInheritability.Inheritable);
        var stdout = new AnonymousPipeServerStream(PipeDirection.In, HandleInheritability.Inheritable);
        var stderr = new AnonymousPipeServerStream(PipeDirection.In, HandleInheritability.Inheritable);
        IntPtr attributes = IntPtr.Zero, handles = IntPtr.Zero, jobHandle = IntPtr.Zero, environment = IntPtr.Zero;
        ProcessInformation child = default;
        Process? process = null;
        bool attributesInitialized = false;
        try
        {
            if (job.IsInvalid) throw new Win32Exception();
            var limits = new ExtendedLimitInformation();
            limits.Basic.LimitFlags = KillOnJobClose; // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
            if (!SetInformationJobObject(job, 9, ref limits, (uint)Marshal.SizeOf<ExtendedLimitInformation>())) throw new Win32Exception();
            IntPtr size = IntPtr.Zero;
            InitializeProcThreadAttributeList(IntPtr.Zero, 2, 0, ref size);
            attributes = Marshal.AllocHGlobal(size);
            if (!InitializeProcThreadAttributeList(attributes, 2, 0, ref size)) throw new Win32Exception();
            attributesInitialized = true;
            handles = Marshal.AllocHGlobal(3 * IntPtr.Size);
            Marshal.WriteIntPtr(handles, 0, stdin.ClientSafePipeHandle.DangerousGetHandle());
            Marshal.WriteIntPtr(handles, IntPtr.Size, stdout.ClientSafePipeHandle.DangerousGetHandle());
            Marshal.WriteIntPtr(handles, 2 * IntPtr.Size, stderr.ClientSafePipeHandle.DangerousGetHandle());
            if (!UpdateProcThreadAttribute(attributes, 0, (IntPtr)HandleListAttribute, handles, (IntPtr)(3 * IntPtr.Size), IntPtr.Zero, IntPtr.Zero)) throw new Win32Exception();
            // Atomically join the Job during CreateProcess, including a shell crash before ResumeThread.
            jobHandle = Marshal.AllocHGlobal(IntPtr.Size);
            Marshal.WriteIntPtr(jobHandle, job.DangerousGetHandle());
            if (!UpdateProcThreadAttribute(attributes, 0, (IntPtr)JobListAttribute, jobHandle, (IntPtr)IntPtr.Size, IntPtr.Zero, IntPtr.Zero)) throw new Win32Exception();
            var startup = new StartupInfoEx();
            startup.Info.Size = Marshal.SizeOf<StartupInfoEx>();
            startup.Info.Flags = 0x100; // STARTF_USESTDHANDLES
            startup.Info.Input = stdin.ClientSafePipeHandle.DangerousGetHandle();
            startup.Info.Output = stdout.ClientSafePipeHandle.DangerousGetHandle();
            startup.Info.Error = stderr.ClientSafePipeHandle.DangerousGetHandle();
            startup.Attributes = attributes;
            string block = string.Join('\0', info.Environment.OrderBy(pair => pair.Key, StringComparer.OrdinalIgnoreCase).Select(pair => $"{pair.Key}={pair.Value}")) + "\0\0";
            environment = Marshal.StringToHGlobalUni(block);
            var command = new StringBuilder($"\"{info.FileName}\" {info.Arguments}");
            // CREATE_SUSPENDED | EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW
            if (!CreateProcessW(null, command, IntPtr.Zero, IntPtr.Zero, true, LaunchFlags, environment, info.WorkingDirectory, ref startup, out child)) throw new Win32Exception();
            process = Process.GetProcessById((int)child.ProcessId);
            // Open the managed handle while the native handle still pins this process identity.
            _ = process.Handle;
            if (ResumeThread(child.Thread) == uint.MaxValue) throw new Win32Exception();
            stdin.DisposeLocalCopyOfClientHandle();
            stdout.DisposeLocalCopyOfClientHandle();
            stderr.DisposeLocalCopyOfClientHandle();
            return new SidecarProcessLifetime(job, process, stdin, stdout, stderr, info.Environment.TryGetValue("NEXUS_DESKTOP_PARENT_PIPE", out string? parentPipe) && parentPipe == "1");
        }
        catch
        {
            if (child.Process != IntPtr.Zero) TerminateProcess(child.Process, 1);
            process?.Dispose();
            job.Dispose();
            stdin.Dispose();
            stdout.Dispose();
            stderr.Dispose();
            throw;
        }
        finally
        {
            if (child.Thread != IntPtr.Zero) CloseHandle(child.Thread);
            if (child.Process != IntPtr.Zero) CloseHandle(child.Process);
            if (attributesInitialized) DeleteProcThreadAttributeList(attributes);
            if (attributes != IntPtr.Zero) Marshal.FreeHGlobal(attributes);
            if (handles != IntPtr.Zero) Marshal.FreeHGlobal(handles);
            if (jobHandle != IntPtr.Zero) Marshal.FreeHGlobal(jobHandle);
            if (environment != IntPtr.Zero) Marshal.FreeHGlobal(environment);
        }
    }

    public Task CaptureOutputAsync(Action<string> output, Action<string> error) =>
        Task.WhenAll(DrainAsync(stdout, output), DrainAsync(stderr, error));

    private static async Task DrainAsync(Stream stream, Action<string> receive)
    {
        try
        {
            using var reader = new StreamReader(stream, Encoding.UTF8);
            while (await reader.ReadLineAsync() is string line) receive(line);
        }
        catch (Exception error) when (error is IOException or ObjectDisposedException) { }
    }

    public async Task StopAsync(TimeSpan timeout)
    {
        // EOF asks the Go host to drain runtimes and persist retirement before process exit.
        // The Job remains owned until all descendants have stopped, including timeout fallback.
        stdin.Dispose();
        if (gracefulStop)
        {
            using var grace = new CancellationTokenSource(timeout);
            try { await Process.WaitForExitAsync(grace.Token); }
            catch (OperationCanceledException) when (grace.IsCancellationRequested) { }
        }
        if (!TerminateJobObject(job, 1)) throw new Win32Exception();
        using var deadline = new CancellationTokenSource(timeout);
        await Process.WaitForExitAsync(deadline.Token);
        while (true)
        {
            if (!QueryInformationJobObject(job, 1, out BasicAccountingInformation accounting, (uint)Marshal.SizeOf<BasicAccountingInformation>(), IntPtr.Zero)) throw new Win32Exception();
            if (accounting.ActiveProcesses == 0) return;
            await Task.Delay(25, deadline.Token);
        }
    }

    public void Dispose()
    {
        // Also covers abrupt application shutdown; no UI-thread waits or tree enumeration.
        job.Dispose();
        stdin.Dispose();
        stdout.Dispose();
        stderr.Dispose();
        Process.Dispose();
    }

    [StructLayout(LayoutKind.Sequential)] private struct BasicLimitInformation { public long ProcessTime, JobTime; public uint LimitFlags; public UIntPtr MinWorkingSet, MaxWorkingSet; public uint ActiveProcessLimit; public UIntPtr Affinity; public uint PriorityClass, SchedulingClass; }
    [StructLayout(LayoutKind.Sequential)] private struct IoCounters { public ulong ReadOperations, WriteOperations, OtherOperations, ReadBytes, WriteBytes, OtherBytes; }
    [StructLayout(LayoutKind.Sequential)] private struct ExtendedLimitInformation { public BasicLimitInformation Basic; public IoCounters Io; public UIntPtr ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory; }
    [StructLayout(LayoutKind.Sequential)] private struct BasicAccountingInformation { public long UserTime, KernelTime, PeriodUserTime, PeriodKernelTime; public uint PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses; }
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)] private struct StartupInfo { public int Size; public IntPtr Reserved, Desktop, Title; public uint X, Y, XSize, YSize, XChars, YChars, Fill, Flags; public ushort Show, ReservedSize; public IntPtr ReservedBytes, Input, Output, Error; }
    [StructLayout(LayoutKind.Sequential)] private struct StartupInfoEx { public StartupInfo Info; public IntPtr Attributes; }
    [StructLayout(LayoutKind.Sequential)] private struct ProcessInformation { public IntPtr Process, Thread; public uint ProcessId, ThreadId; }
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)] private static extern SafeFileHandle CreateJobObjectW(IntPtr security, string? name);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool SetInformationJobObject(SafeFileHandle job, int kind, ref ExtendedLimitInformation information, uint size);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool QueryInformationJobObject(SafeFileHandle job, int kind, out BasicAccountingInformation information, uint size, IntPtr returned);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool TerminateJobObject(SafeFileHandle job, uint code);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool InitializeProcThreadAttributeList(IntPtr list, int count, int flags, ref IntPtr size);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool UpdateProcThreadAttribute(IntPtr list, uint flags, IntPtr attribute, IntPtr value, IntPtr size, IntPtr previous, IntPtr returned);
    [DllImport("kernel32.dll")] private static extern void DeleteProcThreadAttributeList(IntPtr list);
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)] private static extern bool CreateProcessW(string? application, StringBuilder command, IntPtr processSecurity, IntPtr threadSecurity, bool inherit, uint flags, IntPtr environment, string directory, ref StartupInfoEx startup, out ProcessInformation process);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern uint ResumeThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool TerminateProcess(IntPtr process, uint code);
    [DllImport("kernel32.dll")] private static extern bool CloseHandle(IntPtr handle);
}
