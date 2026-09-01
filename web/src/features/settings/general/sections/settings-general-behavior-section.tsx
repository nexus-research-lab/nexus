/**
 * INPUT: 通用偏好、Echo 与默认模型目录状态。
 * OUTPUT: 分域恢复提示和通用行为设置控件。
 * POS: General 行为分区视图；Preferences 写入仍由版本化控制器负责。
 */
"use client";

import {
  Brain,
  Bug,
  HeartPulse,
  Image,
  MessageSquareText,
  MonitorCog,
  Moon,
  RadioTower,
  ScanEye,
  Sparkles,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";

import { SettingsDefaultModelRow } from "../components/settings-default-model-row";
import { DELIVERY_POLICY_OPTIONS } from "../model/settings-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "../../shared/settings-panel-ui";
import type { DefaultModelPreferenceRole } from "../model/default-model-preferences-model";
import { SettingsOnboardingRow } from "../components/settings-onboarding-row";
import { PreferencesReliabilityNotice } from "../components/preferences-reliability-notice";
import { EchoSettingsReliabilityNotice } from "../components/echo-settings-reliability-notice";
import type {
  EchoSettingsFeedback,
  EchoSettingsRecoveryControls,
} from "../model/echo-settings-reliability-model";
import type {
  PreferenceFeedback,
  PreferenceRecoveryControls,
} from "../model/settings-preferences-model";

interface SettingsGeneralBehaviorSectionProps {
  agentSdkDiagnosticsEnabled: boolean;
  autoMemoryEnabled: boolean;
  autoDreamEnabled: boolean;
  chatDefaultDeliveryPolicy: AgentConversationDefaultDeliveryPolicy;
  emotionEnabled: boolean;
  echoDisabled: boolean;
  echoEnabled: boolean;
  echoFeedback: EchoSettingsFeedback | null;
  echoLoading: boolean;
  echoRecovery: EchoSettingsRecoveryControls;
  echoSaving: boolean;
  defaultBackgroundModelOptions: UiSelectMenuOption[];
  defaultBackgroundModelValue: string;
  defaultImageModelOptions: UiSelectMenuOption[];
  defaultImageModelValue: string;
  defaultVisionModelOptions: UiSelectMenuOption[];
  defaultVisionModelValue: string;
  defaultModelCatalogFailed: boolean;
  defaultModelOptions: UiSelectMenuOption[];
  defaultModelSavingRole: DefaultModelPreferenceRole | null;
  defaultModelValue: string;
  onAgentSdkDiagnosticsChange: (checked: boolean) => void;
  onAutoMemoryEnabledChange: (checked: boolean) => void;
  onAutoDreamEnabledChange: (checked: boolean) => void;
  onEmotionEnabledChange: (checked: boolean) => void;
  onEchoEnabledChange: (checked: boolean) => void;
  onDefaultDeliveryPolicyChange: (
    value: AgentConversationDefaultDeliveryPolicy,
  ) => void;
  onDefaultModelChange: (
    value: string,
    role: DefaultModelPreferenceRole,
  ) => void;
  onRetryDefaultModelCatalog: () => void;
  onResetTours: () => void;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
  preferencesFeedback: PreferenceFeedback | null;
  preferencesRecovery: PreferenceRecoveryControls;
  providerOptionsLoading: boolean;
}

export function SettingsGeneralBehaviorSection({
  agentSdkDiagnosticsEnabled,
  autoMemoryEnabled,
  autoDreamEnabled,
  chatDefaultDeliveryPolicy,
  emotionEnabled,
  echoDisabled,
  echoEnabled,
  echoFeedback,
  echoLoading,
  echoRecovery,
  echoSaving,
  defaultBackgroundModelOptions,
  defaultBackgroundModelValue,
  defaultImageModelOptions,
  defaultImageModelValue,
  defaultVisionModelOptions,
  defaultVisionModelValue,
  defaultModelCatalogFailed,
  defaultModelOptions,
  defaultModelSavingRole,
  defaultModelValue,
  onAgentSdkDiagnosticsChange,
  onAutoMemoryEnabledChange,
  onAutoDreamEnabledChange,
  onEmotionEnabledChange,
  onEchoEnabledChange,
  onDefaultDeliveryPolicyChange,
  onDefaultModelChange,
  onRetryDefaultModelCatalog,
  onResetTours,
  preferencesLoading,
  preferencesSaving,
  preferencesFeedback,
  preferencesRecovery,
  providerOptionsLoading,
}: SettingsGeneralBehaviorSectionProps) {
  const { t } = useI18n();

  return (
    <section className="space-y-2.5">
      <PreferencesReliabilityNotice
        feedback={preferencesFeedback}
        recovery={preferencesRecovery}
      />
      <EchoSettingsReliabilityNotice
        feedback={echoFeedback}
        recovery={echoRecovery}
      />
      {defaultModelCatalogFailed ? (
        <UiResourceState
          impact={t("settings.general.default_model_catalog_failed_impact")}
          primaryAction={{
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
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Bug className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.agent_sdk_diagnostics_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.agent_sdk_diagnostics_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.agent_sdk_diagnostics_label")}
            </span>
            <GlassSwitch
              aria-label={t("settings.general.agent_sdk_diagnostics_label")}
              checked={agentSdkDiagnosticsEnabled}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onAgentSdkDiagnosticsChange}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Brain className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.auto_memory_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.auto_memory_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.auto_memory_label")}
            </span>
            <GlassSwitch
              aria-label={t("settings.general.auto_memory_title")}
              checked={autoMemoryEnabled}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onAutoMemoryEnabledChange}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Moon className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.auto_dream_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.auto_dream_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.auto_dream_label")}
            </span>
            <GlassSwitch
              aria-label={t("settings.general.auto_dream_title")}
              checked={autoDreamEnabled}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onAutoDreamEnabledChange}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <HeartPulse className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.emotion_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.emotion_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.emotion_label")}
            </span>
            <GlassSwitch
              aria-label={t("settings.general.emotion_label")}
              checked={emotionEnabled}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onEmotionEnabledChange}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <RadioTower className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.echo_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.echo_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.echo_label")}
            </span>
            <GlassSwitch
              aria-label={t("settings.general.echo_label")}
              checked={echoEnabled}
              disabled={echoDisabled || echoLoading || echoSaving}
              onChange={onEchoEnabledChange}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          disabled={preferencesSaving}
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

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          disabled={preferencesSaving}
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

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          disabled={preferencesSaving}
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

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          disabled={preferencesSaving}
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

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <MessageSquareText className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.runtime_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.runtime_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 flex-col gap-1.5">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.default_delivery")}
            </span>
            <SettingsSegmentedControl
              ariaLabel={t("settings.general.default_delivery")}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onDefaultDeliveryPolicyChange}
              options={DELIVERY_POLICY_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={chatDefaultDeliveryPolicy}
            />
          </div>
        </div>

        <SettingsOnboardingRow onReset={onResetTours} />
      </div>
    </section>
  );
}
