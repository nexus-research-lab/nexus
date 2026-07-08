/**
 * AgentOptions Identity Tab
 *
 * 包含 Avatar、Name、Description、Vibe Tags、Provider
 * 从原 basic tab 拆分并增强
 */

"use client";

import { useCallback, useMemo, useState } from "react";
import { Plus, X as XIcon } from "lucide-react";
import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
  cn,
} from "@/lib/utils";
import type { AgentNameValidationResult, AgentProvider } from "@/types/agent/agent";
import {
  formatProviderLabel,
  formatProviderOptionLabel,
  type ProviderOption,
} from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiIconButton } from "@/shared/ui/button";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { UiSelectMenu } from "@/shared/ui/select-menu";

interface AgentOptionsIdentityTabProps {
  avatar: string;
  onAvatarChange: (value: string) => void;
  title: string;
  onTitleChange: (value: string) => void;
  description: string;
  onDescriptionChange: (value: string) => void;
  vibeTags: string[];
  onVibeTagsChange: (tags: string[]) => void;
  provider: AgentProvider;
  model: string;
  defaultProvider: AgentProvider;
  defaultModel: string;
  providerOptions: ProviderOption[];
  providerOptionsError: string | null;
  providerOptionsLoading: boolean;
  onProviderChange: (value: AgentProvider) => void;
  onModelChange: (value: string) => void;
  nameValidation: AgentNameValidationResult | null;
  isValidatingName: boolean;
  variant?: "dialog" | "inline";
}

