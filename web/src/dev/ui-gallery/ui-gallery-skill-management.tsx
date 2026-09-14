// INPUT: 真实 Skill 详情、导入与来源组件，固定目录数据及本地交互状态。
// OUTPUT: 详情排版与开关、长来源行、凭据保留和导入交互样例。
// POS: 开发专用展示；只更新本地样例，不请求网络或修改真实配置。

import { SkillDetailView } from "@/features/capability/skills/detail/skill-detail-view";
import { useRef, useState } from "react";
import { SkillSourceManagerDialog } from "@/features/capability/skills/external/skill-source-manager-dialog";
import { SkillImportDialog } from "@/features/capability/skills/import/skill-import-dialog";
import type { SkillImportDialogMode } from "@/features/capability/skills/controller/skill-marketplace-controller";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";
import { galleryText } from "./ui-gallery-copy";

const initialSources: ExternalSkillSourceInfo[] = [
  { auth_type: "bearer", credential_configured: true, deletable: true, enabled: true,
    kind: "private_registry", managed_by: "user", name: "Engineering research and design systems",
    sort_order: 0, source_id: "private-gallery", trust: "private",
    url: "https://registry.example.test/organization/engineering/design-systems/skills",
    last_error: "Fixture diagnostic detail is not rendered" },
  { auth_type: "none", credential_configured: false, deletable: false, enabled: true,
    kind: "well_known", managed_by: "system", name: "Public skill catalog", sort_order: 1,
    source_id: "public-gallery", trust: "public", url: "https://skills.example.test" },
];

export function SkillManagementGallery() {
  const { locale } = useI18n();
  const [bindings, setBindings] = useState([
    { agent_id: "main", agent_name: "Nexus", is_main: true, available: true, enabled: true },
    { agent_id: "amy", agent_name: "Amy", is_main: false, available: true, enabled: false },
    { agent_id: "long", agent_name: "Research assistant with a long display name", is_main: false, available: false, enabled: false },
  ]);
  const [sources, setSources] = useState(initialSources);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [mode, setMode] = useState<SkillImportDialogMode | null>(null);
  const [commands, setCommands] = useState<unknown[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const record = (command: unknown) => setCommands((current) => [...current, command]);
  return <section className="col-span-full min-w-0 space-y-3" data-gallery-skill-management>
    <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
      {galleryText(locale, "Skill 导入与来源", "Skill import and sources")}
    </h2>
    <div className="flex flex-wrap gap-2">
      <UiButton onClick={() => setMode("git")} size="sm" variant="surface">{galleryText(locale, "打开 Skill 导入", "Open Skill import")}</UiButton>
      <UiButton onClick={() => setSourcesOpen(true)} size="sm" variant="surface">{galleryText(locale, "打开来源管理", "Open source manager")}</UiButton>
    </div>
    <input accept=".zip" hidden onChange={(event) => record({ type: "file", name: event.currentTarget.files?.[0]?.name })} ref={fileInputRef} type="file" />
    <SkillImportDialog fileInputRef={fileInputRef} importing={false} mode={mode} onClose={() => setMode(null)}
      onImportGit={(url, branch, path) => record({ type: "import-git", url, branch, path })} onSelectMode={setMode} />
    <SkillSourceManagerDialog isOpen={sourcesOpen} loading={false} onClose={() => setSourcesOpen(false)}
      onDelete={(source) => {
        record({ type: "delete", sourceId: source.source_id });
        setSources((current) => current.filter((item) => item.source_id !== source.source_id));
      }}
      onSave={async (source, draft) => {
        record({ type: "save", sourceId: source?.source_id ?? null, name: draft.name, url: draft.url,
          authType: draft.authType, tokenProvided: Boolean(draft.token) });
        return false;
      }}
      onToggle={(source, enabled) => {
        record({ type: "toggle", sourceId: source.source_id, enabled });
        setSources((current) => current.map((item) => item.source_id === source.source_id ? { ...item, enabled } : item));
      }} sources={sources} />
    <div data-gallery-skill-detail>
      <SkillDetailView
        activeAction={null} agentBindings={bindings} agentsLoading={false}
        bindingsFailure={null} busyAgentId={null} toggleFailures={{}}
        onAgentToggle={(binding) => setBindings((current) => current.map((item) =>
          item.agent_id === binding.agent_id ? { ...item, enabled: !item.enabled } : item))}
        onBack={() => {}} onDelete={() => {}} onRetry={() => {}}
        onRetryBindings={() => {}} onUpdate={() => {}}
        snapshot={{ status: "ready", skill: {
          name: "ima-skill", title: "IMA 笔记与知识库",
          description: "连接笔记与知识库，搜索、浏览、创建和编辑笔记。",
          readme_markdown: "# ima-skill\n\nUnified IMA OpenAPI skill. Supports **notes** and **knowledge-base**.\n\n## 使用说明\n\n1. 搜索笔记并读取内容。\n2. 编辑前确认目标笔记。\n\n| 用户意图 | 模块 |\n| --- | --- |\n| 搜索、创建和编辑笔记 | notes |\n| 浏览知识库和上传文件 | knowledge-base |",
          category_key: "productivity", category_name: "Productivity", version: "1.1.8",
          deletable: false, locked: false, has_update: false, scope: "any",
          enabled_agent_count: 1, enabled_for_agent: true, deploy_failures: [], deploy_successes: [],
          import_mode: "copy", last_error: "", origin_kind: "builtin", recommendation: "",
          source_kind: "user_global", source_name: "Nexus", source_ref: "", source_trust: "trusted",
          source_type: "builtin", storage_scope: "user_global", tags: [],
        } }}
      />
    </div>
    <output className="sr-only" data-gallery-skill-commands>{JSON.stringify(commands)}</output>
  </section>;
}
