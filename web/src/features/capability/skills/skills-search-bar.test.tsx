// INPUT: Skill 目录/社区模式、查询、来源和筛选命令。
// OUTPUT: 证明页面模式切换与搜索动作复用共享控件，并保持点击和 Enter 搜索行为。
// POS: Skill 搜索工具区 DOM 合同；查询竞态与外部来源状态归 controller。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillsSearchBar } from "./skills-search-bar";

describe("SkillsSearchBar", () => {
  it("uses shared page tabs and a text search action while preserving external search", async () => {
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
    expect(modes.className).toContain("w-fit");
    expect(modes.className).not.toContain("segmented-control");
    const externalMode = screen.getByRole("button", {
      name: "capability.skills_tab_external",
    });
    expect(externalMode.getAttribute("aria-pressed")).toBe("true");
    expect(externalMode.className).toContain("border-(--text-strong)");
    expect(externalMode.className).toContain("whitespace-nowrap");

    await user.click(screen.getByRole("button", { name: "capability.skills_tab_catalog" }));
    expect(onChangeDiscoveryMode).toHaveBeenCalledWith("catalog");

    const searchAction = screen.getByRole("button", {
      name: "capability.skills_tour_search_title",
    });
    expect(searchAction.textContent).toBe("capability.skills_tour_search_title");
    expect(searchAction.querySelector("svg")).toBeNull();
    await user.click(searchAction);
    fireEvent.keyDown(screen.getByRole("searchbox"), { key: "Enter" });
    expect(onSubmitExternalSearch).toHaveBeenCalledTimes(2);
    const input = screen.getByRole("searchbox");
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });
    fireEvent.compositionStart(input);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSubmitExternalSearch).toHaveBeenCalledTimes(2);
    fireEvent.compositionEnd(input);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSubmitExternalSearch).toHaveBeenCalledTimes(3);
  });
});

it.each([{ query: "a", loading: false }, { query: "valid", loading: true }])("keeps Enter aligned with disabled search ($query, $loading)", ({ query, loading }) => {
  const submit = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <SkillsSearchBar activeCategory="" catalogQuery="" categories={[]} discoveryMode="external"
      externalLoading={loading} externalQuery={query} externalSourceId="" externalSources={[]}
      onChangeCategory={vi.fn()} onChangeCatalogQuery={vi.fn()} onChangeDiscoveryMode={vi.fn()}
      onChangeExternalQuery={vi.fn()} onChangeExternalSource={vi.fn()} onSubmitExternalSearch={submit} />
  </I18N_CONTEXT.Provider>);
  expect((screen.getByRole("button", { name: "capability.skills_tour_search_title" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.keyDown(screen.getByRole("searchbox"), { key: "Enter" });
  expect(submit).not.toHaveBeenCalled();
});
