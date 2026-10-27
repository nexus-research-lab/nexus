import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { AuthStatus } from "@/lib/api/account/auth-api";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { TeamCapabilityNotice } from "./team-capability-notice";

const base: AuthStatus = {
  auth_method: "password",
  auth_required: true,
  authenticated: true,
  organization_id: "org",
  password_login_enabled: true,
  username: "lee",
};

function view(status: AuthStatus) {
  return render(
    <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <AUTH_CONTEXT.Provider value={{ error: null, isBootstrapped: true, loading: false, login: vi.fn(), logout: vi.fn(), refreshStatus: vi.fn(), status }}>
        <TeamCapabilityNotice />
      </AUTH_CONTEXT.Provider>
    </I18N_CONTEXT.Provider>,
  );
}

describe("TeamCapabilityNotice", () => {
  it("explains a failed capability probe for an organization account and can be dismissed", () => {
    view({ ...base, multiplayer_unavailable: true });
    expect(screen.getByText("team.capability_unavailable")).toBeTruthy();

    const closeButton = screen.getByLabelText("common.close");
    fireEvent.click(closeButton);
    expect(screen.queryByText("team.capability_unavailable")).toBeNull();
  });

  it.each([
    { multiplayer_enabled: true },
    { multiplayer_enabled: false, multiplayer_unavailable: false },
    { multiplayer_unavailable: true, organization_id: null },
    { multiplayer_unavailable: true, auth_method: "local" },
  ])("stays hidden when the probe is not the problem (%o)", (patch) => {
    view({ ...base, ...patch });
    expect(screen.queryByText("team.capability_unavailable")).toBeNull();
  });
});
