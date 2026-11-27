// INPUT: Desktop/browser font catalogs, denied access and unsupported enumeration.
// OUTPUT: Preset and saved choices remain usable without granting real font permissions.
// POS: Font picker boundary regressions with real shared controls.
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { useState } from "react";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { SettingsFontPicker } from "./settings-font-picker";

const bridge = vi.hoisted(() => ({ available: vi.fn(), fonts: vi.fn() }));
vi.mock("@/lib/desktop-bridge", () => ({ isDesktopBridgeAvailable: bridge.available, getDesktopSystemFonts: bridge.fonts }));
beforeEach(() => {
  bridge.available.mockReturnValue(false);
  bridge.fonts.mockReset();
  vi.stubGlobal("queryLocalFonts", undefined);
});
function Form({ initial = "default" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <SettingsFontPicker value={value} onChange={setValue} /><output aria-label="Selected font">{value}</output>
  </I18N_CONTEXT.Provider>;
}
it("deduplicates the desktop catalog and retains the saved custom font", async () => {
  bridge.available.mockReturnValue(true);
  bridge.fonts.mockResolvedValue({ families: ["Arial", "Arial", "Georgia"] });
  render(<Form initial="My Font" />);
  await waitFor(() => expect(bridge.fonts).toHaveBeenCalledOnce());
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  expect(screen.getAllByRole("option", { name: "Arial" })).toHaveLength(1);
  await userEvent.click(screen.getByRole("option", { name: "Georgia" }));
  expect(screen.getByLabelText("Selected font").textContent).toBe("Georgia");
});

it("keeps presets usable while loading and after an empty catalog", async () => {
  bridge.available.mockReturnValue(true);
  let finish!: (value: { families: string[] }) => void;
  bridge.fonts.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  render(<Form />);
  expect(screen.getByText("common.loading").getAttribute("role")).toBe("status");
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  await userEvent.click(screen.getByRole("option", { name: "settings.reading.default" }));
  expect(screen.getByLabelText("Selected font").textContent).toBe("default");
  await act(async () => finish({ families: [] }));
  expect(screen.queryByText("common.loading")).toBeNull();
  expect(screen.queryByRole("textbox")).toBeNull();
});
it.each(["granted", "prompt"])("only enumerates automatically with %s browser permission", async (state) => {
  const permission = vi.fn().mockResolvedValue({ state });
  vi.stubGlobal("navigator", Object.create(navigator, { permissions: { value: { query: permission } } }));
  const query = vi.fn().mockResolvedValue([{ family: "Arial" }]);
  vi.stubGlobal("queryLocalFonts", query);
  render(<Form />);
  await act(async () => { await Promise.resolve(); });
  expect(permission).toHaveBeenCalledWith({ name: "local-fonts" });
  expect(query).toHaveBeenCalledTimes(state === "granted" ? 1 : 0);
});
