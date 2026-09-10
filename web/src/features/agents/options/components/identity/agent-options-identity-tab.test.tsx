// INPUT: Create/edit identity fields, template loading and main-Agent model constraints.
// OUTPUT: Named fields with exact hints, source-specific content and unchanged commands.
// POS: Identity composition regression; no Agent files, services or persistence are used.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { AgentOptionsIdentityTab } from "./agent-options-identity-tab";

const props: ComponentProps<typeof AgentOptionsIdentityTab> = {
  avatar: "1", businessTags: [], defaultModel: "Model", defaultProvider: "Provider", description: "Description",
  isMain: false, isValidatingName: false, model: "", nameValidation: null,
  onAvatarChange: vi.fn(), onBusinessTagsChange: vi.fn(), onDescriptionChange: vi.fn(), onModelChange: vi.fn(),
  onProfileTemplateChange: vi.fn(), onRetryProfileTemplate: vi.fn(), onProviderChange: vi.fn(), onTitleChange: vi.fn(), onVibeTagsChange: vi.fn(),
  profileTemplate: "# Role", profileTemplateError: null, profileTemplateLoading: false,
  provider: "", providerOptions: [], providerOptionsError: null, providerOptionsLoading: false,
  scopeKey: "create-agent", sourceMode: "create", title: "Nova", variant: "dialog", vibeTags: [],
};

function view(overrides: Partial<typeof props> = {}) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <AgentOptionsIdentityTab {...props} {...overrides} />
  </I18N_CONTEXT.Provider>;
}

it("names the creation template, associates its hint and retains loading/retry boundaries", async () => {
  const onRetryProfileTemplate = vi.fn();
  const onProfileTemplateChange = vi.fn();
  const { rerender } = render(view({ profileTemplateLoading: true }));
  const template = screen.getByRole("textbox", { name: "agent_options.identity.profile_template" }) as HTMLTextAreaElement;
  expect(template.disabled).toBe(true);
  expect(template.getAttribute("aria-busy")).toBe("true");
  expect(document.getElementById(template.getAttribute("aria-describedby")!)?.textContent).toBe("agent_options.identity.profile_template_hint");
  rerender(view({ profileTemplateError: "Default template unavailable", onRetryProfileTemplate, onProfileTemplateChange }));
  expect(template.disabled).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "state.retry" }));
  expect(onRetryProfileTemplate).toHaveBeenCalledOnce();
  await userEvent.type(template, "!");
  expect(onProfileTemplateChange).toHaveBeenCalledWith("# Role!");
  expect(screen.queryByRole("textbox", { name: "agent_options.identity.description" })).toBeNull();
});

it("keeps the main Agent on the default model and hides non-applicable profile fields", () => {
  render(view({ agentId: "main", isMain: true, sourceMode: "edit", variant: "inline" }));
  const model = screen.getByRole("button", { name: "agent_options.identity.model" }) as HTMLButtonElement;
  expect(model.disabled).toBe(true);
  expect(document.getElementById(model.getAttribute("aria-describedby")!)?.textContent).toBe("agent_options.identity.main_model_hint");
  expect(screen.queryByRole("textbox", { name: "agent_options.identity.profile_template" })).toBeNull();
  expect(screen.queryByRole("textbox", { name: "agent_options.identity.description" })).toBeNull();
});

it("binds the retained edit-description field and forwards its draft", async () => {
  const onDescriptionChange = vi.fn();
  render(view({ sourceMode: "edit", onDescriptionChange }));
  await userEvent.type(screen.getByRole("textbox", { name: "agent_options.identity.description" }), "!");
  expect(onDescriptionChange).toHaveBeenCalledWith("Description!");
});

it("keeps one field tree and independent tag drafts when switching identity presentation", async () => {
  const user = userEvent.setup();
  const onBusinessTagsChange = vi.fn();
  const onVibeTagsChange = vi.fn();
  const callbacks = { onBusinessTagsChange, onVibeTagsChange };
  const rendered = render(view(callbacks));
  const business = screen.getByRole("textbox", { name: "agent_options.identity.business_tags" });
  const vibe = screen.getByRole("textbox", { name: "agent_options.identity.vibe_tags" });
  const name = screen.getByRole("textbox", { name: "agent_options.identity.name" });
  await user.type(business, "research");
  await user.type(vibe, "friendly");
  rendered.rerender(view({ ...callbacks, variant: "inline" }));
  expect(screen.getByRole("textbox", { name: "agent_options.identity.name" })).toBe(name);
  expect(screen.getByRole("textbox", { name: "agent_options.identity.business_tags" })).toBe(business);
  expect(screen.getByRole("textbox", { name: "agent_options.identity.vibe_tags" })).toBe(vibe);
  expect((business as HTMLInputElement).value).toBe("research");
  expect((vibe as HTMLInputElement).value).toBe("friendly");
  business.focus();
  await user.keyboard("{Enter}");
  expect(onBusinessTagsChange).toHaveBeenCalledExactlyOnceWith(["research"]);
  expect(onVibeTagsChange).not.toHaveBeenCalled();
  rendered.rerender(view(callbacks));
  expect((vibe as HTMLInputElement).value).toBe("friendly");
  vibe.focus();
  await user.keyboard("{Enter}");
  expect(onVibeTagsChange).toHaveBeenCalledExactlyOnceWith(["friendly"]);
});
