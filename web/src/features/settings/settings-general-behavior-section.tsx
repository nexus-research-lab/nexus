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

import { SettingsDefaultModelRow } from "./settings-default-model-row";
import {
  AGENT_RUNTIME_KIND_OPTIONS,
  DELIVERY_POLICY_OPTIONS,
} from "./settings-options";
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
} from "./settings-panel-ui";
import type { DefaultModelPreferenceRole } from "./settings-preferences-model";
import { SettingsOnboardingRow } from "./settings-onboarding-row";

interface SettingsGeneralBehaviorSectionProps {
  agent_runtime_kind: AgentRuntimeKind;
  agent_sdk_diagnostics_enabled: boolean;
  chat_default_delivery_policy: AgentConversationDefaultDeliveryPolicy;
  default_background_model_options: UiSelectMenuOption[];
  default_background_model_value: string;
  default_image_model_options: UiSelectMenuOption[];
  default_image_model_value: string;
  default_model_feedback_message?: string | null;
  default_model_options: UiSelectMenuOption[];
  default_model_saving_role: DefaultModelPreferenceRole | null;
  default_model_value: string;
  nxs_runtime_checking: boolean;
  on_agent_runtime_kind_change: (value: AgentRuntimeKind) => void;
  on_agent_sdk_diagnostics_change: (checked: boolean) => void;
  on_default_delivery_policy_change: (
    value: AgentConversationDefaultDeliveryPolicy,
  ) => void;
  on_default_model_change: (
    value: string,
    role: DefaultModelPreferenceRole,
  ) => void;
  on_reset_tours: () => void;
  preferences_loading: boolean;
  preferences_saving: boolean;
  provider_options_loading: boolean;
}

export function SettingsGeneralBehaviorSection({
  agent_runtime_kind,
  agent_sdk_diagnostics_enabled,
  chat_default_delivery_policy,
  default_background_model_options,
  default_background_model_value,
  default_image_model_options,
  default_image_model_value,
  default_model_feedback_message,
  default_model_options,
  default_model_saving_role,
  default_model_value,
  nxs_runtime_checking,
  on_agent_runtime_kind_change,
  on_agent_sdk_diagnostics_change,
  on_default_delivery_policy_change,
  on_default_model_change,
  on_reset_tours,
  preferences_loading,
  preferences_saving,
  provider_options_loading,
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
              aria_label={t("settings.general.agent_runtime_label")}
              disabled={
                preferences_loading ||
                preferences_saving ||
                nxs_runtime_checking
              }
              on_change={on_agent_runtime_kind_change}
              options={AGENT_RUNTIME_KIND_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label_key),
              }))}
              value={agent_runtime_kind}
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
              checked={agent_sdk_diagnostics_enabled}
              disabled={preferences_loading || preferences_saving}
              on_change={on_agent_sdk_diagnostics_change}
              size="sm"
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          description_key="settings.general.default_model_description"
          empty_placeholder_key="settings.general.default_model_empty"
          icon={<MonitorCog className="h-3.5 w-3.5" />}
          on_change={on_default_model_change}
          options={default_model_options}
          provider_options_loading={provider_options_loading}
          model_category="agent_runtime"
          saving_role={default_model_saving_role}
          title_key="settings.general.default_model_title"
          value={default_model_value}
        />

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          description_key="settings.general.default_image_model_description"
          empty_placeholder_key="settings.general.default_image_model_empty"
          icon={<Image className="h-3.5 w-3.5" />}
          on_change={on_default_model_change}
          options={default_image_model_options}
          provider_options_loading={provider_options_loading}
          model_category="image_generation"
          saving_role={default_model_saving_role}
          title_key="settings.general.default_image_model_title"
          value={default_image_model_value}
        />

        <div className="border-t border-(--divider-subtle-color)" />

        <SettingsDefaultModelRow
          description_key="settings.general.default_background_model_description"
          empty_placeholder_key="settings.general.default_background_model_empty"
          feedback_message={default_model_feedback_message}
          icon={<Sparkles className="h-3.5 w-3.5" />}
          on_change={on_default_model_change}
          options={default_background_model_options}
          provider_options_loading={provider_options_loading}
          model_category="background_task"
          saving_role={default_model_saving_role}
          title_key="settings.general.default_background_model_title"
          value={default_background_model_value}
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
              aria_label={t("settings.general.default_delivery")}
              disabled={preferences_loading}
              on_change={on_default_delivery_policy_change}
              options={DELIVERY_POLICY_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label_key),
              }))}
              value={chat_default_delivery_policy}
            />
          </div>
        </div>

        <SettingsOnboardingRow on_reset={on_reset_tours} />
      </div>
    </section>
  );
}
