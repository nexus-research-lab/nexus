// INPUT: Real Contacts directory with local, deliberately long Agent identities and filter facts.
// OUTPUT: Responsive grid/list, independent profile/chat/team commands and recoverable no-match states.
// POS: Development-only page fixture; no Agent creation, runtime, API or persistent preferences.

import { useState } from "react";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import type { Agent } from "@/types/agent/agent";

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
  return <section className="min-w-0 xl:col-span-2" data-gallery-contacts>
    <div className="flex h-[680px] min-w-0">
      <ContactsDirectory agents={agents} onCreateAgent={() => record("create")}
        onOpenAgent={(id) => record("profile", id)} onOpenDirectRoom={(id) => record("chat", id)}
        onCreateTeam={(id) => record("team", id)} />
    </div>
    <output className="sr-only" data-gallery-contacts-commands>{JSON.stringify(commands)}</output>
  </section>;
}
