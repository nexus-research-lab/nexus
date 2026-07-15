"use client";

import { useEffect, useState, type ReactNode } from "react";
import { KeyRound, Search, Terminal, Trash2, Wrench } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import type {
  AnySearchSettings,
  WebSearchProvider,
  WebSearchSettings,
} from "@/types/settings/preferences";
import type { TranslationKey } from "@/shared/i18n/messages";

import { AGENT_RUNTIME_KIND_OPTIONS } from "./model/settings-runtime-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "../shared/settings-panel-ui";
import { useRuntimeSettingsController } from "./use-runtime-settings-controller";

export function SettingsRuntimeSection() {
  const { t } = useI18n();
  const settings = useRuntimeSettingsController();

  return (
    <div
      className={cn(
        "mx-auto flex w-full flex-col gap-5 px-1 py-3",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}
    >
      <section className="space-y-2.5">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.runtime.section_title")}
        </h2>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <Terminal className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.runtime.kernel_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.runtime.kernel_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.runtime.kernel_label")}
              </span>
              <SettingsSegmentedControl
                ariaLabel={t("settings.runtime.kernel_label")}
                disabled={
                  settings.loading ||
                  settings.preferencesBusy ||
                  settings.nxsRuntimeChecking
                }
                onChange={settings.onRuntimeKindChange}
                options={AGENT_RUNTIME_KIND_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
                }))}
                value={settings.runtimeKind}
              />
            </div>
          </div>

          {settings.runtimeKind === "nxs" ? (
            <>
              <div className="border-t border-(--divider-subtle-color)" />
              <ToolSearchRow
                checked={settings.toolSearchEnabled}
                disabled={settings.loading || settings.preferencesBusy}
                onChange={settings.onToolSearchChange}
              />
              <div className="border-t border-(--divider-subtle-color)" />
              <WebSearchRow
                apiKey={settings.webSearchAPIKey}
                disabled={settings.loading || settings.preferencesBusy}
                onAPIKeyChange={settings.onWebSearchAPIKeyChange}
                onChange={settings.onWebSearchChange}
                onPatch={settings.onWebSearchPatch}
                settings={settings.webSearch ?? { enabled: false, provider: "brave" }}
              />
            </>
          ) : (
            <>
              <div className="border-t border-(--divider-subtle-color)" />
              <RuntimeWithoutSettings />
            </>
          )}
        </div>
        {settings.feedbackMessage ? (
          <p className="px-1 text-xs text-(--danger-text-color)">
            {settings.feedbackMessage}
          </p>
        ) : null}
      </section>
    </div>
  );
}

