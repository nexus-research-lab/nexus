// INPUT: Skill 目录/社区模式、查询、来源和筛选命令。
// OUTPUT: 证明页面模式切换与搜索动作复用共享控件，并保持点击和 Enter 搜索行为。
// POS: Skill 搜索工具区 DOM 合同；查询竞态与外部来源状态归 controller。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillsSearchBar } from "./skills-search-bar";

describe("SkillsSearchBar", () => {
  it("uses shared page tabs and icon actions while preserving external search", async () => {
    const user = userEvent.setup();
    const onChangeDiscoveryMode = vi.fn();
    const onSubmitExternalSearch = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SkillsSearchBar
          activeCategory=""
          catalogQuery=""
          categories={[]}
          discoveryMode="external"
          externalLoading={false}
          externalQuery="agent skill"
          externalSourceId=""
          externalSources={[{ label: "全部来源", value: "" }]}
          onChangeCategory={vi.fn()}
          onChangeCatalogQuery={vi.fn()}
          onChangeDiscoveryMode={onChangeDiscoveryMode}
          onChangeExternalQuery={vi.fn()}
          onChangeExternalSource={vi.fn()}
          onSubmitExternalSearch={onSubmitExternalSearch}
        />
      </I18N_CONTEXT.Provider>,
    );

    const modes = screen.getByRole("group", {
      name: "capability.skills_tour_modes_title",
    });
    expect(modes.querySelector(".ui-navigation-tab")).not.toBeNull();
    expect(modes.className).not.toContain("segmented-control");
    expect(screen.getByRole("button", { name: "capability.skills_tab_external" })
      .getAttribute("aria-pressed")).toBe("true");

    await user.click(screen.getByRole("button", { name: "capability.skills_tab_catalog" }));
    expect(onChangeDiscoveryMode).toHaveBeenCalledWith("catalog");

    const searchAction = screen.getByRole("button", {
      name: "capability.skills_tour_search_title",
    });
    expect(searchAction.className).toContain("radius-control-sm");
    await user.click(searchAction);
    fireEvent.keyDown(screen.getByRole("searchbox"), { key: "Enter" });
    expect(onSubmitExternalSearch).toHaveBeenCalledTimes(2);
  });
});
