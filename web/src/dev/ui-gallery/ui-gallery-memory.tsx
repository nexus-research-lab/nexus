// INPUT: Local Memory snapshot, type/query selection and simulated editor state.
// OUTPUT: Real catalog/header at their container breakpoints, with no resource hook or file command.
// POS: Development-only Memory navigation and typography fixture; content is synthetic.

import { useState } from "react";
import { AgentMemoryCatalog } from "@/features/memory/catalog/agent-memory-catalog";
import { projectMemoryCatalog, type MemoryFilter } from "@/features/memory/catalog/memory-catalog-model";
import { MemoryDocumentHeader } from "@/features/memory/document/memory-document-header";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import type { MemorySnapshot } from "@/types/memory/memory";
import "@/features/memory/memory-view.css";

const memorySnapshot: MemorySnapshot = { layout: "topic", truncated: false, documents: [{
  kind: "topic", indexed: true, modified_at: "2026-09-06T04:00:00Z", size: 128,
  title: "project-reference.md", path: "memory/project-reference.md", type: "reference",
  description: "跨区域项目资料与长期协作约定 · Cross-region project reference and long-term collaboration agreements",
}] };

export function MemoryGallery() {
  const { locale } = useI18n();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<MemoryFilter>("all");
  const [empty, setEmpty] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [writing, setWriting] = useState(true);
  const [commands, setCommands] = useState<string[]>([]);
  const selected = memorySnapshot.documents[0];
  const projected = projectMemoryCatalog(empty ? { layout: "empty", truncated: false, documents: [] } : memorySnapshot,
    selected.path, filter, query);
  const record = (command: string) => setCommands((current) => [...current, command]);
  return <section className="min-w-0 space-y-3 xl:col-span-2" data-gallery-memory>
    <UiButton aria-pressed={empty} onClick={() => { setEmpty((current) => !current); setOpen(false); }}>
      Toggle empty memory
    </UiButton>
    <UiButton aria-pressed={writing} onClick={() => setWriting((current) => !current)}>
      Toggle runtime writing
    </UiButton>
    <div className="nexus-memory-view flex h-[520px] min-w-0 flex-col" data-document-open={open ? "true" : "false"}>
      <div className="nexus-memory-layout min-h-0 min-w-0 flex-1">
        <AgentMemoryCatalog {...projected} filter={filter} onFilterChange={setFilter} onQueryChange={setQuery}
          onRefresh={() => record("refresh")} onSelectDocument={(path) => { record(path); setOpen(true); }}
          query={query} refreshing={false} />
        <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col">
          <MemoryDocumentHeader controller={{
            cancelEditing: () => setEditing(false), dirty: editing, editing, isReconciling: false, isSaving: false,
            revision: "fixture", save: async () => { record("save"); setEditing(false); }, saveBlocked: false,
            startEditing: () => setEditing(true),
          }} deleteBusy={false} deleting={false} document={selected} locale={locale}
            onBack={() => setOpen(false)} onDelete={() => record("delete")} runtimeWriting={writing} />
          <div className="soft-scrollbar min-h-0 flex-1 overflow-auto">
            <UiMarkdownContent className="nexus-memory-document-content py-4" content="Fixture memory content · 记忆正文示例。" />
          </div>
        </div>
      </div>
    </div>
    <output className="sr-only" data-gallery-memory-commands>{JSON.stringify(commands)}</output>
  </section>;
}
