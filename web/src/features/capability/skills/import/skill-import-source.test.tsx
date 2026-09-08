// INPUT: Skill 导入模式、草稿、文件入口与模式切换动作。
// OUTPUT: 证明 Git 字段实例隔离、原样输入和共享分段/Panel/Typography/Spinner。
// POS: Skill 导入来源 DOM 合同；实际文件读取与网络提交归 controller。

import { createRef } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillImportSource } from "./skill-import-source";

function view(mode: "git" | "local", importing = false) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <SkillImportSource
        draft={{ branch: "", path: "", url: "" }}
        fileInputRef={createRef<HTMLInputElement>()}
        gitUrlInputRef={createRef<HTMLInputElement>()}
        importing={importing}
        mode={mode}
        onSelectMode={onSelectMode}
        setDraftField={vi.fn()}
      />
    </I18N_CONTEXT.Provider>
  );
}

const onSelectMode = vi.fn();

describe("SkillImportSource", () => {
  it("keeps Git labels in their own instance and forwards exact branch/path input", () => {
    const commands = [vi.fn(), vi.fn()];
    const { container } = render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      {commands.map((setDraftField, index) => <section aria-label={`Import ${index}`} key={index}>
        <SkillImportSource draft={{ branch: "main", path: "skills", url: "https://example.test/repo" }}
          fileInputRef={createRef<HTMLInputElement>()} gitUrlInputRef={createRef<HTMLInputElement>()}
          importing={false} mode="git" onSelectMode={vi.fn()} setDraftField={setDraftField} />
      </section>)}
    </I18N_CONTEXT.Provider>);
    for (const label of container.querySelectorAll<HTMLLabelElement>("label[for]")) {
      expect(label.control).not.toBeNull();
      expect(label.control?.closest("section[aria-label]")).toBe(label.closest("section[aria-label]"));
    }
    const second = within(screen.getByRole("region", { name: "Import 1" }));
    fireEvent.change(second.getByRole("textbox", { name: "capability.skills_import_git_branch" }), { target: { value: " feature/skill " } });
    fireEvent.change(second.getByRole("textbox", { name: "capability.skills_import_git_path" }), { target: { value: "  skill folder/说明  " } });
    expect(commands[1].mock.calls).toEqual([["branch", " feature/skill "], ["path", "  skill folder/说明  "]]);
    expect(commands[0]).not.toHaveBeenCalled();
  });

  it("uses the shared segmented owner and dispatches mode changes", async () => {
    onSelectMode.mockClear();
    const user = userEvent.setup();
    render(view("git"));

    const modes = screen.getByRole("group", {
      name: "capability.skills_import_title",
    });
    expect(modes.className).toContain("segmented-control");
    expect(screen.getByRole("button", {
      name: "capability.skills_import_mode_git",
    }).getAttribute("aria-pressed")).toBe("true");

    await user.click(screen.getByRole("button", {
      name: "capability.skills_import_mode_local",
    }));
    expect(onSelectMode).toHaveBeenCalledWith("local");
  });

  it("renders local import through shared panel, typography, and loading recipes", () => {
    const { container } = render(view("local", true));

    expect(screen.getByText("capability.skills_import_zip_title").className)
      .toContain("ui-type-section-title");
    expect(container.querySelector("section.surface-radius-md")).toBeTruthy();
    expect(screen.getByRole("button", {
      name: /capability.skills_importing/,
    }).querySelector("svg")?.className.baseVal).toContain("motion-reduce:animate-none");
  });
});
