/**
 * 通用可折叠分区
 *
 * 侧边栏面板中的统一 section 容器。
 * 布局：[▸ 标题 数量] ···· [操作按钮]
 * - count 紧跟标题右侧
 * - 操作按钮（+ / →）在最右边，固定宽度占位保证对齐
 */

import { ChevronDown, ChevronRight, Pencil, Trash2 } from "lucide-react";
import { type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiListActionButton } from "@/shared/ui/list-action";
import { useSidebarStore } from "@/store/sidebar";

const SIDEBAR_LIST_ITEM_CLASS_NAME =
  "flex min-w-0 flex-1 items-center gap-2.5 text-left text-[14px]";
const SIDEBAR_SECTION_TRIGGER_CLASS_NAME =
  "flex flex-1 items-center gap-1.5 text-[13px] font-semibold uppercase tracking-[0.12em] text-(--text-default) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)";
const SIDEBAR_SECTION_CHEVRON_SLOT_CLASS_NAME =
  "flex h-6 w-6 shrink-0 items-center justify-center";

interface CollapsibleSectionProps {
  section_id: string;
  title: string;
  count?: number;
  /** 标题左侧图标 */
  icon?: ReactNode;
  children: React.ReactNode;
  /** 标题点击行为，与折叠切换分离 */
  on_title_click?: () => void;
  /** 标题是否处于激活态 */
  is_title_active?: boolean;
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
  label_class_name?: string;
  label_style?: CSSProperties;
  meta?: string;
  is_active?: boolean;
  active_variant?: "default" | "avatar_emphasis";
  on_click: () => void;
  on_rename?: () => void;
  on_delete?: () => void;
}

export function SidebarListItem({
  icon,
  label,
  label_class_name,
  label_style,
  meta,
  is_active = false,
  active_variant = "default",
  on_click,
  on_rename,
  on_delete,
}: SidebarListItemProps) {
  const { t } = useI18n();
  const has_actions = Boolean(on_rename || on_delete);
  const is_avatar_emphasis_active = is_active && active_variant === "avatar_emphasis";
  const handle_key_down = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) {
      return;
    }
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    on_click();
  };

  return (
    <div
      className={cn(
        "group/item relative box-border flex w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-[12px] px-2.5 py-[7px] transition-[background,color,transform] duration-(--motion-duration-fast)",
        is_avatar_emphasis_active
          ? "text-(--text-strong)"
          : is_active
          ? "text-(--text-strong)"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      onClick={on_click}
      onKeyDown={handle_key_down}
      role="button"
      tabIndex={0}
      style={is_avatar_emphasis_active ? undefined : is_active ? {
        background: "color-mix(in srgb, var(--surface-interactive-active-background) 72%, transparent)",
      } : undefined}
    >
      {is_active && !is_avatar_emphasis_active ? (
        <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}

      <div
        className={cn(
          SIDEBAR_LIST_ITEM_CLASS_NAME,
          is_active
            ? "font-medium text-(--text-strong)"
            : "text-(--text-default) group-hover/item:text-(--text-strong)",
        )}
      >
        <span
          className={cn(
            "relative flex h-6 w-6 shrink-0 items-center justify-center",
            is_avatar_emphasis_active
              ? "text-(--text-strong)"
              : is_active
                ? "text-(--primary)"
                : "text-(--icon-muted)",
          )}
        >
          {is_avatar_emphasis_active ? (
            <>
              {/* 中文注释：系统入口激活时只强调头像，不复用常规列表项的底色和左侧指示条。 */}
              <span className="pointer-events-none absolute inset-[-3px] rounded-full bg-[conic-gradient(from_180deg,color-mix(in_srgb,var(--primary)_72%,transparent),transparent_32%,color-mix(in_srgb,var(--primary)_38%,white),transparent_76%,color-mix(in_srgb,var(--primary)_72%,transparent))] opacity-90 animate-[spin_5.5s_linear_infinite]" />
              <span className="pointer-events-none absolute inset-[-1px] rounded-full border border-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] animate-[pulse_2.2s_ease-in-out_infinite]" />
              <span className="relative z-10 flex h-5 w-5 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_92%,white)] shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_14%,transparent),0_8px_20px_color-mix(in_srgb,var(--primary)_12%,transparent)]">
                {icon}
              </span>
            </>
          ) : (
            icon
          )}
        </span>
        <span
          className={cn("min-w-0 flex-1 truncate", label_class_name)}
          style={label_style}
        >
          {label}
        </span>
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
      </div>

      {has_actions ? (
        <div className="flex shrink-0 items-center gap-1">
          {on_rename ? (
            <UiListActionButton
              aria-label={t("home.rename")}
              onClick={() => {
                on_rename();
              }}
              stop_propagation
              title={t("home.rename")}
              visibility={is_active ? "visible" : "subtle"}
            >
              <Pencil className="h-3.5 w-3.5" />
            </UiListActionButton>
          ) : null}

          {on_delete ? (
            <UiListActionButton
              aria-label={t("common.delete")}
              onClick={() => {
                on_delete();
              }}
              stop_propagation
              title={t("common.delete")}
              tone="danger"
              visibility={is_active ? "visible" : "subtle"}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </UiListActionButton>
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
  on_title_click,
  is_title_active = false,
  on_action,
  action_title = "新建",
  action_icon,
}: CollapsibleSectionProps) {
  const is_collapsed = useSidebarStore(
    (s) => s.collapsed_sections[section_id] ?? false,
  );
  const toggle = useSidebarStore((s) => s.toggle_section);
  const title_content = (
    <>
      {icon ? <span className="flex items-center">{icon}</span> : null}
      <span>{title}</span>
      {typeof count === "number" ? (
        <span className="text-[12px] font-medium tabular-nums text-(--text-muted)">{count}</span>
      ) : null}
    </>
  );

  return (
    <section className="border-b divider-subtle pb-1.5 last:border-b-0">
      <div className="group/section flex w-full items-center justify-between px-2.5 py-2">
        {on_title_click ? (
          <div className="flex min-w-0 flex-1 items-center">
            <button
              className={cn(
                SIDEBAR_SECTION_CHEVRON_SLOT_CLASS_NAME,
                "rounded-full text-(--icon-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--icon-default)",
              )}
              onClick={() => toggle(section_id)}
              type="button"
            >
              {is_collapsed ? (
                <ChevronRight className="h-3.5 w-3.5" />
              ) : (
                <ChevronDown className="h-3.5 w-3.5" />
              )}
            </button>
            <button
              className={cn(
                SIDEBAR_SECTION_TRIGGER_CLASS_NAME,
                "min-w-0 flex-1",
                is_title_active && "text-(--text-strong)",
              )}
              onClick={on_title_click}
              type="button"
            >
              {title_content}
            </button>
          </div>
        ) : (
          <button
            className={SIDEBAR_SECTION_TRIGGER_CLASS_NAME}
            onClick={() => toggle(section_id)}
            type="button"
          >
            <span className={SIDEBAR_SECTION_CHEVRON_SLOT_CLASS_NAME}>
              {is_collapsed ? (
                <ChevronRight className="h-3.5 w-3.5" />
              ) : (
                <ChevronDown className="h-3.5 w-3.5" />
              )}
            </span>
            {title_content}
          </button>
        )}

        {/* 右侧操作按钮，固定宽度占位保证对齐 */}
        {on_action ? (
          <UiListActionButton
            onClick={on_action}
            shape="round"
            size="md"
            stop_propagation
            title={action_title}
            visibility="visible"
          >
            {action_icon}
          </UiListActionButton>
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
