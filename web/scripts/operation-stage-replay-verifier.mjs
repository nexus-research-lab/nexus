export function verify_completed_round_replay_uses_event_slice({
  assert,
  now,
  planOperationDesktop,
  projectOperationSnapshot,
}) {
  const messages = [{
    role: "assistant",
    message_id: "msg-completed-replay",
    session_key: "session:stage-replay",
    agent_id: "agent-stage",
    round_id: "round-completed-replay",
    timestamp: now - 1800,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-write",
        name: "Write",
        input: {
          file_path: "gomoku.html",
          content: "<html><body>gomoku</body></html>",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-write",
        content: "created gomoku.html",
        is_error: false,
      },
      {
        type: "tool_use",
        id: "tool-bash",
        name: "Bash",
        input: {
          command: "open gomoku.html",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-bash",
        content: "opened gomoku.html",
        is_error: false,
      },
    ],
    result_summary: {
      subtype: "success",
      duration_ms: 1800,
      duration_api_ms: 1400,
      num_turns: 1,
      result: "delivered gomoku html",
      is_error: false,
      timestamp: now - 100,
    },
  }];
  const workspace_events = [{
    id: "workspace-replay-html",
    agent_id: "agent-stage",
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: "session:stage-replay",
    tool_use_id: "tool-write",
    event_type: "file_write_end",
    live_content: "<html><body>gomoku</body></html>",
    updated_at: now - 700,
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage-replay",
    session_key: "session:stage-replay",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events,
  });
  assert(snapshot.active_event?.kind === "round_summary", `completed replay fixture should finish on summary, got ${snapshot.active_event?.kind}`);

  const final_desktop = planOperationDesktop({
    event: snapshot.active_event,
    snapshot,
  });
  assert(final_desktop.active_window_id?.includes(":browser"), `completed desktop should keep artifact browser focused, got ${final_desktop.active_window_id}`);
  assert(!final_desktop.windows.some((window) => window.kind === "handoff"), "completed desktop with app windows should not include delivery handoff");
  const completed_terminal_window = final_desktop.windows.find((window) => window.kind === "terminal");
  assert(completed_terminal_window?.phase === "minimized", `completed desktop should return terminal to Dock, got ${completed_terminal_window?.phase}`);
  const completed_browser_window = final_desktop.windows.find((window) => window.kind === "browser");
  assert(completed_browser_window?.phase === "focused", `completed desktop should keep browser artifact focused, got ${completed_browser_window?.phase}`);

  const bash_event = snapshot.events.find((event) => event.tool_use_id === "tool-bash");
  assert(bash_event, "replay fixture should project Bash event");
  const replay_desktop = planOperationDesktop({
    event: bash_event,
    snapshot,
  });
  const replay_browser_window = replay_desktop.windows.find((window) => window.kind === "browser");
  assert(replay_browser_window, "event replay slice should keep the opened browser artifact");
  assert(replay_desktop.active_window_id === replay_browser_window.id, `event replay slice should focus opened Safari session, got ${replay_desktop.active_window_id}`);
  assert(replay_browser_window.target === "gomoku.html", `event replay Safari session should keep artifact target, got ${replay_browser_window.target}`);
  assert(!replay_desktop.windows.some((window) => window.kind === "handoff" || window.kind === "run_manifest"), "event replay slice should not keep final handoff as the active scene");
  const terminal_window = replay_desktop.windows.find((window) => window.kind === "terminal");
  assert(terminal_window?.phase === "background", `event replay terminal should remain as background evidence, got ${terminal_window?.phase}`);
  assert(terminal_window?.payload.event.id === bash_event.id, "event replay terminal should keep selected Bash identity");
  assert(terminal_window?.payload.command === "open gomoku.html", `event replay terminal should keep selected command, got ${terminal_window?.payload.command}`);
}
