"use client";

import { useEffect, useState, type ReactNode } from "react";
import {
  ChevronDown,
  ChevronUp,
  Database,
  ExternalLink,
  Globe2,
  KeyRound,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  Terminal,
  Timer,
  Trash2,
  Wrench,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import {
  DEFAULT_WEB_SEARCH_PROVIDER,
  type AnySearchSettings,
  type WebSearchProvider,
  type WebSearchSettings,
} from "@/types/settings/preferences";
import type { TranslationKey } from "@/shared/i18n/messages";

import { AGENT_RUNTIME_KIND_OPTIONS } from "./model/settings-runtime-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "../shared/settings-panel-ui";
import { useRuntimeSettingsController } from "./use-runtime-settings-controller";

type WebSearchProviderOption = {
  apiKeyURL?: string;
  labelKey: TranslationKey;
  requiredField?: "api_key" | "base_url";
  value: WebSearchProvider;
};

const WEB_SEARCH_PROVIDERS: ReadonlyArray<WebSearchProviderOption> = [
  {
    apiKeyURL: "https://brave.com/search/api/",
    labelKey: "settings.runtime.web_search_provider_brave",
    requiredField: "api_key",
    value: "brave",
  },
  {
    apiKeyURL: "https://app.tavily.com/",
    labelKey: "settings.runtime.web_search_provider_tavily",
    requiredField: "api_key",
    value: "tavily",
  },
  {
    apiKeyURL: "https://dashboard.exa.ai/api-keys",
    labelKey: "settings.runtime.web_search_provider_exa",
    requiredField: "api_key",
    value: "exa",
  },
  {
    apiKeyURL: "https://www.firecrawl.dev/app",
    labelKey: "settings.runtime.web_search_provider_firecrawl",
    requiredField: "api_key",
    value: "firecrawl",
  },
  {
    labelKey: "settings.runtime.web_search_provider_searxng",
    requiredField: "base_url",
    value: "searxng",
  },
  {
    apiKeyURL: "https://www.anysearch.com/docs#quick-start",
    labelKey: "settings.runtime.web_search_provider_anysearch",
    value: "anysearch",
  },
];

interface WebSearchProviderCapabilities {
  country: boolean;
  customBaseURL: boolean;
  extractDepth: boolean;
  freshness: boolean;
  language: boolean;
  privateNetwork: boolean;
  searchDepth: boolean;
  searchLanguage: boolean;
}

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
                onPatch={settings.onWebSearchPatch}
                onProviderChange={settings.onWebSearchProviderChange}
                settings={settings.webSearch ?? {
                  enabled: true,
                  provider: DEFAULT_WEB_SEARCH_PROVIDER,
                }}
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
  onPatch,
  onProviderChange,
  settings,
}: {
  apiKey: string;
  disabled: boolean;
  onAPIKeyChange: (value: string) => void;
  onPatch: (patch: Partial<WebSearchSettings>) => void;
  onProviderChange: (provider: WebSearchProvider) => void;
  settings: WebSearchSettings;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(settings);
  const [anySearchContentTypesText, setAnySearchContentTypesText] = useState("");
  const [anySearchParamsText, setAnySearchParamsText] = useState("{}");
  const [anySearchParamsError, setAnySearchParamsError] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);

  useEffect(() => {
    setDraft(settings);
    setAnySearchContentTypesText((settings.anysearch?.content_types ?? []).join(", "));
    setAnySearchParamsText(formatAnySearchParams(settings.anysearch?.params));
    setAnySearchParamsError(false);
  }, [settings]);
  const provider = WEB_SEARCH_PROVIDERS.find((item) => item.value === draft.provider)
    ?? WEB_SEARCH_PROVIDERS.find((item) => item.value === DEFAULT_WEB_SEARCH_PROVIDER)
    ?? WEB_SEARCH_PROVIDERS[0];
  const apiKeyRequired = provider.requiredField === "api_key";
  const apiKeySupported = apiKeyRequired || provider.value === "anysearch";
  const baseURLRequired = provider.requiredField === "base_url";
  const capabilities = getWebSearchProviderCapabilities(provider.value);
  const showCustomBaseURL = capabilities.customBaseURL
    || (provider.value === "anysearch" && draft.base_url !== "");
  const patchDraft = (patch: Partial<WebSearchSettings>) => {
    setDraft((current) => ({ ...current, ...patch }));
  };
  const commitPatch = (patch: Partial<WebSearchSettings>) => {
    patchDraft(patch);
    onPatch(patch);
  };
  const commitText = (
    key: "base_url" | "country" | "freshness" | "language" | "search_language",
  ) => {
    commitPatch({ [key]: draft[key]?.trim() ?? "" });
  };
  const commitNumber = (
    key: "cache_ttl_seconds" | "default_count" | "timeout_seconds",
    fallback: number,
    min: number,
    max: number,
  ) => {
    const value = normalizeNumber(draft[key], fallback, min, max);
    commitPatch({ [key]: value });
  };
  const patchAnySearch = (patch: Partial<AnySearchSettings>) => {
    commitPatch({
      anysearch: {
        ...draft.anysearch,
        ...patch,
      },
    });
  };

  return (
    <>
      <div className={cn(SETTINGS_ROW_CLASS_NAME, "md:items-start")}>
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
        <div className="flex min-w-0 flex-col gap-1.5">
          <span className="text-[11px] font-medium text-(--text-soft)">
            {t("settings.runtime.web_search_provider")}
          </span>
          <UiSelectMenu
            ariaLabel={t("settings.runtime.web_search_provider")}
            buttonClassName={SETTINGS_SELECT_BUTTON_CLASS_NAME}
            className={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
            disabled={disabled}
            menuClassName="rounded-[12px]"
            onChange={(value) => {
              const nextProvider = value as WebSearchProvider;
              setDraft((current) => ({
                ...current,
                enabled: nextProvider === DEFAULT_WEB_SEARCH_PROVIDER,
                provider: nextProvider,
              }));
              onProviderChange(nextProvider);
            }}
            options={WEB_SEARCH_PROVIDERS.map((providerOption) => ({
              value: providerOption.value,
              label: t(providerOption.labelKey),
            }))}
            placement="bottom"
            size="xs"
            value={draft.provider ?? DEFAULT_WEB_SEARCH_PROVIDER}
          />
        </div>
      </div>
      <div className="border-t border-(--divider-subtle-color) px-4 pb-2 pt-2 md:pl-14">
        <div className="grid gap-2 md:grid-cols-2">
          <div className="md:col-span-2">
            {apiKeySupported ? (
              <WebSearchAPIKeyField
                apiKey={apiKey}
                apiKeyConfigured={settings.api_key_configured === true}
                apiKeyMasked={settings.api_key_masked ?? ""}
                disabled={disabled}
                onChange={onAPIKeyChange}
                provider={provider}
                required={apiKeyRequired}
              />
            ) : baseURLRequired ? (
              <SettingsField label={t("settings.runtime.web_search_base_url")}>
                <input
                  className="input-shell h-9 w-full rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("base_url")}
                  onChange={(event) => patchDraft({ base_url: event.target.value })}
                  placeholder={t("settings.runtime.web_search_base_url_placeholder")}
                  required
                  value={draft.base_url ?? ""}
                />
              </SettingsField>
            ) : (
              <div className="flex min-h-9 items-center text-[11px] text-(--text-soft)">
                {t("settings.runtime.web_search_no_extra_config")}
              </div>
            )}
          </div>
        </div>
        <div className="mt-2 flex justify-end border-t border-(--divider-subtle-color) pt-1.5">
          <button
            aria-controls="web-search-more-settings"
            aria-expanded={moreOpen}
            className="inline-flex h-6 items-center gap-1 rounded-[8px] px-1.5 text-[10px] font-medium text-(--text-soft) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            disabled={disabled}
            onClick={() => setMoreOpen((current) => !current)}
            type="button"
          >
            <SlidersHorizontal className="h-3 w-3" />
            {t("settings.runtime.web_search_more")}
            {moreOpen ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
          </button>
        </div>
        {moreOpen ? (
          <div
            className="grid gap-2 border-t border-(--divider-subtle-color) pt-2 md:grid-cols-2"
            id="web-search-more-settings"
          >
            {showCustomBaseURL ? (
              <SettingsField
                className="md:col-span-2"
                label={t("settings.runtime.web_search_custom_base_url")}
              >
                <input
                  className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("base_url")}
                  onChange={(event) => patchDraft({ base_url: event.target.value })}
                  placeholder={t("settings.runtime.web_search_custom_base_url_placeholder")}
                  value={draft.base_url ?? ""}
                />
              </SettingsField>
            ) : null}
            <SettingsSubsectionTitle>
              {t("settings.runtime.web_search_common_settings")}
            </SettingsSubsectionTitle>
            <SettingsField
              icon={<Database className="h-3.5 w-3.5" />}
              label={t("settings.runtime.web_search_result_count")}
            >
              <input
                className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none"
                disabled={disabled}
                max={20}
                min={1}
                onBlur={() => commitNumber("default_count", 5, 1, 20)}
                onChange={(event) => patchDraft({ default_count: Number(event.target.value) })}
                type="number"
                value={draft.default_count ?? 5}
              />
            </SettingsField>
            <SettingsField
              icon={<Timer className="h-3.5 w-3.5" />}
              label={t("settings.runtime.web_search_timeout")}
            >
              <input
                className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none"
                disabled={disabled}
                max={120}
                min={1}
                onBlur={() => commitNumber("timeout_seconds", 20, 1, 120)}
                onChange={(event) => patchDraft({ timeout_seconds: Number(event.target.value) })}
                type="number"
                value={draft.timeout_seconds ?? 20}
              />
            </SettingsField>
            <SettingsField
              icon={<Database className="h-3.5 w-3.5" />}
              label={t("settings.runtime.web_search_cache")}
            >
              <input
                className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none"
                disabled={disabled}
                max={86400}
                min={0}
                onBlur={() => commitNumber("cache_ttl_seconds", 900, 0, 86400)}
                onChange={(event) => patchDraft({ cache_ttl_seconds: Number(event.target.value) })}
                type="number"
                value={draft.cache_ttl_seconds ?? 900}
              />
            </SettingsField>
            <SettingsSubsectionTitle>
              {t("settings.runtime.web_search_provider_settings")} · {t(provider.labelKey)}
            </SettingsSubsectionTitle>
            {capabilities.country ? (
              <SettingsField
                icon={<Globe2 className="h-3.5 w-3.5" />}
                label={t("settings.runtime.web_search_country")}
              >
                <input
                  className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("country")}
                  onChange={(event) => patchDraft({ country: event.target.value })}
                  placeholder={t("settings.runtime.web_search_country_placeholder")}
                  value={draft.country ?? ""}
                />
              </SettingsField>
            ) : null}
            {capabilities.language ? (
              <SettingsField label={t("settings.runtime.web_search_language")}>
                <input
                  className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("language")}
                  onChange={(event) => patchDraft({ language: event.target.value })}
                  placeholder={t("settings.runtime.web_search_language_placeholder")}
                  value={draft.language ?? ""}
                />
              </SettingsField>
            ) : null}
            {capabilities.searchLanguage ? (
              <SettingsField label={t("settings.runtime.web_search_search_language")}>
                <input
                  className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("search_language")}
                  onChange={(event) => patchDraft({ search_language: event.target.value })}
                  placeholder={t("settings.runtime.web_search_search_language_placeholder")}
                  value={draft.search_language ?? ""}
                />
              </SettingsField>
            ) : null}
            {capabilities.freshness ? (
              <SettingsField label={t("settings.runtime.web_search_freshness")}>
                <input
                  className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                  disabled={disabled}
                  onBlur={() => commitText("freshness")}
                  onChange={(event) => patchDraft({ freshness: event.target.value })}
                  placeholder={t("settings.runtime.web_search_freshness_placeholder")}
                  value={draft.freshness ?? ""}
                />
              </SettingsField>
            ) : null}
            {capabilities.searchDepth || capabilities.extractDepth ? (
              <>
                {capabilities.searchDepth ? (
                  <SettingsField
                    icon={<Wrench className="h-3.5 w-3.5" />}
                    label={t("settings.runtime.web_search_depth")}
                  >
                    <SettingsSegmentedControl
                      ariaLabel={t("settings.runtime.web_search_depth")}
                      disabled={disabled}
                      onChange={(value) => commitPatch({ search_depth: value as "basic" | "advanced" })}
                      options={[
                        { label: t("settings.runtime.web_search_basic"), value: "basic" },
                        { label: t("settings.runtime.web_search_advanced"), value: "advanced" },
                      ]}
                      value={draft.search_depth ?? "basic"}
                    />
                  </SettingsField>
                ) : null}
                {capabilities.extractDepth ? (
                  <SettingsField label={t("settings.runtime.web_search_extract_depth")}>
                    <SettingsSegmentedControl
                      ariaLabel={t("settings.runtime.web_search_extract_depth")}
                      disabled={disabled}
                      onChange={(value) => commitPatch({ extract_depth: value as "basic" | "advanced" })}
                      options={[
                        { label: t("settings.runtime.web_search_basic"), value: "basic" },
                        { label: t("settings.runtime.web_search_advanced"), value: "advanced" },
                      ]}
                      value={draft.extract_depth ?? "basic"}
                    />
                  </SettingsField>
                ) : null}
              </>
            ) : null}
            {capabilities.privateNetwork ? (
              <SettingsCheckSetting
                checked={draft.allow_private_network === true}
                disabled={disabled}
                icon={<ShieldCheck className="h-3.5 w-3.5" />}
                label={t("settings.runtime.web_search_private_network")}
                onChange={(checked) => commitPatch({ allow_private_network: checked })}
              />
            ) : null}
            {supportsProviderExtract(provider.value) ? (
              <SettingsCheckSetting
                checked={draft.use_provider_extract === true}
                disabled={disabled}
                label={t("settings.runtime.web_search_provider_extract")}
                onChange={(checked) => commitPatch({ use_provider_extract: checked })}
              />
            ) : null}
            {provider.value === "anysearch" ? (
              <>
                <SettingsField label={t("settings.runtime.web_search_anysearch_domain")}>
                  <input
                    className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ domain: draft.anysearch?.domain?.trim() ?? "" })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, domain: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_domain_placeholder")}
                    value={draft.anysearch?.domain ?? ""}
                  />
                </SettingsField>
                <SettingsField label={t("settings.runtime.web_search_anysearch_tag")}>
                  <input
                    className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ tag: draft.anysearch?.tag?.trim() ?? "" })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, tag: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_tag_placeholder")}
                    value={draft.anysearch?.tag ?? ""}
                  />
                </SettingsField>
                <SettingsField label={t("settings.runtime.web_search_anysearch_content_types")}>
                  <input
                    className="input-shell h-8 w-full rounded-[8px] bg-transparent px-2.5 text-[11px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                    disabled={disabled}
                    onBlur={() => patchAnySearch({ content_types: splitSearchValues(anySearchContentTypesText) })}
                    onChange={(event) => setAnySearchContentTypesText(event.target.value)}
                    placeholder={t("settings.runtime.web_search_anysearch_content_types_placeholder")}
                    value={anySearchContentTypesText}
                  />
                </SettingsField>
                <SettingsField
                  className="md:col-span-2"
                  label={t("settings.runtime.web_search_anysearch_params")}
                >
                  <textarea
                    aria-invalid={anySearchParamsError}
                    className="input-shell min-h-16 w-full resize-y rounded-[8px] bg-transparent px-2.5 py-1.5 font-mono text-[10px] leading-4 text-(--text-strong) outline-none placeholder:text-(--text-soft)"
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
          </div>
        ) : null}
      </div>
    </>
  );
}

