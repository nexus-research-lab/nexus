using System.Drawing;
using Forms = System.Windows.Forms;
using Nexus.Desktop.Diagnostics;

namespace Nexus.Desktop.Lifecycle;

internal sealed class DesktopTrayController : IDisposable
{
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly Action restoreWindow;
    private readonly Action exitApplication;
    private readonly Forms.ContextMenuStrip contextMenu;
    private readonly Icon icon;
    private readonly Forms.NotifyIcon notifyIcon;
    private bool disposed;

    public DesktopTrayController(
        DesktopStartupTimeline startupTimeline,
        Action restoreWindow,
        Action exitApplication)
    {
        this.startupTimeline = startupTimeline;
        this.restoreWindow = restoreWindow;
        this.exitApplication = exitApplication;

        contextMenu = new Forms.ContextMenuStrip();
        Forms.ToolStripMenuItem openItem = new("打开 Nexus");
        openItem.Click += (_, _) => RestoreWindow();
        Forms.ToolStripMenuItem exitItem = new("退出 Nexus");
        exitItem.Click += (_, _) => ExitApplication();
        contextMenu.Items.Add(openItem);
        contextMenu.Items.Add(new Forms.ToolStripSeparator());
        contextMenu.Items.Add(exitItem);

        icon = LoadIcon();
        notifyIcon = new Forms.NotifyIcon
        {
            ContextMenuStrip = contextMenu,
            Icon = icon,
            Text = "Nexus",
            Visible = true,
        };
        notifyIcon.MouseClick += HandleMouseClick;
    }

    public void Dispose()
    {
        if (disposed)
        {
            return;
        }

        disposed = true;
        notifyIcon.MouseClick -= HandleMouseClick;
        notifyIcon.Visible = false;
        notifyIcon.Dispose();
        contextMenu.Dispose();
        icon.Dispose();
    }

    private static Icon LoadIcon()
    {
        string processPath = Environment.ProcessPath ?? string.Empty;
        try
        {
            if (!string.IsNullOrWhiteSpace(processPath) && System.IO.File.Exists(processPath))
            {
                Icon? appIcon = Icon.ExtractAssociatedIcon(processPath);
                if (appIcon is not null)
                {
                    return appIcon;
                }
            }
        }
        catch
        {
        }

        return (Icon)SystemIcons.Application.Clone();
    }

    private void HandleMouseClick(object? sender, Forms.MouseEventArgs e)
    {
        if (e.Button == Forms.MouseButtons.Left)
        {
            RestoreWindow();
        }
    }

    private void RestoreWindow()
    {
        startupTimeline.Mark("tray.restore_requested");
        restoreWindow();
    }

    private void ExitApplication()
    {
        startupTimeline.Mark("tray.exit_requested");
        exitApplication();
    }
}
