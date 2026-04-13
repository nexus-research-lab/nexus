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
import { useI18n } from "@/shared/i18n/i18n-context";
import { useSidebarStore } from "@/store/sidebar";

const CONTEXT_MENU_CLASS_NAME =
  "fixed z-[9990] w-40 rounded-xl py-1 animate-in fade-in zoom-in-95 duration-100";
const CONTEXT_MENU_ITEM_CLASS_NAME =
  "flex w-full items-center gap-2.5 px-3 py-2 text-[13px] transition-[background,color] duration-[var(--motion-duration-fast)]";
const SIDEBAR_LIST_ITEM_CLASS_NAME =
  "group/item box-border flex w-full items-center gap-2.5 rounded-[12px] px-2.5 py-[7px] text-left text-[14px] transition-[background,color] duration-[var(--motion-duration-fast)]";
const SIDEBAR_SECTION_TRIGGER_CLASS_NAME =
  "flex flex-1 items-center gap-1.5 text-[13px] font-semibold uppercase tracking-[0.12em] text-[color:var(--text-default)] transition-colors duration-[var(--motion-duration-fast)] hover:text-[color:var(--text-strong)]";
const SIDEBAR_SECTION_ACTION_CLASS_NAME =
  "flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-[color:var(--icon-muted)] transition-[background,color,transform] duration-[var(--motion-duration-fast)] hover:-translate-y-[1px] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--icon-default)]";

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
  const { t } = useI18n();
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
          SIDEBAR_LIST_ITEM_CLASS_NAME,
          "relative",
          is_active
            ? "font-medium text-[color:var(--text-strong)]"
            : "text-[color:var(--text-default)] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)] hover:translate-x-[2px]",
        )}
        style={is_active ? {
          background: "color-mix(in srgb, var(--surface-interactive-active-background) 72%, transparent)",
        } : undefined}
        onClick={on_click}
        onContextMenu={handle_context_menu}
        type="button"
      >
        {/* 激活态左侧强调条 */}
        {is_active ? (
          <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-[var(--primary)]" />
        ) : null}
        <span
          className={cn(
            "flex h-5 w-5 shrink-0 items-center justify-center",
            is_active ? "text-[color:var(--primary)]" : "text-[color:var(--icon-muted)]",
          )}
        >
          {icon}
        </span>
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {has_actions ? (
          <span
            className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-[color:var(--icon-muted)] opacity-0 transition-all hover:text-[color:var(--icon-default)] group-hover/item:opacity-100"
            onClick={handle_more_click}
            role="button"
            tabIndex={-1}
          >
            <MoreHorizontal className="h-3.5 w-3.5" />
          </span>
        ) : meta ? (
          <span
            className={cn(
              "shrink-0 text-[12px] font-medium tabular-nums",
              is_active ? "text-[color:var(--text-muted)]" : "text-[color:var(--text-soft)]",
            )}
          >
            {meta}
          </span>
        ) : null}
      </button>

      {menu_pos ? createPortal(
        <div
          className={CONTEXT_MENU_CLASS_NAME}
          style={{
            top: menu_pos.y,
            left: menu_pos.x,
            background: "var(--surface-popover-background)",
            border: "1px solid var(--surface-popover-border)",
          }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          {on_rename ? (
            <button
              className={cn(CONTEXT_MENU_ITEM_CLASS_NAME, "text-[color:var(--text-default)] hover:bg-[var(--surface-interactive-hover-background)]")}
              onClick={() => {
                set_menu_pos(null);
                on_rename();
              }}
              type="button"
            >
              <Pencil className="h-3.5 w-3.5" />
              {t("home.rename")}
            </button>
          ) : null}

          {on_delete ? (
            <button
              className={cn(CONTEXT_MENU_ITEM_CLASS_NAME, "text-rose-500 hover:bg-rose-500/10")}
              onClick={() => {
                set_menu_pos(null);
                on_delete();
              }}
              type="button"
            >
              <Trash2 className="h-3.5 w-3.5" />
              {t("common.delete")}
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
    <section className="border-b divider-subtle pb-1.5 last:border-b-0">
      <div className="group/section flex w-full items-center justify-between px-2.5 py-2">
        <button
          className={SIDEBAR_SECTION_TRIGGER_CLASS_NAME}
          onClick={() => toggle(section_id)}
          type="button"
        >
          {is_collapsed ? (
            <ChevronRight className="h-3.5 w-3.5" />
          ) : (
            <ChevronDown className="h-3.5 w-3.5" />
          )}
          {icon ? <span className="flex items-center">{icon}</span> : null}
          <span>{title}</span>
          {typeof count === "number" ? (
            <span className="text-[12px] font-medium tabular-nums text-[color:var(--text-muted)]">{count}</span>
          ) : null}
        </button>

        {/* 右侧操作按钮，固定宽度占位保证对齐 */}
        {on_action ? (
          <button
            className={SIDEBAR_SECTION_ACTION_CLASS_NAME}
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