function WebSearchRow({
  apiKey,
  disabled,
  onAPIKeyChange,
  onChange,
  onPatch,
  settings,
}: {
  apiKey: string;
  disabled: boolean;
  onAPIKeyChange: (value: string) => void;
  onChange: (checked: boolean) => void;
  onPatch: (patch: Partial<WebSearchSettings>) => void;
  settings: WebSearchSettings;
}) {
  const { t } = useI18n();
  const providerLabels: Record<WebSearchProvider, TranslationKey> = {
    brave: "settings.runtime.web_search_provider_brave",
    tavily: "settings.runtime.web_search_provider_tavily",
    exa: "settings.runtime.web_search_provider_exa",
    firecrawl: "settings.runtime.web_search_provider_firecrawl",
    searxng: "settings.runtime.web_search_provider_searxng",
    anysearch: "settings.runtime.web_search_provider_anysearch",
  };
  const [draft, setDraft] = useState(settings);
  const [draftAPIKey, setDraftAPIKey] = useState(apiKey);
  const [anySearchContentTypesText, setAnySearchContentTypesText] = useState("");
  const [anySearchParamsText, setAnySearchParamsText] = useState("{}");
  const [anySearchParamsError, setAnySearchParamsError] = useState(false);

  useEffect(() => {
    setDraft(settings);
    setAnySearchContentTypesText((settings.anysearch?.content_types ?? []).join(", "));
    setAnySearchParamsText(formatAnySearchParams(settings.anysearch?.params));
    setAnySearchParamsError(false);
  }, [settings]);
  useEffect(() => setDraftAPIKey(apiKey), [apiKey]);

  const providerSupportsExtract = supportsProviderExtract(draft.provider);

  const patchDraft = (patch: Partial<WebSearchSettings>, commit = false) => {
    const next = { ...draft, ...patch };
    setDraft(next);
    if (commit) {
      onPatch(patch);
    }
  };

  const patchAnySearch = (patch: Partial<AnySearchSettings>) => {
    const anysearch = { ...draft.anysearch, ...patch };
    setDraft({ ...draft, anysearch });
    onPatch({ anysearch });
  };

  return (
    <>
      <div className={SETTINGS_ROW_CLASS_NAME}>
        <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
          <div className={SETTINGS_ICON_CLASS_NAME}>
            <Search className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
              {t("settings.runtime.web_search_title")}
            </h3>
            <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
              {t("settings.runtime.web_search_description")}
            </p>
          </div>
        </div>
        <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.runtime.web_search_label")}
          </span>
          <GlassSwitch
            checked={settings.enabled}
            disabled={disabled}
            onChange={onChange}
            size="sm"
          />
        </div>
      </div>
      {settings.enabled ? (
        <div className="border-t border-(--divider-subtle-color) px-4 pb-4 pt-3 md:pl-14">
          <div className="grid gap-3 md:grid-cols-2">
            <SettingsField label={t("settings.runtime.web_search_provider")} icon={<Wrench className="h-3.5 w-3.5" />}>
              <select
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none"
                disabled={disabled}
                onChange={(event) => {
                  const provider = event.target.value as WebSearchProvider;
                  patchDraft({
                    provider,
                    ...(supportsProviderExtract(provider) ? {} : { use_provider_extract: false }),
                  }, true);
                }}
                value={draft.provider ?? "brave"}
              >
                {(["brave", "tavily", "exa", "firecrawl", "searxng", "anysearch"] as WebSearchProvider[]).map((provider) => (
                  <option key={provider} value={provider}>
                    {t(providerLabels[provider])}
                  </option>
                ))}
              </select>
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_api_key")} icon={<KeyRound className="h-3.5 w-3.5" />}>
              <div className="flex gap-2">
                <input
                  className="input-shell h-9 min-w-0 flex-1 rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => {
                    if (draftAPIKey.trim() !== "") {
                      onAPIKeyChange(draftAPIKey.trim());
                    }
                  }}
                  onChange={(event) => setDraftAPIKey(event.target.value)}
                  placeholder={settings.api_key_configured
                    ? t("settings.runtime.web_search_api_key_configured")
                    : draft.provider === "anysearch"
                      ? t("settings.runtime.web_search_api_key_placeholder_anysearch")
                      : t("settings.runtime.web_search_api_key_placeholder")}
                  type="password"
                  value={draftAPIKey}
                />
                {settings.api_key_configured ? (
                  <button
                    aria-label={t("settings.runtime.web_search_api_key_clear")}
                    className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] text-(--text-soft) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--danger-text-color)"
                    disabled={disabled}
                    onClick={() => {
                      setDraftAPIKey("");
                      onAPIKeyChange("");
                    }}
                    title={t("settings.runtime.web_search_api_key_clear")}
                    type="button"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                ) : null}
              </div>
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_base_url")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                disabled={disabled}
                onBlur={() => onPatch({ base_url: draft.base_url?.trim() })}
                onChange={(event) => patchDraft({ base_url: event.target.value })}
                placeholder={t("settings.runtime.web_search_base_url_placeholder")}
                value={draft.base_url ?? ""}
              />
            </SettingsField>
            {draft.provider === "anysearch" ? (
              <>
                <SettingsField label={t("settings.runtime.web_search_anysearch_domain")}>
                  <input
                    className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ domain: draft.anysearch?.domain?.trim() })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, domain: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_domain_placeholder")}
                    value={draft.anysearch?.domain ?? ""}
                  />
                </SettingsField>
                <SettingsField label={t("settings.runtime.web_search_anysearch_tag")}>
                  <input
                    className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ tag: draft.anysearch?.tag?.trim() })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, tag: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_tag_placeholder")}
                    value={draft.anysearch?.tag ?? ""}
                  />
                </SettingsField>
                <SettingsField label={t("settings.runtime.web_search_anysearch_content_types")}>
                  <input
                    className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ content_types: splitSearchValues(anySearchContentTypesText) })}
                    onChange={(event) => setAnySearchContentTypesText(event.target.value)}
                    placeholder={t("settings.runtime.web_search_anysearch_content_types_placeholder")}
                    value={anySearchContentTypesText}
                  />
                </SettingsField>
                <SettingsField label={t("settings.runtime.web_search_anysearch_params")}>
                  <textarea
                    aria-invalid={anySearchParamsError}
                    className="input-shell min-h-24 w-full resize-y rounded-[10px] bg-transparent px-3 py-2 font-mono text-[11px] leading-5 text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => {
                      const value = anySearchParamsText.trim();
                      if (value === "") {
                        setAnySearchParamsError(false);
                        patchAnySearch({ params: undefined });
                        return;
                      }
                      try {
                        const parsed: unknown = JSON.parse(value);
                        if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
                          throw new Error("params must be an object");
                        }
                        setAnySearchParamsError(false);
                        patchAnySearch({ params: parsed as Record<string, unknown> });
                      } catch {
                        setAnySearchParamsError(true);
                      }
                    }}
                    onChange={(event) => setAnySearchParamsText(event.target.value)}
                    placeholder={t("settings.runtime.web_search_anysearch_params_placeholder")}
                    value={anySearchParamsText}
                  />
                  {anySearchParamsError ? (
                    <span className="text-[10px] text-(--danger-text-color)">
                      {t("settings.runtime.web_search_anysearch_params_invalid")}
                    </span>
                  ) : null}
                </SettingsField>
              </>
            ) : null}
            <SettingsField label={t("settings.runtime.web_search_country")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                disabled={disabled}
                onBlur={() => onPatch({ country: draft.country?.trim() })}
                onChange={(event) => patchDraft({ country: event.target.value })}
                placeholder={t("settings.runtime.web_search_country_placeholder")}
                value={draft.country ?? ""}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_language")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                disabled={disabled}
                onBlur={() => onPatch({ language: draft.language?.trim() })}
                onChange={(event) => patchDraft({ language: event.target.value })}
                placeholder={t("settings.runtime.web_search_language_placeholder")}
                value={draft.language ?? ""}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_search_language")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                disabled={disabled}
                onBlur={() => onPatch({ search_language: draft.search_language?.trim() })}
                onChange={(event) => patchDraft({ search_language: event.target.value })}
                placeholder={t("settings.runtime.web_search_search_language_placeholder")}
                value={draft.search_language ?? ""}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_freshness")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                disabled={disabled}
                onBlur={() => onPatch({ freshness: draft.freshness?.trim() })}
                onChange={(event) => patchDraft({ freshness: event.target.value })}
                placeholder={t("settings.runtime.web_search_freshness_placeholder")}
                value={draft.freshness ?? ""}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_result_count")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none"
                disabled={disabled}
                max={20}
                min={1}
                onBlur={() => onPatch({ default_count: normalizeNumber(draft.default_count, 5, 1, 20) })}
                onChange={(event) => patchDraft({ default_count: Number(event.target.value) })}
                type="number"
                value={draft.default_count ?? 5}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_timeout")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none"
                disabled={disabled}
                min={1}
                onBlur={() => onPatch({ timeout_seconds: normalizeNumber(draft.timeout_seconds, 20, 1, 120) })}
                onChange={(event) => patchDraft({ timeout_seconds: Number(event.target.value) })}
                type="number"
                value={draft.timeout_seconds ?? 20}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_cache")}>
              <input
                className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none"
                disabled={disabled}
                min={0}
                onBlur={() => onPatch({ cache_ttl_seconds: normalizeNumber(draft.cache_ttl_seconds, 900, 0, 86400) })}
                onChange={(event) => patchDraft({ cache_ttl_seconds: Number(event.target.value) })}
                type="number"
                value={draft.cache_ttl_seconds ?? 900}
              />
            </SettingsField>
            <SettingsField label={t("settings.runtime.web_search_depth")} icon={<Wrench className="h-3.5 w-3.5" />}>
              <SettingsSegmentedControl
                ariaLabel={t("settings.runtime.web_search_depth")}
                disabled={disabled}
                onChange={(value) => patchDraft({ search_depth: value as "basic" | "advanced" }, true)}
                options={[
                  { label: t("settings.runtime.web_search_basic"), value: "basic" },
                  { label: t("settings.runtime.web_search_advanced"), value: "advanced" },
                ]}
                value={draft.search_depth ?? "basic"}
              />
            </SettingsField>
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-2">
            <CheckSetting
              checked={draft.use_provider_extract === true}
              disabled={disabled || !providerSupportsExtract}
              label={t("settings.runtime.web_search_provider_extract")}
              onChange={(checked) => patchDraft({ use_provider_extract: checked }, true)}
            />
            <CheckSetting
              checked={settings.allow_private_network === true}
              disabled={disabled}
              label={t("settings.runtime.web_search_private_network")}
              onChange={(checked) => onPatch({ allow_private_network: checked })}
            />
          </div>
        </div>
      ) : null}
    </>
  );
}

