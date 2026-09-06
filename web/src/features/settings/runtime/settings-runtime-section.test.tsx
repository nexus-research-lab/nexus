// INPUT: Real runtime settings view, isolated preference callbacks and six search providers.
// OUTPUT: Exact labels/groups, blur-save payloads, secret actions and instance-scoped JSON errors.
// POS: Runtime view regression; API transactions remain in the preferences controller tests.

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";
import type { WebSearchProvider, WebSearchSettings } from "@/types/settings/preferences";

import { SettingsRuntimeSection } from "./settings-runtime-section";

const useController = vi.hoisted(() => vi.fn());
vi.mock("./use-runtime-settings-controller", () => ({ useRuntimeSettingsController: useController }));

const messages: Record<string, string> = zhSettingsMessages;
const text = (key: string) => messages[`settings.runtime.${key}`];

function configure(provider: WebSearchProvider, extra: Partial<WebSearchSettings> = {}) {
  const controller = {
    loading: false,
    nxsRuntimeChecking: false,
    preferencesBusy: false,
    runtimeKind: "nxs",
    toolSearchEnabled: false,
    onRuntimeKindChange: vi.fn(),
    onToolSearchChange: vi.fn(),
    onWebSearchAPIKeyChange: vi.fn(),
    onWebSearchPatch: vi.fn(),
    onWebSearchProviderChange: vi.fn(),
    webSearch: { enabled: true, provider, ...extra },
    webSearchAPIKey: "",
  };
  useController.mockReturnValue(controller);
  return controller;
}

function RuntimeTestProviders({ children }: { children: ReactNode }) {
  return (
    <MemoryRouter>
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => messages[key] ?? key }}>
        {children}
      </I18N_CONTEXT.Provider>
    </MemoryRouter>
  );
}

function renderSettings(children: ReactNode = <SettingsRuntimeSection />) {
  return render(children, { wrapper: RuntimeTestProviders });
}

beforeEach(() => vi.clearAllMocks());

