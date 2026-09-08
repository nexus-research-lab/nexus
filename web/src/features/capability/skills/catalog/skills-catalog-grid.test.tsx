// INPUT: Skill 目录加载态与空分组结果。
// OUTPUT: 证明目录的等待和无结果都使用共享 ResourceState 语义与本地化说明。
// POS: Skill catalog 状态 DOM 合同；目录请求和筛选归 controller/model。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillsCatalogGrid } from "./skills-catalog-grid";

function view(loading: boolean) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <SkillsCatalogGrid
        busySkillNames={new Set()}
        groupedSkills={[]}
        loading={loading}
        onDeleteSkill={vi.fn()}
        onOpenSkill={vi.fn()}
      />
    </I18N_CONTEXT.Provider>
  );
}

describe("SkillsCatalogGrid", () => {
  it("projects loading and empty results through the shared state owner", () => {
    const { rerender } = render(view(true));

    expect(screen.getByRole("status").getAttribute("data-resource-state"))
      .toBe("loading");
    expect(screen.getByText("capability.skills_loading")).toBeTruthy();

    rerender(view(false));
    expect(screen.getByRole("status").getAttribute("data-resource-state"))
      .toBe("empty");
    expect(screen.getByText("capability.skills_empty_description").className)
      .toContain("ui-type-supporting");
  });
});
