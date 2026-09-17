/**
 * INPUT: 通用偏好与 Echo 状态。
 * OUTPUT: 分域恢复提示、唯一设置开关行与具名分段偏好字段。
 * POS: General 行为分区视图；Preferences 写入仍由版本化控制器负责。
 */
"use client";

import { SETTINGS_DIVIDER_CLASS_NAME } from "@/features/settings/shared/settings-panel-ui";
import {
  Brain,
  Bug,
  HeartPulse,
  MessageCircle,
  Moon,
  RadioTower,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";

import { DELIVERY_POLICY_OPTIONS } from "../model/settings-options";
import {
  SettingsToggleRow,
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";
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
  onAgentSdkDiagnosticsChange: (checked: boolean) => void;
  onAutoMemoryEnabledChange: (checked: boolean) => void;
  onAutoDreamEnabledChange: (checked: boolean) => void;
  onEmotionEnabledChange: (checked: boolean) => void;
  onEchoEnabledChange: (checked: boolean) => void;
  onDefaultDeliveryPolicyChange: (
    value: AgentConversationDefaultDeliveryPolicy,
  ) => void;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
  preferencesFeedback: PreferenceFeedback | null;
  preferencesRecovery: PreferenceRecoveryControls;
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
  onAgentSdkDiagnosticsChange,
  onAutoMemoryEnabledChange,
  onAutoDreamEnabledChange,
  onEmotionEnabledChange,
  onEchoEnabledChange,
  onDefaultDeliveryPolicyChange,
  preferencesLoading,
  preferencesSaving,
  preferencesFeedback,
  preferencesRecovery,
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
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <SettingsToggleRow
          checked={agentSdkDiagnosticsEnabled}
          description={t("settings.general.agent_sdk_diagnostics_description")}
          disabled={preferencesLoading || preferencesSaving}
          icon={<Bug className="h-3.5 w-3.5" />}
          onChange={onAgentSdkDiagnosticsChange}
          title={t("settings.general.agent_sdk_diagnostics_title")}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsToggleRow
          checked={autoMemoryEnabled}
          description={t("settings.general.auto_memory_description")}
          disabled={preferencesLoading || preferencesSaving}
          icon={<Brain className="h-3.5 w-3.5" />}
          onChange={onAutoMemoryEnabledChange}
          title={t("settings.general.auto_memory_title")}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsToggleRow
          checked={autoDreamEnabled}
          description={t("settings.general.auto_dream_description")}
          disabled={preferencesLoading || preferencesSaving}
          icon={<Moon className="h-3.5 w-3.5" />}
          onChange={onAutoDreamEnabledChange}
          title={t("settings.general.auto_dream_title")}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsToggleRow
          checked={emotionEnabled}
          description={t("settings.general.emotion_description")}
          disabled={preferencesLoading || preferencesSaving}
          icon={<HeartPulse className="h-3.5 w-3.5" />}
          onChange={onEmotionEnabledChange}
          title={t("settings.general.emotion_title")}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <SettingsToggleRow
          checked={echoEnabled}
          description={t("settings.general.echo_description")}
          disabled={echoDisabled || echoLoading || echoSaving}
          icon={<RadioTower className="h-3.5 w-3.5" />}
          onChange={onEchoEnabledChange}
          title={t("settings.general.echo_title")}
        />

        <div className={SETTINGS_DIVIDER_CLASS_NAME} />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <MessageCircle className="h-3.5 w-3.5" />
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
          <UiSegmentedControl
            className="min-w-0"
            density="compact"
            disabled={preferencesLoading || preferencesSaving}
            onChange={onDefaultDeliveryPolicyChange}
            options={DELIVERY_POLICY_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.labelKey),
            }))}
            stretch
            title={t("settings.general.default_delivery")}
            value={chatDefaultDeliveryPolicy}
          />
        </div>

      </div>
    </section>
  );
}
