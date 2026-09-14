// INPUT: 本地模型草稿、长模型标识与多项 Agent 使用者示例。
// OUTPUT: 三个实际 Provider 弹窗的滚动、字段和忙碌动作预览。
// POS: 隔离开发夹具；只更新本地状态，不装配配置命令或删除真实资源。

import { useState } from "react";

import { ProviderAddModelDialog } from "@/features/settings/provider-settings/dialogs/provider-settings-add-model-dialog";
import { ProviderDeleteUsageDialog } from "@/features/settings/provider-settings/dialogs/provider-settings-delete-usage-dialog";
import { ProviderModelOptionsDialog } from "@/features/settings/provider-settings/dialogs/provider-settings-model-options-dialog";
import type { ModelOptionsState } from "@/features/settings/provider-settings/model/provider-settings-types";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ProviderConfigRecord, ProviderModelRecord } from "@/types/capability/provider";

import { galleryText } from "./ui-gallery-copy";

const MODEL: ProviderModelRecord = {
  id: "gallery-model", provider_id: "gallery-provider",
  model_id: "tenant/research-platform/deployments/extended-context-model-with-a-long-deployment-name",
  display_name: "Gallery model", category: "chat", enabled: true, is_default: false,
  capabilities_auto: {}, capabilities_override: {}, provider_options: {},
};
const OPTIONS: ModelOptionsState = {
  model: MODEL, capabilities: { vision: true, tool_calling: true },
  context_window: "128000", max_output_tokens: "4000", provider_options_text: "{}",
};

export function ProviderDialogsGallery() {
  const { locale, t } = useI18n();
  const [open, setOpen] = useState<"add" | "options" | "delete" | null>(null);
  const [manualId, setManualId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [options, setOptions] = useState<ModelOptionsState | null>(OPTIONS);
  const [saving, setSaving] = useState(false);
  const [deleteCount, setDeleteCount] = useState(0);
  const provider: ProviderConfigRecord = {
    id: "gallery-provider", visibility: "private", provider_kind: "llm", provider: "gallery",
    preset_key: "custom", api_format: "responses", display_name: "Gallery Provider", auth_token_masked: "",
    base_url: "https://example.com", models_path: "/models", enabled: true, usage_count: 16,
    last_test_status: "", last_test_error: "", configuration_version: 1, can_manage: true,
    agent_runtime_supported: true, models: [MODEL],
    used_by_agents: Array.from({ length: 16 }, (_, index) => ({
      agent_id: `gallery-agent-${index}`, name: "", is_main: index === 0,
      display_name: `${String(index + 1).padStart(2, "0")} · ${galleryText(locale,
        "项目规划与资料研究协作智能体，负责完整的交付检查与过程记录",
        "Project planning and research assistant responsible for delivery checks and documentation")}`,
    })),
  };

  return <section className="min-w-0 space-y-4 xl:col-span-2" data-gallery-provider-dialogs data-delete-count={deleteCount}>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "Provider 模型弹窗", "Provider model dialogs")}
    </h2>
    <div className="flex flex-wrap gap-3">
      <UiButton onClick={() => setOpen("add")} variant="outline" data-gallery-provider-add>{t("settings.providers.add_model_title")}</UiButton>
      <UiButton onClick={() => { setSaving(false); setOpen("options"); }} variant="outline" data-gallery-provider-options>{t("settings.providers.model_options")}</UiButton>
      <UiButton onClick={() => setOpen("delete")} variant="outline" data-gallery-provider-delete>{galleryText(locale, "查看占用确认", "Preview usage confirmation")}</UiButton>
    </div>
    <ProviderAddModelDialog isOpen={open === "add"} manualModelEnabled={enabled} manualModelId={manualId}
      manualModelPlaceholder="tenant/model" onAdd={() => setOpen(null)} onClose={() => setOpen(null)}
      pendingAction={null} selectedCanManage setManualModelEnabled={setEnabled} setManualModelId={setManualId} />
    <ProviderModelOptionsDialog modelOptions={open === "options" ? options : null} onClose={() => setOpen(null)}
      onSave={() => setSaving(true)} pendingAction={saving ? { kind: "save-model-options", modelId: MODEL.model_id } : null}
      selectedCanManage setModelOptions={setOptions} />
    <ProviderDeleteUsageDialog deleteTargetRecord={provider} isOpen={open === "delete"} onCancel={() => setOpen(null)}
      onForceDelete={() => { setDeleteCount((count) => count + 1); setOpen(null); }} pendingAction={null} />
  </section>;
}
