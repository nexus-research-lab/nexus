// INPUT: 外部 Skill 查询阶段、结果与来源筛选状态。
// OUTPUT: 证明加载和空结果都通过共享 ResourceState 呈现。
// POS: 外部 Skill 结果 DOM 合同；分组排序与请求竞态归 model/controller。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillsExternalResults } from "./skills-external-results";

function view(loading: boolean) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <SkillsExternalResults
        busyExternalKeys={new Set()}
        importedExternalSources={new Map()}
        loading={loading}
        onImport={vi.fn()}
        onPreview={vi.fn()}
        onSelectSource={vi.fn()}
        results={[]}
        selectedSourceKey={null}
        sources={[]}
        sourceStatuses={[]}
        submittedQuery="agent"
      />
    </I18N_CONTEXT.Provider>
  );
}

const configuredSource = {
  auth_type: "none",
  credential_configured: false,
  deletable: true,
  enabled: true,
  kind: "github",
  managed_by: "user",
  name: "Team source",
  sort_order: 0,
  source_id: "team-source",
  trust: "private",
  url: "https://example.com/team",
};

describe("SkillsExternalResults", () => {
  it("projects loading and empty search stages through the shared state owner", () => {
    const { rerender } = render(view(true));

    expect(screen.getByRole("status").getAttribute("data-resource-state"))
      .toBe("loading");
    expect(screen.getByText("capability.skills_external_loading")).toBeTruthy();

    rerender(view(false));
    expect(screen.getByRole("status").getAttribute("data-resource-state"))
      .toBe("empty");
    expect(screen.getByText("capability.skills_external_empty")).toBeTruthy();
  });

  it("uses accessible shared choices for source filters", async () => {
    const user = userEvent.setup();
    const onSelectSource = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SkillsExternalResults
          busyExternalKeys={new Set()}
          importedExternalSources={new Map()}
          loading={false}
          onImport={vi.fn()}
          onPreview={vi.fn()}
          onSelectSource={onSelectSource}
          results={[]}
          selectedSourceKey={null}
          sources={[configuredSource]}
          sourceStatuses={[]}
          submittedQuery="agent"
        />
      </I18N_CONTEXT.Provider>,
    );

    const allSources = screen.getByRole("button", {
      name: /capability\.skills_external_all_sources/,
    });
    const teamSource = screen.getByRole("button", { name: /Team source/ });
    expect(allSources.getAttribute("aria-pressed")).toBe("true");
    expect(teamSource.getAttribute("aria-pressed")).toBe("false");
    await user.click(teamSource);
    expect(onSelectSource).toHaveBeenCalledWith("team-source");
  });
});