function WebSearchAPIKeyField({
  apiKey,
  apiKeyConfigured,
  apiKeyMasked,
  disabled,
  onChange,
  provider,
  required = false,
}: {
  apiKey: string;
  apiKeyConfigured: boolean;
  apiKeyMasked: string;
  disabled: boolean;
  onChange: (value: string) => void;
  provider: WebSearchProviderOption;
  required?: boolean;
}) {
  const { t } = useI18n();
  const [draftAPIKey, setDraftAPIKey] = useState(apiKey);

  useEffect(() => setDraftAPIKey(apiKey), [apiKey, apiKeyConfigured]);

  return (
    <SettingsField
      icon={<KeyRound className="h-3.5 w-3.5" />}
      label={t("settings.runtime.web_search_api_key")}
    >
      <div className="flex gap-2">
        <input
          className="input-shell h-9 min-w-0 flex-1 rounded-[10px] bg-transparent px-3 text-[12px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
          disabled={disabled}
          onBlur={() => {
            const value = draftAPIKey.trim();
            if (value !== "") {
              setDraftAPIKey("");
              onChange(value);
            }
          }}
          onChange={(event) => setDraftAPIKey(event.target.value)}
          placeholder={apiKeyConfigured
            ? apiKeyMasked || t("settings.runtime.web_search_api_key_configured")
            : required
              ? t("settings.runtime.web_search_api_key_placeholder")
              : t("settings.runtime.web_search_api_key_optional_placeholder")}
          required={required}
          type="password"
          value={draftAPIKey}
        />
        {apiKeyConfigured ? (
          <button
            aria-label={t("settings.runtime.web_search_api_key_clear")}
            className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] text-(--text-soft) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--danger-text-color)"
            disabled={disabled}
            onClick={() => {
              setDraftAPIKey("");
              onChange("");
            }}
            title={t("settings.runtime.web_search_api_key_clear")}
            type="button"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        ) : null}
      </div>
      {provider.apiKeyURL ? (
        <a
          className="mt-1 inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
          href={provider.apiKeyURL}
          rel="noreferrer"
          target="_blank"
        >
          {t("settings.runtime.web_search_api_key_get", { provider: t(provider.labelKey) })}
          <ExternalLink className="h-3 w-3" />
        </a>
      ) : null}
    </SettingsField>
  );
}

