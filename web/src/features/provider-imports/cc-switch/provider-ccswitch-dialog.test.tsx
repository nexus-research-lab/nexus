// INPUT: CC Switch preview and mutation receipts.
// OUTPUT: Form submission respects pending commits and synchronous single-flight.
// POS: Import dialog mutation-boundary regression.
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { ProviderCCSwitchDialog } from "./provider-ccswitch-dialog";
const api = vi.hoisted(() => ({ preview: vi.fn(), sync: vi.fn() }));
vi.mock("@/lib/api/settings/provider-api", () => ({ previewCCSwitchApi: api.preview, syncCCSwitchApi: api.sync }));
it("blocks duplicate and accepted-result form submissions", async () => {
  api.preview.mockResolvedValue({ detected: true, config_dir: "/config", providers: [{source_key: "one", name: "One", can_sync: true, current: true, models: [], current_runtime_supported: true}] });
  let reject!: (reason: unknown) => void;
  api.sync.mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: key => key}}><ProviderCCSwitchDialog isOpen onClose={vi.fn()} onSynced={vi.fn()} /></I18N_CONTEXT.Provider>);
  const button = await screen.findByRole("button", {name: "settings.providers.ccswitch_sync_action"});
  await waitFor(() => expect(button.hasAttribute("disabled")).toBe(false));
  const form = button.closest("form")!;
  act(() => { fireEvent.submit(form); fireEvent.submit(form); });
  expect(api.sync).toHaveBeenCalledTimes(1);
  await act(async () => reject(new ApiRequestError("accepted", 503, {version: 1, code: "pending", category: "unavailable", effect: "accepted"})));
  fireEvent.submit(form);
  expect(api.sync).toHaveBeenCalledTimes(1);
});
