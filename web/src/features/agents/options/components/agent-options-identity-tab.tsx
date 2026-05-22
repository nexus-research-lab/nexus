/**
 * AgentOptions Identity Tab
 *
 * 包含 Avatar、Name、Description、Vibe Tags、Provider
 * 从原 basic tab 拆分并增强
 */

"use client";

import { useState, useCallback } from "react";
import { Plus, X as XIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentNameValidationResult, AgentProvider } from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiIconButton } from "@/shared/ui/button";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { AGENT_ICON_ID_END, AGENT_ICON_ID_START } from "@/lib/utils";
import { format_provider_label } from "@/types/capability/provider";

interface AgentOptionsIdentityTabProps {
  avatar: string;
  on_avatar_change: (value: string) => void;
  title: string;
  on_title_change: (value: string) => void;
  description: string;
  on_description_change: (value: string) => void;
  vibe_tags: string[];
  on_vibe_tags_change: (tags: string[]) => void;
  provider: AgentProvider;
  default_provider: AgentProvider;
  provider_options: ProviderOption[];
  provider_options_error: string | null;
  provider_options_loading: boolean;
  on_provider_change: (value: AgentProvider) => void;
  name_validation: AgentNameValidationResult | null;
  is_validating_name: boolean;
  variant?: "dialog" | "inline";
}

