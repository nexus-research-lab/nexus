/**
 * # !/usr/bin/env tsx
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：provider-settings-panel.tsx
 * # @Date   ：2026/04/14 21:57
 * # @Author ：leemysw
 * # 2026/04/14 21:57   Create
 * # =====================================================
 */

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Cable, Loader2, Plus, Star } from "lucide-react";

import { set_default_agent_provider } from "@/config/options";
import { FeedbackBanner } from "@/features/capability/skills/feedback-banner";
import {
  create_provider_config_api,
  delete_provider_config_api,
  list_provider_configs_api,
  update_provider_config_api,
} from "@/lib/provider-config-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/workspace-surface-scaffold";
import type {
  ProviderConfigPayload,
  ProviderConfigRecord,
  UpdateProviderConfigPayload,
} from "@/types/provider";

type SettingsTabKey = "providers";
type FeedbackTone = "success" | "error";
type FormMode = "empty" | "create" | "edit";

interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

interface ProviderDraft extends ProviderConfigPayload { }

interface ProviderSettingsPanelProps {
  embedded?: boolean;
}

const SETTINGS_TABS: { key: SettingsTabKey; label_key: "settings.tabs.providers" }[] = [
  { key: "providers", label_key: "settings.tabs.providers" },
];

const PROVIDER_ACTION_BUTTON_CLASS_NAME = "inline-flex h-9 items-center justify-center rounded-xl border px-3 text-sm font-medium tracking-tight transition-[border-color,background,color,box-shadow,transform] duration-(--motion-duration-fast) ease-out disabled:pointer-events-none disabled:opacity-(--disabled-opacity)";
const PROVIDER_SAVE_BUTTON_CLASS_NAME = `${PROVIDER_ACTION_BUTTON_CLASS_NAME} border-(--surface-interactive-active-border) bg-primary text-white shadow-[0_8px_24px_rgba(16,185,129,0.16)] hover:-translate-y-px hover:shadow-[0_12px_28px_rgba(16,185,129,0.22)]`;
const PROVIDER_SECONDARY_BUTTON_CLASS_NAME = `${PROVIDER_ACTION_BUTTON_CLASS_NAME} border-(--divider-subtle-color) bg-(--surface-base-background) text-(--text-strong) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)`;
const PROVIDER_DANGER_BUTTON_CLASS_NAME = `${PROVIDER_ACTION_BUTTON_CLASS_NAME} border-[rgba(239,68,68,0.22)] bg-[rgba(239,68,68,0.06)] text-[rgb(220,38,38)] hover:border-[rgba(239,68,68,0.32)] hover:bg-[rgba(239,68,68,0.1)]`;

function build_provider_draft(is_first_provider: boolean): ProviderDraft {
  return {
    provider: "",
    display_name: "",
    auth_token: "",
    base_url: "",
    model: "",
    enabled: true,
    is_default: is_first_provider,
  };
}

function to_provider_draft(item: ProviderConfigRecord): ProviderDraft {
  return {
    provider: item.provider,
    display_name: item.display_name || item.provider,
    auth_token: "",
    base_url: item.base_url,
    model: item.model,
    enabled: true,
    is_default: item.is_default,
  };
}

function get_provider_title(item: ProviderConfigRecord): string {
  return item.display_name || item.provider;
}

function order_provider_records(
  items: ProviderConfigRecord[],
  previous_items: ProviderConfigRecord[],
): ProviderConfigRecord[] {
  const previous_index_map = new Map(
    previous_items.map((item, index) => [item.provider, index]),
  );

  return [...items].sort((left, right) => {
    const left_index = previous_index_map.get(left.provider);
    const right_index = previous_index_map.get(right.provider);

    if (left_index !== undefined && right_index !== undefined) {
      return left_index - right_index;
    }
    if (left_index !== undefined) {
      return -1;
    }
    if (right_index !== undefined) {
      return 1;
    }

    return get_provider_title(left).localeCompare(get_provider_title(right), "zh-Hans-CN");
  });
}