function SettingsField({
  children,
  icon,
  label,
}: {
  children: ReactNode;
  icon?: ReactNode;
  label: string;
}) {
  return (
    <label className="min-w-0 space-y-1.5">
      <span className="flex items-center gap-1.5 text-[11px] font-medium text-(--text-soft)">
        {icon}
        {label}
      </span>
      {children}
    </label>
  );
}

function CheckSetting({
  checked,
  disabled,
  label,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex min-h-9 items-center gap-2 rounded-[10px] border border-(--divider-subtle-color) px-3 text-[12px] text-(--text-default)">
      <input
        checked={checked}
        className="h-3.5 w-3.5 accent-(--primary)"
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        type="checkbox"
      />
      <span>{label}</span>
    </label>
  );
}

function normalizeNumber(value: number | undefined, fallback: number, min: number, max: number): number {
  if (value === undefined || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, Math.round(value)));
}

function supportsProviderExtract(provider: WebSearchProvider | undefined): boolean {
  return provider === "tavily" || provider === "exa" || provider === "firecrawl";
}

function splitSearchValues(value: string): string[] {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function formatAnySearchParams(params: Record<string, unknown> | undefined): string {
  return JSON.stringify(params ?? {}, null, 2);
}

function ToolSearchRow({
  checked,
  disabled,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  onChange: (checked: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <Search className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.runtime.tool_search_title")}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t("settings.runtime.tool_search_description")}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
        <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
          {t("settings.runtime.tool_search_label")}
        </span>
        <GlassSwitch
          checked={checked}
          disabled={disabled}
          onChange={onChange}
          size="sm"
        />
      </div>
    </div>
  );
}

function RuntimeWithoutSettings() {
  const { t } = useI18n();
  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <Terminal className="h-3.5 w-3.5" />
        </div>
        <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
          {t("settings.runtime.no_settings")}
        </p>
      </div>
    </div>
  );
}
