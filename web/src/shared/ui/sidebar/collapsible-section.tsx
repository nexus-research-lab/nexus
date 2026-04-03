/**
 * 通用可折叠分区
 *
 * 侧边栏面板中的统一 section 容器。
 * 布局：[▸ 标题 数量] ···· [操作按钮]
 * - count 紧跟标题右侧
 * - 操作按钮（+ / →）在最右边，固定宽度占位保证对齐
 */

import { ChevronDown, ChevronRight, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { type ReactNode, useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";

interface CollapsibleSectionProps {
  section_id: string;
  title: string;
  count?: number;
  /** 标题左侧图标 */
  icon?: ReactNode;
  children: React.ReactNode;
  /** 标题栏右侧操作按钮（+ / → 等），固定宽度占位 */
  on_action?: () => void;
  /** 操作按钮的 title 属性 */
  action_title?: string;
  /** 操作按钮内容 */
  action_icon?: ReactNode;
}

interface SidebarListItemProps {
  icon: ReactNode;
  label: string;
  meta?: string;
  is_active?: boolean;
  on_click: () => void;
  on_rename?: () => void;
  on_delete?: () => void;
}

export function SidebarListItem({
  icon,
  label,
  meta,
  is_active = false,
  on_click,
  on_rename,
  on_delete,
}: SidebarListItemProps) {
  const [menu_pos, set_menu_pos] = useState<{ x: number; y: number } | null>(null);

  const handle_context_menu = useCallback((e: React.MouseEvent<HTMLButtonElement>) => {
    if (!on_rename && !on_delete) return;
    e.preventDefault();
    set_menu_pos({ x: e.clientX, y: e.clientY });
  }, [on_delete, on_rename]);

  const handle_more_click = useCallback((e: React.MouseEvent<HTMLSpanElement>) => {
    e.stopPropagation();
    const rect = e.currentTarget.getBoundingClientRect();
    set_menu_pos({ x: rect.right, y: rect.top });
  }, []);

  useEffect(() => {
    if (!menu_pos) return;
    const close = () => set_menu_pos(null);
    window.addEventListener("mousedown", close);
    return () => window.removeEventListener("mousedown", close);
  }, [menu_pos]);

  const has_actions = Boolean(on_rename || on_delete);

  return (
    <>
      <button
        className={cn(
          "group/item flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-[12px] transition-all duration-150",
          is_active
            ? "bg-white/60 font-semibold text-slate-900 shadow-sm"
            : "text-slate-600 hover:bg-white/30 hover:text-slate-800",
        )}
        onClick={on_click}
        onContextMenu={handle_context_menu}
        type="button"
      >
        <span className="flex h-5 w-5 shrink-0 items-center justify-center text-slate-500">
          {icon}
        </span>
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {has_actions ? (
          <span
            className="flex h-4 w-4 shrink-0 items-center justify-center rounded text-slate-400 opacity-0 transition-all hover:text-slate-700 group-hover/item:opacity-100"
            onClick={handle_more_click}
            role="button"
            tabIndex={-1}
          >
            <MoreHorizontal className="h-3 w-3" />
          </span>
        ) : meta ? (
          <span className="shrink-0 text-[10px] text-slate-400">{meta}</span>
        ) : null}
      </button>

      {menu_pos ? createPortal(
        <div
          className="fixed z-[9990] w-36 rounded-xl border border-slate-200/60 bg-white/95 py-1 shadow-lg backdrop-blur-md animate-in fade-in zoom-in-95 duration-100"
          style={{ top: menu_pos.y, left: menu_pos.x }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          {on_rename ? (
            <button
              className="flex w-full items-center gap-2 px-3 py-1.5 text-[12px] text-slate-700 hover:bg-slate-50"
              onClick={() => {
                set_menu_pos(null);
                on_rename();
              }}
              type="button"
            >
              <Pencil className="h-3 w-3" />
              重命名
            </button>
          ) : null}

          {on_delete ? (
            <button
              className="flex w-full items-center gap-2 px-3 py-1.5 text-[12px] text-red-600 hover:bg-red-50"
              onClick={() => {
                set_menu_pos(null);
                on_delete();
              }}
              type="button"
            >
              <Trash2 className="h-3 w-3" />
              删除
            </button>
          ) : null}
        </div>,
        document.body,
      ) : null}
    </>
  );
}

export function CollapsibleSection({
  section_id,
  title,
  count,
  icon,
  children,
  on_action,
  action_title = "新建",
  action_icon,
}: CollapsibleSectionProps) {
  const is_collapsed = useSidebarStore(
    (s) => s.collapsed_sections[section_id] ?? false,
  );
  const toggle = useSidebarStore((s) => s.toggle_section);

  return (
    <section className="border-b border-white/10 pb-1">
      <div className="group/section flex w-full items-center justify-between px-2 py-2">
        <button
          className="flex flex-1 items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-500 transition-colors hover:text-slate-700"
          onClick={() => toggle(section_id)}
          type="button"
        >
          {is_collapsed ? (
            <ChevronRight className="h-3 w-3" />
          ) : (
            <ChevronDown className="h-3 w-3" />
          )}
          {icon ? <span className="flex items-center">{icon}</span> : null}
          <span>{title}</span>
          {typeof count === "number" ? (
            <span className="text-[10px] text-slate-400">{count}</span>
          ) : null}
        </button>

        {/* 右侧操作按钮，固定宽度占位保证对齐 */}
        {on_action ? (
          <button
            className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-slate-400 transition-colors hover:bg-white/40 hover:text-slate-600"
            onClick={(e) => { e.stopPropagation(); on_action(); }}
            title={action_title}
            type="button"
          >
            {action_icon}
          </button>
        ) : (
          <span className="flex h-5 w-5 shrink-0" />
        )}
      </div>

      {!is_collapsed ? (
        <div className="flex flex-col gap-0.5 pb-1">{children}</div>
      ) : null}
    </section>
  );
}
