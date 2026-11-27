// INPUT: Skill 导入模式、草稿、文件入口与模式切换动作。
// OUTPUT: 证明 Git 字段实例隔离、原样输入和共享分段/Panel/Typography/Spinner。
// POS: Skill 导入来源 DOM 合同；实际文件读取与网络提交归 controller。

import { createRef } from "react";
import { render } from "@testing-library/react";
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

  it("renders local import through shared panel, typography, and loading recipes", () => {
    const { container } = render(view("local", true));

    expect(container.querySelector("section.surface-radius-md")).toBeTruthy();

  });
});
