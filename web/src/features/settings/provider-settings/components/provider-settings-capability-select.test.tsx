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
  await user.click(screen.getByRole("option", { name: "settings.providers.capability_auto" }));
  expect(onChange).toHaveBeenCalledWith(undefined);
});

it("shows Automatic without an inferred result or a manual override", () => {
 const onChange=vi.fn();
 render(<CapabilitySelect label="Vision" checked={undefined} onChange={onChange} />);
 expect(screen.getByRole("button",{name:"Vision"}).textContent).toContain("settings.providers.capability_auto");
 expect(onChange).not.toHaveBeenCalled();
});
