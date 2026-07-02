"use client";

import { ExternalLink } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiInput } from "@/shared/ui/form-control";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/select-menu";
import type {
  ProviderConfigRecord,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";

import {
  API_FORMAT_LABELS,
  API_FORMAT_SHORT_LABELS,
  PROVIDER_LABEL_CLASS_NAME,
  ProviderDraft,
  format_token_preview,
} from "./provider-settings-model";

interface ProviderSettingsConfigFormProps {
  builtin_endpoint_formats: ProviderPresetFormat[];
  current_format: ProviderPresetFormat | null;
  current_preset: ProviderPreset | null;
  detail_title: string;
  draft: ProviderDraft;
  format_options: UiSelectMenuOption[];
  is_custom_provider: boolean;
  is_editing: boolean;
  on_api_format_change: (value: string) => void;
  on_auth_token_change: (value: string) => void;
  on_base_url_change: (value: string) => void;
  on_field_blur: () => void;
  on_provider_display_name_change: (value: string) => void;
  on_provider_kind_change: (value: string) => void;
  provider_kind_options: UiSelectMenuOption[];
  selected_can_manage: boolean;
  selected_record: ProviderConfigRecord | null;
  show_provider_shape_controls: boolean;
  show_runtime_format_badge: boolean;
  uses_builtin_endpoint: boolean;
}

export function ProviderSettingsConfigForm({
  builtin_endpoint_formats: builtinEndpointFormats,
  current_format: currentFormat,
  current_preset: currentPreset,
  detail_title: detailTitle,
  draft,
  format_options: formatOptions,
  is_custom_provider: isCustomProvider,
  is_editing: isEditing,
  on_api_format_change: onApiFormatChange,
  on_auth_token_change: onAuthTokenChange,
  on_base_url_change: onBaseUrlChange,
  on_field_blur: onFieldBlur,
  on_provider_display_name_change: onProviderDisplayNameChange,
  on_provider_kind_change: onProviderKindChange,
  provider_kind_options: providerKindOptions,
  selected_can_manage: selectedCanManage,
  selected_record: selectedRecord,
  show_provider_shape_controls: showProviderShapeControls,
  show_runtime_format_badge: showRuntimeFormatBadge,
  uses_builtin_endpoint: usesBuiltinEndpoint,
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
                control_size="lg"
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
              aria_label={t("settings.providers.kind")}
              class_name="h-11"
              disabled={!selectedCanManage || isEditing || providerKindOptions.length <= 1}
              on_change={onProviderKindChange}
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
              aria_label={t("settings.providers.api_format")}
              class_name="h-11"
              disabled={!selectedCanManage || formatOptions.length <= 1}
              on_change={onApiFormatChange}
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
          control_size="md"
          data-form-type="other"
          data-lpignore="true"
          disabled={!selectedCanManage}
          name="provider-auth-token"
          onChange={(event) => onAuthTokenChange(event.target.value)}
          onBlur={onFieldBlur}
          placeholder={isEditing
            ? format_token_preview(
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
            control_size="md"
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