export function ProviderSettingsPanel({ embedded = false }: ProviderSettingsPanelProps) {
  const { t } = useI18n();
  const [providers, set_providers] = useState<ProviderConfigRecord[]>([]);
  const [selected_provider, set_selected_provider] = useState<string | null>(null);
  const [mode, set_mode] = useState<FormMode>("empty");
  const [draft, set_draft] = useState<ProviderDraft>(build_provider_draft(true));
  const [loading, set_loading] = useState(true);
  const [submitting, set_submitting] = useState(false);
  const [pending_default_provider, set_pending_default_provider] = useState<string | null>(null);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);
  const [delete_confirm_open, set_delete_confirm_open] = useState(false);
  const providers_ref = useRef<ProviderConfigRecord[]>([]);
  const selected_provider_ref = useRef<string | null>(null);

  useEffect(() => {
    providers_ref.current = providers;
  }, [providers]);

  useEffect(() => {
    selected_provider_ref.current = selected_provider;
  }, [selected_provider]);

  const selected_record = useMemo(
    () => providers.find((item) => item.provider === selected_provider) ?? null,
    [providers, selected_provider],
  );
  const is_editing = mode === "edit" && !!selected_record;
  const is_creating = mode === "create";
  const is_empty_mode = mode === "empty";
  const can_delete = !!selected_record && selected_record.usage_count === 0;
  const can_save = useMemo(() => {
    if (is_empty_mode) {
      return false;
    }
    if (!draft.provider.trim() || !draft.base_url.trim() || !draft.model.trim()) {
      return false;
    }
    if (is_creating && !draft.auth_token.trim()) {
      return false;
    }
    return true;
  }, [draft.auth_token, draft.base_url, draft.model, draft.provider, is_creating, is_empty_mode]);

  const sync_provider_snapshot = useCallback((
    items: ProviderConfigRecord[],
    preferred_provider?: string | null,
  ) => {
    const ordered_items = order_provider_records(items, providers_ref.current);
    set_providers(ordered_items);
    set_default_agent_provider(ordered_items.find((item) => item.is_default)?.provider);

    if (ordered_items.length === 0) {
      set_mode("empty");
      set_selected_provider(null);
      set_draft(build_provider_draft(true));
      return;
    }

    const target = ordered_items.find((item) => item.provider === preferred_provider)
      ?? ordered_items.find((item) => item.provider === selected_provider_ref.current)
      ?? ordered_items[0];
    set_mode("edit");
    set_selected_provider(target.provider);
    set_draft(to_provider_draft(target));
  }, []);

  const refresh_providers = useCallback(async (preferred_provider?: string | null) => {
    try {
      const items = await list_provider_configs_api();
      sync_provider_snapshot(items, preferred_provider);
      set_feedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.load_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.load_failed_message"),
      });
    } finally {
      set_loading(false);
    }
  }, [sync_provider_snapshot, t]);

  useEffect(() => {
    void refresh_providers();
  }, [refresh_providers]);

  const handle_select_provider = useCallback((provider: string) => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return;
    }
    set_mode("edit");
    set_selected_provider(target.provider);
    set_draft(to_provider_draft(target));
  }, [providers]);

  const handle_create_mode = useCallback(() => {
    set_mode("create");
    if (providers.length === 0) {
      set_selected_provider(null);
    }
    set_draft(build_provider_draft(providers.length === 0));
  }, [providers.length]);

  const handle_cancel = useCallback(() => {
    if (providers.length === 0) {
      set_mode("empty");
      set_draft(build_provider_draft(true));
      return;
    }
    sync_provider_snapshot(providers, selected_provider);
  }, [providers, selected_provider, sync_provider_snapshot]);

  const handle_set_default = useCallback(async (item: ProviderConfigRecord) => {
    if (submitting || pending_default_provider || item.is_default) {
      return;
    }

    try {
      set_pending_default_provider(item.provider);
      await update_provider_config_api(item.provider, {
        display_name: get_provider_title(item),
        base_url: item.base_url,
        model: item.model,
        enabled: true,
        is_default: true,
      });
      await refresh_providers(selected_provider_ref.current ?? item.provider);
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.save_failed_message"),
      });
    } finally {
      set_pending_default_provider(null);
    }
  }, [pending_default_provider, refresh_providers, submitting, t]);

  const handle_save = useCallback(async () => {
    if (!can_save || submitting) {
      return;
    }

    try {
      set_submitting(true);
      const normalized_provider = draft.provider.trim();
      const normalized_display_name = draft.display_name.trim() || normalized_provider;
      const base_payload: UpdateProviderConfigPayload = {
        display_name: normalized_display_name,
        base_url: draft.base_url.trim(),
        model: draft.model.trim(),
        enabled: true,
        is_default: draft.is_default,
      };
      const normalized_auth_token = draft.auth_token.trim();
      if (normalized_auth_token) {
        base_payload.auth_token = normalized_auth_token;
      }

      const result = is_editing && selected_record
        ? await update_provider_config_api(selected_record.provider, base_payload)
        : await create_provider_config_api({
          provider: normalized_provider,
          auth_token: normalized_auth_token,
          ...base_payload,
        });

      await refresh_providers(result.provider);
      set_feedback({
        tone: "success",
        title: t("settings.providers.save_success_title"),
        message: t("settings.providers.save_success_message", { name: result.display_name }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.save_failed_message"),
      });
    } finally {
      set_submitting(false);
    }
  }, [can_save, draft, is_editing, refresh_providers, selected_record, submitting, t]);

  const handle_delete = useCallback(async () => {
    if (!selected_record || submitting) {
      return;
    }

    try {
      set_submitting(true);
      await delete_provider_config_api(selected_record.provider);
      set_delete_confirm_open(false);
      await refresh_providers();
      set_feedback({
        tone: "success",
        title: t("settings.providers.delete_success_title"),
        message: t("settings.providers.delete_success_message", { name: get_provider_title(selected_record) }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.delete_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.delete_failed_message"),
      });
    } finally {
      set_submitting(false);
    }
  }, [refresh_providers, selected_record, submitting, t]);

  const token_placeholder = is_editing
    ? (selected_record?.auth_token_masked || t("settings.providers.token_empty"))
    : t("settings.providers.auth_token_placeholder");

  const panel_content = (
    <div className="px-5 py-4 xl:px-6">
      <div className="grid min-h-full flex-1 xl:grid-cols-[minmax(0,280px)_minmax(0,1fr)]">
          <aside className="flex min-h-0 flex-col border-r border-(--divider-subtle-color) pr-4 xl:pr-6">
            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto">
              {loading ? (
                <div className="flex min-h-[260px] items-center justify-center text-(--text-soft)">
                  <Loader2 className="h-4 w-4 animate-spin" />
                </div>
              ) : (
                <div className="space-y-1.5 px-1 py-2">
                  {providers.map((item) => {
                    const is_active = item.provider === selected_provider && is_editing;
                    const is_pending_default = pending_default_provider === item.provider;
                    return (
                      <div
                        className={cn(
                          "rounded-2xl border px-3 py-3 transition-[border-color,background,color] duration-(--motion-duration-fast)",
                          is_active
                            ? "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background)"
                            : "border-(--divider-subtle-color) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
                        )}
                        key={item.provider}
                        onClick={() => handle_select_provider(item.provider)}
                        onKeyDown={(event) => {
                          if (event.key === "Enter" || event.key === " ") {
                            event.preventDefault();
                            handle_select_provider(item.provider);
                          }
                        }}
                        role="button"
                        tabIndex={0}
                      >
                        <div className="min-w-0">
                          <div className="flex items-center justify-between gap-3">
                            <div className="min-w-0">
                              <span className="truncate text-sm font-semibold text-(--text-strong)">
                                {get_provider_title(item)}
                              </span>
                            </div>
                            <button
                              aria-label={t("settings.providers.set_default_provider")}
                              className={cn(
                                "inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full transition-[background,color,box-shadow] duration-(--motion-duration-fast)",
                                item.is_default
                                  ? "cursor-default text-primary"
                                  : "text-(--text-soft) hover:bg-(--surface-interactive-hover-background) hover:text-primary",
                                is_pending_default && "opacity-(--disabled-opacity)",
                              )}
                              disabled={item.is_default || is_pending_default}
                              onClick={(event) => {
                                event.stopPropagation();
                                if (!item.is_default) {
                                  void handle_set_default(item);
                                }
                              }}
                              title={t("settings.providers.set_default_provider")}
                              type="button"
                            >
                              {is_pending_default ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              ) : (
                                <Star className={cn("h-3.5 w-3.5", item.is_default ? "fill-current" : "")} />
                              )}
                            </button>
                          </div>

                          <div className="mt-2 truncate text-[11px] text-(--text-soft)">
                            {item.base_url || "--"}
                          </div>

                          <div className="mt-1 truncate text-[11px] text-(--text-soft)">
                            {item.model}
                          </div>

                          <div className="mt-2 flex items-center justify-between gap-3 text-[11px] text-(--text-soft)">
                            <span className="truncate select-none">
                              {item.auth_token_masked || t("settings.providers.token_empty")}
                            </span>
                            <span className="chip-default rounded-full px-2 py-0.5 text-[10px] font-medium">
                              {t("settings.providers.usage", { count: item.usage_count })}
                            </span>
                          </div>
                        </div>
                      </div>
                    );
                  })}

                  <button
                    className={cn(
                      "w-full rounded-2xl border px-3 py-3 text-left transition-[border-color,background,color] duration-(--motion-duration-fast)",
                      is_creating
                        ? "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background)"
                        : "border-dashed border-(--divider-subtle-color) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
                    )}
                    onClick={handle_create_mode}
                    type="button"
                  >
                    <div className="flex items-center gap-2 text-sm font-semibold text-(--text-strong)">
                      <div className="flex h-7 w-7 items-center justify-center rounded-full border border-dashed border-(--surface-interactive-active-border) bg-(--surface-inset-background) text-primary">
                        <Plus className="h-3.5 w-3.5" />
                      </div>
                      {t("settings.providers.add_provider")}
                    </div>
                  </button>
                </div>
              )}
            </div>
          </aside>

          <section className="min-h-[360px] pl-0 pt-1 xl:pl-6">
            {is_empty_mode ? (
              <div className="flex min-h-[360px] flex-1 items-center justify-center">
                <div className="text-center">
                  <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-dashed border-(--divider-subtle-color) bg-(--surface-inset-background) text-primary">
                    <Plus className="h-4 w-4" />
                  </div>
                  <p className="mt-3 text-sm font-semibold text-(--text-strong)">
                    {t("settings.providers.empty")}
                  </p>
                  <p className="mt-1 text-[12px] text-(--text-soft)">
                    {t("settings.providers.empty_hint")}
                  </p>
                </div>
              </div>
            ) : (
              <div
                className="grid min-h-[360px] grid-rows-[auto_1fr_auto]"
                key={is_editing && selected_record ? selected_record.provider : "create"}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-2">
                    <h2 className="truncate text-[17px] font-semibold tracking-tight text-(--text-strong)">
                      {is_editing ? t("settings.providers.edit_provider") : t("settings.providers.add_provider")}
                    </h2>
                  </div>
                </div>

                <div className="min-h-0 pt-4">
                  <div className="grid gap-3 md:grid-cols-2">
                    <label className="space-y-1.5 md:col-span-2">
                      <span className="text-[11px] font-semibold text-(--text-muted)">
                        {t("settings.providers.provider")}
                      </span>
                      <input
                        autoCapitalize="off"
                        autoCorrect="off"
                        className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)"
                        disabled={is_editing}
                        onChange={(event) => set_draft((current) => ({
                          ...current,
                          provider: event.target.value,
                          display_name: current.display_name ? current.display_name : event.target.value,
                        }))}
                        placeholder={t("settings.providers.provider_placeholder")}
                        spellCheck={false}
                        type="text"
                        value={draft.provider}
                      />
                    </label>

                    <label className="space-y-1.5 md:col-span-2">
                      <span className="text-[11px] font-semibold text-(--text-muted)">
                        {t("settings.providers.display_name")}
                      </span>
                      <input
                        autoCapitalize="off"
                        autoCorrect="off"
                        className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none"
                        onChange={(event) => set_draft((current) => ({ ...current, display_name: event.target.value }))}
                        placeholder={draft.provider || t("settings.providers.display_name_placeholder")}
                        spellCheck={false}
                        type="text"
                        value={draft.display_name}
                      />
                    </label>

                    <label className="space-y-1.5 md:col-span-2">
                      <span className="text-[11px] font-semibold text-(--text-muted)">
                        {t("settings.providers.base_url")}
                      </span>
                      <input
                        autoCapitalize="off"
                        autoCorrect="off"
                        className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none"
                        onChange={(event) => set_draft((current) => ({ ...current, base_url: event.target.value }))}
                        placeholder="https://example.com/v1"
                        spellCheck={false}
                        type="text"
                        value={draft.base_url}
                      />
                    </label>

                    <label className="space-y-1.5 md:col-span-2">
                      <span className="text-[11px] font-semibold text-(--text-muted)">
                        {t("settings.providers.model")}
                      </span>
                      <input
                        autoCapitalize="off"
                        autoCorrect="off"
                        className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none"
                        onChange={(event) => set_draft((current) => ({ ...current, model: event.target.value }))}
                        placeholder={t("settings.providers.model_placeholder")}
                        spellCheck={false}
                        type="text"
                        value={draft.model}
                      />
                    </label>

                    <label className="space-y-1.5 md:col-span-2">
                      <span className="text-[11px] font-semibold text-(--text-muted)">
                        {t("settings.providers.auth_token")}
                      </span>
                      <input
                        autoCapitalize="off"
                        autoComplete="off"
                        autoCorrect="off"
                        className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none"
                        data-form-type="other"
                        data-lpignore="true"
                        name="provider-auth-token"
                        onChange={(event) => set_draft((current) => ({ ...current, auth_token: event.target.value }))}
                        placeholder={token_placeholder}
                        spellCheck={false}
                        type="password"
                        value={draft.auth_token}
                      />
                    </label>
                  </div>
                </div>

                <div className="flex items-center justify-between gap-2 pt-4">
                  <button
                    className={cn(
                      can_save ? PROVIDER_SAVE_BUTTON_CLASS_NAME : PROVIDER_SECONDARY_BUTTON_CLASS_NAME,
                      "min-w-24 justify-center",
                    )}
                    disabled={!can_save || submitting}
                    onClick={() => void handle_save()}
                    type="button"
                  >
                    {submitting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : t("common.save")}
                  </button>
                  <div className="flex min-w-24 justify-end">
                    {is_editing ? (
                      <button
                        className={cn(PROVIDER_DANGER_BUTTON_CLASS_NAME, "min-w-24 justify-center")}
                        disabled={!can_delete || submitting}
                        onClick={() => set_delete_confirm_open(true)}
                        type="button"
                      >
                        {t("common.delete")}
                      </button>
                    ) : (
                      <button
                        className={cn(PROVIDER_SECONDARY_BUTTON_CLASS_NAME, "min-w-24 justify-center")}
                        disabled={submitting}
                        onClick={handle_cancel}
                        type="button"
                      >
                        {t("common.cancel")}
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )}
          </section>
        </div>
    </div>
  );

  return (
    <>
      {embedded ? panel_content : (
        <WorkspaceSurfaceScaffold
          body_scrollable
          stable_gutter
          header={(
            <WorkspaceSurfaceHeader
              active_tab="providers"
              density="compact"
              leading={<Cable className="h-4 w-4" />}
              tabs={SETTINGS_TABS.map((item) => ({ key: item.key, label: t(item.label_key) }))}
              title={t("settings.title")}
            />
          )}
        >
          {panel_content}
        </WorkspaceSurfaceScaffold>
      )}

      {feedback ? (
        <div className="pointer-events-none fixed right-6 top-24 z-40 flex flex-col gap-2">
          <FeedbackBanner
            message={feedback.message}
            on_dismiss={() => set_feedback(null)}
            title={feedback.title}
            tone={feedback.tone}
          />
        </div>
      ) : null}

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={delete_confirm_open}
        message={t("settings.providers.delete_confirm_message", {
          name: selected_record ? get_provider_title(selected_record) : "",
        })}
        on_cancel={() => set_delete_confirm_open(false)}
        on_confirm={() => {
          void handle_delete();
        }}
        title={t("settings.providers.delete_confirm_title")}
        variant="danger"
      />
    </>
  );
}
