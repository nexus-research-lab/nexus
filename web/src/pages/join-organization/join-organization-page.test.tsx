import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { JoinOrganizationPage } from "./join-organization-page";

const api = vi.hoisted(() => ({ preview: vi.fn(), accept: vi.fn(), refresh: vi.fn() }));
const auth = vi.hoisted(() => ({ signedIn: false }));
vi.mock("@/lib/api/account/control-api", () => ({
  previewControlOrganizationInvitationApi: api.preview,
  acceptControlOrganizationInvitationApi: api.accept,
}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({ ...await importOriginal<typeof import("@/shared/auth/auth-context")>(), useAuth: () => ({ refreshStatus: api.refresh, status: auth.signedIn ? { authenticated:true, auth_method:"password", username:"existing" } : null }) }));

beforeEach(() => {
  auth.signedIn = false;
  vi.resetAllMocks();
  api.preview.mockResolvedValue({ organization_name: "Research", role: "member", expires_at: "2099-01-01T00:00:00Z" });
  api.accept.mockResolvedValue({ authenticated: true });
  api.refresh.mockResolvedValue({ authenticated: true });
});

it("已有远程账号只确认加入，不重新填写或提交密码", async () => {
  auth.signedIn = true;
  render(<MemoryRouter initialEntries={["/join/token-1"]}><I18nProvider><Routes><Route element={<JoinOrganizationPage />} path="/join/:token" /></Routes></I18nProvider></MemoryRouter>);
  await screen.findByText("existing");
  expect(screen.queryByLabelText(/初始密码|Initial password/)).toBeNull();
  fireEvent.click(screen.getByRole("button",{name:/^加入组织$|^Join organization$/}));
  await waitFor(() => expect(api.accept).toHaveBeenCalledWith("token-1",{}));
});

it("受邀者自设账号后接受组织邀请", async () => {
  render(
    <MemoryRouter initialEntries={["/join/token-1"]}>
      <I18nProvider>
        <Routes><Route element={<JoinOrganizationPage />} path="/join/:token" /></Routes>
      </I18nProvider>
    </MemoryRouter>,
  );
  await screen.findByText(/Research/);
  fireEvent.change(screen.getByLabelText(/用户名|Username/), { target: { value: "member" } });
  fireEvent.change(screen.getByLabelText(/初始密码|Initial password/), { target: { value: "password-123" } });
  fireEvent.change(screen.getByLabelText(/确认密码|Confirm password/), { target: { value: "password-123" } });
  fireEvent.click(screen.getByRole("button", { name: /创建账号并加入|Create account and join/ }));
  await waitFor(() => expect(api.accept).toHaveBeenCalledWith("token-1", {
    username: "member", display_name: "", password: "password-123",
  }));
  expect(api.refresh).toHaveBeenCalledOnce();
});
