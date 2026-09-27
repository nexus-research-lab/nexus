// INPUT: Tri-state capability override.
// OUTPUT: Unknown, explicit false and automatic reset remain distinct.
// POS: Provider capability control behavior regression.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { CapabilitySelect } from "./provider-settings-capability-select";
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
it("preserves explicit false and permits resetting to automatic", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  render(<CapabilitySelect label="Vision" checked={false} onChange={onChange} />);
  await user.click(screen.getByRole("button", { name: "Vision" }));
  await user.click(screen.getByRole("option", { name: "settings.providers.capability_auto_unknown" }));
  expect(onChange).toHaveBeenCalledWith(undefined);
});

it.each([
  [true, "settings.providers.capability_auto_yes"],
  [false, "settings.providers.capability_auto_no"],
  [undefined, "settings.providers.capability_auto_unknown"],
] as const)("shows the actual automatic result %s without creating an override", (automatic, label) => {
  const onChange = vi.fn();
  render(<CapabilitySelect label="Vision" checked={undefined} automatic={automatic} onChange={onChange} />);
  expect(screen.getByRole("button", { name: "Vision" }).textContent).toContain(label);
  expect(onChange).not.toHaveBeenCalled();
});
