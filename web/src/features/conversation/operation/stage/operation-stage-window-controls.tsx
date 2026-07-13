import { cn } from "@/shared/ui/class-name";

import type { StageWindowState } from "../operation-desktop-types";
import {
  displayStageEventTitle,
} from "../operation-stage-labels";
import {
  iconForWindowKind,
  isLowSignalDirectorValue,
  stageAppLabelForWindowKind,
} from "./operation-stage-helpers";
import { buildStageMinimizedWindowTile } from "./operation-stage-minimized-window";
import {
  buildDockAppSlots,
  groupDockWindowsByApp,
  resolveDockSlotPresentation,
} from "./operation-stage-dock-model";
import { dockIconSkinForKind } from "./operation-stage-app-identity";

export function StageWindowDock({
  activeWindowId,
  onRestoreAll,
  windows,
  onRestore,
}: {
  activeWindowId: string | null;
  onRestoreAll: () => void;
  windows: StageWindowState[];
  onRestore: (window_id: string) => void;
}) {
  if (!windows.length) {
    return null;
  }

  const running_windows = windows.filter((window) => (
    window.phase !== "closed" && window.phase !== "minimized"
  ));
  const minimized_windows = windows.filter((window) => window.phase === "minimized");
  const dock_apps = buildDockAppSlots(
    groupDockWindowsByApp(windows, activeWindowId, stageAppLabelForWindowKind),
  );
  const active_window = running_windows.find((window) => window.id === activeWindowId)
    ?? running_windows[0]
    ?? minimized_windows[0]
    ?? null;
  const active_app_label = active_window ? stageAppLabelForWindowKind(active_window.kind) : "Nexus";

  return (
    <div className="absolute inset-x-4 bottom-5 z-30 flex justify-center max-md:relative max-md:inset-x-auto max-md:bottom-auto max-md:mt-3">
      <div className="flex max-w-full flex-col items-center gap-1.5">
        <div className="operation-window-dock soft-scrollbar flex max-w-full items-end gap-1.5 overflow-x-auto rounded-[18px] border border-white/66 bg-[rgba(248,250,252,0.68)] px-2 py-1.5 shadow-[0_18px_44px_rgba(18,28,42,0.13),inset_0_1px_0_rgba(255,255,255,0.76)] backdrop-blur-2xl">
          <button
            aria-label="恢复 Nexus 工作现场"
            className="grid h-9 w-9 shrink-0 place-items-center rounded-[12px] border border-white/62 bg-[rgba(20,28,38,0.88)] text-[12px] font-black text-white shadow-[0_8px_18px_rgba(18,28,42,0.14)] transition hover:bg-[rgba(20,28,38,0.94)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)]"
            onClick={onRestoreAll}
            title={active_window ? `${active_app_label} · ${display_window_title(active_window)}` : "Nexus"}
            type="button"
          >
            N
          </button>
          <div className="h-8 w-px shrink-0 bg-white/56" />
        {dock_apps.map(({ app_label, count, is_active, is_running, kind, window }) => {
          const Icon = iconForWindowKind(window?.kind ?? kind);
          const window_title = window ? display_window_title(window) : "等待工具调用";
          const presentation = resolveDockSlotPresentation({
            app_label,
            count,
            is_active,
            is_running,
            kind,
            window,
          }, window_title);
          return (
            <button
              aria-label={`${presentation.state_label}：${app_label}`}
              className={cn(
	                "group relative grid shrink-0 place-items-center rounded-[14px] border text-left transition duration-150 ease-out hover:-translate-y-1 focus-visible:-translate-y-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)]",
	                is_active
	                  ? "h-11 w-11 border-[rgba(91,114,255,0.36)] bg-[rgba(91,114,255,0.18)] text-[color:var(--primary)] shadow-[0_12px_26px_rgba(91,114,255,0.18)]"
	                  : is_running
	                    ? "h-9 w-9 border-white/54 bg-white/54 text-(--icon-muted) hover:bg-white/76 hover:text-(--text-strong)"
	                    : "h-9 w-9 border-transparent bg-white/22 text-(--icon-muted) opacity-58 hover:bg-white/46 hover:opacity-84",
              )}
              key={app_label}
              disabled={!window || presentation.is_disabled}
              onClick={() => {
                if (window) {
                  onRestore(window.id);
                }
              }}
              title={presentation.title}
              type="button"
            >
              <span className={cn(
                "relative grid h-7 w-7 shrink-0 place-items-center rounded-[11px] border shadow-[inset_0_1px_0_rgba(255,255,255,0.62),0_7px_16px_rgba(18,28,42,0.10)]",
                dockIconSkinForKind(window?.kind ?? kind),
	                is_active ? "ring-2 ring-[rgba(91,114,255,0.26)]" : is_running ? "ring-1 ring-white/46" : "ring-0",
              )}>
                <Icon className="h-4 w-4" />
                {is_running ? (
                  <span className={cn(
                    "absolute -bottom-0.5 left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full border border-white/72 transition",
                    is_active
                      ? "bg-[color:var(--primary)]"
                      : "bg-[rgba(47,184,132,0.72)]",
                  )} />
                ) : null}
                {count > 1 ? (
                  <span className="absolute -right-1 -top-1 grid h-4 min-w-4 place-items-center rounded-full border border-white/80 bg-[rgba(20,28,38,0.72)] px-1 text-[8px] font-black leading-none text-white shadow-[0_4px_10px_rgba(18,28,42,0.18)]">
                    {count}
                  </span>
                ) : null}
              </span>
              <span className={cn(
                  "absolute -bottom-2 left-1/2 h-1.5 -translate-x-1/2 rounded-full transition",
                  is_active
                    ? "w-5 bg-[color:var(--primary)]"
                    : is_running
                      ? "w-2 bg-[rgba(47,184,132,0.54)]"
                      : "w-0 bg-transparent",
              )} />
              <span className="pointer-events-none absolute bottom-[calc(100%+10px)] left-1/2 hidden max-w-[230px] -translate-x-1/2 whitespace-nowrap rounded-[10px] border border-white/70 bg-[rgba(20,28,38,0.82)] px-2.5 py-1.5 text-[10px] font-semibold text-white shadow-[0_12px_30px_rgba(18,28,42,0.22)] backdrop-blur-xl group-hover:block group-focus-visible:block">
                <span className="block max-w-[160px] truncate">{app_label}</span>
                <span className="block text-[9px] font-medium text-white/66">
                  {count > 1 ? `${count} 个窗口` : window_title} · {presentation.state_label}
                </span>
              </span>
            </button>
          );
        })}
        {minimized_windows.length ? (
          <>
            <div className="h-8 w-px shrink-0 bg-white/56" />
            {minimized_windows.map((window) => {
              const Icon = iconForWindowKind(window.kind);
              const app_label = stageAppLabelForWindowKind(window.kind);
              const window_title = display_window_title(window);
              const tile = buildStageMinimizedWindowTile({
                app_label,
                title: window_title,
              });
              return (
                <button
                  aria-label={tile.aria_label}
	                  className="operation-window-dock-minimized group relative grid h-9 w-12 shrink-0 place-items-center overflow-hidden rounded-[11px] border border-[rgba(223,157,46,0.24)] bg-[rgba(255,249,236,0.58)] text-(--icon-muted) shadow-[inset_0_1px_0_rgba(255,255,255,0.70)] transition duration-150 ease-out hover:-translate-y-1 hover:bg-white/72 hover:text-(--text-strong) focus-visible:-translate-y-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.42)]"
                  key={window.id}
                  onClick={() => onRestore(window.id)}
                  title={tile.title}
                  type="button"
                >
                  <div className="absolute inset-x-1 top-1 h-3 rounded-[8px] border border-white/54 bg-white/48" />
                  <Icon className="relative z-10 h-4 w-4" />
                  <span className="absolute bottom-1 left-1/2 h-1.5 w-2 -translate-x-1/2 rounded-full bg-[rgba(223,157,46,0.78)]" />
                  <span className="pointer-events-none absolute bottom-[calc(100%+10px)] left-1/2 hidden max-w-[230px] -translate-x-1/2 whitespace-nowrap rounded-[10px] border border-white/70 bg-[rgba(20,28,38,0.82)] px-2.5 py-1.5 text-[10px] font-semibold text-white shadow-[0_12px_30px_rgba(18,28,42,0.22)] backdrop-blur-xl group-hover:block group-focus-visible:block">
                    <span className="block max-w-[160px] truncate">{window_title}</span>
                    <span className="block text-[9px] font-medium text-white/66">{app_label} · 已最小化</span>
                  </span>
                </button>
              );
            })}
          </>
        ) : null}
        </div>
      </div>
    </div>
  );
}

function display_window_title(window: StageWindowState): string {
  if (!isLowSignalDirectorValue(window.title)) {
    return window.title;
  }
  return displayStageEventTitle(window.payload.event, stageAppLabelForWindowKind(window.kind));
}
