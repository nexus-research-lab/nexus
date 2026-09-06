// INPUT: 当前语言和本地分类、来源、状态及上下文快照夹具。
// OUTPUT: 真实目录筛选、自定义 MCP 动态表单与 DM/Room 上下文指标的可复现交互。
// POS: 开发期消费场景；复用产品组件，无 API 请求或业务写入。

import { useState } from "react";

import { ConnectorsSearchBar, type ConnectorDirectoryMode } from "@/features/capability/connectors/catalog/connectors-search-bar";
import { CustomMCPDialog } from "@/features/capability/connectors/custom/custom-mcp-dialog";
import { UiFilterSelect } from "@/shared/ui/menu/filter-select";
import { SkillsSearchBar } from "@/features/capability/skills/skills-search-bar";
import type { DiscoveryMode } from "@/features/capability/skills/controller/skill-marketplace-controller";
import { ComposerContextUsage } from "@/features/conversation/shared/composer/components/footer/composer-context-usage";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { galleryText } from "./ui-gallery-copy";

export function ProductControlsGallery() {
  const { locale, t } = useI18n();
  const [skillCategory, setSkillCategory] = useState("all");
  const [connectorCategory, setConnectorCategory] = useState("all");
  const [status, setStatus] = useState("all");
  const [query, setQuery] = useState("");
  const [source, setSource] = useState("all");
  const [skillMode, setSkillMode] = useState<DiscoveryMode>("catalog");
  const [connectorMode, setConnectorMode] = useState<ConnectorDirectoryMode>("catalog");
  const [mcpDialogOpen, setMCPDialogOpen] = useState(false);
  const [mcpSaveCount, setMCPSaveCount] = useState(0);
  const usage = { max_tokens: 1_000_000, percentage: 2, total_tokens: 16_900 };

  return <section className="min-w-0 space-y-4 xl:col-span-2" data-gallery-product-controls>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "目录筛选与上下文详情", "Catalog filters and context details")}
    </h2>
    <div data-gallery-filter="skills">
      <SkillsSearchBar activeCategory={skillCategory} catalogQuery={query}
        categories={[{ key: "all", label: t("capability.category_all") }, { key: "writing", label: galleryText(locale, "写作", "Writing") }]}
        discoveryMode={skillMode} externalLoading={false} externalQuery={query} externalSourceId={source}
        externalSources={[{ label: "Community", value: "all" }]} onChangeCategory={setSkillCategory}
        onChangeCatalogQuery={setQuery} onChangeDiscoveryMode={setSkillMode} onChangeExternalQuery={setQuery}
        onChangeExternalSource={setSource} onSubmitExternalSearch={() => undefined} />
    </div>
    <div data-gallery-filter="connectors">
      <ConnectorsSearchBar activeCategory={connectorCategory} categoryKeys={["productivity", "development"]}
        mode={connectorMode} onCategoryChange={setConnectorCategory} onModeChange={setConnectorMode}
        onQueryChange={setQuery} searchQuery={query} />
    </div>
    <div data-gallery-filter="status">
      <UiFilterSelect ariaLabel={t("capability.channels_filter_aria")} label={t("capability.status_label")}
        onChange={setStatus} options={[{ label: t("capability.category_all"), value: "all" },
          { label: galleryText(locale, "已连接", "Connected"), value: "connected" }]} value={status} />
    </div>
    <div data-gallery-custom-mcp data-save-count={mcpSaveCount}>
      <UiButton onClick={() => setMCPDialogOpen(true)} variant="outline">
        {t("capability.custom_mcp_edit_title")}
      </UiButton>
      {mcpDialogOpen ? <CustomMCPDialog
        busy={false}
        onClose={() => setMCPDialogOpen(false)}
        onSave={async () => { setMCPSaveCount((count) => count + 1); setMCPDialogOpen(false); return true; }}
        server={{
          args: ["server.js", "--read-only"], command: "node", configuration_state: "ready",
          connector_id: "custom-mcp:gallery", enabled: true, env: { ACCOUNT: null, TOKEN: null },
          name: "gallery-service", type: "stdio",
        }}
      /> : null}
    </div>
    <div className="flex flex-wrap items-center justify-end gap-6">
      <div className="flex items-center gap-2" data-gallery-context="dm"><span>DM</span><ComposerContextUsage usage={usage} /></div>
      <div className="flex items-center gap-2" data-gallery-context="room"><span>Room</span><ComposerContextUsage items={[
        { agentId: "reader", name: "Reader", usage }, { agentId: "writer", name: "Writer", usage: null },
        { agentId: "reviewer", name: "Reviewer", usage: { ...usage, percentage: 3, total_tokens: 29_500 } },
      ]} usage={null} /></div>
      <div className="flex items-center gap-2" data-gallery-context="room-many"><span>Room · 12</span><ComposerContextUsage items={Array.from({ length: 12 }, (_, index) => ({
        agentId: `member-${index + 1}`, name: `Agent ${index + 1}`, usage,
      }))} usage={null} /></div>
    </div>
  </section>;
}
