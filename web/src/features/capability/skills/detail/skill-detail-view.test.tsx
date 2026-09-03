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
});
