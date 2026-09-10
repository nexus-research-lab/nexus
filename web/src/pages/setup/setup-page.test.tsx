// INPUT: 可提交的首次设置草稿、失败的 owner 创建请求和可重读认证状态。
// OUTPUT: 证明 Setup 提交失败使用共享 danger notice，并保持 alert 播报与只读对账行为。
// POS: SetupPage 反馈 DOM 合同；Control 请求实现与 Auth 状态机在各自模块测试。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AuthStatus } from "@/lib/api/account/auth-api";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SetupPage } from "./setup-page";

const { setupControlOwnerApiMock } = vi.hoisted(() => ({
  setupControlOwnerApiMock: vi.fn(),
}));

vi.mock("@/lib/api/account/control-api", () => ({
  setupControlOwnerApi: setupControlOwnerApiMock,
}));

beforeEach(() => setupControlOwnerApiMock.mockReset());

const SETUP_STATUS: AuthStatus = {
  auth_required: true,
  authenticated: false,
  password_login_enabled: true,
  setup_enabled: true,
  setup_required: true,
  username: null,
};

describe("SetupPage failure notice", () => {
  it("uses the shared assertive danger notice without replaying the mutation", async () => {
    const refreshStatus = vi.fn().mockResolvedValue(SETUP_STATUS);
    setupControlOwnerApiMock.mockRejectedValueOnce(new Error("network unavailable"));

    render(
      <MemoryRouter>
        <I18N_CONTEXT.Provider
          value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
        >
          <AUTH_CONTEXT.Provider
            value={{
              error: null,
              isBootstrapped: true,
              loading: false,
              login: vi.fn(),
              logout: vi.fn(),
              refreshStatus,
              status: SETUP_STATUS,
            }}
          >
            <SetupPage />
          </AUTH_CONTEXT.Provider>
        </I18N_CONTEXT.Provider>
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByLabelText(/^setup\.capability/), {
      target: { value: "a".repeat(32) },
    });
    fireEvent.change(screen.getByLabelText(/^setup\.password/), {
      target: { value: "password" },
    });
    fireEvent.change(screen.getByLabelText(/^setup\.confirm_password/), {
      target: { value: "password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "setup.submit" }));

    const alert = await screen.findByRole("alert");
    expect(alert.getAttribute("aria-live")).toBe("assertive");
    expect(alert.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(alert.getAttribute("data-inline-notice-width")).toBe("full");
    expect(screen.getByText("setup.failed_title")).toBeTruthy();
    expect(screen.getByText("setup.failed_description")).toBeTruthy();

    await waitFor(() => expect(refreshStatus).toHaveBeenCalledOnce());
    expect(setupControlOwnerApiMock).toHaveBeenCalledOnce();
  });
});

it("keeps the draft and duplicate submissions blocked until status reconciliation finishes", async () => {
  let finishRead!: (status: AuthStatus) => void;
  const refreshStatus = vi.fn(() => new Promise<AuthStatus>((resolve) => { finishRead = resolve; }));
  setupControlOwnerApiMock.mockRejectedValueOnce(new Error("unknown outcome"));
  render(<MemoryRouter><I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <AUTH_CONTEXT.Provider value={{ error: null, isBootstrapped: true, loading: false, login: vi.fn(), logout: vi.fn(), refreshStatus, status: SETUP_STATUS }}>
      <SetupPage />
    </AUTH_CONTEXT.Provider>
  </I18N_CONTEXT.Provider></MemoryRouter>);
  const token = screen.getByLabelText(/^setup\.capability/) as HTMLInputElement;
  fireEvent.change(token, { target: { value: "a".repeat(32) } });
  fireEvent.change(screen.getByLabelText(/^setup\.password/), { target: { value: "password" } });
  fireEvent.change(screen.getByLabelText(/^setup\.confirm_password/), { target: { value: "password" } });
  const form = token.closest("form")!;
  fireEvent.submit(form);
  fireEvent.submit(form);
  await waitFor(() => expect(refreshStatus).toHaveBeenCalledOnce());
  expect(setupControlOwnerApiMock).toHaveBeenCalledOnce();
  expect(token.matches(":disabled")).toBe(true);
  fireEvent.submit(form);
  expect(setupControlOwnerApiMock).toHaveBeenCalledOnce();
  finishRead(SETUP_STATUS);
  await waitFor(() => expect(token.matches(":disabled")).toBe(false));
  expect(token.value).toBe("a".repeat(32));
});
