// INPUT: Real Contacts directory with local, deliberately long Agent identities and filter facts.
// OUTPUT: Responsive grid/list, communication search and pending friend dialog fixtures with local command records.
// POS: Development-only page fixture; no Agent creation, runtime, API or persistent preferences.

import { useEffect, useRef, useState } from "react";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { AgentCommunicationDirectory } from "@/features/contacts/agent-communication-directory";
import { UiButton } from "@/shared/ui/button/button";
import type { Agent, AgentContact } from "@/types/agent/agent";

const agents: Agent[] = [
  { agent_id: "research", name: "Research and product planning · 跨产品研究规划", avatar: null,
    business_tags: ["Research and writing", "Product decisions", "Additional tag"],
    created_at: 1, description: "Research questions, compare evidence and prepare a product decision. 研究问题、比较证据并整理产品决策。",
    status: "idle", workspace_path: "/workspace/research", skills_count: 12,
    options: { provider: "custom-research-model-provider-with-a-long-name", permission_mode: "acceptEdits", allowed_tools: ["Read", "Write"] } },
  { agent_id: "writer", name: "Writer · 写作者", avatar: null, business_tags: ["Writing"],
    created_at: 2, description: "", status: "idle", workspace_path: "/workspace/writer", options: {} },
  { agent_id: "operations", name: "Operations · 运营", avatar: null, business_tags: ["Operations"],
    created_at: 3, description: "Service operations and automation", status: "idle", workspace_path: "/workspace/operations",
    options: { provider: "anthropic", permission_mode: "default" } },
];

export function ContactsGallery() {
  const [commands, setCommands] = useState<string[]>([]);
  const record = (kind: string, id?: string) => setCommands((current) => [...current, id ? `${kind}:${id}` : kind]);
  return <><section className="min-w-0 xl:col-span-2" data-gallery-contacts>
    <div className="flex h-[680px] min-w-0">
      <ContactsDirectory agents={agents} onCreateAgent={() => record("create")}
        onOpenAgent={(id) => record("profile", id)} onOpenDirectRoom={(id) => record("chat", id)}
        onCreateTeam={(id) => record("team", id)} />
    </div>
    <output className="sr-only" data-gallery-contacts-commands>{JSON.stringify(commands)}</output>
  </section><CommunicationDirectoryGallery /></>;
}

const communicationContacts: AgentContact[] = [{
  id: "operations-contact", contact_agent_id: "operations", owner_agent_id: "research",
  alias: "Operations partner · 跨区域运营协作伙伴", name: "Operations · 运营",
  created_at: "2026-09-06", updated_at: "2026-09-06",
}];

function CommunicationDirectoryGallery() {
  const [commands, setCommands] = useState<string[]>([]);
  const [stale, setStale] = useState(false);
  const [pendingAgentId, setPendingAgentId] = useState<string | null>(null);
  const finishRef = useRef<((result: boolean) => void) | null>(null);
  // Browser harness signals the mocked response; no request is sent to product services.
  useEffect(() => {
    const finish = () => {
      setPendingAgentId(null);
      finishRef.current?.(false);
      finishRef.current = null;
    };
    window.addEventListener("nexus-gallery-contact-add-result", finish);
    return () => {
      window.removeEventListener("nexus-gallery-contact-add-result", finish);
      finishRef.current?.(false);
    };
  }, []);
  return <section className="min-w-0 space-y-3 xl:col-span-2" data-gallery-contact-communication>
    <UiButton aria-pressed={stale} onClick={() => setStale((current) => !current)}>Toggle stale contacts</UiButton>
    <div className="grid h-[420px] w-full max-w-72">
      <AgentCommunicationDirectory
        agent={agents[0]} agents={agents} contacts={communicationContacts}
        directoryFailure={stale ? { kind: "directory", stale: true } : null} isDirectoryLoading={false}
        onAddContact={(id, alias) => {
          setCommands((current) => [...current, `add:${id}:${alias}`]);
          setPendingAgentId(id);
          return new Promise<boolean>((resolve) => { finishRef.current = resolve; });
        }}
        onRefresh={() => setStale(false)} onSelectContact={(id) => setCommands((current) => [...current, `select:${id}`])}
        pendingAgentId={pendingAgentId} selectedContactId={null}
      />
    </div>
    <output className="sr-only" data-gallery-communication-commands>{JSON.stringify(commands)}</output>
  </section>;
}
