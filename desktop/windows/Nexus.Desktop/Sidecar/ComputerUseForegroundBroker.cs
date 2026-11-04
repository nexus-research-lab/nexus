// INPUT: One owned backend process and its direct CUA child over a private local pipe.
// OUTPUT: Best-effort Windows foreground eligibility for that exact live child.
// POS: Native host handoff only; target/session authority remains in the CUA runtime.
using System.Buffers.Binary;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace Nexus.Desktop.Sidecar;

internal sealed class ComputerUseForegroundBroker : IDisposable
{
    private readonly CancellationTokenSource stop = new();
    private readonly Func<Process?> backend;
    private readonly Task worker;
    internal string PipeName { get; } = "NexusCUAForeground-" + Guid.NewGuid().ToString("N");

    internal ComputerUseForegroundBroker(Func<Process?> backend)
    {
        this.backend = backend;
        worker = Task.Run(RunAsync);
    }

    private async Task RunAsync()
    {
        while (!stop.IsCancellationRequested)
        {
            try
            {
                using var pipe = new NamedPipeServerStream(PipeName, PipeDirection.InOut, 1,
                    PipeTransmissionMode.Byte, PipeOptions.Asynchronous | PipeOptions.CurrentUserOnly,
                    4, 1);
                await pipe.WaitForConnectionAsync(stop.Token);
                using var deadline = CancellationTokenSource.CreateLinkedTokenSource(stop.Token);
                deadline.CancelAfter(TimeSpan.FromMilliseconds(750));
                byte[] request = new byte[4];
                await pipe.ReadExactlyAsync(request, deadline.Token);
                bool granted = TryGrant(pipe.SafePipeHandle, BinaryPrimitives.ReadUInt32LittleEndian(request));
                await pipe.WriteAsync(new byte[] { granted ? (byte)1 : (byte)0 }, deadline.Token);
            }
            catch (OperationCanceledException) { }
            catch (IOException) { }
            catch (UnauthorizedAccessException) { }
        }
    }

    private bool TryGrant(SafePipeHandle pipe, uint childId)
    {
        // Never accept ASFW_ANY, a web-supplied PID, or another process's child.
        Process? owner = backend();
        if (owner is null || childId == 0 || childId > int.MaxValue) return false;
        try
        {
            if (owner.HasExited || !GetNamedPipeClientProcessId(pipe, out uint caller)
                || caller != owner.Id) return false;
            using Process child = Process.GetProcessById((int)childId);
            if (!IsOwnedChild(owner, child)) return false;
            // The OS still decides whether this host currently has eligibility.
            // Do not simulate input, attach input queues, or activate the host.
            return AllowSetForegroundWindow(childId);
        }
        catch (ArgumentException) { return false; }
        catch (InvalidOperationException) { return false; }
        catch (System.ComponentModel.Win32Exception) { return false; }
    }

    internal static bool IsOwnedChild(Process owner, Process child)
    {
        try
        {
            return !owner.HasExited && !child.HasExited && owner.Id != child.Id
                && child.StartTime.ToUniversalTime() >= owner.StartTime.ToUniversalTime()
                && NtQueryInformationProcess(child.SafeHandle, 0, out ProcessBasicInformation info,
                    Marshal.SizeOf<ProcessBasicInformation>(), out _) == 0
                && info.ParentProcessId.ToInt64() == owner.Id;
        }
        catch (InvalidOperationException) { return false; }
        catch (System.ComponentModel.Win32Exception) { return false; }
    }

    public void Dispose()
    {
        stop.Cancel();
        // Closing the cancellation source waits for no I/O on the UI thread.
        _ = worker.ContinueWith(_ => stop.Dispose(), TaskScheduler.Default);
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct ProcessBasicInformation
    {
        public IntPtr Reserved1, PebBaseAddress, Reserved2, Reserved3, ProcessId, ParentProcessId;
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetNamedPipeClientProcessId(SafePipeHandle pipe, out uint processId);
    [DllImport("ntdll.dll")]
    private static extern int NtQueryInformationProcess(SafeProcessHandle process, int kind,
        out ProcessBasicInformation information, int length, out int returned);
    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AllowSetForegroundWindow(uint processId);
}
