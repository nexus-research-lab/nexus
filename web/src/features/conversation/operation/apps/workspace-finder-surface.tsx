/**
 * INPUT: One workspace operation event, its live file activity, and a file-open command.
 * OUTPUT: Responsive, interactive Agent workspace file management.
 * POS: Operation Stage Files view; resource loading and window creation stay outside this component.
 */
import {
  ArrowUpRight,
  ChevronRight,
  FolderOpen,
  FolderTree,
  History,
  List,
  LoaderCircle,
  RefreshCw,
  Search,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

import { cn } from "@/shared/ui/class-name";

import type { StageWindowState } from "../operation-desktop-types";
import type { NexusOperationEvent } from "../operation-types";
import { PHASE_LABELS } from "../operation-tool-catalog";
import { finderFileKindLabel } from "./finder-item-details";
import {
  buildFinderSessionView,
  type FinderViewScope,
  workspaceStatusLabel,
} from "./finder-session";
import {
  type OperationWorkspaceFilesStatus,
  useOperationWorkspaceFiles,
} from "./use-operation-workspace-files";
import {
  formatWorkspaceFileTime,
  workspaceFileIcon,
} from "./workspace-finder-presentation";
import { WorkspaceFinderRow } from "./workspace-finder-row";

export function WorkspaceFinder({
  activePath,
  event,
  items,
  onOpenFile,
}: {
  activePath?: string | null;
  event: NexusOperationEvent;
  items: NonNullable<StageWindowState["payload"]["workspace_items"]>;
  onOpenFile?: (path: string) => void;
}) {
  const root_ref = useRef<HTMLDivElement | null>(null);
  const [selected_path, set_selected_path] = useState<string | null>(null);
  const [query, set_query] = useState("");
  const [scope, set_scope] = useState<FinderViewScope>("workspace");
  const [collapsed_paths, set_collapsed_paths] = useState<Set<string>>(() => new Set());
  const [container_width, set_container_width] = useState(0);
  const { files, reload, status } = useOperationWorkspaceFiles(
    event.agent_id,
    `${event.id}:${event.phase}`,
  );
  const finder_session = buildFinderSessionView({
    active_path: selected_path ?? activePath,
    event,
    files,
    items,
    query,
    scope,
  });
  const visible_rows = finder_session.rows.filter((row) => (
    !row.path.split("/").slice(0, -1).some((_, index, parts) => (
      collapsed_paths.has(parts.slice(0, index + 1).join("/"))
    ))
  ));
  const show_navigation = container_width >= 680;
  const show_inspector = container_width >= 820;
  const compact_list = container_width < 520;
  const selected_status = finder_session.selected_item?.status;
  const can_open_selected = Boolean(
    onOpenFile
    && finder_session.selected_entry
    && !finder_session.selected_entry.is_dir
    && selected_status !== "deleted",
  );

  const open_selected_file = () => {
    if (can_open_selected) {
      onOpenFile?.(finder_session.selected_path);
    }
  };

  const toggle_folder = (path: string) => {
    set_collapsed_paths((current) => {
      const next = new Set(current);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  useEffect(() => {
    const element = root_ref.current;
    if (!element) {
      return;
    }
    const update_width = () => set_container_width(element.clientWidth);
    update_width();
    const observer = new ResizeObserver(update_width);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={root_ref} className="flex min-h-0 flex-1 overflow-hidden bg-[#f8fafc]">
      <div className={cn(
        "w-40 shrink-0 border-r border-(--divider-subtle-color) bg-[#eef3f8]/88 p-2 text-[11px] font-semibold text-(--text-soft)",
        !show_navigation && "hidden",
      )}>
        <div className="px-2 pb-1 pt-1 text-[9px] font-black uppercase tracking-[0.14em] text-(--text-soft)">工作区</div>
        <FinderSidebarItem
          active={scope === "workspace"}
          icon={FolderTree}
          label="全部文件"
          onClick={() => set_scope("workspace")}
          value={String(finder_session.entries.filter((entry) => !entry.is_dir).length)}
        />
        <FinderSidebarItem
          active={scope === "changes"}
          icon={History}
          label="本轮变更"
          onClick={() => set_scope("changes")}
          value={String(finder_session.changed_count)}
        />
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) bg-white/64 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <span
              aria-label="列表视图"
              className="grid h-7 w-7 shrink-0 place-items-center rounded-[8px] border border-(--divider-subtle-color) bg-white text-(--text-strong)"
              title="列表视图"
            >
              <List className="h-3.5 w-3.5" />
            </span>
            <div className="min-w-0">
              <p className="truncate text-[11px] font-black text-(--text-strong)">
                {scope === "changes" ? "本轮变更" : "Nexus Workspace"}
              </p>
              <p className="truncate text-[10px] text-(--text-soft)">
                {finder_session.item_count} 个文件
              </p>
            </div>
          </div>
          <label className="flex min-w-0 max-w-[280px] flex-1 items-center rounded-[8px] border border-(--divider-subtle-color) bg-white/72 px-2 py-1 text-[10px] text-(--text-soft)">
            <Search className="mr-1.5 h-3 w-3 shrink-0" />
            <input
              aria-label="搜索工作区"
              className="min-w-0 flex-1 bg-transparent text-[10.5px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
              onChange={(input_event) => set_query(input_event.target.value)}
              placeholder="搜索"
              type="search"
              value={query}
            />
          </label>
          <FinderSyncIndicator phase={event.phase} status={status} />
          <FinderToolbarButton label="刷新文件" onClick={() => void reload()}>
            <RefreshCw className={cn("h-3.5 w-3.5", status === "loading" && "animate-spin")} />
          </FinderToolbarButton>
        </div>
        <div className={cn(
          "grid min-h-0 flex-1",
          show_inspector ? "grid-cols-[minmax(0,1fr)_220px]" : "grid-cols-1",
        )}>
          <div className="soft-scrollbar min-h-0 overflow-auto p-2">
            <div className={cn(
              "grid gap-2 px-2 pb-1 text-[9px] font-bold uppercase tracking-[0.12em] text-(--text-soft)",
              compact_list ? "grid-cols-1" : "grid-cols-[minmax(0,1fr)_72px_86px]",
            )}>
              <span>名称</span>
              {compact_list ? null : <span>标签</span>}
              {compact_list ? null : <span>修改时间</span>}
            </div>
            {visible_rows.length ? visible_rows.map((row) => (
              <WorkspaceFinderRow
                active={row.path === finder_session.selected_path}
                collapsed={collapsed_paths.has(row.path)}
                compact={compact_list}
                depth={row.depth}
                entry={finder_session.entries.find((entry) => entry.path === row.path)}
                item={finder_session.display_items.find((item) => item.path === row.path)}
                key={row.path}
                label={row.label}
                onOpen={row.type === "file" && onOpenFile ? () => onOpenFile(row.path) : undefined}
                onSelect={() => set_selected_path(row.path)}
                onToggle={() => toggle_folder(row.path)}
                path={row.path}
                type={row.type}
              />
            )) : (
              <div className="grid min-h-36 place-items-center px-6 text-center text-[11px] text-(--text-soft)">
                {query ? "没有匹配的文件" : scope === "changes" ? "本轮暂无文件变更" : "工作区为空"}
              </div>
            )}
          </div>
          <aside className={cn(
            "min-h-0 border-l border-(--divider-subtle-color) bg-white/54 p-3",
            !show_inspector && "hidden",
          )}>
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">信息</p>
            <div className="mt-3 grid h-16 w-16 place-items-center rounded-[16px] border border-(--divider-subtle-color) bg-white/74 text-(--icon-default)">
              {(() => {
                const Icon = workspaceFileIcon(finder_session.selected_path);
                return <Icon className="h-7 w-7" />;
              })()}
            </div>
            <p className="mt-3 line-clamp-2 text-[12px] font-black text-(--text-strong)">
              {basename(finder_session.selected_path)}
            </p>
            <p className="truncate text-[10px] text-(--text-soft)">
              {finderFileKindLabel(finder_session.selected_path)}
            </p>
            <button
              className="mt-3 inline-flex h-8 items-center gap-1.5 rounded-[8px] bg-[color:var(--primary)] px-3 text-[10.5px] font-bold text-white transition hover:brightness-105 disabled:cursor-default disabled:opacity-35"
              disabled={!can_open_selected}
              onClick={open_selected_file}
              type="button"
            >
              <ArrowUpRight className="h-3.5 w-3.5" />
              打开
            </button>
            <div className="mt-4 space-y-2 border-t border-(--divider-subtle-color) pt-3">
              <FinderInspectorRow label="状态" value={finder_session.selected_item ? workspaceStatusLabel(finder_session.selected_item.status) : "未变更"} />
              <FinderInspectorRow label="位置" value={finder_session.selected_path} />
              <FinderInspectorRow label="大小" value={format_file_size(finder_session.selected_entry?.size)} />
              <FinderInspectorRow label="修改时间" value={formatWorkspaceFileTime(
                finder_session.selected_entry?.modified_at ?? finder_session.selected_item?.updated_at,
              )} />
              {finder_session.selected_item ? (
                <FinderInspectorRow label="版本" value={`v${finder_session.selected_item.version}`} />
              ) : null}
            </div>
            {finder_session.previewLines.length ? (
              <div className="mt-4 overflow-hidden rounded-[11px] border border-(--divider-subtle-color) bg-[#101820] p-2 font-mono text-[10px] leading-4 text-[#dce8ee]">
                {finder_session.previewLines.map((line, index) => (
                  <div className="flex min-w-0 gap-2" key={`${index}:${line}`}>
                    <span className="w-5 shrink-0 select-none text-right text-[#6f8190]">{index + 1}</span>
                    <span className="min-w-0 truncate">{line}</span>
                  </div>
                ))}
              </div>
            ) : null}
          </aside>
        </div>
        <FinderPathBar
          changedCount={finder_session.changed_count}
          compact={compact_list}
          itemCount={finder_session.item_count}
          pathParts={finder_session.path_parts}
        />
      </div>
    </div>
  );
}

function FinderInspectorRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">{label}</p>
      <p className="mt-0.5 truncate text-[10.5px] font-semibold text-(--text-strong)" title={value}>{value}</p>
    </div>
  );
}

function FinderSyncIndicator({
  phase,
  status,
}: {
  phase: NexusOperationEvent["phase"];
  status: OperationWorkspaceFilesStatus;
}) {
  if (status === "ready" && phase !== "running" && phase !== "error") {
    return null;
  }
  const label = status === "loading"
    ? "刷新中"
    : status === "unavailable"
      ? "暂不可用"
      : phase === "running"
        ? "同步中"
        : "同步失败";
  return (
    <span
      className={cn(
        "hidden shrink-0 items-center gap-1 rounded-[8px] px-1.5 py-1 text-[10px] font-semibold md:inline-flex",
        phase === "running" || status === "loading"
          ? "bg-[rgba(91,114,255,0.08)] text-[color:var(--primary)]"
          : "bg-white/42 text-(--text-soft)",
      )}
      title={`${PHASE_LABELS[phase]} · ${label}`}
    >
      {status === "loading" ? (
        <LoaderCircle className="h-3 w-3 animate-spin" />
      ) : (
        <span className={cn(
          "h-1.5 w-1.5 rounded-full",
          phase === "running" && "operation-focus-dot bg-[color:var(--primary)]",
          (phase === "error" || status === "unavailable") && "bg-[color:var(--destructive)]",
        )} />
      )}
      <span>{label}</span>
    </span>
  );
}

function FinderToolbarButton({
  children,
  label,
  onClick,
}: {
  children: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-label={label}
      className={cn(
        "grid h-7 w-7 shrink-0 place-items-center rounded-[8px] border transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
        "border-transparent bg-white/42 text-(--icon-muted) hover:bg-white/76 hover:text-(--text-strong)",
      )}
      onClick={onClick}
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}

function FinderSidebarItem({
  active = false,
  icon: Icon,
  label,
  onClick,
  value,
}: {
  active?: boolean;
  icon: LucideIcon;
  label: string;
  onClick: () => void;
  value: string;
}) {
  return (
    <button
      className={cn(
      "flex w-full items-center gap-2 rounded-[9px] px-2 py-1.5 text-left transition",
      active ? "bg-white/78 text-(--text-strong) shadow-[inset_0_1px_0_rgba(255,255,255,0.64)]" : "text-(--text-soft)",
      )}
      onClick={onClick}
      type="button"
    >
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span className="text-[9px] tabular-nums text-(--text-soft)">{value}</span>
    </button>
  );
}

function FinderPathBar({
  changedCount,
  compact,
  itemCount,
  pathParts,
}: {
  changedCount: number;
  compact: boolean;
  itemCount: number;
  pathParts: string[];
}) {
  return (
    <div className="flex min-h-8 items-center justify-between gap-3 border-t border-(--divider-subtle-color) bg-white/58 px-3 py-1.5 text-[10px] text-(--text-soft)">
      <div className="flex min-w-0 items-center gap-1.5">
        <FolderOpen className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
        <span className="shrink-0 font-semibold text-(--text-strong)">workspace</span>
        {(compact ? pathParts.slice(-1) : pathParts).map((part, index) => (
          <span className="flex min-w-0 items-center gap-1.5" key={`${index}:${part}`}>
            <ChevronRight className="h-3 w-3 shrink-0 text-(--icon-muted)" />
            <span className="max-w-[120px] truncate">{part}</span>
          </span>
        ))}
      </div>
      <span className="shrink-0">
        {itemCount} 个文件{changedCount ? ` · ${changedCount} 个变更` : ""}
      </span>
    </div>
  );
}

function basename(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? path;
}

function format_file_size(size?: number | null): string {
  if (size == null || size < 0) {
    return "--";
  }
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(size < 10 * 1024 ? 1 : 0)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}
