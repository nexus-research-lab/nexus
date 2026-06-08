import {
  ChevronDown,
  ChevronRight,
  Columns3,
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
  FolderOpen,
  HardDrive,
  List,
  Search,
  SquareStack,
  Tag,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

import type { StageWindowState } from "../operation-desktop-types";
import type { NexusOperationEvent } from "../operation-types";
import { PHASE_LABELS } from "../operation-tool-catalog";
import { finder_file_kind_label } from "./finder-item-details";
import {
  build_finder_session_view,
  workspace_status_label,
} from "./finder-session";

export function WorkspaceFinder({
  active_path,
  event,
  items,
}: {
  active_path?: string | null;
  event: NexusOperationEvent;
  items: NonNullable<StageWindowState["payload"]["workspace_items"]>;
}) {
  const root_ref = useRef<HTMLDivElement | null>(null);
  const [selected_path, set_selected_path] = useState<string | null>(null);
  const [container_width, set_container_width] = useState(0);
  const finder_session = build_finder_session_view({
    active_path: selected_path ?? active_path,
    event,
    items,
  });
  const show_navigation = container_width >= 660;
  const show_inspector = container_width >= 760;

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
        <div className="px-2 pb-1 pt-1 text-[9px] font-black uppercase tracking-[0.14em] text-(--text-soft)">收藏</div>
        <FinderSidebarItem icon={Search} label="最近项目" />
        <FinderSidebarItem icon={FileText} label="文稿" />
        <FinderSidebarItem active icon={FolderOpen} label="Nexus Workspace" />
        <div className="mt-4 px-2 text-[9px] font-black uppercase tracking-[0.14em] text-(--text-soft)">位置</div>
        <FinderSidebarItem icon={HardDrive} label="Macintosh HD" />
        <div className="mt-1 rounded-[9px] px-2 py-1.5 text-[10px] text-(--text-muted)">~/workspace</div>
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) bg-white/64 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <FinderToolbarButton label="列表视图" active>
              <List className="h-3.5 w-3.5" />
            </FinderToolbarButton>
            <FinderToolbarButton label="分栏视图">
              <Columns3 className="h-3.5 w-3.5" />
            </FinderToolbarButton>
            <div className="min-w-0">
              <p className="truncate text-[11px] font-black text-(--text-strong)">
                {basename(finder_session.selected_path) || "Nexus Workspace"}
              </p>
              <p className="truncate text-[10px] text-(--text-soft)">
                {finder_session.item_count} 个项目 · {finder_session.changed_count ? `${finder_session.changed_count} 个已修改` : "已同步"}
              </p>
            </div>
          </div>
          <div className="hidden min-w-0 flex-1 items-center rounded-[8px] border border-(--divider-subtle-color) bg-white/72 px-2 py-1 text-[10px] text-(--text-soft) md:flex">
            <Search className="mr-1.5 h-3 w-3 shrink-0" />
            <span className="truncate">{active_path ?? event.target ?? "搜索工作区"}</span>
          </div>
          <FinderSyncIndicator phase={event.phase} />
          <FinderToolbarButton label="分组">
            <SquareStack className="h-3.5 w-3.5" />
          </FinderToolbarButton>
        </div>
        <div className={cn(
          "grid min-h-0 flex-1",
          show_inspector ? "grid-cols-[minmax(0,1fr)_220px]" : "grid-cols-1",
        )}>
          <div className="soft-scrollbar min-h-0 overflow-auto p-2">
            <div className="grid grid-cols-[minmax(0,1fr)_72px_86px] gap-2 px-2 pb-1 text-[9px] font-bold uppercase tracking-[0.12em] text-(--text-soft)">
              <span>名称</span>
              <span>标签</span>
              <span>修改时间</span>
            </div>
            {finder_session.rows.map((row) => (
              <WorkspaceTreeRow
                active={row.path === finder_session.selected_path}
                depth={row.depth}
                item={finder_session.display_items.find((item) => item.path === row.path)}
                key={row.path}
                label={row.label}
                on_select={() => set_selected_path(row.path)}
                path={row.path}
                type={row.type}
              />
            ))}
          </div>
          <aside className={cn(
            "min-h-0 border-l border-(--divider-subtle-color) bg-white/54 p-3",
            !show_inspector && "hidden",
          )}>
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">信息</p>
            <div className="mt-3 grid h-16 w-16 place-items-center rounded-[16px] border border-(--divider-subtle-color) bg-white/74 text-(--icon-default)">
              {(() => {
                const Icon = icon_for_workspace_path(finder_session.selected_path);
                return <Icon className="h-7 w-7" />;
              })()}
            </div>
            <p className="mt-3 line-clamp-2 text-[12px] font-black text-(--text-strong)">
              {basename(finder_session.selected_path)}
            </p>
            <p className="truncate text-[10px] text-(--text-soft)">
              {finder_file_kind_label(finder_session.selected_path)}
            </p>
            <div className="mt-4 space-y-2 border-t border-(--divider-subtle-color) pt-3">
              <FinderInspectorRow label="状态" value={finder_session.selected_item ? workspace_status_label(finder_session.selected_item.status) : "未选择"} />
              <FinderInspectorRow label="位置" value={finder_session.selected_path} />
              <FinderInspectorRow label="修改时间" value={finder_session.selected_item ? format_workspace_time(finder_session.selected_item.updated_at) : "--"} />
              <FinderInspectorRow label="版本" value={finder_session.selected_item ? `v${finder_session.selected_item.version}` : "--"} />
            </div>
            {finder_session.preview_lines.length ? (
              <div className="mt-4 overflow-hidden rounded-[11px] border border-(--divider-subtle-color) bg-[#101820] p-2 font-mono text-[10px] leading-4 text-[#dce8ee]">
                {finder_session.preview_lines.map((line, index) => (
                  <div className="flex min-w-0 gap-2" key={`${index}:${line}`}>
                    <span className="w-5 shrink-0 select-none text-right text-[#6f8190]">{index + 1}</span>
                    <span className="min-w-0 truncate">{line}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="mt-4 rounded-[11px] border border-(--divider-subtle-color) bg-white/54 px-3 py-2 text-[10px] leading-4 text-(--text-soft)">
                {finder_session.changed_count ? `${finder_session.changed_count} 个变更待查看` : "没有新的文件变更"}
              </p>
            )}
          </aside>
        </div>
        <FinderPathBar
          changed_count={finder_session.changed_count}
          item_count={finder_session.item_count}
          path_parts={finder_session.path_parts}
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

function FinderSyncIndicator({ phase }: { phase: NexusOperationEvent["phase"] }) {
  return (
    <span
      className={cn(
        "hidden shrink-0 items-center gap-1 rounded-[8px] px-1.5 py-1 text-[10px] font-semibold md:inline-flex",
        phase === "running"
          ? "bg-[rgba(91,114,255,0.08)] text-[color:var(--primary)]"
          : "bg-white/42 text-(--text-soft)",
      )}
      title={PHASE_LABELS[phase]}
    >
      <span className={cn(
        "h-1.5 w-1.5 rounded-full",
        phase === "running" && "operation-focus-dot bg-[color:var(--primary)]",
        phase === "done" && "bg-[color:var(--success)]",
        phase === "error" && "bg-[color:var(--destructive)]",
        (phase === "queued" || phase === "waiting" || phase === "cancelled") && "bg-[color:var(--icon-muted)]",
      )} />
      <span>{phase === "running" ? "同步中" : "已同步"}</span>
    </span>
  );
}

function FinderToolbarButton({
  active = false,
  children,
  label,
}: {
  active?: boolean;
  children: ReactNode;
  label: string;
}) {
  return (
    <button
      aria-label={label}
      className={cn(
        "grid h-7 w-7 shrink-0 place-items-center rounded-[8px] border transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
        active
          ? "border-(--divider-subtle-color) bg-white text-(--text-strong)"
          : "border-transparent bg-white/42 text-(--icon-muted) hover:bg-white/76 hover:text-(--text-strong)",
      )}
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
}: {
  active?: boolean;
  icon: LucideIcon;
  label: string;
}) {
  return (
    <div className={cn(
      "flex items-center gap-2 rounded-[9px] px-2 py-1.5",
      active ? "bg-white/78 text-(--text-strong) shadow-[inset_0_1px_0_rgba(255,255,255,0.64)]" : "text-(--text-soft)",
    )}>
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">{label}</span>
    </div>
  );
}

function WorkspaceTreeRow({
  active,
  depth,
  item,
  label,
  on_select,
  path,
  type,
}: {
  active: boolean;
  depth: number;
  item?: NonNullable<StageWindowState["payload"]["workspace_items"]>[number];
  label: string;
  on_select: () => void;
  path: string;
  type: "folder" | "file";
}) {
  const status = item?.status;
  const Icon = type === "folder" ? FolderOpen : icon_for_workspace_path(path);
  return (
    <button
      aria-label={`查看 ${label}`}
      className={cn(
        "grid w-full grid-cols-[auto_auto_auto_minmax(0,1fr)_72px_86px] items-center gap-2 rounded-[9px] px-2 py-1.5 text-left text-[11px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
        active ? "bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]" : "text-(--text-muted) hover:bg-white/70",
      )}
      onClick={on_select}
      title={path}
      type="button"
    >
      <span style={{ width: depth * 12 }} className="shrink-0" />
      {type === "folder" ? (
        <ChevronDown className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      ) : (
        <span className="h-3 w-3 shrink-0" />
      )}
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className={cn("min-w-0 flex-1 truncate", type === "folder" && "font-bold text-(--text-strong)")}>
        {label}
      </span>
      {status ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-[9px] font-bold text-(--text-soft)">
          <Tag className={cn(
            "h-2.5 w-2.5 fill-current",
            status === "writing" && "text-[color:var(--primary)]",
            status === "updated" && "text-[color:var(--success)]",
            status === "deleted" && "text-[color:var(--destructive)]",
            status === "idle" && "text-(--icon-muted)",
          )} />
          {workspace_status_label(status)}
        </span>
      ) : <span />}
      <span className="truncate text-[9px] text-(--text-soft)">
        {item ? format_workspace_time(item.updated_at) : "--"}
      </span>
    </button>
  );
}

function FinderPathBar({
  changed_count,
  item_count,
  path_parts,
}: {
  changed_count: number;
  item_count: number;
  path_parts: string[];
}) {
  return (
    <div className="flex min-h-8 items-center justify-between gap-3 border-t border-(--divider-subtle-color) bg-white/58 px-3 py-1.5 text-[10px] text-(--text-soft)">
      <div className="flex min-w-0 items-center gap-1.5">
        <FolderOpen className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
        <span className="shrink-0 font-semibold text-(--text-strong)">workspace</span>
        {path_parts.map((part, index) => (
          <span className="flex min-w-0 items-center gap-1.5" key={`${index}:${part}`}>
            <ChevronRight className="h-3 w-3 shrink-0 text-(--icon-muted)" />
            <span className="max-w-[120px] truncate">{part}</span>
          </span>
        ))}
      </div>
      <span className="shrink-0">
        {item_count} 个项目 · {changed_count ? `${changed_count} 个变更` : "没有变更"}
      </span>
    </div>
  );
}

function basename(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? path;
}

function icon_for_workspace_path(path: string): LucideIcon {
  if (/\.(tsx?|jsx?|json|ya?ml|toml|css|scss|html?)$/i.test(path)) {
    return FileCode2;
  }
  if (/\.(csv|xlsx?|ods)$/i.test(path)) {
    return FileSpreadsheet;
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(path)) {
    return FileImage;
  }
  return FileText;
}

function format_workspace_time(timestamp: number): string {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp));
}
