// INPUT: 登录状态/提交失败投影及可选的安全状态重读动作。
// OUTPUT: 证明登录页通过 UiInlineNotice 展示恢复事实，并保持动作与下一步互斥。
// POS: LoginAuthPanel 反馈 DOM 合同；失败分类仍由 login-page-model 负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { LoginAuthPanel } from "./login-auth-panel";
import type { LoginRecoveryNotice } from "./login-page-model";

const I18N_VALUE = {
  locale: "zh" as const,
  setLocale: vi.fn(),
  t: (key: string) => key,
};

const UNKNOWN_NOTICE: LoginRecoveryNotice = {
  action: "check_status",
  blocksSubmit: true,
  impact: "login.submit_unknown_impact",
  nextStep: "login.submit_unknown_next_step",
  title: "login.submit_unknown_title",
};

function renderPanel({
  authFailure = null,
  onRefresh = vi.fn(),
  submitFailure = null,
}: {
  authFailure?: LoginRecoveryNotice | null;
  onRefresh?: () => void;
  submitFailure?: LoginRecoveryNotice | null;
}) {
  render(
    <I18N_CONTEXT.Provider value={I18N_VALUE}>
      <LoginAuthPanel
        authFailure={authFailure}
        formMode="password"
        isSubmitting={false}
        onChangePassword={vi.fn()}
        onChangeUsername={vi.fn()}
        onRefresh={onRefresh}
        onSubmit={(event) => event.preventDefault()}
        password=""
        submitFailure={submitFailure}
        username=""
      />
    </I18N_CONTEXT.Provider>,
  );
}

describe("LoginAuthPanel recovery notice", () => {
  it("uses the shared danger notice and delegates status reconciliation", () => {
    const onRefresh = vi.fn();
    renderPanel({ authFailure: UNKNOWN_NOTICE, onRefresh });

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(status.getAttribute("data-inline-notice-width")).toBe("full");
    expect(screen.getByText("login.submit_unknown_title")).toBeTruthy();
    expect(screen.getByText("login.submit_unknown_impact")).toBeTruthy();
    expect(screen.queryByText("login.submit_unknown_next_step")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "login.refresh" }));
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it("keeps the safe next step when no executable recovery action exists", () => {
    renderPanel({
      submitFailure: {
        action: null,
        blocksSubmit: false,
        impact: "login.submit_not_applied_impact",
        nextStep: "login.submit_not_applied_next_step",
        title: "login.submit_failed_title",
      },
    });

    expect(screen.getByText("login.submit_not_applied_impact")).toBeTruthy();
    expect(screen.getByText("login.submit_not_applied_next_step")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "login.refresh" })).toBeNull();
  });
});