function SettingsField({
  children,
  className,
  icon,
  label,
}: {
  children: ReactNode;
  className?: string;
  icon?: ReactNode;
  label: string;
}) {
  return (
    <label className={cn("min-w-0 space-y-1.5", className)}>
      <span className="flex items-center gap-1.5 text-[11px] font-medium text-(--text-soft)">
        {icon}
        {label}
      </span>
      {children}
    </label>
  );
}

function SettingsSubsectionTitle({ children }: { children: ReactNode }) {
  return (
    <div className="md:col-span-2 flex items-center gap-1.5 border-t border-(--divider-subtle-color) pt-1.5 text-[10px] font-semibold text-(--text-default)">
      {children}
    </div>
  );
}

function SettingsCheckSetting({
  checked,
  disabled,
  icon,
  label,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  icon?: ReactNode;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex min-h-8 items-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) px-2.5 text-[11px] text-(--text-default)">
      <input
        checked={checked}
        className="h-3.5 w-3.5 accent-(--primary)"
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        type="checkbox"
      />
      {icon}
      <span>{label}</span>
    </label>
  );
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

function normalizeNumber(value: number | undefined, fallback: number, min: number, max: number): number {
  if (value === undefined || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, Math.round(value)));
}

function supportsProviderExtract(provider: WebSearchProvider): boolean {
  return provider === "tavily" || provider === "exa" || provider === "firecrawl";
}

