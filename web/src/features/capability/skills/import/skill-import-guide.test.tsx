// INPUT: Localized authoring guide and native download action.
// OUTPUT: Confirms the selected language downloads the exact bundled markdown without submitting the form.
// POS: Skill import guide download contract.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { SkillImportGuide } from "./skill-import-guide";
import english from "../../../../../../docs/guides/room-skill-authoring.en.md?raw";
import chinese from "../../../../../../docs/guides/room-skill-authoring.md?raw";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });
it.each(["zh", "en"] as const)("downloads the %s guide with its original content", async (locale) => {
  const user = userEvent.setup();
  let download = "";
  const createObjectURL = vi.fn((_blob: Blob) => "blob:guide-test");
  const revokeObjectURL = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = createObjectURL; static revokeObjectURL = revokeObjectURL; });
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) { download = this.download; });
  const submit = vi.fn((event) => event.preventDefault());
  render(<I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <form onSubmit={submit}><SkillImportGuide importing={false} /></form>
  </I18N_CONTEXT.Provider>);
  await user.click(screen.getByText(MESSAGES[locale]["capability.skills_import_guide_title"]));
  await user.click(screen.getByRole("button", { name: MESSAGES[locale]["capability.skills_import_guide_download_aria"] }));
  const blob = createObjectURL.mock.calls[0][0] as Blob;
  const content = await new Promise<string>((resolve) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.readAsText(blob); });
  expect(content).toBe(locale === "zh" ? chinese : english);
  expect(download).toBe(locale === "zh" ? "room-skill-authoring.md" : "room-skill-authoring.en.md");
  expect(revokeObjectURL).toHaveBeenCalledWith("blob:guide-test");
  expect(submit).not.toHaveBeenCalled();
});
