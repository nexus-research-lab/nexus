"use client";

import { ExternalLink } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type {
  ProviderConfigRecord,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";

import {
  API_FORMAT_LABELS,
  API_FORMAT_SHORT_LABELS,
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

function ProviderShapeControls({
  draft,
  formatOptions,
  isEditing,
  onApiFormatChange,
  onFieldBlur,
  onProviderDisplayNameChange,
  onProviderKindChange,
  providerKindOptions,
  selectedCanManage,
  showProviderShapeControls,
  showRuntimeFormatBadge,
}: Pick<
  ProviderSettingsConfigFormProps,
  | "draft"
  | "formatOptions"
  | "isEditing"
  | "onApiFormatChange"
  | "onFieldBlur"
  | "onProviderDisplayNameChange"
  | "onProviderKindChange"
  | "providerKindOptions"
  | "selectedCanManage"
  | "showProviderShapeControls"
  | "showRuntimeFormatBadge"
>) {
  const { t } = useI18n();
  if (!showProviderShapeControls) {
    return null;
  }
  return (
    <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_180px_260px]">
      <UiField
        htmlFor="provider-display-name"
        label={t("settings.providers.provider_name")}
        required={draft.preset_key === "custom"}
      >
        <UiInput
          autoCapitalize="off"
          autoCorrect="off"
          controlSize="lg"
          disabled={!selectedCanManage || draft.preset_key !== "custom"}
          id="provider-display-name"
          onChange={(event) => onProviderDisplayNameChange(event.target.value)}
          onBlur={onFieldBlur}
          placeholder={t("settings.providers.provider_name_placeholder")}
          required={draft.preset_key === "custom"}
          spellCheck={false}
          type="text"
          value={draft.display_name}
        />
      </UiField>

      <UiField
        label={t("settings.providers.kind")}
      >
        <UiSelectMenu
          ariaLabel={t("settings.providers.kind")}
          disabled={!selectedCanManage || isEditing || providerKindOptions.length <= 1}
          onChange={onProviderKindChange}
          options={providerKindOptions}
          size="lg"
          value={draft.provider_kind}
        />
      </UiField>

      <UiField
        label={(
          <span className="inline-flex items-center gap-2">
            {t("settings.providers.api_format")}
          {showRuntimeFormatBadge ? (
            <UiBadge
              size="xs"
              tone="idle"
              title={t("settings.providers.api_format_runtime_hint")}
            >
              {t("settings.providers.api_format_runtime_badge")}
            </UiBadge>
          ) : null}
          </span>
        )}
        required
      >
        <UiSelectMenu
          ariaLabel={t("settings.providers.api_format")}
          disabled={!selectedCanManage || formatOptions.length <= 1}
          onChange={onApiFormatChange}
          options={formatOptions}
          size="lg"
          value={draft.api_format}
        />
      </UiField>
    </div>
  );
}

function ProviderApiKeyField({
  currentPreset,
  detailTitle,
  draft,
  isEditing,
  onAuthTokenChange,
  onFieldBlur,
  selectedCanManage,
  selectedRecord,
}: Pick<
  ProviderSettingsConfigFormProps,
  | "currentPreset"
  | "detailTitle"
  | "draft"
  | "isEditing"
  | "onAuthTokenChange"
  | "onFieldBlur"
  | "selectedCanManage"
  | "selectedRecord"
>) {
  const { t } = useI18n();
  const placeholder = isEditing
    ? formatTokenPreview(
        selectedRecord?.auth_token_masked,
        t("settings.providers.api_key_empty"),
      )
    : t("settings.providers.api_key_placeholder");
  return (
    <UiField
      htmlFor="provider-auth-token"
      label={t("settings.providers.api_key")}
      required={!isEditing}
    >
      <UiInput
        autoCapitalize="off"
        autoComplete="off"
        autoCorrect="off"
        controlSize="md"
        data-form-type="other"
        data-lpignore="true"
        disabled={!selectedCanManage}
        id="provider-auth-token"
        name="provider-auth-token"
        onChange={(event) => onAuthTokenChange(event.target.value)}
        onBlur={onFieldBlur}
        placeholder={placeholder}
        required={!isEditing}
        spellCheck={false}
        type="password"
        value={draft.auth_token}
      />
      {currentPreset?.key_url ? (
        <a
          className={cn(
            "inline-flex items-center gap-1 hover:underline",
            getUiTypographyClassName({ role: "metadata", tone: "brand", weight: "medium" }),
          )}
          href={currentPreset.key_url}
          rel="noreferrer"
          target="_blank"
        >
          {t("settings.providers.get_api_key_from", { name: detailTitle })}
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      ) : null}
    </UiField>
  );
}

function ProviderEndpointField({
  builtinEndpointFormats,
  currentFormat,
  draft,
  onBaseUrlChange,
  onFieldBlur,
  selectedCanManage,
  usesBuiltinEndpoint,
}: Pick<
  ProviderSettingsConfigFormProps,
  | "builtinEndpointFormats"
  | "currentFormat"
  | "draft"
  | "onBaseUrlChange"
  | "onFieldBlur"
  | "selectedCanManage"
  | "usesBuiltinEndpoint"
>) {
  const { t } = useI18n();
  return (
    <UiField
      htmlFor="provider-base-url"
      label={t("settings.providers.base_url")}
      required={!usesBuiltinEndpoint}
    >
      {usesBuiltinEndpoint ? (
        <div className="space-y-1.5">
          {builtinEndpointFormats.map((format) => (
            <div
              className={cn(
                "input-shell radius-control-lg grid min-h-9 grid-cols-1 items-center gap-1.5 px-3.5 py-1.5 sm:grid-cols-[88px_minmax(0,1fr)] sm:gap-3",
                getUiTypographyClassName({ role: "control", tone: "default" }),
              )}
              key={format.api_format}
            >
              <UiBadge
                className="w-fit max-w-full"
                size="sm"
                tone="idle"
                title={API_FORMAT_LABELS[format.api_format]}
              >
                {API_FORMAT_SHORT_LABELS[format.api_format]}
              </UiBadge>
              <span className={cn("min-w-0 break-all", getUiTypographyClassName({ role: "code", tone: "strong" }))}>
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
          id="provider-base-url"
          onChange={(event) => onBaseUrlChange(event.target.value)}
          onBlur={onFieldBlur}
          placeholder={currentFormat?.base_url_placeholder
            || currentFormat?.base_url
            || "https://api.example.com/v1"}
          required
          spellCheck={false}
          type="text"
          value={draft.base_url}
        />
      )}
    </UiField>
  );
}

export function ProviderSettingsConfigForm(
  props: ProviderSettingsConfigFormProps,
) {

  return (
    <>
      <ProviderShapeControls {...props} />
      <ProviderApiKeyField {...props} />
      <ProviderEndpointField {...props} />
    </>
  );
}
