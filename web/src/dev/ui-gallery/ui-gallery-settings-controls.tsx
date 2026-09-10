// INPUT: 当前语言及本地默认模型、权限、记忆开关和保存中状态。
// OUTPUT: 真实设置行和公共紧凑控件的密度、可读性及选择回调预览。
// POS: 开发期消费夹具；不请求设置服务，不写入用户偏好。

import { Brain, MonitorCog } from "lucide-react";
import { useState } from "react";

import { SettingsDefaultModelRow } from "@/features/settings/general/components/settings-default-model-row";
import { SettingsPermissionsSection } from "@/features/settings/general/sections/settings-permissions-section";
import { SettingsNavigationGroupLabel, SettingsToggleRow } from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { galleryText } from "./ui-gallery-copy";

export function SettingsControlsGallery() {
  const { locale, t } = useI18n();
  const [model, setModel] = useState("balanced");
  const [permission, setPermission] = useState("default");
  const [saving, setSaving] = useState(false);
  const [consolidation, setConsolidation] = useState(true);
  const [commands, setCommands] = useState<string[]>([]);
  const options = [
    { value: "balanced", label: galleryText(locale, "均衡模型", "Balanced model") },
    { value: "reasoning", label: galleryText(locale, "推理模型", "Reasoning model") },
  ];

  return <section className="min-w-0 space-y-4 bg-background xl:col-span-2" data-gallery-settings-controls>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "设置与紧凑表单", "Settings and compact forms")}
    </h2>
    <UiButton aria-pressed={saving} onClick={() => setSaving(!saving)} size="sm" variant="surface" data-gallery-settings-saving>
      {galleryText(locale, "切换保存中状态", "Toggle saving state")}
    </UiButton>
    <div data-gallery-settings-model>
      <SettingsNavigationGroupLabel>{galleryText(locale, "默认行为", "Defaults")}</SettingsNavigationGroupLabel>
      <SettingsDefaultModelRow disabled={false} descriptionKey="settings.general.default_model_description"
        emptyPlaceholderKey="settings.general.default_model_empty" icon={<MonitorCog className="h-3.5 w-3.5" />}
        modelCategory="agent_runtime" onChange={(value, role) => {
          setModel(value);
          setCommands((current) => [...current, `${role}:${value}`]);
        }} options={options} providerOptionsLoading={false} savingRole={saving ? "agent_runtime" : null}
        titleKey="settings.general.default_model_title" value={model} />
    </div>
    <div data-gallery-settings-permission>
      <SettingsPermissionsSection onPermissionModeChange={(value) => {
        setPermission(value);
        setCommands((current) => [...current, `permission:${value}`]);
      }} permissionMode={permission} preferencesFeedback={null} preferencesLoading={false}
        preferencesRecovery={{ canCompare: false, canRepairProjection: false, checking: false,
          checkLatest: () => undefined, reapplyDraft: () => undefined, repairProjection: () => undefined, repairing: false }}
        preferencesSaving={saving} />
    </div>
    <div data-gallery-settings-toggle>
      <SettingsToggleRow checked={consolidation} disabled={saving}
        description={t("settings.general.auto_dream_description")}
        title={t("settings.general.auto_dream_title")} icon={<Brain className="h-3.5 w-3.5" />}
        onChange={(checked) => {
          setConsolidation(checked);
          setCommands((current) => [...current, `consolidation:${checked}`]);
        }} />
    </div>
    <div className="grid gap-4 sm:grid-cols-2">
      {(["xs", "sm"] as const).map((size) => <UiField key={size} htmlFor={`gallery-compact-${size}`}
        label={galleryText(locale, "紧凑表单", "Compact form")}
        description={galleryText(locale, "输入、选择和动作保持同一尺寸。", "Fields, choices and actions share one size.")}>
        <div className="grid grid-cols-3 items-center gap-2" data-gallery-compact-size={size}>
          <UiInput aria-label={`Input ${size}`} id={`gallery-compact-${size}`} controlSize={size} defaultValue="Nexus" />
          <UiSelectMenu ariaLabel={`Select ${size}`} onChange={setModel} options={options} size={size} value={model} />
          <UiButton size={size} variant="surface">{galleryText(locale, "保存", "Save")}</UiButton>
        </div>
      </UiField>)}
    </div>
    <output className="sr-only" data-gallery-settings-commands>{JSON.stringify(commands)}</output>
  </section>;
}
