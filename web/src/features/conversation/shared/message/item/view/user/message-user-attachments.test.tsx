// INPUT: Attachment with exact or mismatched workspace ownership.
// OUTPUT: Only scoped file actions can invoke the workspace callback.
// POS: User attachment interaction regression.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { MessageUserAttachments } from "./message-user-attachments";

it("opens an exact workspace attachment by keyboard and leaves other scopes static", async () => {
  const open = vi.fn();
  const attachment = { file_name: "report.md", workspace_path: "report.md", workspace_agent_id: "nova", kind: "file" as const };
  const view = (owner: string) => <I18nProvider><MessageUserAttachments attachments={[attachment]} workspaceAgentId={owner} onOpenWorkspaceFile={open} /></I18nProvider>;
  const { rerender } = render(view("nova"));
  const user = userEvent.setup();
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: /report.md/ }));
  await user.keyboard("{Enter}");
  expect(open).toHaveBeenCalledWith("report.md", "nova");
  rerender(view("other"));
  expect(screen.queryByRole("button")).toBeNull();
  expect(screen.getByText("report.md")).toBeTruthy();
});
