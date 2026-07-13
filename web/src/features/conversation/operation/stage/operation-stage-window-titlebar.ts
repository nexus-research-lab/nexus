export interface StageWindowTitlebarState {
  aria_label: string;
  close_label: string;
  minimize_label: string;
  proxy_label: string | null;
  state_dot_title: string;
  state_dot_tone: "active" | "background" | "minimized";
  status_label: string;
  title_label: string;
  zoom_label: string;
  zoom_title: string;
}

export function buildStageWindowTitlebarState({
  app_label,
  focused,
  maximized,
  minimized,
  title,
}: {
  app_label?: string | null;
  focused: boolean;
  maximized: boolean;
  minimized: boolean;
  title: string;
}): StageWindowTitlebarState {
  const title_label = app_label ? `${app_label} · ${title}` : title;
  const status_label = minimized ? "已最小化" : focused ? "前台" : "后台";
  return {
    aria_label: app_label ? `${app_label} window: ${title}` : title,
    close_label: `关闭 ${title}`,
    minimize_label: `最小化 ${title}`,
    proxy_label: app_label ?? null,
    state_dot_title: `${title} · ${status_label}`,
    state_dot_tone: minimized ? "minimized" : focused ? "active" : "background",
    status_label,
    title_label,
    zoom_label: `${maximized ? "还原" : "缩放"} ${title}`,
    zoom_title: maximized ? "还原窗口" : "缩放窗口",
  };
}
