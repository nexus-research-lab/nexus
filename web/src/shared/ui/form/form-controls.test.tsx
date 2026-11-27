// INPUT: Field/Select 关联、原生/业务校验、SearchInput、Checkbox 与选择控件的用户事件。
// OUTPUT: 证明描述/错误归属、最新调用方属性恢复、清除与互斥选择使用真实 DOM/ARIA 合同。
// POS: 表单原语交互测试；业务草稿和网络提交由各 feature 测试负责。

import { render, screen } from "@testing-library/react";
import { type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import {
  UiField,
  UiInput,
} from "@/shared/ui/form/form-control";

function renderWithI18n(children: ReactNode) {
  const messages: Record<string, string> = {
    "common.clear": "清除",
    "common.search": "搜索",
    "common.invalid_field": "字段格式不正确",
    "common.required_field": "请填写此字段",
  };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => messages[key] ?? key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>
  );
  return render(children, { wrapper });
}

describe("form primitives", () => {

  it("names compound fields as groups without assigning their aggregate error to every input", () => {
    const { container } = renderWithI18n(
      <UiField error="Duplicate key" label="Environment variables">
        <UiInput aria-label="Key" />
        <UiInput aria-label="Value" />
      </UiField>,
    );
    const group = screen.getByRole("group", { name: "Environment variables" });
    expect(group.getAttribute("aria-invalid")).toBe("true");
    expect(group.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    expect(container.querySelector("label")).toBeNull();
    for (const input of screen.getAllByRole("textbox")) {
      expect(input.hasAttribute("aria-invalid")).toBe(false);
    }
  });
});
