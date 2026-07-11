"use client";

import { ExternalLink } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/menu/select-menu";
import type {
  ProviderConfigRecord,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";

import {
  API_FORMAT_LABELS,
  API_FORMAT_SHORT_LABELS,
  PROVIDER_LABEL_CLASS_NAME,
  formatTokenPreview,
} from "../model/provider-settings-presentation";
import type { ProviderDraft } from "../model/provider-settings-types";

interface ProviderSettingsConfigFormProps {
  builtinEndpointFormats: ProviderPresetFormat[];
  currentFormat: ProviderPresetFormat | null;
  currentPreset: ProviderPreset | null;
  detailTitle: string;
  draft: ProviderDraft;
  formatOptions: UiSelectMenuOption[];
  isCustomProvider: boolean;
  isEditing: boolean;
  onApiFormatChange: (value: string) => void;
  onAuthTokenChange: (value: string) => void;
  onBaseUrlChange: (value: string) => void;
  onFieldBlur: () => void;
  onProviderDisplayNameChange: (value: string) => void;
  onProviderKindChange: (value: string) => void;
  providerKindOptions: UiSelectMenuOption[];
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  showProviderShapeControls: boolean;
  showRuntimeFormatBadge: boolean;
  usesBuiltinEndpoint: boolean;
}

export function ProviderSettingsConfigForm({
  builtinEndpointFormats,
  currentFormat,
  currentPreset,
  detailTitle,
  draft,
  formatOptions,
  isCustomProvider,
  isEditing,
  onApiFormatChange,
  onAuthTokenChange,
  onBaseUrlChange,
  onFieldBlur,
  onProviderDisplayNameChange,
  onProviderKindChange,
  providerKindOptions,
  selectedCanManage,
  selectedRecord,
  showProviderShapeControls,
  showRuntimeFormatBadge,
  usesBuiltinEndpoint,
}: ProviderSettingsConfigFormProps) {
  const { t } = useI18n();

  return (
    <>
      {showProviderShapeControls ? (
        <div className={isCustomProvider ? "grid gap-4 md:grid-cols-[minmax(0,1fr)_180px_260px]" : "grid gap-4 md:grid-cols-[180px_260px]"}>
          {isCustomProvider ? (
            <label className="space-y-2">
              <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.provider_name")}</span>
              <UiInput
                autoCapitalize="off"
                autoCorrect="off"
                controlSize="lg"
                disabled={!selectedCanManage}
                onChange={(event) => onProviderDisplayNameChange(event.target.value)}
                onBlur={onFieldBlur}
                placeholder={t("settings.providers.provider_name_placeholder")}
                spellCheck={false}
                type="text"
                value={draft.display_name}
              />
            </label>
          ) : null}

          <label className="space-y-2">
            <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.kind")}</span>
            <UiSelectMenu
              ariaLabel={t("settings.providers.kind")}
              className="h-11"
              disabled={!selectedCanManage || isEditing || providerKindOptions.length <= 1}
              onChange={onProviderKindChange}
              options={providerKindOptions}
              size="sm"
              value={draft.provider_kind}
            />
          </label>

          <label className="space-y-2">
            <span className="flex items-center gap-2">
              <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.api_format")}</span>
              {showRuntimeFormatBadge ? (
                <span
                  className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-medium leading-4 text-(--text-muted)"
                  title={t("settings.providers.api_format_runtime_hint")}
                >
                  {t("settings.providers.api_format_runtime_badge")}
                </span>
              ) : null}
            </span>
            <UiSelectMenu
              ariaLabel={t("settings.providers.api_format")}
              className="h-11"
              disabled={!selectedCanManage || formatOptions.length <= 1}
              onChange={onApiFormatChange}
              options={formatOptions}
              size="sm"
              value={draft.api_format}
            />
          </label>
        </div>
      ) : null}

      <label className="block space-y-2">
        <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.api_key")}</span>
        <UiInput
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          controlSize="md"
          data-form-type="other"
          data-lpignore="true"
          disabled={!selectedCanManage}
          name="provider-auth-token"
          onChange={(event) => onAuthTokenChange(event.target.value)}
          onBlur={onFieldBlur}
          placeholder={isEditing
            ? formatTokenPreview(
              selectedRecord?.auth_token_masked,
              t("settings.providers.api_key_empty"),
            )
            : t("settings.providers.api_key_placeholder")}
          spellCheck={false}
          type="password"
          value={draft.auth_token}
        />
        {currentPreset?.key_url ? (
          <a
            className="inline-flex items-center gap-1 text-[12px] font-medium text-primary hover:underline"
            href={currentPreset.key_url}
            rel="noreferrer"
            target="_blank"
          >
            {t("settings.providers.get_api_key_from", { name: detailTitle })}
            <ExternalLink className="h-3.5 w-3.5" />
          </a>
        ) : null}
      </label>

      <div className="block space-y-2">
        <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.base_url")}</span>
        {usesBuiltinEndpoint ? (
          <div className="space-y-1.5">
            {builtinEndpointFormats.map((format) => (
              <div
                className="input-shell grid min-h-9 grid-cols-1 items-center gap-1.5 rounded-[12px] px-3.5 py-1.5 text-sm text-(--text-default) sm:grid-cols-[88px_minmax(0,1fr)] sm:gap-3"
                key={format.api_format}
              >
                <span
                  className="inline-flex h-6 w-fit max-w-full items-center rounded-full bg-(--surface-muted-background) px-2 text-[11px] font-semibold text-(--text-muted)"
                  title={API_FORMAT_LABELS[format.api_format]}
                >
                  {API_FORMAT_SHORT_LABELS[format.api_format]}
                </span>
                <span className="min-w-0 break-all font-mono text-(--text-strong)">
                  {format.base_url}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            disabled={!selectedCanManage}
            onChange={(event) => onBaseUrlChange(event.target.value)}
            onBlur={onFieldBlur}
            placeholder={currentFormat?.base_url || "https://api.example.com/v1"}
            spellCheck={false}
            type="text"
            value={draft.base_url}
          />
        )}
      </div>
    </>
  );
}
