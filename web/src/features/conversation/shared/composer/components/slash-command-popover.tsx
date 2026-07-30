"use client";

import { memo, useEffect, useRef, type CSSProperties } from "react";
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

import { isSelectableSlashCommand } from "../slash-command-model";

interface SlashCommandPopoverProps {
  activeIndex: number;
  anchorRect: DOMRect | null;
  commands: CommandDescriptor[];
  onSelect: (command: CommandDescriptor) => void;
  status: CommandCatalogStatus;
}

export const SlashCommandPopover = memo(function SlashCommandPopover({
  activeIndex,
  anchorRect,
  commands,
  onSelect,
  status,
}: SlashCommandPopoverProps) {
  const { t } = useI18n();
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const activeElement = listRef.current?.children[activeIndex] as
      | HTMLElement
      | undefined;
    activeElement?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  if (!anchorRect) {
    return null;
  }
  const layout = getSlashCommandPopoverLayout(anchorRect);
  const emptyCopy = resolveSlashCommandEmptyCopy(status, t);

  return createPortal(
    <div
      aria-label={t("composer.slash_commands")}
      className={cn(
        "fixed z-[9999] max-h-72 overflow-y-auto",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      role="listbox"
      style={layout}
    >
      {commands.length === 0 ? (
        <p className="px-3 py-3 text-sm text-(--text-soft)">
          {emptyCopy}
        </p>
      ) : (
        <div className="p-1" ref={listRef}>
          {commands.map((command, index) => {
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
                    onSelect(command);
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
          })}
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

function getSlashCommandPopoverLayout(
  anchorRect: DOMRect,
): CSSProperties {
  const viewportPadding = 12;
  const preferredWidth = Math.max(anchorRect.width, 360);
  const width = Math.min(
    preferredWidth,
    window.innerWidth - viewportPadding * 2,
  );
  const left = Math.min(
    Math.max(viewportPadding, anchorRect.left),
    window.innerWidth - width - viewportPadding,
  );
  return {
    bottom: window.innerHeight - anchorRect.top + 8,
    left,
    maxHeight: Math.max(120, anchorRect.top - 24),
    width,
  };
}
