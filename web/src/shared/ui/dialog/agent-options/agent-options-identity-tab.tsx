/**
 * AgentOptions Identity Tab
 *
 * 包含 Avatar、Name、Description、Vibe Tags、Model Provider
 * 从原 basic tab 拆分并增强
 */

"use client";

import { useState, useCallback } from "react";
import { Plus, X as XIcon, User } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentNameValidationResult } from "@/types/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { getIconAvatarSrc } from "@/lib/utils";
import { AVAILABLE_MODELS } from "./agent-options-constants";

interface AgentOptionsIdentityTabProps {
  avatar: string;
  onAvatarChange: (value: string) => void;
  title: string;
  onTitleChange: (value: string) => void;
  description: string;
  onDescriptionChange: (value: string) => void;
  vibeTags: string[];
  onVibeTagsChange: (tags: string[]) => void;
  model: string;
  onModelChange: (value: string) => void;
  nameValidation: AgentNameValidationResult | null;
  isValidatingName: boolean;
}

/** Identity Tab 组件 */
export function AgentOptionsIdentityTab({
  avatar,
  onAvatarChange,
  title,
  onTitleChange,
  description,
  onDescriptionChange,
  vibeTags,
  onVibeTagsChange,
  model,
  onModelChange,
  nameValidation,
  isValidatingName,
}: AgentOptionsIdentityTabProps) {
  const { t } = useI18n();
  const [tagInput, setTagInput] = useState("");

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

  return (
    <div className="space-y-5 animate-in slide-in-from-right-4 duration-300">
      <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(260px,0.9fr)] gap-5">
        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-[14px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
              {getIconAvatarSrc(avatar) ? (
                <img
                  alt={t("agent_options.identity.avatar_alt")}
                  className="h-full w-full object-cover"
                  src={getIconAvatarSrc(avatar) ?? undefined}
                />
              ) : (
                <User className="h-7 w-7 text-primary" />
              )}
            </div>
            <div className="min-w-0 flex-1 space-y-1.5">
              <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {t("agent_options.identity.name")} <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => onTitleChange(e.target.value)}
                className="dialog-input rounded-xl flex h-10 w-full px-3.5 py-2 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) transition-all"
                placeholder={t("agent_options.identity.name_placeholder")}
              />
            </div>
          </div>

          <IconPicker
            columns={6}
            icon_size="md"
            layout="row"
            max_icons={12}
            on_select={onAvatarChange}
            show_clear={false}
            value={avatar}
          />

          <div className="min-h-5 text-xs">
            {isValidatingName ? (
              <span className="text-muted-foreground">{t("agent_options.identity.validating_name")}</span>
            ) : null}
            {!isValidatingName && nameValidation?.reason ? (
              <span className="text-red-500">{nameValidation.reason}</span>
            ) : null}
            {!isValidatingName &&
              nameValidation?.is_valid &&
              nameValidation?.is_available ? (
              <span className="text-emerald-600">
                {t("agent_options.identity.name_available", {
                  path: nameValidation.workspace_path ?? "",
                })}
              </span>
            ) : null}
          </div>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.vibe_tags")}</label>
            <div className="rounded-[18px] border border-(--divider-subtle-color) px-3.5 py-3">
              <div className="flex flex-wrap items-center gap-2">
                {vibeTags.map((tag) => (
                  <span
                    key={tag}
                    className={cn(
                      "inline-flex items-center gap-1 rounded-full border border-primary/18 bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary"
                    )}
                  >
                    {tag}
                    <button
                      type="button"
                      onClick={() => handleRemoveTag(tag)}
                      className="ml-0.5 rounded-full p-0.5 transition-colors hover:bg-primary/20"
                    >
                      <XIcon className="h-3 w-3" />
                    </button>
                  </span>
                ))}
                <div className="flex items-center gap-1">
                  <input
                    type="text"
                    value={tagInput}
                    onChange={(e) => setTagInput(e.target.value)}
                    onKeyDown={handleTagKeyDown}
                    className="dialog-input rounded-lg h-7 w-24 px-2 text-xs text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none transition-all"
                    placeholder={t("agent_options.identity.add_tag")}
                  />
                  <button
                    type="button"
                    onClick={handleAddTag}
                    className="flex h-7 w-7 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                  >
                    <Plus className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-[11px] font-semibold text-(--text-muted)">
              {t("agent_options.identity.model")} <span className="text-red-500">*</span>
            </label>
            <div className="relative">
              <select
                value={model}
                onChange={(e) => onModelChange(e.target.value)}
                className="dialog-input rounded-xl flex h-10 w-full appearance-none px-3.5 py-2 text-sm text-(--text-strong) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) transition-all"
              >
                {AVAILABLE_MODELS.map((m) => (
                  <option key={m.value} value={m.value}>
                    {m.label}
                  </option>
                ))}
              </select>
              <div className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                <svg
                  width="10"
                  height="6"
                  viewBox="0 0 10 6"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <path
                    d="M1 1L5 5L9 1"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-(--text-muted)">{t("agent_options.identity.description")}</label>
        <textarea
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          className="dialog-input rounded-2xl flex min-h-[72px] w-full resize-y px-3.5 py-2.5 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) transition-all"
          rows={3}
          placeholder={t("agent_options.identity.description_placeholder")}
        />
      </div>
    </div>
  );
}
