// INPUT: Default model catalog and versioned preference commands.
// OUTPUT: A dedicated model settings page for chat, generation, vision and background tasks.
// POS: Default model settings owner; provider credentials remain in Provider settings.
import { Image, MonitorCog, ScanEye, Sparkles } from "lucide-react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";
import { normalizeAgentRuntimeKind } from "@/types/settings/preferences";
import { SETTINGS_CONTENT_BODY_CLASS_NAME, SETTINGS_CARD_CLASS_NAME, SETTINGS_DIVIDER_CLASS_NAME } from "../shared/settings-panel-ui";
import { PreferencesReliabilityNotice } from "../general/components/preferences-reliability-notice";
import type { PreferenceFeedback, PreferenceRecoveryControls } from "../general/model/settings-preferences-model";
import { useUserPreferences } from "../general/use-user-preferences";
import { useDefaultModelPreferences } from "./use-default-model-preferences";
import type { DefaultModelPreferenceRole } from "./default-model-preferences-model";
import { SettingsDefaultModelRow } from "./settings-default-model-row";

interface SettingsDefaultModelsViewProps {
  defaultBackgroundModelOptions: UiSelectMenuOption[];
  defaultBackgroundModelValue: string;
  defaultImageModelOptions: UiSelectMenuOption[];
  defaultImageModelValue: string;
  defaultVisionModelOptions: UiSelectMenuOption[];
  defaultVisionModelValue: string;
  unavailableVisionSelection: string | null;
  defaultModelCatalogFailed: boolean;
  defaultModelOptions: UiSelectMenuOption[];
  defaultModelSavingRole: DefaultModelPreferenceRole | null;
  defaultModelValue: string;
  onDefaultModelChange: (value: string, role: DefaultModelPreferenceRole) => void;
  onRetryDefaultModelCatalog: () => void;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
  preferencesFeedback: PreferenceFeedback | null;
  preferencesRecovery: PreferenceRecoveryControls;
  providerOptionsLoading: boolean;
}

export function SettingsDefaultModelsSection() {
  const p = useUserPreferences();
  const busy = p.saving || !p.writable || p.recovery.checking || p.hasUnresolvedMutation;
  const models = useDefaultModelPreferences({
    agentRuntimeKind: normalizeAgentRuntimeKind(p.preferences.agent_runtime_kind),
    preferences: p.preferences, getCurrentPreferences: p.getCurrentPreferences,
    persistPreferences: p.persistPreferences, preferencesSaving: busy || p.loading,
  });
  return <SettingsDefaultModelsView
    defaultBackgroundModelOptions={models.options.background} defaultBackgroundModelValue={models.values.background}
    defaultImageModelOptions={models.options.image} defaultImageModelValue={models.values.image}
    defaultVisionModelOptions={models.options.vision} defaultVisionModelValue={models.values.vision}
    unavailableVisionSelection={models.unavailableVisionSelection}
    defaultModelOptions={models.options.agent} defaultModelValue={models.values.agent}
    defaultModelCatalogFailed={models.catalogFailed} defaultModelSavingRole={models.savingRole}
    onDefaultModelChange={models.handleChange} onRetryDefaultModelCatalog={models.retryCatalog}
    preferencesLoading={p.loading} preferencesSaving={busy} preferencesFeedback={p.feedback}
    preferencesRecovery={p.recovery} providerOptionsLoading={models.loading}
  />;
}

export function SettingsDefaultModelsView({
  defaultBackgroundModelOptions,
  defaultBackgroundModelValue,
  defaultImageModelOptions,
  defaultImageModelValue,
  defaultVisionModelOptions,
  defaultVisionModelValue,
  unavailableVisionSelection,
  defaultModelCatalogFailed,
  defaultModelOptions,
  defaultModelSavingRole,
  defaultModelValue,
  onDefaultModelChange,
  onRetryDefaultModelCatalog,
  providerOptionsLoading,
  preferencesLoading,
  preferencesSaving,
  preferencesFeedback,
  preferencesRecovery,
}: SettingsDefaultModelsViewProps) {
  const { t } = useI18n();
  return <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
    <WorkspaceContentHeader className="max-sm:hidden" title={t("settings.tabs.default_models")} />
    <div className={`${SETTINGS_CONTENT_BODY_CLASS_NAME} flex flex-col gap-3`}>
      <PreferencesReliabilityNotice feedback={preferencesFeedback} recovery={preferencesRecovery} />
      {defaultModelCatalogFailed ? (
        <UiResourceState
          impact={t("settings.general.default_model_catalog_failed_impact")}
          primaryAction={{
            busy: providerOptionsLoading,
            busyLabel: t("settings.general.default_model_loading"),
            label: t("settings.general.default_model_catalog_retry"),
            onClick: onRetryDefaultModelCatalog,
          }}
          size="sm"
          state="error"
          title={t("settings.general.default_model_catalog_failed_title")}
          urgency="polite"
        />
      ) : null}
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <SettingsDefaultModelRow
          disabled={preferencesLoading || preferencesSaving}
          descriptionKey="settings.general.default_model_description"
          emptyPlaceholderKey="settings.general.default_model_empty"
          icon={<MonitorCog className="h-3.5 w-3.5" />}
          onChange={onDefaultModelChange}
          options={defaultModelOptions}
          providerOptionsLoading={providerOptionsLoading}
          modelCategory="agent_runtime"
          savingRole={defaultModelSavingRole}
          titleKey="settings.general.default_model_title"
          value={defaultModelValue}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsDefaultModelRow
          disabled={preferencesLoading || preferencesSaving}
          descriptionKey="settings.general.default_image_model_description"
          emptyPlaceholderKey="settings.general.default_image_model_empty"
          icon={<Image className="h-3.5 w-3.5" />}
          onChange={onDefaultModelChange}
          options={defaultImageModelOptions}
          providerOptionsLoading={providerOptionsLoading}
          modelCategory="image_generation"
          savingRole={defaultModelSavingRole}
          titleKey="settings.general.default_image_model_title"
          value={defaultImageModelValue}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsDefaultModelRow
          disabled={preferencesLoading || preferencesSaving}
          descriptionKey="settings.general.default_vision_model_description"
          emptyPlaceholderKey="settings.general.default_vision_model_empty"
          icon={<ScanEye className="h-3.5 w-3.5" />}
          onChange={onDefaultModelChange}
          options={defaultVisionModelOptions}
          providerOptionsLoading={providerOptionsLoading}
          modelCategory="vision_understanding"
          savingRole={defaultModelSavingRole}
          titleKey="settings.general.default_vision_model_title"
          value={defaultVisionModelValue}
        />
        {unavailableVisionSelection ? (
          <p className={`px-4 pb-3 ${getUiTypographyClassName({ role: "caption", tone: "warning" })}`} role="status">
            {t("settings.general.default_vision_model_unavailable")} {unavailableVisionSelection}
          </p>
        ) : null}

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsDefaultModelRow
          disabled={preferencesLoading || preferencesSaving}
          descriptionKey="settings.general.default_background_model_description"
          emptyPlaceholderKey="settings.general.default_background_model_empty"
          icon={<Sparkles className="h-3.5 w-3.5" />}
          onChange={onDefaultModelChange}
          options={defaultBackgroundModelOptions}
          providerOptionsLoading={providerOptionsLoading}
          modelCategory="background_task"
          savingRole={defaultModelSavingRole}
          titleKey="settings.general.default_background_model_title"
          value={defaultBackgroundModelValue}
        />

      </div>
    </div>
  </div>;
}
