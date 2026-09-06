// INPUT: Simultaneous identity forms and changing name validation.
// OUTPUT: Exactly bound labels, required names and current validation feedback.
// POS: Identity field regression; persistence remains with the editor controller.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { IdentityProfileFields } from "./identity-profile-fields";

const base = {
  avatar: "1", avatarAlt: "Avatar", isValidatingName: false,
  nameLabel: "Agent name", namePlaceholder: "Enter a name", nameValidation: null,
  onAvatarChange: vi.fn(), onTitleChange: vi.fn(), title: "Nova",
  validatingLabel: "Checking name", variant: "dialog" as const,
};

it("binds each label to its own required input and forwards edits", async () => {
  const onTitleChange = vi.fn();
  render(<I18nProvider><IdentityProfileFields {...base} onTitleChange={onTitleChange} /><IdentityProfileFields {...base} variant="inline" /></I18nProvider>);
  const inputs = screen.getAllByRole("textbox", { name: "Agent name" });
  expect(inputs).toHaveLength(2);
  expect(inputs[0].id).not.toBe(inputs[1].id);
  expect((inputs[0] as HTMLInputElement).required).toBe(true);
  await userEvent.click(screen.getAllByText("Agent name")[1]);
  expect(document.activeElement).toBe(inputs[1]);
  await userEvent.type(inputs[0], "a");
  expect(onTitleChange).toHaveBeenCalledWith("Novaa");
});

it("associates current validation and removes obsolete error/description space", () => {
  const { rerender } = render(<I18nProvider><IdentityProfileFields {...base} isValidatingName /></I18nProvider>);
  const input = screen.getByRole("textbox", { name: "Agent name" });
  expect(document.getElementById(input.getAttribute("aria-describedby")!)?.textContent).toBe("Checking name");
  expect(input.getAttribute("aria-invalid")).toBeNull();
  rerender(<I18nProvider><IdentityProfileFields {...base} nameValidation={{ name: "Nova", normalized_name: "Nova", is_valid: false, is_available: true, reason: "Name cannot contain a newline" }} /></I18nProvider>);
  expect(input.getAttribute("aria-invalid")).toBe("true");
  expect(document.getElementById(input.getAttribute("aria-errormessage")!)?.textContent).toBe("Name cannot contain a newline");
  expect(screen.queryByText("Checking name")).toBeNull();
  rerender(<I18nProvider><IdentityProfileFields {...base} /></I18nProvider>);
  expect(input.getAttribute("aria-invalid")).toBeNull();
  expect(input.getAttribute("aria-describedby")).toBeNull();
  expect(input.getAttribute("aria-errormessage")).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
});
