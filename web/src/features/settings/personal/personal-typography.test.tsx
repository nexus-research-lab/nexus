// INPUT: Personal 用量与密码 Section 的普通、禁用和摘要状态。
// OUTPUT: 证明整页文本层级和卡片形状由共享语义角色投影。
// POS: Personal 视图合同测试；资料请求和 mutation 由控制器测试负责。

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { PersonalPasswordSection } from "./personal-password-section";
import { PersonalTokenUsageSection } from "./personal-token-usage-section";

function renderWithI18n(children: ReactNode) {
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

describe("Personal settings typography", () => {
  it("uses semantic hierarchy for usage totals, metrics, and supporting facts", () => {
    const { container } = renderWithI18n(
      <PersonalTokenUsageSection usage={undefined} />,
    );

    expect(
      screen.getByText("settings.personal.token_usage_title").className,
    ).toContain("ui-type-section-title");
    expect(container.querySelector(".ui-type-object-title")).toBeTruthy();
    expect(container.querySelectorAll(".ui-type-caption").length).toBeGreaterThan(0);
    expect(container.querySelector("section")?.className).toContain("surface-radius-md");
  });

  it("keeps an unavailable password action explanatory without rendering fields", () => {
    const { container } = renderWithI18n(
      <PersonalPasswordSection
        canChange={false}
        canSubmit={false}
        draft={{ confirmPassword: "", currentPassword: "", newPassword: "" }}
        hasInput={false}
        isSubmitting={false}
        mutationBlocked={false}
        onFieldChange={vi.fn()}
        onSubmit={vi.fn()}
        validationError={null}
      />,
    );

    expect(
      screen.getByText("settings.personal.password_title").className,
    ).toContain("ui-type-section-title");
    expect(
      screen.getByText("settings.personal.password_disabled").className,
    ).toContain("ui-type-metadata");
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(container.querySelector("section")?.className).toContain("surface-radius-md");
  });

  it("uses the shared compact Spinner while changing a password", () => {
    const { container } = renderWithI18n(
      <PersonalPasswordSection
        canChange
        canSubmit={false}
        draft={{ confirmPassword: "new-password", currentPassword: "old-password", newPassword: "new-password" }}
        hasInput
        isSubmitting
        mutationBlocked
        onFieldChange={vi.fn()}
        onSubmit={vi.fn()}
        validationError={null}
      />,
    );

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner?.getAttribute("class")).toContain("h-3.5 w-3.5");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });
});
