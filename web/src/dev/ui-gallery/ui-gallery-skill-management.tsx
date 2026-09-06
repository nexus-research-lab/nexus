// INPUT: Real Skill import/source dialogs with fixed local catalog facts and draft commands.
// OUTPUT: Long source rows, credential-preserving editing and Git/local import interaction fixtures.
// POS: Development-only composition; records local commands without network, persistence or token values.

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
  const [sources, setSources] = useState(initialSources);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [mode, setMode] = useState<SkillImportDialogMode | null>(null);
  const [commands, setCommands] = useState<unknown[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const record = (command: unknown) => setCommands((current) => [...current, command]);
  return <section className="min-w-0 space-y-3" data-gallery-skill-management>
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
    <output className="sr-only" data-gallery-skill-commands>{JSON.stringify(commands)}</output>
  </section>;
}
