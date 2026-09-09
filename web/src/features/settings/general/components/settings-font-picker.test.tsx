// INPUT: Desktop/browser font catalogs, denied access and unsupported enumeration.
// OUTPUT: Preset/custom choices remain usable without granting real font permissions.
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
it("allows manual font input when browser enumeration is unsupported", async () => {
  render(<Form />);
  await userEvent.type(screen.getByRole("textbox", { name: "settings.reading.custom_font" }), "Georgia");
  expect(screen.getByLabelText("Selected font").textContent).toBe("Georgia");
  expect(bridge.fonts).not.toHaveBeenCalled();
});
it("preserves a custom value and exposes manual recovery after desktop failure", async () => {
  bridge.available.mockReturnValue(true);
  bridge.fonts.mockRejectedValue(new Error("native unavailable"));
  render(<Form initial="My Font" />);
  const input = await screen.findByRole("textbox", { name: "settings.reading.custom_font" });
  expect((input as HTMLInputElement).value).toBe("My Font");
  await userEvent.clear(input);
  await userEvent.type(input, "Georgia");
  expect(screen.getByLabelText("Selected font").textContent).toBe("Georgia");
  expect(screen.getByText("settings.reading.font_error").getAttribute("role")).toBe("status");
  expect(screen.queryByText("native unavailable")).toBeNull();
});
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
it("requests browser fonts on opening and falls back after denied access", async () => {
  const query = vi.fn().mockRejectedValue(new Error("permission denied"));
  vi.stubGlobal("queryLocalFonts", query);
  render(<Form />);
  expect(query).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  expect(query).toHaveBeenCalledOnce();
  expect(await screen.findByRole("textbox", { name: "settings.reading.custom_font" })).toBeTruthy();
});
it("does not duplicate a pending browser request when reopened", async () => {
  let resolve!: (fonts: { family: string }[]) => void;
  const query = vi.fn(() => new Promise<{ family: string }[]>((done) => { resolve = done; }));
  vi.stubGlobal("queryLocalFonts", query);
  render(<Form />);
  const trigger = screen.getByRole("button", { name: "settings.reading.font" });
  await userEvent.click(trigger);
  await userEvent.click(trigger);
  await userEvent.click(trigger);
  expect(query).toHaveBeenCalledOnce();
  await act(async () => resolve([{ family: "Arial" }]));
  expect(screen.getByRole("option", { name: "Arial" })).toBeTruthy();
});
