"use client";

// INPUT: Slash picker 目录、读取状态与显式重载动作。
// OUTPUT: Skill/Model 选择列表或完整的 Problem/Impact/Recovery 状态。
// POS: Composer picker 可见错误边界；只读失败不触碰输入草稿。

import {
  memo,
  useCallback,
  useEffect,
  useRef,
  type KeyboardEvent,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import { Check, Lock } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
  MENU_LIST_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";
import { SelectMenuPanel } from "@/shared/ui/menu/select-menu-primitives";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayPosition,
} from "@/shared/ui/overlay/anchored-overlay-model";
import type {
  CommandCatalogStatus,
  CommandDescriptor,
} from "@/types/generated/protocol";
import type { SkillInfo } from "@/types/capability/skill";

import {
  isSelectableSlashCommand,
  type SlashModelOption,
} from "../slash-command-model";
import type { ComposerReadFailure } from "../controller/composer-settings-reliability";

const SLASH_COMMAND_PANEL_MAX_HEIGHT_PX = 296;
const SLASH_PICKER_PANEL_MAX_HEIGHT_PX = 336;
const SLASH_LIST_CLASS_NAME = cn(
  MENU_LIST_CLASS_NAME,
  "soft-scrollbar min-h-0 max-h-72 flex-1 overflow-y-auto overscroll-contain p-1",
);

const SLASH_COMMAND_DESCRIPTION_KEYS: Record<string, TranslationKey> = {
  browser: "composer.slash_command_description_browser",
  "build-ship": "composer.slash_command_description_build_ship",
  compact: "composer.slash_command_description_compact",
  "decision-brief": "composer.slash_command_description_decision_brief",
  "deep-research": "composer.slash_command_description_deep_research",
  goal: "composer.slash_command_description_goal",
  model: "composer.slash_command_description_model",
  "review-improve": "composer.slash_command_description_review_improve",
  skills: "composer.slash_command_description_skills",
  visualize: "composer.slash_command_description_visualize",
  workgraph: "composer.workgraph_command_description",
};

interface SlashCommandPopoverProps {
  activeIndex: number;
  anchorRef: RefObject<HTMLDivElement | null>;
  commands: CommandDescriptor[];
  mode: "commands" | "models" | "skills";
  modelError: ComposerReadFailure | null;
  modelItems: SlashModelOption[];
  modelLoading: boolean;
  modelQuery: string;
  modelSearchRef: RefObject<HTMLInputElement | null>;
  onModelQueryChange: (query: string) => void;
  onModelQueryKeyDown: (event: KeyboardEvent<HTMLInputElement>) => boolean;
  onModelRetry: () => void;
  onClose: () => void;
  onSelectCommand: (command: CommandDescriptor) => void;
  onSelectModel: (model: SlashModelOption) => void;
  onSelectSkill: (skill: SkillInfo) => void;
  onSkillQueryChange: (query: string) => void;
  onSkillQueryKeyDown: (event: KeyboardEvent<HTMLInputElement>) => boolean;
  onSkillRetry: () => void;
  skillError: ComposerReadFailure | null;
  skillItems: SkillInfo[];
  skillLoading: boolean;
  skillQuery: string;
  skillSearchRef: RefObject<HTMLInputElement | null>;
  status: CommandCatalogStatus;
}

export const SlashCommandPopover = memo(function SlashCommandPopover({
  activeIndex,
  anchorRef,
  commands,
  mode,
  modelError,
  modelItems,
  modelLoading,
  modelQuery,
  modelSearchRef,
  onModelQueryChange,
  onModelQueryKeyDown,
  onModelRetry,
  onClose,
  onSelectCommand,
  onSelectModel,
  onSelectSkill,
  onSkillQueryChange,
  onSkillQueryKeyDown,
  onSkillRetry,
  skillError,
  skillItems,
  skillLoading,
  skillQuery,
  skillSearchRef,
  status,
}: SlashCommandPopoverProps) {
  const { t } = useI18n();
  const listRef = useRef<HTMLDivElement>(null);
  const estimatePosition = useCallback(
    (anchor: HTMLDivElement) => getSlashCommandPopoverPosition(anchor, mode),
    [mode],
  );
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: false,
    estimatePosition,
    isOpen: true,
    onClose,
  });

  useEffect(() => {
    const activeElement = listRef.current?.children[activeIndex] as
      | HTMLElement
      | undefined;
    activeElement?.scrollIntoView({ block: "nearest" });
  }, [activeIndex, mode, modelItems.length, skillItems.length]);

  if (!portalContainer) {
    return null;
  }

  const ariaLabel = mode === "skills"
    ? t("composer.skills_picker_title")
    : mode === "models"
      ? t("composer.models_picker_title")
      : t("composer.slash_commands");

  return createPortal(
    <SelectMenuPanel
      ariaLabel={ariaLabel}
      id={overlayId}
      layoutClassName="flex flex-col overflow-hidden"
      panelRef={overlayRef}
      placement={overlayPosition?.placement ?? "top"}
      style={overlayStyle}
      surface="surface"
    >
      {mode === "skills" ? (
        <SlashSearchInput
          inputRef={skillSearchRef}
          onChange={onSkillQueryChange}
          onKeyDown={onSkillQueryKeyDown}
          placeholder={t("composer.skills_search_placeholder")}
          value={skillQuery}
        />
      ) : mode === "models" ? (
        <SlashSearchInput
          inputRef={modelSearchRef}
          onChange={onModelQueryChange}
          onKeyDown={onModelQueryKeyDown}
          placeholder={t("composer.models_search_placeholder")}
          value={modelQuery}
        />
      ) : null}

      {mode === "commands" ? (
        <SlashCommandList
          activeIndex={activeIndex}
          commands={commands}
          listRef={listRef}
          onSelect={onSelectCommand}
          status={status}
          t={t}
        />
      ) : mode === "skills" ? (
        <SlashSkillList
          activeIndex={activeIndex}
          items={skillItems}
          listRef={listRef}
          loading={skillLoading}
          error={skillError}
          retrying={skillLoading}
          onRetry={onSkillRetry}
          onSelect={onSelectSkill}
          t={t}
        />
      ) : (
        <SlashModelList
          activeIndex={activeIndex}
          error={modelError}
          items={modelItems}
          listRef={listRef}
          loading={modelLoading}
          retrying={modelLoading}
          onRetry={onModelRetry}
          onSelect={onSelectModel}
          t={t}
        />
      )}
    </SelectMenuPanel>,
    portalContainer,
  );
});

