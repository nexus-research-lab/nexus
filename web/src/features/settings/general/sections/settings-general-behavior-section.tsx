"use client";

import {
  Bug,
  Image,
  MessageSquareText,
  MonitorCog,
  Sparkles,
  Terminal,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { UiSelectMenuOption } from "@/shared/ui/select-menu";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import { SettingsDefaultModelRow } from "../components/settings-default-model-row";
import {
  AGENT_RUNTIME_KIND_OPTIONS,
  DELIVERY_POLICY_OPTIONS,
} from "../model/settings-options";
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
} from "../../shared/settings-panel-ui";
import type { DefaultModelPreferenceRole } from "../model/settings-preferences-model";
import { SettingsOnboardingRow } from "../components/settings-onboarding-row";

interface SettingsGeneralBehaviorSectionProps {
  agentRuntimeKind: AgentRuntimeKind;
  agentSdkDiagnosticsEnabled: boolean;
  chatDefaultDeliveryPolicy: AgentConversationDefaultDeliveryPolicy;
  defaultBackgroundModelOptions: UiSelectMenuOption[];
  defaultBackgroundModelValue: string;
  defaultImageModelOptions: UiSelectMenuOption[];
  defaultImageModelValue: string;
  defaultModelFeedbackMessage?: string | null;
  defaultModelOptions: UiSelectMenuOption[];
  defaultModelSavingRole: DefaultModelPreferenceRole | null;
  defaultModelValue: string;
  nxsRuntimeChecking: boolean;
  onAgentRuntimeKindChange: (value: AgentRuntimeKind) => void;
  onAgentSdkDiagnosticsChange: (checked: boolean) => void;
  onDefaultDeliveryPolicyChange: (
    value: AgentConversationDefaultDeliveryPolicy,
  ) => void;
  onDefaultModelChange: (
    value: string,
    role: DefaultModelPreferenceRole,
  ) => void;
  onResetTours: () => void;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
  providerOptionsLoading: boolean;
}

export function SettingsGeneralBehaviorSection({
  agentRuntimeKind,
  agentSdkDiagnosticsEnabled,
  chatDefaultDeliveryPolicy,
  defaultBackgroundModelOptions,
  defaultBackgroundModelValue,
  defaultImageModelOptions,
  defaultImageModelValue,
  defaultModelFeedbackMessage,
  defaultModelOptions,
  defaultModelSavingRole,
  defaultModelValue,
  nxsRuntimeChecking,
  onAgentRuntimeKindChange,
  onAgentSdkDiagnosticsChange,
  onDefaultDeliveryPolicyChange,
  onDefaultModelChange,
  onResetTours,
  preferencesLoading,
  preferencesSaving,
  providerOptionsLoading,
}: SettingsGeneralBehaviorSectionProps) {
  const { t } = useI18n();

  return (
    <section className="space-y-2.5">
      <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
        {t("settings.general.section_general")}
      </h2>
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Terminal className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.agent_runtime_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.agent_runtime_description")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 flex-col gap-1.5">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
              {t("settings.general.agent_runtime_label")}
            </span>
            <SettingsSegmentedControl
              ariaLabel={t("settings.general.agent_runtime_label")}
              disabled={
                preferencesLoading ||
                preferencesSaving ||
                nxsRuntimeChecking
              }
              onChange={onAgentRuntimeKindChange}
              options={AGENT_RUNTIME_KIND_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={agentRuntimeKind}
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

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
              checked={agentSdkDiagnosticsEnabled}
              disabled={preferencesLoading || preferencesSaving}
              onChange={onAgentSdkDiagnosticsChange}
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
          descriptionKey="settings.general.default_background_model_description"
          emptyPlaceholderKey="settings.general.default_background_model_empty"
          feedbackMessage={defaultModelFeedbackMessage}
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
