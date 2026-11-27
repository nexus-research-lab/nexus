// INPUT: 外观设置中的侧栏天空单选卡片。
// OUTPUT: 点击与方向键都即时写入偏好，选中态与 roving tabindex 同步。
// POS: 侧栏天空选择器交互回归。
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { useSidebarSkyPreference } from "@/shared/theme/sidebar-sky-preference";

import { SettingsSidebarSkyPicker } from "./settings-sidebar-sky-picker";

beforeEach(() => useSidebarSkyPreference.setState({ preference: "auto" }));

it("offers follow-time, four fixed skies and off, and selects on click", () => {
  render(<><h3 id="sky">sky</h3><SettingsSidebarSkyPicker labelledBy="sky" /></>, { wrapper: I18nProvider });
  const radios = screen.getAllByRole("radio");
  expect(radios).toHaveLength(6);
  expect(radios[0].getAttribute("aria-checked")).toBe("true");

  fireEvent.click(radios[5]);
  expect(useSidebarSkyPreference.getState().preference).toBe("off");
  expect(radios[5].getAttribute("aria-checked")).toBe("true");
  expect(radios[5].tabIndex).toBe(0);
  expect(radios[0].tabIndex).toBe(-1);
});

it("moves the selection with arrow keys and wraps around", () => {
  render(<SettingsSidebarSkyPicker labelledBy="sky" />, { wrapper: I18nProvider });
  const radios = screen.getAllByRole("radio");
  fireEvent.keyDown(radios[0], { key: "ArrowRight" });
  expect(useSidebarSkyPreference.getState().preference).toBe("dawn");
  expect(document.activeElement).toBe(radios[1]);
  fireEvent.keyDown(radios[1], { key: "ArrowLeft" });
  fireEvent.keyDown(radios[0], { key: "ArrowLeft" });
  expect(useSidebarSkyPreference.getState().preference).toBe("off");
});
