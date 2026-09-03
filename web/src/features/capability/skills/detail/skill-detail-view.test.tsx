// INPUT: Skill 详情加载态与稳定的本地化上下文。
// OUTPUT: 证明详情返回动作和资源状态使用共享 Button、Typography 与 State Block。
// POS: Skill 详情视图合同；请求代次、mutation 和展示投影由 controller/model 负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillDetailView } from "./skill-detail-view";

describe("SkillDetailView", () => {
  it("renders loading detail chrome through shared UI owners", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SkillDetailView
          activeAction={null}
          agentBindings={[]}
          agentsLoading={false}
          bindingsFailure={null}
          busyAgentId={null}
          onAgentToggle={vi.fn()}
          onBack={vi.fn()}
          onDelete={vi.fn()}
          onRetry={vi.fn()}
          onRetryBindings={vi.fn()}
          onUpdate={vi.fn()}
          snapshot={{ skill: null, status: "loading" }}
          toggleFailures={{}}
        />
      </I18N_CONTEXT.Provider>,
    );

    const backButton = screen.getByRole("button", {
      name: "capability.skills_detail_back",
    });
    expect(backButton.className).toContain("radius-control-sm");
    expect(backButton.className).toContain("ui-type-metadata");
    expect(screen.getByRole("heading", {
      name: "capability.skills_detail_loading",
    }).className).toContain("ui-type-object-title");
    expect(container.querySelector(".surface-radius-md")).toBeTruthy();
  });

  it("places Agent configuration in the shared detail rail beside the reading column", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SkillDetailView
          activeAction={null}
          agentBindings={[{
            agent_id: "agent-1",
            agent_name: "Nexus",
            available: true,
            enabled: true,
            is_main: true,
          }]}
          agentsLoading={false}
          bindingsFailure={null}
          busyAgentId={null}
          onAgentToggle={vi.fn()}
          onBack={vi.fn()}
          onDelete={vi.fn()}
          onRetry={vi.fn()}
          onRetryBindings={vi.fn()}
          onUpdate={vi.fn()}
          snapshot={{
            skill: {
              category_key: "productivity",
              category_name: "Productivity",
              deletable: false,
              deploy_failures: [],
              deploy_successes: [],
              description: "A reusable Skill.",
              enabled_agent_count: 1,
              enabled_for_agent: true,
              has_update: false,
              import_mode: "copy",
              last_error: "",
              locked: false,
              name: "sample-skill",
              origin_kind: "builtin",
              readme_markdown: "# Usage\n\nFollow these instructions.",
              recommendation: "",
              scope: "any",
              source_kind: "user_global",
              source_name: "Nexus",
              source_ref: "",
              source_trust: "trusted",
              source_type: "builtin",
              storage_scope: "user_global",
              tags: [],
              title: "Sample Skill",
              version: "1.0.0",
            },
            status: "ready",
          }}
          toggleFailures={{}}
        />
      </I18N_CONTEXT.Provider>,
    );

    const aside = container.querySelector("[data-slot='capability-detail-aside']");
    const main = container.querySelector("[data-slot='capability-detail-main']");

    expect(aside?.contains(screen.getByRole("heading", {
      name: "capability.skills_detail_agent_scope",
    }))).toBe(true);
    expect(main?.contains(screen.getByRole("heading", {
      name: "capability.skills_detail_description",
    }))).toBe(true);
    expect(aside?.compareDocumentPosition(main as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });
});
