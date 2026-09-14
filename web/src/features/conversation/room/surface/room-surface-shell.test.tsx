// INPUT: Delayed fork completion across exact conversation navigation.
// OUTPUT: Valid forks navigate once; obsolete completions do not steal the new surface.
// POS: Shell navigation lifecycle test with resource/layout boundaries stubbed.
import { act, render } from "@testing-library/react";
import { createRef, type ComponentProps } from "react";
import { expect, it, vi } from "vitest";
import { RoomSurfaceShell } from "./room-surface-shell";
import type { RoomSurfaceLayoutProps } from "./layout/room-surface-layout-types";
const boundary = vi.hoisted(() => ({ props: null as RoomSurfaceLayoutProps | null }));
vi.mock("@/shared/lib/react/use-media-query", () => ({ useMediaQuery: () => false }));
vi.mock("@/hooks/settings/use-default-agent-runtime-kind", () => ({ useDefaultAgentRuntimeKind: () => "nxs" }));
vi.mock("@/features/conversation/shared/execution/use-execution-resource", () => ({ useExecutionResource: () => ({}) }));
vi.mock("./mobile/room-mobile-surface", () => ({ RoomMobileSurface: () => null }));
vi.mock("./layout/room-surface-layout", () => ({ RoomSurfaceLayout: (props: RoomSurfaceLayoutProps) => { boundary.props = props; return null; } }));
function baseProps(): ComponentProps<typeof RoomSurfaceShell> {
  const agent = { agent_id: "agent", name: "Nova", workspace_path: "/workspace", created_at: 1, options: {}, status: "idle" as const };
  return {
    currentAgent: agent, currentRoomType: "group", roomId: "room-a", roomMembers: [agent], availableRoomAgents: [agent],
    currentRoomTitle: "Research", roomSkillNames: [], roomHostAgentId: "agent",
    roomHostAutoReplyEnabled: true, roomPrivateMessagesEnabled: true, currentAgentSessionIdentity: null,
    conversationId: "conversation-a", currentRoomConversations: [], activeWorkspacePath: null, currentRoomConversation: null, onBackToDirectory: vi.fn(),
    externalSessionsReliability: { failure: null, isLoading: false, isStale: false, refresh: vi.fn() },
    sidePanelWidthPercent: 42, isResizingSidePanel: false, currentTodos: [], surfaceSplitRef: createRef<HTMLElement>(),
    onCreateConversation: vi.fn(async () => null),
    onReplaceFinalConversation: vi.fn(async () => undefined), onSelectConversation: vi.fn(), onCloseConversation: vi.fn(async () => undefined),
    onDeleteConversation: vi.fn(async () => null), onForkConversation: vi.fn(async () => null),
    onManageRoom: vi.fn(async () => undefined), onOpenMemberManager: vi.fn(async () => undefined),
    onSaveAgentOptions: vi.fn(async () => undefined), onValidateAgentName: vi.fn(),
    onUpdateConversationTitle: vi.fn(async () => undefined), onOpenWorkspaceFile: vi.fn(), onStartSidePanelResize: vi.fn(),
    onSidePanelWidthChange: vi.fn(), onTodosChange: vi.fn(), onConversationSnapshotChange: vi.fn(),
  };
}

it.each([false, true])("keeps fork completion scoped when navigation changed: %s", async (changed) => {
  let resolve!: (id: string) => void;
  const props = baseProps();
  props.onForkConversation = vi.fn(() => new Promise<string>((done) => { resolve = done; }));
  const { rerender } = render(<RoomSurfaceShell {...props} />);
  let pending!: Promise<void>;
  act(() => { pending = boundary.props!.onForkConversation("round"); });
  if (changed) rerender(<RoomSurfaceShell {...props} conversationId="conversation-b" />);
  await act(async () => { resolve("fork"); await pending; });
  expect(props.onForkConversation).toHaveBeenCalledExactlyOnceWith("conversation-a", "round");
  if (changed) expect(props.onSelectConversation).not.toHaveBeenCalled();
  else expect(props.onSelectConversation).toHaveBeenCalledExactlyOnceWith("fork");
});
