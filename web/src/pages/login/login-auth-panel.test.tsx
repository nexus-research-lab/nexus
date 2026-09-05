// INPUT: 登录字段、部署禁用状态、提交失败投影及安全状态重读动作。
// OUTPUT: 证明共享表单保留字段、键盘提交、阻塞态与互斥恢复动作。
// POS: LoginAuthPanel 交互 DOM 合同；失败分类及认证事务仍由 model/controller 负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ComponentProps } from "react";
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

type PanelProps = ComponentProps<typeof LoginAuthPanel>;

function renderPanel(props: Partial<PanelProps> = {}) {
  function PanelHarness() {
    const [username, setUsername] = useState(props.username ?? "");
    const [password, setPassword] = useState(props.password ?? "");
    return (
      <LoginAuthPanel
        authFailure={null}
        formMode="password"
        isSubmitting={false}
        onChangePassword={setPassword}
        onChangeUsername={setUsername}
        onRefresh={vi.fn()}
        onSubmit={(event) => event.preventDefault()}
        password={password}
        submitFailure={null}
        username={username}
        {...props}
      />
    );
  }
  return render(
    <I18N_CONTEXT.Provider value={I18N_VALUE}>
      <PanelHarness />
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

describe("LoginAuthPanel shared form", () => {
  it("keeps named credential fields and delegates keyboard submission once", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<PanelProps["onSubmit"]>((event) => event.preventDefault());
    renderPanel({ onSubmit });

    const username = screen.getByRole("textbox", { name: "login.username" });
    const password = screen.getByLabelText("login.password");
    expect(username.getAttribute("autocomplete")).toBe("username");
    expect(password.getAttribute("autocomplete")).toBe("current-password");
    expect(password.getAttribute("type")).toBe("password");

    await user.tab();
    expect(document.activeElement).toBe(username);
    await user.type(username, "owner");
    await user.tab();
    expect(document.activeElement).toBe(password);
    await user.type(password, "password-123{Enter}");

    expect((username as HTMLInputElement).value).toBe("owner");
    expect((password as HTMLInputElement).value).toBe("password-123");
    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it.each([
    { name: "pending submission", props: { isSubmitting: true } },
    { name: "authentication uncertainty", props: { authFailure: UNKNOWN_NOTICE } },
    { name: "submission uncertainty", props: { submitFailure: UNKNOWN_NOTICE } },
  ])("preserves credentials and prevents submission during $name", async ({ props }) => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<PanelProps["onSubmit"]>((event) => event.preventDefault());
    renderPanel({ onSubmit, password: "draft-password", username: "owner", ...props });

    const submit = screen.getByRole("button", { name: /login\.(submit|submitting)$/ });
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByLabelText("login.username") as HTMLInputElement).value).toBe("owner");
    expect((screen.getByLabelText("login.password") as HTMLInputElement).value).toBe("draft-password");
    await user.click(submit);
    await user.click(screen.getByLabelText("login.password"));
    await user.keyboard("{Enter}");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("preserves deployment status and the refresh action when password sign-in is disabled", async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    const onSubmit = vi.fn<PanelProps["onSubmit"]>((event) => event.preventDefault());
    renderPanel({ formMode: "disabled", onRefresh, onSubmit });

    expect(screen.getByRole("heading", { name: "login.disabled_title" })).toBeTruthy();
    expect(screen.getByText("login.disabled_description")).toBeTruthy();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByLabelText("login.password")).toBeNull();
    await user.click(screen.getByRole("button", { name: "login.refresh" }));
    expect(onRefresh).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
