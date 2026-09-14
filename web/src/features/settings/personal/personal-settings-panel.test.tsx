// INPUT: Personal profile loading and snapshot availability.
// OUTPUT: Initial load state and retained successful content without fake empty profiles.
// POS: Personal page composition regression; no account API calls.
import { render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { PersonalSettingsPanel } from "./personal-settings-panel";

const state = vi.hoisted(() => ({ profile: { isLoading: true, value: null as object | null } }));
vi.mock("./use-personal-settings-controller", () => ({ usePersonalSettingsController: () => ({
  profile: state.profile, avatar: {}, password: {}, feedback: { value: null },
}) }));
vi.mock("./personal-profile-section", () => ({ PersonalProfileSection: () => <div>Profile content</div> }));
vi.mock("./personal-token-usage-section", () => ({ PersonalTokenUsageSection: () => <div>Usage content</div> }));
vi.mock("./personal-password-section", () => ({ PersonalPasswordSection: () => <div>Password content</div> }));
beforeEach(() => { state.profile = { isLoading: true, value: null }; });

it("names the first load and does not render empty profile sections after failure", () => {
  const view = render(<I18nProvider><PersonalSettingsPanel /></I18nProvider>);
  expect(screen.getByRole("status").getAttribute("aria-label")).toBeTruthy();
  expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
  state.profile = { isLoading: false, value: null };
  view.rerender(<I18nProvider><PersonalSettingsPanel /></I18nProvider>);
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.queryByText("Profile content")).toBeNull();
  expect(screen.queryByText("Usage content")).toBeNull();
  expect(screen.queryByText("Password content")).toBeNull();
});

it("keeps successful sections mounted during refresh", () => {
  state.profile = { isLoading: false, value: {} };
  const view = render(<I18nProvider><PersonalSettingsPanel /></I18nProvider>);
  const content = screen.getByText("Profile content");
  state.profile = { ...state.profile, isLoading: true };
  view.rerender(<I18nProvider><PersonalSettingsPanel /></I18nProvider>);
  expect(screen.getByText("Profile content")).toBe(content);
  expect(screen.getByText("Usage content")).toBeTruthy();
  expect(screen.getByText("Password content")).toBeTruthy();
  expect(screen.queryByRole("status")).toBeNull();
});
