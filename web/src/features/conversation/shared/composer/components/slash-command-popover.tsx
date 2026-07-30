"use client";

import {
  memo,
  useEffect,
  useRef,
  type CSSProperties,
  type KeyboardEvent,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import type {
  CommandCatalogStatus,
  CommandDescriptor,
} from "@/types/generated/protocol";
import type { SkillInfo } from "@/types/capability/skill";

import { isSelectableSlashCommand } from "../slash-command-model";

interface SlashCommandPopoverProps {
  activeIndex: number;
  anchorRect: DOMRect | null;
  commands: CommandDescriptor[];
  mode: "commands" | "skills";
  onSelectCommand: (command: CommandDescriptor) => void;
  onSelectSkill: (skill: SkillInfo) => void;
  onSkillQueryChange: (query: string) => void;
  onSkillQueryKeyDown: (event: KeyboardEvent<HTMLInputElement>) => boolean;
  query: string;
  skillCount: number;
  skillError: string | null;
  skillItems: SkillInfo[];
  skillLoading: boolean;
  skillQuery: string;
  skillSearchRef: RefObject<HTMLInputElement | null>;
  status: CommandCatalogStatus;
}

export const SlashCommandPopover = memo(function SlashCommandPopover({
  activeIndex,
  anchorRect,
  commands,
  mode,
  onSelectCommand,
  onSelectSkill,
  onSkillQueryChange,
  onSkillQueryKeyDown,
  query,
  skillCount,
  skillError,
  skillItems,
  skillLoading,
  skillQuery,
  skillSearchRef,
  status,
}: SlashCommandPopoverProps) {
  const { t } = useI18n();
  const itemsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const activeElement = itemsRef.current?.children[activeIndex] as
      | HTMLElement
      | undefined;
    activeElement?.scrollIntoView({ block: "nearest" });
  }, [activeIndex, mode, skillItems.length]);

  if (!anchorRect) {
    return null;
  }
  const layout = getSlashCommandPopoverLayout(anchorRect, mode);
  const title = mode === "skills" ? t("composer.skills_picker_title") : t("composer.slash_commands");
  const subtitle = mode === "skills"
    ? t("composer.skills_picker_subtitle")
    : resolveSlashCommandSubtitle(commands.length, query, status, t);

  return createPortal(
    <div
      aria-label={title}
      className={cn(
        "fixed z-[9999] overflow-hidden border border-(--divider-subtle-color)",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      role="listbox"
      style={layout}
    >
      <div className="flex items-start justify-between gap-3 border-b border-(--divider-subtle-color) px-3 py-2.5">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-mono text-sm font-semibold text-(--text-strong)">
              /{mode === "skills" ? "skills" : ""}
            </span>
            <span className="truncate text-sm font-medium text-(--text-strong)">
              {title}
            </span>
          </div>
          <p className="mt-0.5 text-xs text-(--text-soft)">
            {subtitle}
          </p>
        </div>
        <span className="shrink-0 rounded-full bg-(--surface-interactive-hover-background) px-2 py-1 text-xs text-(--text-soft)">
          {mode === "skills" ? skillCount : commands.length}
        </span>
      </div>

      {mode === "skills" ? (
        <div className="border-b border-(--divider-subtle-color) p-2">
          <input
            ref={skillSearchRef}
            aria-label={t("composer.skills_search_placeholder")}
            className={cn(
              "w-full rounded-[14px] border border-(--divider-subtle-color) bg-(--surface-background) px-3 py-2 text-sm text-(--text-strong) outline-none",
              "placeholder:text-(--text-soft) focus:border-(--brand-action) focus:ring-2 focus:ring-[color:color-mix(in_srgb,var(--brand-action)_16%,transparent)]",
            )}
            onChange={(event) => onSkillQueryChange(event.target.value)}
            onKeyDown={(event) => {
              if (onSkillQueryKeyDown(event)) {
                return;
              }
            }}
            placeholder={t("composer.skills_search_placeholder")}
            value={skillQuery}
          />
        </div>
      ) : null}

      {mode === "skills" ? (
        <div className="max-h-80 overflow-y-auto p-1" ref={itemsRef}>
          {skillLoading ? (
            <p className="px-3 py-3 text-sm text-(--text-soft)">
              {t("composer.skills_loading")}
            </p>
          ) : skillError ? (
            <p className="px-3 py-3 text-sm text-(--destructive)">
              {skillError}
            </p>
          ) : skillItems.length === 0 ? (
            <p className="px-3 py-3 text-sm text-(--text-soft)">
              {t("composer.skills_empty")}
            </p>
          ) : (
            skillItems.map((skill, index) => {
              const selectable = true;
              return (
                <button
                  aria-disabled={!selectable}
                  aria-selected={index === activeIndex}
                  className={cn(
                    MENU_ITEM_BASE_CLASS_NAME,
                    "flex items-start gap-3 px-3 py-2.5 text-left text-sm",
                    getMenuItemStateClassName({
                      active: index === activeIndex,
                    }),
                  )}
                  key={skill.name}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    onSelectSkill(skill);
                  }}
                  role="option"
                  type="button"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium text-(--text-strong)">
                      {skill.title || skill.name}
                    </span>
                    <span className="block truncate font-mono text-xs text-(--text-soft)">
                      /{skill.name}
                    </span>
                    <span className="block truncate text-xs text-(--text-muted)">
                      {skill.description}
                    </span>
                  </span>
                  <span className="shrink-0 rounded-full bg-(--surface-interactive-hover-background) px-2 py-1 text-[11px] text-(--text-soft)">
                    {skill.locked || !skill.enabled_for_agent
                      ? t("composer.skills_unavailable_badge")
                      : t("composer.skills_available_badge")}
                  </span>
                </button>
              );
            })
          )}
        </div>
      ) : (
        <div className="max-h-80 overflow-y-auto p-1" ref={itemsRef}>
          {commands.length === 0 ? (
            <p className="px-3 py-3 text-sm text-(--text-soft)">
              {resolveSlashCommandEmptyCopy(status, t)}
            </p>
          ) : (
            commands.map((command, index) => {
              const selectable = isSelectableSlashCommand(command);
              return (
                <button
                  aria-disabled={!selectable}
                  aria-selected={index === activeIndex}
                  className={cn(
                    MENU_ITEM_BASE_CLASS_NAME,
                    "flex items-start gap-3 px-3 py-2.5 text-left text-sm",
                    getMenuItemStateClassName({
                      active: index === activeIndex,
                    }),
                    !selectable && "cursor-not-allowed opacity-(--disabled-opacity)",
                  )}
                  key={`${command.execution}:${command.name}`}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    if (selectable) {
                      onSelectCommand(command);
                    }
                  }}
                  role="option"
                  type="button"
                >
                  <span className="shrink-0 font-mono font-semibold text-(--text-strong)">
                    /{command.name}
                  </span>
                  <span className="min-w-0 flex-1">
                    {command.description ? (
                      <span className="block truncate text-(--text-muted)">
                        {command.description}
                      </span>
                    ) : null}
                    {command.argument_hint ? (
                      <span className="block truncate font-mono text-xs text-(--text-soft)">
                        {command.argument_hint}
                      </span>
                    ) : null}
                    {!selectable ? (
                      <span className="block truncate text-xs text-(--text-soft)">
                        {command.disabled_reason
                          ?? t("composer.slash_command_unavailable")}
                      </span>
                    ) : null}
                  </span>
                </button>
              );
            })
          )}
        </div>
      )}
    </div>,
    document.body,
  );
});