function SlashSearchInput({
  inputRef,
  onChange,
  onKeyDown,
  placeholder,
  value,
}: {
  inputRef: RefObject<HTMLInputElement | null>;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => boolean;
  placeholder: string;
  value: string;
}) {
  return (
    <div className="border-b border-(--divider-subtle-color) px-2 py-1.5">
      <UiSearchInput
        ref={inputRef}
        aria-label={placeholder}
        className="w-full"
        controlSize="xs"
        inputClassName="text-xs leading-4"
        onChange={onChange}
        onKeyDown={(event) => {
          onKeyDown(event);
        }}
        placeholder={placeholder}
        value={value}
        variant="surface"
      />
    </div>
  );
}

function SlashCommandList({
  activeIndex,
  commands,
  listRef,
  onSelect,
  status,
  t,
}: {
  activeIndex: number;
  commands: CommandDescriptor[];
  listRef: RefObject<HTMLDivElement | null>;
  onSelect: (command: CommandDescriptor) => void;
  status: CommandCatalogStatus;
  t: ReturnType<typeof useI18n>["t"];
}) {
  if (commands.length === 0) {
    return <SlashEmptyState>{resolveSlashCommandEmptyCopy(status, t)}</SlashEmptyState>;
  }

  return (
    <div
      className={SLASH_LIST_CLASS_NAME}
      ref={listRef}
    >
      {commands.map((command, index) => {
        const selectable = isSelectableSlashCommand(command);
        const descriptionKey = SLASH_COMMAND_DESCRIPTION_KEYS[command.name];
        return (
          <button
            aria-disabled={!selectable}
            aria-selected={index === activeIndex}
            className={cn(
              MENU_ITEM_BASE_CLASS_NAME,
              "flex min-h-7 w-full items-center gap-2 px-2 py-1 text-left",
              getMenuItemStateClassName({ active: index === activeIndex }),
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
            title={selectable
              ? undefined
              : command.disabled_reason
                ?? t("composer.slash_command_unavailable")}
            type="button"
          >
            <span className="w-28 shrink-0 truncate font-mono text-xs font-semibold leading-4 text-(--text-strong)">
              /{command.name}
            </span>
            <span className="min-w-0 flex-1 truncate text-xs leading-4 text-(--text-default)">
              {descriptionKey
                ? t(descriptionKey)
                : command.description || t("composer.slash_command_unavailable")}
            </span>
            {command.argument_hint ? (
              <span className="max-w-[32%] shrink-0 truncate font-mono text-[9px] leading-4 text-(--text-soft)">
                {command.argument_hint}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

function SlashSkillList({
  activeIndex,
  error,
  items,
  listRef,
  loading,
  onRetry,
  onSelect,
  retrying,
  t,
}: {
  activeIndex: number;
  error: ComposerReadFailure | null;
  items: SkillInfo[];
  listRef: RefObject<HTMLDivElement | null>;
  loading: boolean;
  onRetry: () => void;
  onSelect: (skill: SkillInfo) => void;
  retrying: boolean;
  t: ReturnType<typeof useI18n>["t"];
}) {
  if (error) {
    return (
      <SlashPickerFailure
        failure={error}
        onRetry={onRetry}
        retrying={retrying}
      />
    );
  }
  if (loading) {
    return <SlashEmptyState>{t("composer.skills_loading")}</SlashEmptyState>;
  }
  if (items.length === 0) {
    return <SlashEmptyState>{t("composer.skills_empty")}</SlashEmptyState>;
  }

  return (
    <div
      className={SLASH_LIST_CLASS_NAME}
      ref={listRef}
    >
      {items.map((skill, index) => {
        const title = skill.title?.trim() || skill.name;
        const description = getSkillDisplayDescription(skill, t);
        return (
          <button
            aria-selected={index === activeIndex}
            className={cn(
              MENU_ITEM_BASE_CLASS_NAME,
              "flex min-h-10 w-full items-center gap-2 px-2 py-1.5 text-left",
              getMenuItemStateClassName({ active: index === activeIndex }),
            )}
            key={skill.name}
            onMouseDown={(event) => {
              event.preventDefault();
              onSelect(skill);
            }}
            role="option"
            title={description || title}
            type="button"
          >
            <span
              aria-hidden="true"
              className={cn(
                "flex h-4 w-4 shrink-0 items-center justify-center",
                skill.enabled_for_agent && !skill.locked
                  ? "text-(--brand-action)"
                  : "text-(--text-soft)",
              )}
            >
              {skill.locked ? (
                <Lock className="h-2.5 w-2.5" />
              ) : skill.enabled_for_agent ? (
                <Check className="h-2.5 w-2.5" />
              ) : null}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate font-mono text-xs font-medium leading-4 text-(--text-strong)">
                /{skill.name}
              </span>
              {description ? (
                <span className="block truncate text-2xs leading-4 text-(--text-muted)">
                  {description}
                </span>
              ) : null}
            </span>
            {!skill.locked && !skill.enabled_for_agent ? (
              <span className="shrink-0 text-[9px] leading-4 text-(--text-soft)">
                {t("composer.skill_use_once")}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

function SlashModelList({
  activeIndex,
  error,
  items,
  listRef,
  loading,
  onRetry,
  onSelect,
  retrying,
  t,
}: {
  activeIndex: number;
  error: ComposerReadFailure | null;
  items: SlashModelOption[];
  listRef: RefObject<HTMLDivElement | null>;
  loading: boolean;
  onRetry: () => void;
  onSelect: (model: SlashModelOption) => void;
  retrying: boolean;
  t: ReturnType<typeof useI18n>["t"];
}) {
  if (error) {
    return (
      <SlashPickerFailure
        failure={error}
        onRetry={onRetry}
        retrying={retrying}
      />
    );
  }
  if (loading) {
    return <SlashEmptyState>{t("composer.models_loading")}</SlashEmptyState>;
  }
  if (items.length === 0) {
    return <SlashEmptyState>{t("composer.models_empty")}</SlashEmptyState>;
  }

  return (
    <div
      className={SLASH_LIST_CLASS_NAME}
      ref={listRef}
    >
      {items.map((model, index) => (
        <button
          aria-selected={index === activeIndex}
          className={cn(
            MENU_ITEM_BASE_CLASS_NAME,
            "flex min-h-7 w-full items-center gap-2 px-2 py-1 text-left",
            getMenuItemStateClassName({ active: index === activeIndex }),
          )}
          key={`${model.provider ?? "runtime"}:${model.id}`}
          onMouseDown={(event) => {
            event.preventDefault();
            onSelect(model);
          }}
          role="option"
          title={model.providerLabel
            ? `${model.label} · ${model.providerLabel}`
            : model.label}
          type="button"
        >
          <span className="min-w-0 flex-1 truncate text-xs leading-4 text-(--text-default)">
            {model.label}
          </span>
          {model.providerLabel ? (
            <span className="max-w-[34%] shrink-0 truncate text-[9px] leading-4 text-(--text-soft)">
              {model.providerLabel}
            </span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

function SlashPickerFailure({
  failure,
  onRetry,
  retrying,
}: {
  failure: ComposerReadFailure;
  onRetry: () => void;
  retrying: boolean;
}) {
  const { t } = useI18n();
  return (
    <UiResourceState
      className="m-1"
      impact={failure.impact}
      primaryAction={{
        busy: retrying,
        label: t("state.retry"),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={failure.title}
      variant="card"
    />
  );
}

function SlashEmptyState({
  children,
  tone = "default",
}: {
  children: string;
  tone?: "default" | "danger";
}) {
  return (
    <p
      className={cn(
        "px-2.5 py-2 text-2xs leading-4",
        tone === "danger" ? "text-(--destructive)" : "text-(--text-soft)",
      )}
    >
      {children}
    </p>
  );
}

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

function getSlashCommandPopoverPosition(
  anchor: HTMLDivElement,
  mode: "commands" | "models" | "skills",
): UiAnchoredOverlayPosition {
  const maxHeight = mode === "commands"
    ? SLASH_COMMAND_PANEL_MAX_HEIGHT_PX
    : SLASH_PICKER_PANEL_MAX_HEIGHT_PX;
  return resolveAnchoredOverlayPosition({
    anchor,
    estimatedHeight: maxHeight,
    maxHeight,
    minHeight: 44,
    placement: "top",
  });
}
