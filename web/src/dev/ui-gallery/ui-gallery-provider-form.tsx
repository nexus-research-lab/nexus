// INPUT: 本地 Provider 草稿、详情栏宽度与固定端点分支。
// OUTPUT: 实际 Provider 表单在宽窄容器中的布局、只读端点及输入回调预览。
// POS: 开发期隔离夹具；不装配 Provider 控制器或写入服务。

import { useState } from "react";

import { ProviderSettingsConfigForm } from "@/features/settings/provider-settings/components/provider-settings-config-form";
import type { ProviderDraft } from "@/features/settings/provider-settings/model/provider-settings-types";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { galleryText } from "./ui-gallery-copy";

const FORMATS = [
  { api_format: "responses" as const, base_url: "https://example.com/api/responses", models_path: "/models" },
  { api_format: "chat_completions" as const, base_url: "https://example.com/api/compatible/chat/completions", models_path: "/models" },
];

export function ProviderFormGallery() {
  const { locale } = useI18n();
  const [width, setWidth] = useState("800");
  const [fixed, setFixed] = useState(false);
  const [blurCount, setBlurCount] = useState(0);
  const [draft, setDraft] = useState<ProviderDraft>({
    provider_kind: "llm", provider: "gallery", preset_key: "custom", api_format: "responses",
    display_name: "Gallery Provider", auth_token: "", base_url: "https://example.com/api",
    models_path: "/models", enabled: true,
  });
  const update = <Key extends keyof ProviderDraft>(key: Key, value: ProviderDraft[Key]) => (
    setDraft((current) => ({ ...current, [key]: value }))
  );

  return <section className="min-w-0 space-y-4 xl:col-span-2" data-gallery-provider-form>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "Provider 配置表单", "Provider configuration form")}
    </h2>
    <div className="flex flex-wrap items-center gap-3">
      <UiSegmentedControl density="compact" onChange={setWidth}
        options={["320", "560", "800"].map((value) => ({ value, label: `${value}px` }))}
        title={galleryText(locale, "表单宽度", "Form width")} value={width} />
      <UiButton aria-pressed={fixed} onClick={() => setFixed(!fixed)} variant="outline" data-gallery-provider-fixed>
        {galleryText(locale, "固定端点", "Fixed endpoints")}
      </UiButton>
    </div>
    <div className="flex min-w-0 flex-col gap-4" data-gallery-provider-host data-blur-count={blurCount}
      style={{ maxWidth: Number(width), width: "100%" }}>
      <ProviderSettingsConfigForm builtinEndpointFormats={FORMATS} currentFormat={FORMATS[0]} currentPreset={null}
        detailTitle="Gallery Provider" draft={draft}
        formatOptions={[{ value: "responses", label: "Responses" }, { value: "chat_completions", label: "Chat Completions" }]}
        isEditing={false} onApiFormatChange={(value) => update("api_format", value as ProviderDraft["api_format"])}
        onAuthTokenChange={(value) => update("auth_token", value)} onBaseUrlChange={(value) => update("base_url", value)}
        onFieldBlur={() => setBlurCount((count) => count + 1)} onProviderDisplayNameChange={(value) => update("display_name", value)}
        onProviderKindChange={(value) => update("provider_kind", value as ProviderDraft["provider_kind"])}
        providerKindOptions={[
          { value: "llm", label: galleryText(locale, "语言模型", "Language model") },
          { value: "image_generation", label: galleryText(locale, "图像生成", "Image generation") },
        ]}
        selectedCanManage selectedRecord={null} showProviderShapeControls={!fixed}
        showRuntimeFormatBadge usesBuiltinEndpoint={fixed} />
    </div>
  </section>;
}