describe("runtime fields", () => {
  it.each<WebSearchProvider>(["brave", "tavily", "exa", "firecrawl", "searxng", "anysearch"])(
    "binds all %s fields to exact controls and names each visible selection group once",
    async (provider) => {
      const user = userEvent.setup();
      const controller = configure(provider);
      const { container } = renderSettings();
      await user.click(screen.getByText(text("web_search_provider"), { selector: "label" }));
      expect(screen.getByRole("listbox")).toBeTruthy();
      await user.keyboard("{Escape}");
      expect(controller.onWebSearchProviderChange).not.toHaveBeenCalled();

      await user.click(screen.getByRole("button", { name: text("web_search_more") }));
      const labels = container.querySelectorAll<HTMLLabelElement>("label[for]");
      expect(labels.length).toBeGreaterThanOrEqual(5);
      for (const label of labels) {
        expect(label.control, label.textContent ?? "").not.toBeNull();
        expect(label.control?.id).toBe(label.htmlFor);
        expect(label.contains(label.control)).toBe(false);
      }
      const ids = [...container.querySelectorAll("[id]")].map((node) => node.id);
      expect(new Set(ids).size).toBe(ids.length);
      expect(screen.getAllByRole("group", { name: text("kernel_label") })).toHaveLength(1);
      if (provider === "tavily") {
        for (const key of ["web_search_depth", "web_search_extract_depth"]) {
          const groups = screen.getAllByRole("group", { name: text(key) });
          expect(groups).toHaveLength(1);
          expect(groups[0].querySelector("[role=group], label button")).toBeNull();
        }
      }
      expect(controller.onWebSearchPatch).not.toHaveBeenCalled();
    },
  );

  it("saves normalized numeric and trimmed text drafts only on blur", async () => {
    const user = userEvent.setup();
    const controller = configure("brave");
    renderSettings();
    await user.click(screen.getByRole("button", { name: text("web_search_more") }));
    const count = screen.getByLabelText(text("web_search_result_count")) as HTMLInputElement;
    fireEvent.change(count, { target: { value: "999" } });
    expect(controller.onWebSearchPatch).not.toHaveBeenCalled();
    fireEvent.blur(count);
    expect(controller.onWebSearchPatch).toHaveBeenLastCalledWith({ default_count: 20 });
    expect(count.value).toBe("20");

    const url = screen.getByLabelText(text("web_search_custom_base_url")) as HTMLInputElement;
    fireEvent.change(url, { target: { value: " https://example.test/search " } });
    expect(controller.onWebSearchPatch).toHaveBeenCalledTimes(1);
    fireEvent.blur(url);
    expect(controller.onWebSearchPatch).toHaveBeenLastCalledWith({ base_url: "https://example.test/search" });
    expect(url.value).toBe("https://example.test/search");
  });

  it("keeps replacement and clearing of a configured secret independent from its label", async () => {
    const user = userEvent.setup();
    const controller = configure("brave", { api_key_configured: true, api_key_masked: "••••1234" });
    const { rerender } = renderSettings();
    const key = screen.getByLabelText(text("web_search_api_key")) as HTMLInputElement;
    const clear = screen.getByRole("button", { name: text("web_search_api_key_clear") }) as HTMLButtonElement;
    expect(key.value).toBe("");
    expect(key.placeholder).toBe("••••1234");
    expect(key.type).toBe("password");
    expect(clear.closest("label")).toBeNull();
    await user.click(screen.getByText(text("web_search_api_key"), { selector: "label" }));
    expect(document.activeElement).toBe(key);
    expect(controller.onWebSearchAPIKeyChange).not.toHaveBeenCalled();
    await user.type(key, " new-key ");
    await user.tab();
    expect(controller.onWebSearchAPIKeyChange).toHaveBeenLastCalledWith("new-key");
    expect(key.value).toBe("");
    await user.click(clear);
    expect(controller.onWebSearchAPIKeyChange).toHaveBeenLastCalledWith("");
    expect(controller.onWebSearchAPIKeyChange).toHaveBeenCalledTimes(2);

    useController.mockReturnValue({ ...controller, preferencesBusy: true });
    rerender(<SettingsRuntimeSection />);
    expect(key.disabled).toBe(true);
    expect(clear.disabled).toBe(true);
    await user.click(clear);
    expect(controller.onWebSearchAPIKeyChange).toHaveBeenCalledTimes(2);
  });

  it("isolates JSON errors and advanced panel identities between two mounted instances", async () => {
    const user = userEvent.setup();
    const controller = configure("anysearch", { anysearch: { domain: "web", tag: "docs" } });
    const { container } = renderSettings(<>
      <section data-testid="first"><SettingsRuntimeSection /></section>
      <section data-testid="second"><SettingsRuntimeSection /></section>
    </>);
    const first = within(screen.getByTestId("first"));
    const second = within(screen.getByTestId("second"));
    const more = [first, second].map((scope) => scope.getByRole("button", { name: text("web_search_more") }));
    await user.click(more[0]);
    await user.click(more[1]);
    expect(more[0].getAttribute("aria-controls")).not.toBe(more[1].getAttribute("aria-controls"));
    const params = first.getByLabelText(text("web_search_anysearch_params"));
    const otherParams = second.getByLabelText(text("web_search_anysearch_params"));
    for (const value of ["{", "[]", "null", "42"]) {
      fireEvent.change(params, { target: { value } });
      fireEvent.blur(params);
      expect(params.getAttribute("aria-invalid")).toBe("true");
      const error = document.getElementById(params.getAttribute("aria-errormessage")!);
      expect(error?.textContent).toContain(text("web_search_anysearch_params_invalid"));
      expect(first.getAllByRole("alert")).toHaveLength(1);
      expect(otherParams.hasAttribute("aria-invalid")).toBe(false);
      expect(second.queryByRole("alert")).toBeNull();
      expect(controller.onWebSearchPatch).not.toHaveBeenCalled();
    }
    fireEvent.change(params, { target: { value: '{"language":"en"}' } });
    fireEvent.blur(params);
    expect(params.hasAttribute("aria-invalid")).toBe(false);
    expect(params.hasAttribute("aria-errormessage")).toBe(false);
    expect(first.queryByRole("alert")).toBeNull();
    expect(controller.onWebSearchPatch).toHaveBeenLastCalledWith({ anysearch: { domain: "web", tag: "docs", params: { language: "en" } } });
    fireEvent.change(params, { target: { value: "" } });
    fireEvent.blur(params);
    expect(controller.onWebSearchPatch).toHaveBeenLastCalledWith({ anysearch: { domain: "web", tag: "docs", params: undefined } });
    const ids = [...container.querySelectorAll("[id]")].map((node) => node.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("changes only the chosen search-depth group", async () => {
    const user = userEvent.setup();
    const controller = configure("tavily");
    renderSettings();
    await user.click(screen.getByRole("button", { name: text("web_search_more") }));
    const depth = screen.getByRole("group", { name: text("web_search_depth") });
    const extract = screen.getByRole("group", { name: text("web_search_extract_depth") });
    await user.click(within(depth).getByRole("button", { name: text("web_search_advanced") }));
    expect(controller.onWebSearchPatch).toHaveBeenCalledExactlyOnceWith({ search_depth: "advanced" });
    expect(within(extract).getByRole("button", { name: text("web_search_basic") }).getAttribute("aria-pressed")).toBe("true");
  });
});
