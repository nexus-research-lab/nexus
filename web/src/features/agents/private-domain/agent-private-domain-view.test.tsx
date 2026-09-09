// INPUT: Deferred private-domain reads and repeated Agent scope visits.
// OUTPUT: Old successful or failed requests cannot replace current private records.
// POS: Private read-lifecycle regression; timeline rendering is independently covered.
import { act, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { AgentPrivateThread } from "@/types/agent/private-domain";
import type { PrivateEventTimeline } from "./timeline/agent-private-domain-timeline";
import { AgentPrivateDomainView } from "./agent-private-domain-view";

const api = vi.hoisted(() => ({ threads: vi.fn(), events: vi.fn() }));
vi.mock("@/lib/api/agent/private-domain-api", () => ({ listAgentPrivateThreadsApi: api.threads, listAgentPrivateEventsApi: api.events }));
vi.mock("./timeline/agent-private-domain-timeline", () => ({
  PrivateEventTimeline: ({ events, isLoading }: ComponentProps<typeof PrivateEventTimeline>) => <output data-testid="events" aria-busy={isLoading}>{events.map((event) => event.content).join("|")}</output>,
}));
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function thread(preview: string): AgentPrivateThread {
  return { agent_id: "a", thread_id: "thread", scope: "self", participants: [], participant_agent_ids: [], peer_agent_ids: [], message_count: 1, last_content_preview: preview };
}
function view(id: string) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}><AgentPrivateDomainView agent={{ agent_id: id, name: id, options: {}, created_at: 1, status: "idle", workspace_path: "" }} /></I18N_CONTEXT.Provider>;
}
beforeEach(() => { api.threads.mockReset(); api.events.mockReset(); api.events.mockResolvedValue({ items: [] }); });

it("does not accept an old A response after visiting B and returning to A", async () => {
  const old = deferred<{ items: AgentPrivateThread[] }>();
  const fresh = deferred<{ items: AgentPrivateThread[] }>();
  api.threads.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ items: [] }).mockReturnValueOnce(fresh.promise);
  const { rerender } = render(view("a"));
  expect(screen.getByRole("status", { name: "common.loading" })).toBeTruthy();
  rerender(view("b"));
  rerender(view("a"));
  await act(async () => { fresh.resolve({ items: [thread("Fresh summary")] }); });
  expect(screen.getByText("Fresh summary")).toBeTruthy();
  await act(async () => { old.resolve({ items: [thread("Old summary")] }); });
  expect(screen.queryByText("Old summary")).toBeNull();
  expect(screen.getByText("Fresh summary")).toBeTruthy();
});

it("keeps a new event request busy when an old matching-scope request fails", async () => {
  const old = deferred<{ items: never[] }>();
  const fresh = deferred<{ items: never[] }>();
  api.threads.mockResolvedValue({ items: [thread("Summary")] });
  api.events.mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
  const { rerender } = render(view("a"));
  await act(async () => undefined);
  rerender(view("b"));
  rerender(view("a"));
  await act(async () => undefined);
  expect(api.events).toHaveBeenCalledTimes(2);
  await act(async () => { old.reject(new Error("late failure")); });
  expect(screen.getByTestId("events").getAttribute("aria-busy")).toBe("true");
  await act(async () => { fresh.resolve({ items: [] }); });
  expect(screen.getByTestId("events").getAttribute("aria-busy")).toBe("false");
});


it("does not restrict Agent-wide preview to a single room", async () => {
  api.threads.mockResolvedValue({items: []});
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: key => key}}>
    <AgentPrivateDomainView agent={{agent_id: "a", name: "A", options: {}, created_at: 1, status: "idle", workspace_path: ""}} variant="preview" />
  </I18N_CONTEXT.Provider>);
  await act(async () => undefined);
  expect(api.threads).toHaveBeenCalledWith("a", expect.objectContaining({room_id: null, conversation_id: null, room_limit: 160}));
});
