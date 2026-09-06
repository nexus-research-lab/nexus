/**
 * INPUT: 运行引擎、工具发现与网页搜索偏好。
 * OUTPUT: 以公共 Field 精确关联标签、错误和输入，保留失焦保存的运行与搜索设置。
 * POS: 设置目录的运行分区，不暴露底层 schema 或 bridge 教学。
 */
"use client";

import { useEffect, useId, useState, type ReactNode } from "react";
import {
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  Terminal,
  Trash2,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { PreferencesReliabilityNotice } from "../general/components/preferences-reliability-notice";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
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
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
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
        WORKSPACE_CONTENT_PAGE_CLASS_NAME,
        "flex flex-col",
      )}
    >
      <WorkspaceContentHeader
        className="max-sm:hidden"
        description={t("settings.runtime.section_description")}
        title={t("settings.runtime.section_title")}
      />
      <section className="space-y-2.5">
        <PreferencesReliabilityNotice
          feedback={settings.preferencesFeedback ?? settings.runtimeFeedback}
          recovery={settings.preferencesFeedback
            ? settings.preferencesRecovery
            : undefined}
        />
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
            <UiSegmentedControl
              density="compact"
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
              showLabel
              stretch
              title={t("settings.runtime.kernel_label")}
              value={settings.runtimeKind}
            />
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
  const webSearchId = useId();
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
        <UiField htmlFor={`${webSearchId}-provider`} label={t("settings.runtime.web_search_provider")}>
          <UiSelectMenu
            ariaLabel={t("settings.runtime.web_search_provider")}
            disabled={disabled}
            id={`${webSearchId}-provider`}
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
            size="sm"
            value={draft.provider ?? DEFAULT_WEB_SEARCH_PROVIDER}
          />
        </UiField>
      </div>
      <div className="border-t border-(--divider-subtle-color) px-4 pb-2 pt-2 md:pl-14">
        <div className="grid gap-2 md:grid-cols-2">
          <div className="md:col-span-2">
            {apiKeySupported ? (
              <WebSearchAPIKeyField
                key={provider.value}
                apiKey={apiKey}
                apiKeyConfigured={settings.api_key_configured === true}
                apiKeyMasked={settings.api_key_masked ?? ""}
                disabled={disabled}
                onChange={onAPIKeyChange}
                provider={provider}
                required={apiKeyRequired}
              />
            ) : baseURLRequired ? (
              <UiField htmlFor={`${webSearchId}-base-url`} label={t("settings.runtime.web_search_base_url")}>
                <UiInput
                  controlSize="md"
                  disabled={disabled}
                  id={`${webSearchId}-base-url`}
                  onBlur={() => commitText("base_url")}
                  onChange={(event) => patchDraft({ base_url: event.target.value })}
                  placeholder={t("settings.runtime.web_search_base_url_placeholder")}
                  required
                  value={draft.base_url ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : (
              <div className={cn(
                "flex min-h-9 items-center",
                getUiTypographyClassName({ role: "caption", tone: "soft" }),
              )}>
                {t("settings.runtime.web_search_no_extra_config")}
              </div>
            )}
          </div>
        </div>
        <div className="mt-2 flex justify-end border-t border-(--divider-subtle-color) pt-1.5">
          <UiButton
            aria-controls={`${webSearchId}-more`}
            aria-expanded={moreOpen}
            disabled={disabled}
            onClick={() => setMoreOpen((current) => !current)}
            size="xs"
            variant="text"
          >
            <SlidersHorizontal className="h-3 w-3" />
            {t("settings.runtime.web_search_more")}
            {moreOpen ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
          </UiButton>
        </div>
        {moreOpen ? (
          <div
            className="grid gap-2 border-t border-(--divider-subtle-color) pt-2 md:grid-cols-2"
            id={`${webSearchId}-more`}
          >
            {showCustomBaseURL ? (
              <UiField
                className="md:col-span-2"
                htmlFor={`${webSearchId}-base-url`}
                label={t("settings.runtime.web_search_custom_base_url")}
              >
                <UiInput
                  controlSize="sm"
                  disabled={disabled}
                  id={`${webSearchId}-base-url`}
                  onBlur={() => commitText("base_url")}
                  onChange={(event) => patchDraft({ base_url: event.target.value })}
                  placeholder={t("settings.runtime.web_search_custom_base_url_placeholder")}
                  value={draft.base_url ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : null}
            <SettingsSubsectionTitle>
              {t("settings.runtime.web_search_common_settings")}
            </SettingsSubsectionTitle>
            <UiField
              htmlFor={`${webSearchId}-count`}
              label={t("settings.runtime.web_search_result_count")}
            >
              <UiInput
                controlSize="sm"
                disabled={disabled}
                id={`${webSearchId}-count`}
                max={20}
                min={1}
                onBlur={() => commitNumber("default_count", 5, 1, 20)}
                onChange={(event) => patchDraft({ default_count: Number(event.target.value) })}
                type="number"
                value={draft.default_count ?? 5}
                variant="surface"
              />
            </UiField>
            <UiField
              htmlFor={`${webSearchId}-timeout`}
              label={t("settings.runtime.web_search_timeout")}
            >
              <UiInput
                controlSize="sm"
                disabled={disabled}
                id={`${webSearchId}-timeout`}
                max={120}
                min={1}
                onBlur={() => commitNumber("timeout_seconds", 20, 1, 120)}
                onChange={(event) => patchDraft({ timeout_seconds: Number(event.target.value) })}
                type="number"
                value={draft.timeout_seconds ?? 20}
                variant="surface"
              />
            </UiField>
            <UiField
              htmlFor={`${webSearchId}-cache`}
              label={t("settings.runtime.web_search_cache")}
            >
              <UiInput
                controlSize="sm"
                disabled={disabled}
                id={`${webSearchId}-cache`}
                max={86400}
                min={0}
                onBlur={() => commitNumber("cache_ttl_seconds", 900, 0, 86400)}
                onChange={(event) => patchDraft({ cache_ttl_seconds: Number(event.target.value) })}
                type="number"
                value={draft.cache_ttl_seconds ?? 900}
                variant="surface"
              />
            </UiField>
            <SettingsSubsectionTitle>
              {t("settings.runtime.web_search_provider_settings")} · {t(provider.labelKey)}
            </SettingsSubsectionTitle>
            {capabilities.country ? (
              <UiField
                htmlFor={`${webSearchId}-country`}
                label={t("settings.runtime.web_search_country")}
              >
                <UiInput
                  controlSize="sm"
                  disabled={disabled}
                  id={`${webSearchId}-country`}
                  onBlur={() => commitText("country")}
                  onChange={(event) => patchDraft({ country: event.target.value })}
                  placeholder={t("settings.runtime.web_search_country_placeholder")}
                  value={draft.country ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : null}
            {capabilities.language ? (
              <UiField htmlFor={`${webSearchId}-language`} label={t("settings.runtime.web_search_language")}>
                <UiInput
                  controlSize="sm"
                  disabled={disabled}
                  id={`${webSearchId}-language`}
                  onBlur={() => commitText("language")}
                  onChange={(event) => patchDraft({ language: event.target.value })}
                  placeholder={t("settings.runtime.web_search_language_placeholder")}
                  value={draft.language ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : null}
            {capabilities.searchLanguage ? (
              <UiField htmlFor={`${webSearchId}-search-language`} label={t("settings.runtime.web_search_search_language")}>
                <UiInput
                  controlSize="sm"
                  disabled={disabled}
                  id={`${webSearchId}-search-language`}
                  onBlur={() => commitText("search_language")}
                  onChange={(event) => patchDraft({ search_language: event.target.value })}
                  placeholder={t("settings.runtime.web_search_search_language_placeholder")}
                  value={draft.search_language ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : null}
            {capabilities.freshness ? (
              <UiField htmlFor={`${webSearchId}-freshness`} label={t("settings.runtime.web_search_freshness")}>
                <UiInput
                  controlSize="sm"
                  disabled={disabled}
                  id={`${webSearchId}-freshness`}
                  onBlur={() => commitText("freshness")}
                  onChange={(event) => patchDraft({ freshness: event.target.value })}
                  placeholder={t("settings.runtime.web_search_freshness_placeholder")}
                  value={draft.freshness ?? ""}
                  variant="surface"
                />
              </UiField>
            ) : null}
            {capabilities.searchDepth || capabilities.extractDepth ? (
              <>
                {capabilities.searchDepth ? (
                  <UiSegmentedControl
                    density="compact"
                    disabled={disabled}
                    onChange={(value) => commitPatch({ search_depth: value as "basic" | "advanced" })}
                    options={[
                      { label: t("settings.runtime.web_search_basic"), value: "basic" },
                      { label: t("settings.runtime.web_search_advanced"), value: "advanced" },
                    ]}
                    showLabel
                    stretch
                    title={t("settings.runtime.web_search_depth")}
                    value={draft.search_depth ?? "basic"}
                  />
                ) : null}
                {capabilities.extractDepth ? (
                  <UiSegmentedControl
                    density="compact"
                    disabled={disabled}
                    onChange={(value) => commitPatch({ extract_depth: value as "basic" | "advanced" })}
                    options={[
                      { label: t("settings.runtime.web_search_basic"), value: "basic" },
                      { label: t("settings.runtime.web_search_advanced"), value: "advanced" },
                    ]}
                    showLabel
                    stretch
                    title={t("settings.runtime.web_search_extract_depth")}
                    value={draft.extract_depth ?? "basic"}
                  />
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
                <UiField htmlFor={`${webSearchId}-domain`} label={t("settings.runtime.web_search_anysearch_domain")}>
                  <UiInput
                    controlSize="sm"
                    disabled={disabled}
                    id={`${webSearchId}-domain`}
                    onBlur={() => patchAnySearch({ domain: draft.anysearch?.domain?.trim() ?? "" })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, domain: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_domain_placeholder")}
                    value={draft.anysearch?.domain ?? ""}
                    variant="surface"
                  />
                </UiField>
                <UiField htmlFor={`${webSearchId}-tag`} label={t("settings.runtime.web_search_anysearch_tag")}>
                  <UiInput
                    controlSize="sm"
                    disabled={disabled}
                    id={`${webSearchId}-tag`}
                    onBlur={() => patchAnySearch({ tag: draft.anysearch?.tag?.trim() ?? "" })}
                    onChange={(event) => patchDraft({ anysearch: { ...draft.anysearch, tag: event.target.value } })}
                    placeholder={t("settings.runtime.web_search_anysearch_tag_placeholder")}
                    value={draft.anysearch?.tag ?? ""}
                    variant="surface"
                  />
                </UiField>
                <UiField htmlFor={`${webSearchId}-content-types`} label={t("settings.runtime.web_search_anysearch_content_types")}>
                  <UiInput
                    controlSize="sm"
                    disabled={disabled}
                    id={`${webSearchId}-content-types`}
                    onBlur={() => patchAnySearch({ content_types: splitSearchValues(anySearchContentTypesText) })}
                    onChange={(event) => setAnySearchContentTypesText(event.target.value)}
                    placeholder={t("settings.runtime.web_search_anysearch_content_types_placeholder")}
                    value={anySearchContentTypesText}
                    variant="surface"
                  />
                </UiField>
                <UiField
                  className="md:col-span-2"
                  error={anySearchParamsError ? [
                    t("settings.runtime.web_search_anysearch_params_invalid"),
                    t("settings.runtime.web_search_anysearch_params_invalid_impact"),
                    t("settings.runtime.web_search_anysearch_params_invalid_next_step"),
                  ].join(" ") : undefined}
                  htmlFor={`${webSearchId}-params`}
                  label={t("settings.runtime.web_search_anysearch_params")}
                >
                  <UiTextarea
                    controlSize="sm"
                    disabled={disabled}
                    id={`${webSearchId}-params`}
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
                    textRole="code"
                    value={anySearchParamsText}
                    variant="surface"
                  />
                </UiField>
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
  const fieldId = useId();
  const [draftAPIKey, setDraftAPIKey] = useState(apiKey);

  useEffect(() => setDraftAPIKey(apiKey), [apiKey, apiKeyConfigured]);

  return (
    <UiField
      htmlFor={fieldId}
      label={t("settings.runtime.web_search_api_key")}
    >
      <div className="flex gap-2">
        <UiInput
          className="min-w-0 flex-1"
          controlSize="md"
          disabled={disabled}
          id={fieldId}
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
          variant="surface"
        />
        {apiKeyConfigured ? (
          <UiIconButton
            aria-label={t("settings.runtime.web_search_api_key_clear")}
            className="shrink-0"
            disabled={disabled}
            onClick={() => {
              setDraftAPIKey("");
              onChange("");
            }}
            size="lg"
            tone="danger"
            tooltip={t("settings.runtime.web_search_api_key_clear")}
            variant="ghost"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </UiIconButton>
        ) : null}
      </div>
      {provider.apiKeyURL ? (
        <a
          className={cn(
            "inline-flex items-center gap-1 hover:underline",
            getUiTypographyClassName({ role: "supporting", tone: "brand" }),
          )}
          href={provider.apiKeyURL}
          rel="noreferrer"
          target="_blank"
        >
          {t("settings.runtime.web_search_api_key_get", { provider: t(provider.labelKey) })}
          <ExternalLink className="h-3 w-3" />
        </a>
      ) : null}
    </UiField>
  );
}

function SettingsSubsectionTitle({ children }: { children: ReactNode }) {
  return (
    <h4 className={cn(
      "md:col-span-2 flex items-center gap-1.5 border-t border-(--divider-subtle-color) pt-3",
      getUiTypographyClassName({ role: "supporting", tone: "default", weight: "medium" }),
    )}>
      {children}
    </h4>
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
    <UiCheckboxRow
      checked={checked}
      density="compact"
      disabled={disabled}
      icon={icon}
      label={label}
      onChange={onChange}
    />
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
          aria-label={t("settings.runtime.tool_search_label")}
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
