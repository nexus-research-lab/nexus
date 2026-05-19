using System.IO.Pipes;
using System.Text;
using Nexus.Desktop.Diagnostics;

namespace Nexus.Desktop.Lifecycle;

internal sealed class DesktopSingleInstanceCoordinator : IDisposable
{
    private const string MutexName = @"Local\NexusDesktop";
    private const string PipeName = "NexusDesktopActivation";

    private readonly DesktopStartupTimeline startupTimeline;
    private readonly Mutex mutex;
    private readonly CancellationTokenSource cancellation = new();
    private Task? serverTask;

    public DesktopSingleInstanceCoordinator(DesktopStartupTimeline startupTimeline)
    {
        this.startupTimeline = startupTimeline;
        mutex = new Mutex(initiallyOwned: true, MutexName, out bool createdNew);
        IsPrimary = createdNew;
        startupTimeline.Mark("single_instance.resolved", new Dictionary<string, string>
        {
            ["primary"] = IsPrimary ? "true" : "false",
        });
    }

    public bool IsPrimary { get; }

    public void StartServer(Func<string, Task> handleActivationAsync)
    {
        if (!IsPrimary)
        {
            return;
        }

        serverTask = Task.Run(async () =>
        {
            while (!cancellation.IsCancellationRequested)
            {
                await using var pipe = new NamedPipeServerStream(
                    PipeName,
                    PipeDirection.In,
                    maxNumberOfServerInstances: 1,
                    PipeTransmissionMode.Byte,
                    PipeOptions.Asynchronous);
                try
                {
                    await pipe.WaitForConnectionAsync(cancellation.Token);
                    using StreamReader reader = new(pipe, Encoding.UTF8);
                    string message = await reader.ReadToEndAsync();
                    startupTimeline.Mark("single_instance.activation_received");
                    await handleActivationAsync(message);
                }
                catch (OperationCanceledException)
                {
                    return;
                }
                catch (IOException exception)
                {
                    startupTimeline.Mark("single_instance.pipe_error", new Dictionary<string, string>
                    {
                        ["error"] = exception.Message,
                    });
                }
            }
        }, cancellation.Token);
    }

    public async Task NotifyPrimaryAsync(string message)
    {
        try
        {
            await using var pipe = new NamedPipeClientStream(".", PipeName, PipeDirection.Out, PipeOptions.Asynchronous);
            await pipe.ConnectAsync(1000, cancellation.Token);
            byte[] bytes = Encoding.UTF8.GetBytes(message);
            await pipe.WriteAsync(bytes, cancellation.Token);
            await pipe.FlushAsync(cancellation.Token);
            startupTimeline.Mark("single_instance.activation_sent");
        }
        catch (Exception exception) when (exception is IOException or TimeoutException or OperationCanceledException)
        {
            startupTimeline.Mark("single_instance.activation_send_failed", new Dictionary<string, string>
            {
                ["error"] = exception.Message,
            });
        }
    }

    public void Dispose()
    {
        cancellation.Cancel();
        try
        {
            if (IsPrimary)
            {
                mutex.ReleaseMutex();
            }
        }
        catch (ApplicationException)
        {
        }
        try
        {
            serverTask?.Wait(500);
        }
        catch (AggregateException)
        {
        }
        mutex.Dispose();
        cancellation.Dispose();
    }
}
