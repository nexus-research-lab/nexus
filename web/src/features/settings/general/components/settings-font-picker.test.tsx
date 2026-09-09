// INPUT: Desktop/browser font catalogs, denied access and unsupported enumeration.
// OUTPUT: Preset/custom choices remain usable without granting real font permissions.
// POS: Font picker boundary regressions with real shared controls.
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { StrictMode, useState } from "react";
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

it("keeps presets usable while loading and provides input for an empty catalog", async () => {
  bridge.available.mockReturnValue(true);
  let finish!: (value: { families: string[] }) => void;
  bridge.fonts.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  render(<Form />);
  expect(screen.getByText("common.loading").getAttribute("role")).toBe("status");
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  await userEvent.click(screen.getByRole("option", { name: "settings.reading.serif" }));
  expect(screen.getByLabelText("Selected font").textContent).toBe("serif");
  await act(async () => finish({ families: [] }));
  expect(screen.queryByText("common.loading")).toBeNull();
  expect(screen.getByRole("textbox", { name: "settings.reading.custom_font" })).toBeTruthy();
});
it("retries desktop failures on reopening without discarding the manual field", async () => {
  bridge.available.mockReturnValue(true);
  bridge.fonts.mockRejectedValueOnce(new Error("offline"));
  let finish!: (value: { families: string[] }) => void;
  bridge.fonts.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  render(<Form initial="Saved Font" />);
  const input = await screen.findByRole("textbox", { name: "settings.reading.custom_font" });
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  expect(bridge.fonts).toHaveBeenCalledTimes(2);
  expect(screen.getByRole("textbox", { name: "settings.reading.custom_font" })).toBe(input);
  await act(async () => finish({ families: ["Arial"] }));
  expect(screen.queryByText("settings.reading.font_error")).toBeNull();
  expect(screen.getByLabelText("Selected font").textContent).toBe("Saved Font");
});
it("ignores stale desktop responses after StrictMode effect cleanup", async () => {
  bridge.available.mockReturnValue(true);
  const finish: Array<(value: { families: string[] }) => void> = [];
  bridge.fonts.mockImplementation(() => new Promise((resolve) => { finish.push(resolve); }));
  render(<StrictMode><Form /></StrictMode>);
  expect(finish).toHaveLength(2);
  await act(async () => finish[1]({ families: ["Current Font"] }));
  await act(async () => finish[0]({ families: ["Stale Font"] }));
  await userEvent.click(screen.getByRole("button", { name: "settings.reading.font" }));
  expect(screen.getByRole("option", { name: "Current Font" })).toBeTruthy();
  expect(screen.queryByRole("option", { name: "Stale Font" })).toBeNull();
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