function getWebSearchProviderCapabilities(provider: WebSearchProvider): WebSearchProviderCapabilities {
  switch (provider) {
    case "anysearch":
      return {
        country: true,
        customBaseURL: false,
        extractDepth: false,
        freshness: false,
        language: true,
        privateNetwork: false,
        searchDepth: false,
        searchLanguage: false,
      };
    case "brave":
      return {
        country: true,
        customBaseURL: true,
        extractDepth: false,
        freshness: true,
        language: false,
        privateNetwork: true,
        searchDepth: false,
        searchLanguage: true,
      };
    case "tavily":
      return {
        country: false,
        customBaseURL: true,
        extractDepth: true,
        freshness: true,
        language: false,
        privateNetwork: true,
        searchDepth: true,
        searchLanguage: false,
      };
    case "exa":
    case "firecrawl":
      return {
        country: false,
        customBaseURL: true,
        extractDepth: false,
        freshness: false,
        language: false,
        privateNetwork: true,
        searchDepth: false,
        searchLanguage: false,
      };
    case "searxng":
      return {
        country: false,
        customBaseURL: false,
        extractDepth: false,
        freshness: false,
        language: true,
        privateNetwork: true,
        searchDepth: false,
        searchLanguage: false,
      };
  }
}

function splitSearchValues(value: string): string[] {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function formatAnySearchParams(params: Record<string, unknown> | undefined): string {
  return JSON.stringify(params ?? {}, null, 2);
}
