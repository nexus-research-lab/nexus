/**
 * 通用可折叠分区
 *
 * 侧边栏面板中的统一 section 容器。
 * 布局：[▸ 标题 数量] ···· [操作按钮]
 * - count 紧跟标题右侧
 * - 操作按钮（+ / →）在最右边，固定宽度占位保证对齐
 */

import { ChevronDown, ChevronRight, Pencil, Trash2 } from "lucide-react";
import { type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useSidebarStore } from "@/store/sidebar";

const SIDEBAR_LIST_ITEM_CLASS_NAME =
  "relative box-border flex w-full min-w-0 items-center gap-2.5 rounded-[12px] px-2.5 py-[7px] text-left text-[14px] transition-[background,color,transform] duration-(--motion-duration-fast)";
const SIDEBAR_SECTION_TRIGGER_CLASS_NAME =
  "flex flex-1 items-center gap-1.5 text-[13px] font-semibold uppercase tracking-[0.12em] text-(--text-default) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)";
const SIDEBAR_SECTION_ACTION_CLASS_NAME =
  "flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)";
const SIDEBAR_LIST_ACTION_BUTTON_CLASS_NAME =
  "flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px] border border-transparent text-(--icon-muted) transition-[background,color,border-color,opacity,transform] duration-(--motion-duration-fast) focus-visible:opacity-100 focus-visible:outline-none";

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
  const has_actions = Boolean(on_rename || on_delete);

  return (
    <div
      className={cn(
        "group/item relative flex min-w-0 items-center gap-1.5 rounded-[12px] transition-[background,color,transform] duration-(--motion-duration-fast)",
        is_active
          ? "text-(--text-strong)"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      style={is_active ? {
        background: "color-mix(in srgb, var(--surface-interactive-active-background) 72%, transparent)",
      } : undefined}
    >
      {is_active ? (
        <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}

      <button
        className={cn(
          SIDEBAR_LIST_ITEM_CLASS_NAME,
          "flex-1 bg-transparent",
          is_active
            ? "font-medium text-(--text-strong)"
            : "text-(--text-default) group-hover/item:text-(--text-strong) hover:translate-x-[2px]",
        )}
        onClick={on_click}
        type="button"
      >
        <span
          className={cn(
            "flex h-5 w-5 shrink-0 items-center justify-center",
            is_active ? "text-(--primary)" : "text-(--icon-muted)",
          )}
        >
          {icon}
        </span>
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {meta ? (
          <span
            className={cn(
              "shrink-0 text-[12px] font-medium tabular-nums",
              is_active ? "text-(--text-muted)" : "text-(--text-soft)",
            )}
          >
            {meta}
          </span>
        ) : null}
      </button>

      {has_actions ? (
        <div className="flex shrink-0 items-center gap-1 pr-2">
          {on_rename ? (
            <button
              aria-label={t("home.rename")}
              className={cn(
                SIDEBAR_LIST_ACTION_BUTTON_CLASS_NAME,
                is_active
                  ? "opacity-100 hover:-translate-y-[1px] hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                  : "opacity-60 hover:-translate-y-[1px] hover:opacity-100 hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)",
              )}
              onClick={(e) => {
                e.stopPropagation();
                on_rename();
              }}
              title={t("home.rename")}
              type="button"
            >
              <Pencil className="h-3.5 w-3.5" />
            </button>
          ) : null}

          {on_delete ? (
            <button
              aria-label={t("common.delete")}
              className={cn(
                SIDEBAR_LIST_ACTION_BUTTON_CLASS_NAME,
                is_active
                  ? "opacity-100 hover:-translate-y-[1px] hover:border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)"
                  : "opacity-60 hover:-translate-y-[1px] hover:opacity-100 hover:border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)",
              )}
              onClick={(e) => {
                e.stopPropagation();
                on_delete();
              }}
              title={t("common.delete")}
              type="button"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
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
            <span className="text-[12px] font-medium tabular-nums text-(--text-muted)">{count}</span>
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