function resolveSlashCommandEmptyCopy(
  status: CommandCatalogStatus,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (status === "cold") {
    return t("composer.slash_commands_loading");
  }
  if (status === "unavailable") {
    return t("composer.slash_commands_unavailable");
  }
  return t("composer.slash_commands_empty");
}

function resolveSlashCommandSubtitle(
  commandCount: number,
  query: string,
  status: CommandCatalogStatus,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (status === "cold") {
    return t("composer.slash_commands_loading");
  }
  if (status === "unavailable") {
    return t("composer.slash_commands_unavailable");
  }
  if (!query.trim()) {
    return t("composer.slash_commands_subtitle_all", {
      count: commandCount,
    });
  }
  return t("composer.slash_commands_subtitle_filtered", {
    count: commandCount,
  });
}

function getSlashCommandPopoverLayout(
  anchorRect: DOMRect,
  mode: "commands" | "skills",
): CSSProperties {
  const viewportPadding = 12;
  const preferredWidth = mode === "skills" ? 560 : 480;
  const width = Math.min(
    preferredWidth,
    window.innerWidth - viewportPadding * 2,
  );
  const left = Math.min(
    Math.max(viewportPadding, anchorRect.left),
    window.innerWidth - width - viewportPadding,
  );
  return {
    bottom: window.innerHeight - anchorRect.top + 10,
    left,
    maxHeight: Math.min(
      mode === "skills" ? 520 : 420,
      Math.max(180, anchorRect.top - 20),
    ),
    width,
  };
}