/** Identity Tab 组件 */
export function AgentOptionsIdentityTab({
  avatar,
  onAvatarChange: onAvatarChange,
  title,
  onTitleChange: onTitleChange,
  description,
  onDescriptionChange: onDescriptionChange,
  vibeTags: vibeTags,
  onVibeTagsChange: onVibeTagsChange,
  provider,
  model,
  defaultProvider: defaultProvider,
  defaultModel: defaultModel,
  providerOptions: providerOptions,
  providerOptionsError: providerOptionsError,
  providerOptionsLoading: providerOptionsLoading,
  onProviderChange: onProviderChange,
  onModelChange: onModelChange,
  nameValidation: nameValidation,
  isValidatingName: isValidatingName,
  variant = "dialog",
}: AgentOptionsIdentityTabProps) {
  const { t } = useI18n();
  const [tagInput, setTagInput] = useState("");
  const defaultModelOptionLabel = defaultProvider && defaultModel
    ? t("agent_options.identity.follow_default_provider_named", {
      name: `${formatProviderLabel(defaultProvider)} / ${defaultModel}`,
    })
    : t("agent_options.identity.follow_default_provider");
  const selectedModelValue = provider.trim() && model.trim()
    ? JSON.stringify([provider.trim(), model.trim()])
    : "";
  const modelSelectOptions = useMemo(() => [
    { value: "", label: defaultModelOptionLabel },
    ...providerOptions.flatMap((providerOption) => providerOption.models.map((modelOption) => {
      const providerLabel = formatProviderOptionLabel(
        providerOption,
        t("settings.providers.subscription_badge"),
      );
      const modelLabel = modelOption.display_name || modelOption.model_id;
      return {
        value: JSON.stringify([providerOption.provider, modelOption.model_id]),
        label: `${providerLabel} / ${modelLabel}`,
      };
    })),
  ], [defaultModelOptionLabel, providerOptions, t]);

  const handleModelSelectChange = useCallback((value: string) => {
    if (!value) {
      onProviderChange("");
      onModelChange("");
      return;
    }
    try {
      const parsed = JSON.parse(value) as unknown;
      if (!Array.isArray(parsed) || parsed.length !== 2) {
        return;
      }
      const [nextProvider, nextModel] = parsed;
      if (typeof nextProvider !== "string" || typeof nextModel !== "string") {
        return;
      }
      onProviderChange(nextProvider.trim());
      onModelChange(nextModel.trim());
    } catch {
      return;
    }
  }, [onModelChange, onProviderChange]);

  /** 添加标签 */
  const handleAddTag = useCallback(() => {
    const trimmed = tagInput.trim();
    if (trimmed && !vibeTags.includes(trimmed)) {
      onVibeTagsChange([...vibeTags, trimmed]);
    }
    setTagInput("");
  }, [tagInput, vibeTags, onVibeTagsChange]);

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
      onVibeTagsChange(vibeTags.filter((t) => t !== tag));
    },
    [vibeTags, onVibeTagsChange]
  );

  const validationMessage = (
    <div className="min-h-5 text-xs">
      {isValidatingName ? (
        <span className="text-muted-foreground">{t("agent_options.identity.validating_name")}</span>
      ) : null}
      {!isValidatingName && nameValidation?.reason ? (
        <span className="text-(--destructive)">{nameValidation.reason}</span>
      ) : null}
      {!isValidatingName &&
        nameValidation?.is_valid &&
        nameValidation?.is_available ? (
        <span className="text-(--success)">
          {t("agent_options.identity.name_available", {
            path: nameValidation.workspace_path ?? "",
          })}
        </span>
      ) : null}
    </div>
  );

  const renderVibeTagsRow = (
    inputClassName: string,
    addButtonSize: "sm" | "md",
    gapClassName: string
  ) => (
    <div className="soft-scrollbar flex flex-nowrap items-center gap-2 overflow-x-auto overflow-y-hidden pb-1">
      {vibeTags.map((tag) => (
        <span
          key={tag}
          className={cn(
            "inline-flex shrink-0 items-center gap-1 rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_16%,transparent)] bg-transparent px-2 py-0.5 text-[11px] font-medium text-primary"
          )}
        >
          {tag}
          <UiIconButton
            aria-label={`移除 ${tag}`}
            className="ml-0.5 h-5 w-5 rounded-full"
            onClick={() => handleRemoveTag(tag)}
            size="xs"
            type="button"
            variant="ghost"
          >
            <XIcon className="h-3 w-3" />
          </UiIconButton>
        </span>
      ))}
      <div className={cn("flex shrink-0 items-center", gapClassName)}>
        <UiInput
          className={inputClassName}
          controlSize={addButtonSize === "md" ? "sm" : "xs"}
          onChange={(e) => setTagInput(e.target.value)}
          onKeyDown={handleTagKeyDown}
          placeholder={t("agent_options.identity.add_tag")}
          type="text"
          value={tagInput}
        />
        <UiIconButton
          aria-label={t("agent_options.identity.add_tag")}
          size={addButtonSize}
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
                className="h-13 w-13 rounded-[12px]"
                name={title || t("agent_options.identity.avatar_alt")}
                shape="rounded"
              />
              <div className="min-w-0 flex-1 space-y-1.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                  {t("agent_options.identity.name")} <span className="text-(--destructive)">*</span>
                </label>
                <UiInput
                  className="rounded-xl"
                  controlSize="md"
                  onChange={(e) => onTitleChange(e.target.value)}
                  placeholder={t("agent_options.identity.name_placeholder")}
                  type="text"
                  value={title}
                />
              </div>
            </div>

            <IconPicker
              columns={6}
              iconSize="sm"
              layout="row"
              maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
              onSelect={onAvatarChange}
              showClear={false}
              startIconId={AGENT_ICON_ID_START}
              value={avatar}
            />

            {validationMessage}
          </div>

          <div className="w-full space-y-4 pt-0.5 xl:w-[340px] xl:shrink-0">
            <div className="space-y-2.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.vibe_tags")}
              </label>
              {renderVibeTagsRow(
                "w-[112px] rounded-full",
                "md",
                "gap-2"
              )}
            </div>

            <div className="space-y-2.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.model")}
              </label>
              <UiSelectMenu
                allowLabelWrap
                ariaLabel={t("agent_options.identity.model")}
                buttonClassName="h-auto min-h-9 py-2"
                className="h-auto min-h-9"
                disabled={providerOptionsLoading && providerOptions.length === 0}
                menuMinWidth={460}
                onChange={handleModelSelectChange}
                options={modelSelectOptions}
                size="sm"
                surface="dialog"
                value={selectedModelValue}
              />
              {providerOptionsError ? (
                <p className="text-xs text-rose-500">{providerOptionsError}</p>
              ) : null}
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.description")}</label>
          <UiTextarea
            className="min-h-[72px] rounded-2xl"
            onChange={(e) => onDescriptionChange(e.target.value)}
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
              className="h-14 w-14 rounded-[14px]"
              name={title || t("agent_options.identity.avatar_alt")}
              shape="rounded"
              size="lg"
            />
            <div className="min-w-0 flex-1 space-y-1.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.name")} <span className="text-(--destructive)">*</span>
              </label>
              <UiInput
                className="h-10 rounded-xl"
                controlSize="md"
                onChange={(e) => onTitleChange(e.target.value)}
                placeholder={t("agent_options.identity.name_placeholder")}
                type="text"
                value={title}
              />
            </div>
          </div>

          <IconPicker
            columns={6}
            iconSize="md"
            layout="row"
            maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
            onSelect={onAvatarChange}
            showClear={false}
            startIconId={AGENT_ICON_ID_START}
            value={avatar}
          />

          {validationMessage}
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.vibe_tags")}</label>
            <div className="rounded-[12px] border border-transparent px-0 py-0">
              {renderVibeTagsRow(
                "w-[108px] rounded-lg",
                "sm",
                "gap-1"
              )}
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">
              {t("agent_options.identity.model")}
            </label>
            <UiSelectMenu
              allowLabelWrap
              ariaLabel={t("agent_options.identity.model")}
              buttonClassName="h-auto min-h-10 py-2"
              className="h-auto min-h-10"
              disabled={providerOptionsLoading && providerOptions.length === 0}
              menuMinWidth={460}
              onChange={handleModelSelectChange}
              options={modelSelectOptions}
              surface="dialog"
              value={selectedModelValue}
            />
            {providerOptionsError ? (
              <p className="mt-2 text-xs text-rose-500">{providerOptionsError}</p>
            ) : null}
          </div>
        </div>
      </div>

      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.description")}</label>
        <UiTextarea
          className="min-h-[72px] rounded-2xl"
          onChange={(e) => onDescriptionChange(e.target.value)}
          placeholder={t("agent_options.identity.description_placeholder")}
          rows={3}
          value={description}
        />
      </div>
    </div>
  );
}
