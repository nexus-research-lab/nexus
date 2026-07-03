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
  sectionId: string;
  title: string;
  count?: number;
  /** 标题左侧图标 */
  icon?: ReactNode;
  children: React.ReactNode;
  /** 标题点击行为，与折叠切换分离 */
  onTitleClick?: () => void;
  /** 标题是否处于激活态 */
  isTitleActive?: boolean;
  /** 标题栏右侧操作按钮（+ / → 等），固定宽度占位 */
  onAction?: () => void;
  /** 操作按钮的 title 属性 */
  actionTitle?: string;
  /** 操作按钮内容 */
  actionIcon?: ReactNode;
}

interface SidebarListItemProps {
  icon: ReactNode;
  label: string;
  labelClassName?: string;
  labelStyle?: CSSProperties;
  meta?: string;
  isActive?: boolean;
  activeVariant?: "default" | "avatar_emphasis";
  onClick: () => void;
  onRename?: () => void;
  onDelete?: () => void;
}

export function SidebarListItem({
  icon,
  label,
  labelClassName: labelClassName,
  labelStyle: labelStyle,
  meta,
  isActive: isActive = false,
  activeVariant: activeVariant = "default",
  onClick: onClick,
  onRename: onRename,
  onDelete: onDelete,
}: SidebarListItemProps) {
  const { t } = useI18n();
  const hasActions = Boolean(onRename || onDelete);
  const isAvatarEmphasisActive = isActive && activeVariant === "avatar_emphasis";
  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) {
      return;
    }
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    onClick();
  };

  return (
    <div
      className={cn(
        "group/item relative box-border flex w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-[12px] px-2.5 py-[7px] transition-[background,color,transform] duration-(--motion-duration-fast)",
        isAvatarEmphasisActive
          ? "text-(--text-strong)"
          : isActive
          ? "text-(--text-strong)"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      onClick={onClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      style={isAvatarEmphasisActive ? undefined : isActive ? {
        background: "color-mix(in srgb, var(--surface-interactive-active-background) 72%, transparent)",
      } : undefined}
    >
      {isActive && !isAvatarEmphasisActive ? (
        <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}

      <div
        className={cn(
          SIDEBAR_LIST_ITEM_CLASS_NAME,
          isActive
            ? "font-medium text-(--text-strong)"
            : "text-(--text-default) group-hover/item:text-(--text-strong)",
        )}
      >
        <span
          className={cn(
            "relative flex h-6 w-6 shrink-0 items-center justify-center",
            isAvatarEmphasisActive
              ? "text-(--text-strong)"
              : isActive
                ? "text-(--primary)"
                : "text-(--icon-muted)",
          )}
        >
          {isAvatarEmphasisActive ? (
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
          className={cn("min-w-0 flex-1 truncate", labelClassName)}
          style={labelStyle}
        >
          {label}
        </span>
        {meta ? (
          <span
            className={cn(
              "shrink-0 text-[12px] font-medium tabular-nums",
              isActive ? "text-(--text-muted)" : "text-(--text-soft)",
            )}
          >
            {meta}
          </span>
        ) : null}
      </div>

      {hasActions ? (
        <div className="flex shrink-0 items-center gap-1">
          {onRename ? (
            <UiListActionButton
              aria-label={t("home.rename")}
              onClick={() => {
                onRename();
              }}
              stopPropagation
              title={t("home.rename")}
              visibility={isActive ? "visible" : "subtle"}
            >
              <Pencil className="h-3.5 w-3.5" />
            </UiListActionButton>
          ) : null}

          {onDelete ? (
            <UiListActionButton
              aria-label={t("common.delete")}
              onClick={() => {
                onDelete();
              }}
              stopPropagation
              title={t("common.delete")}
              tone="danger"
              visibility={isActive ? "visible" : "subtle"}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </UiListActionButton>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function CollapsibleSection({
  sectionId: sectionId,
  title,
  count,
  icon,
  children,
  onTitleClick: onTitleClick,
  isTitleActive: isTitleActive = false,
  onAction: onAction,
  actionTitle: actionTitle = "新建",
  actionIcon: actionIcon,
}: CollapsibleSectionProps) {
  const isCollapsed = useSidebarStore(
    (s) => s.collapsed_sections[sectionId] ?? false,
  );
  const toggle = useSidebarStore((s) => s.toggle_section);
  const titleContent = (
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
        {onTitleClick ? (
          <div className="flex min-w-0 flex-1 items-center">
            <button
              className={cn(
                SIDEBAR_SECTION_CHEVRON_SLOT_CLASS_NAME,
                "rounded-full text-(--icon-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--icon-default)",
              )}
              onClick={() => toggle(sectionId)}
              type="button"
            >
              {isCollapsed ? (
                <ChevronRight className="h-3.5 w-3.5" />
              ) : (
                <ChevronDown className="h-3.5 w-3.5" />
              )}
            </button>
            <button
              className={cn(
                SIDEBAR_SECTION_TRIGGER_CLASS_NAME,
                "min-w-0 flex-1",
                isTitleActive && "text-(--text-strong)",
              )}
              onClick={onTitleClick}
              type="button"
            >
              {titleContent}
            </button>
          </div>
        ) : (
          <button
            className={SIDEBAR_SECTION_TRIGGER_CLASS_NAME}
            onClick={() => toggle(sectionId)}
            type="button"
          >
            <span className={SIDEBAR_SECTION_CHEVRON_SLOT_CLASS_NAME}>
              {isCollapsed ? (
                <ChevronRight className="h-3.5 w-3.5" />
              ) : (
                <ChevronDown className="h-3.5 w-3.5" />
              )}
            </span>
            {titleContent}
          </button>
        )}

        {/* 右侧操作按钮，固定宽度占位保证对齐 */}
        {onAction ? (
          <UiListActionButton
            onClick={onAction}
            shape="round"
            size="md"
            stopPropagation
            title={actionTitle}
            visibility="visible"
          >
            {actionIcon}
          </UiListActionButton>
        ) : (
          <span className="flex h-5 w-5 shrink-0" />
        )}
      </div>

      {!isCollapsed ? (
        <div className="flex flex-col gap-0.5 pb-1">{children}</div>
      ) : null}
    </section>
  );
}
