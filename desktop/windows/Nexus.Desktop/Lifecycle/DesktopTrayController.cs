using System.Drawing;
using System.Drawing.Drawing2D;
using Forms = System.Windows.Forms;
using Nexus.Desktop.Diagnostics;

namespace Nexus.Desktop.Lifecycle;

internal sealed class DesktopTrayController : IDisposable
{
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly Action restoreWindow;
    private readonly Action reloadWindow;
    private readonly Action checkForUpdates;
    private readonly Action exitApplication;
    private readonly Forms.ContextMenuStrip contextMenu;
    private readonly Icon icon;
    private readonly Forms.NotifyIcon notifyIcon;
    private bool disposed;

    public DesktopTrayController(
        DesktopStartupTimeline startupTimeline,
        Action restoreWindow,
        Action reloadWindow,
        Action checkForUpdates,
        Action exitApplication)
    {
        this.startupTimeline = startupTimeline;
        this.restoreWindow = restoreWindow;
        this.reloadWindow = reloadWindow;
        this.checkForUpdates = checkForUpdates;
        this.exitApplication = exitApplication;

        contextMenu = new Forms.ContextMenuStrip
        {
            BackColor = Color.FromArgb(250, 251, 255),
            Font = new Font("Segoe UI", 9F, FontStyle.Regular),
            Padding = new Forms.Padding(8, 8, 8, 8),
            Renderer = new TrayMenuRenderer(),
            ShowImageMargin = false,
        };

        Forms.ToolStripLabel titleItem = new("Nexus")
        {
            Font = new Font("Segoe UI", 10F, FontStyle.Bold),
            ForeColor = Color.FromArgb(25, 35, 58),
            Padding = new Forms.Padding(8, 4, 8, 6),
        };
        Forms.ToolStripMenuItem openItem = MenuItem("打开 Nexus");
        openItem.Click += (_, _) => RestoreWindow();
        Forms.ToolStripMenuItem reloadItem = MenuItem("重新载入界面");
        reloadItem.Click += (_, _) => ReloadWindow();
        Forms.ToolStripMenuItem updateItem = MenuItem("检查更新");
        updateItem.Click += (_, _) => CheckForUpdates();
        Forms.ToolStripMenuItem exitItem = MenuItem("退出 Nexus");
        exitItem.Click += (_, _) => ExitApplication();

        contextMenu.Items.Add(titleItem);
        contextMenu.Items.Add(new Forms.ToolStripSeparator());
        contextMenu.Items.Add(openItem);
        contextMenu.Items.Add(reloadItem);
        contextMenu.Items.Add(updateItem);
        contextMenu.Items.Add(new Forms.ToolStripSeparator());
        contextMenu.Items.Add(exitItem);

        icon = LoadIcon();
        notifyIcon = new Forms.NotifyIcon
        {
            ContextMenuStrip = contextMenu,
            Icon = icon,
            Text = "Nexus 正在后台运行",
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

    private static Forms.ToolStripMenuItem MenuItem(string text) => new(text)
    {
        AutoSize = true,
        ForeColor = Color.FromArgb(35, 44, 66),
        Margin = new Forms.Padding(2, 1, 2, 1),
        Padding = new Forms.Padding(10, 7, 34, 7),
    };

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

    private void CheckForUpdates()
    {
        startupTimeline.Mark("tray.update_check_clicked");
        checkForUpdates();
    }

    private void ReloadWindow()
    {
        startupTimeline.Mark("tray.reload_requested");
        reloadWindow();
    }

    private void ExitApplication()
    {
        startupTimeline.Mark("tray.exit_requested");
        exitApplication();
    }

    private sealed class TrayMenuRenderer : Forms.ToolStripProfessionalRenderer
    {
        private static readonly Color BorderColor = Color.FromArgb(222, 226, 236);
        private static readonly Color HoverColor = Color.FromArgb(235, 239, 255);
        private static readonly Color SeparatorColor = Color.FromArgb(229, 232, 240);

        protected override void OnRenderToolStripBorder(Forms.ToolStripRenderEventArgs e)
        {
            using Pen pen = new(BorderColor);
            Rectangle bounds = new(0, 0, e.ToolStrip.Width - 1, e.ToolStrip.Height - 1);
            e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
            using GraphicsPath path = RoundedRectangle(bounds, 10);
            e.Graphics.DrawPath(pen, path);
        }

        protected override void OnRenderMenuItemBackground(Forms.ToolStripItemRenderEventArgs e)
        {
            if (!e.Item.Selected || !e.Item.Enabled)
            {
                return;
            }

            Rectangle bounds = new(4, 1, e.Item.Width - 8, e.Item.Height - 2);
            e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
            using SolidBrush brush = new(HoverColor);
            using GraphicsPath path = RoundedRectangle(bounds, 8);
            e.Graphics.FillPath(brush, path);
        }

        protected override void OnRenderSeparator(Forms.ToolStripSeparatorRenderEventArgs e)
        {
            using Pen pen = new(SeparatorColor);
            int y = e.Item.Height / 2;
            e.Graphics.DrawLine(pen, 8, y, e.Item.Width - 8, y);
        }

        private static GraphicsPath RoundedRectangle(Rectangle bounds, int radius)
        {
            int diameter = radius * 2;
            GraphicsPath path = new();
            path.AddArc(bounds.Left, bounds.Top, diameter, diameter, 180, 90);
            path.AddArc(bounds.Right - diameter, bounds.Top, diameter, diameter, 270, 90);
            path.AddArc(bounds.Right - diameter, bounds.Bottom - diameter, diameter, diameter, 0, 90);
            path.AddArc(bounds.Left, bounds.Bottom - diameter, diameter, diameter, 90, 90);
            path.CloseFigure();
            return path;
        }
    }
}