/** Identity Tab 组件 */
export function AgentOptionsIdentityTab({
  avatar,
  on_avatar_change,
  title,
  on_title_change,
  description,
  on_description_change,
  vibe_tags,
  on_vibe_tags_change,
  provider,
  default_provider,
  provider_options,
  provider_options_error,
  provider_options_loading,
  on_provider_change,
  name_validation,
  is_validating_name,
  variant = "dialog",
}: AgentOptionsIdentityTabProps) {
  const { t } = useI18n();
  const [tagInput, setTagInput] = useState("");
  const defaultProviderOptionLabel = default_provider
    ? t("agent_options.identity.follow_default_provider_named", {
      name: format_provider_label(default_provider),
    })
    : t("agent_options.identity.follow_default_provider");
  const provider_select_options = [
    { value: "", label: defaultProviderOptionLabel },
    ...provider_options.map((item) => ({
      value: item.provider,
      label: item.display_name,
    })),
  ];

  /** 添加标签 */
  const handleAddTag = useCallback(() => {
    const trimmed = tagInput.trim();
    if (trimmed && !vibe_tags.includes(trimmed)) {
      on_vibe_tags_change([...vibe_tags, trimmed]);
    }
    setTagInput("");
  }, [tagInput, vibe_tags, on_vibe_tags_change]);

  /** 按回车添加标签 */
  const handleTagKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        e.preventDefault();
        handleAddTag();
      }
    },
    [handleAddTag]
  );

  /** 删除标签 */
  const handleRemoveTag = useCallback(
    (tag: string) => {
      on_vibe_tags_change(vibe_tags.filter((t) => t !== tag));
    },
    [vibe_tags, on_vibe_tags_change]
  );

  const validation_message = (
    <div className="min-h-5 text-xs">
      {is_validating_name ? (
        <span className="text-muted-foreground">{t("agent_options.identity.validating_name")}</span>
      ) : null}
      {!is_validating_name && name_validation?.reason ? (
        <span className="text-red-500">{name_validation.reason}</span>
      ) : null}
      {!is_validating_name &&
        name_validation?.is_valid &&
        name_validation?.is_available ? (
        <span className="text-emerald-600">
          {t("agent_options.identity.name_available", {
            path: name_validation.workspace_path ?? "",
          })}
        </span>
      ) : null}
    </div>
  );

  const render_vibe_tags_row = (
    input_class_name: string,
    add_button_size: "sm" | "md",
    gap_class_name: string
  ) => (
    <div className="soft-scrollbar flex flex-nowrap items-center gap-2 overflow-x-auto overflow-y-hidden pb-1">
      {vibe_tags.map((tag) => (
        <span
          key={tag}
          className={cn(
            "inline-flex shrink-0 items-center gap-1 rounded-full border border-primary/18 bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary"
          )}
        >
          {tag}
          <UiIconButton
            aria-label={`移除 ${tag}`}
            class_name="ml-0.5 h-5 w-5 rounded-full"
            onClick={() => handleRemoveTag(tag)}
            size="xs"
            type="button"
            variant="ghost"
          >
            <XIcon className="h-3 w-3" />
          </UiIconButton>
        </span>
      ))}
      <div className={cn("flex shrink-0 items-center", gap_class_name)}>
        <UiInput
          class_name={input_class_name}
          control_size={add_button_size === "md" ? "sm" : "xs"}
          onChange={(e) => setTagInput(e.target.value)}
          onKeyDown={handleTagKeyDown}
          placeholder={t("agent_options.identity.add_tag")}
          type="text"
          value={tagInput}
        />
        <UiIconButton
          aria-label={t("agent_options.identity.add_tag")}
          size={add_button_size}
          onClick={handleAddTag}
          type="button"
          variant="ghost"
        >
          <Plus className="h-3.5 w-3.5" />
        </UiIconButton>
      </div>
    </div>
  );

  if (variant === "inline") {
    return (
      <div className="space-y-5 animate-in slide-in-from-right-4 duration-300">
        <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
          <div className="min-w-0 flex-1 space-y-3 xl:max-w-[480px]">
            <div className="flex items-end gap-2.5">
              <UiAgentAvatar
                avatar={avatar}
                class_name="h-13 w-13 rounded-[12px]"
                name={title || t("agent_options.identity.avatar_alt")}
                shape="rounded"
              />
              <div className="min-w-0 flex-1 space-y-1.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                  {t("agent_options.identity.name")} <span className="text-red-500">*</span>
                </label>
                <UiInput
                  class_name="rounded-xl"
                  control_size="md"
                  onChange={(e) => on_title_change(e.target.value)}
                  placeholder={t("agent_options.identity.name_placeholder")}
                  type="text"
                  value={title}
                />
              </div>
            </div>

            <IconPicker
              columns={6}
              icon_size="sm"
              layout="row"
              max_icons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
              on_select={on_avatar_change}
              show_clear={false}
              start_icon_id={AGENT_ICON_ID_START}
              value={avatar}
            />

            {validation_message}
          </div>

          <div className="w-full space-y-4 pt-0.5 xl:w-[188px] xl:shrink-0">
            <div className="space-y-2.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.vibe_tags")}
              </label>
              {render_vibe_tags_row(
                "w-[112px] rounded-full",
                "md",
                "gap-2"
              )}
            </div>

            <div className="space-y-2.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.provider")}
              </label>
              <UiSelectMenu
                aria_label={t("agent_options.identity.provider")}
                button_class_name="dialog-input"
                disabled={provider_options_loading && provider_options.length === 0}
                on_change={(value) => on_provider_change(value as AgentProvider)}
                options={provider_select_options}
                size="sm"
                value={provider}
              />
              {provider_options_error ? (
                <p className="text-xs text-rose-500">{provider_options_error}</p>
              ) : null}
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.description")}</label>
          <UiTextarea
            class_name="min-h-[72px] rounded-2xl"
            onChange={(e) => on_description_change(e.target.value)}
            placeholder={t("agent_options.identity.description_placeholder")}
            rows={3}
            value={description}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 animate-in slide-in-from-right-4 duration-300">
      <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(260px,0.9fr)] gap-5">
        <div className="space-y-3">
          <div className="flex items-end gap-3">
            <UiAgentAvatar
              avatar={avatar}
              class_name="h-14 w-14 rounded-[14px]"
              name={title || t("agent_options.identity.avatar_alt")}
              shape="rounded"
              size="lg"
            />
            <div className="min-w-0 flex-1 space-y-1.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.name")} <span className="text-red-500">*</span>
              </label>
              <UiInput
                class_name="h-10 rounded-xl"
                control_size="md"
                onChange={(e) => on_title_change(e.target.value)}
                placeholder={t("agent_options.identity.name_placeholder")}
                type="text"
                value={title}
              />
            </div>
          </div>

          <IconPicker
            columns={6}
            icon_size="md"
            layout="row"
            max_icons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
            on_select={on_avatar_change}
            show_clear={false}
            start_icon_id={AGENT_ICON_ID_START}
            value={avatar}
          />

          {validation_message}
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.vibe_tags")}</label>
            <div className="rounded-[18px] border border-(--divider-subtle-color) px-3.5 py-3">
              {render_vibe_tags_row(
                "w-[108px] rounded-lg",
                "sm",
                "gap-1"
              )}
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">
              {t("agent_options.identity.provider")}
            </label>
            <UiSelectMenu
              aria_label={t("agent_options.identity.provider")}
              button_class_name="dialog-input"
              disabled={provider_options_loading && provider_options.length === 0}
              on_change={(value) => on_provider_change(value as AgentProvider)}
              options={provider_select_options}
              value={provider}
            />
            {provider_options_error ? (
              <p className="mt-2 text-xs text-rose-500">{provider_options_error}</p>
            ) : null}
          </div>
        </div>
      </div>

      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.description")}</label>
        <UiTextarea
          class_name="min-h-[72px] rounded-2xl"
          onChange={(e) => on_description_change(e.target.value)}
          placeholder={t("agent_options.identity.description_placeholder")}
          rows={3}
          value={description}
        />
      </div>
    </div>
  );
}
