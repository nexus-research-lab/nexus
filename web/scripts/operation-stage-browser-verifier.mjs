export function verify_html_artifact_opens_browser_srcdoc({
  assert,
  now,
  planOperationDesktop,
  projectOperationSnapshot,
}) {
  const html_content = "<html><body><main>gomoku board</main></body></html>";
  const messages = [{
    role: "assistant",
    message_id: "msg-browser-preview",
    session_key: "session:stage-browser",
    agent_id: "agent-stage",
    round_id: "round-browser-preview",
    timestamp: now - 1200,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-html-preview",
        name: "Write",
        input: {
          file_path: "gomoku.html",
          content: html_content,
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-html-preview",
        content: "created gomoku.html",
        is_error: false,
      },
    ],
    result_summary: {
      subtype: "success",
      duration_ms: 1200,
      duration_api_ms: 900,
      num_turns: 1,
      result: "created html gomoku",
      is_error: false,
      timestamp: now - 100,
    },
  }];
  const workspace_events = [{
    id: "workspace-browser-preview",
    agent_id: "agent-stage",
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: "session:stage-browser",
    tool_use_id: "tool-html-preview",
    event_type: "file_write_end",
    live_content: html_content,
    updated_at: now - 700,
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage-browser",
    session_key: "session:stage-browser",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events,
  });
  const desktop = planOperationDesktop({
    event: snapshot.active_event,
    snapshot,
  });
  const browser_window = desktop.windows.find((window) => window.kind === "browser");
  assert(browser_window, "html stage should include a browser preview window");
  assert(browser_window.title === "gomoku.html", `browser preview should use artifact title, got ${browser_window.title}`);
  assert(browser_window.payload.srcdoc === html_content, "browser preview should carry workspace live html content as srcdoc");
  const terminal_event = {
    ...snapshot.active_event,
    id: "tool-open-preview",
    kind: "command_run",
    surface: "terminal",
    phase: "done",
    tool_name: "Bash",
    target: "open gomoku.html",
    input_preview: {
      command: "open gomoku.html",
    },
    result_preview: {
      content: "Opening gomoku.html\nSafari preview launched\n",
      exit_code: 0,
      is_error: false,
    },
    updated_at: now,
  };
  const opened_snapshot = {
    ...snapshot,
    active_event: terminal_event,
    events: [...snapshot.events, terminal_event],
    updated_at: now,
  };
  const opened_desktop = planOperationDesktop({
    event: terminal_event,
    snapshot: opened_snapshot,
  });
  const opened_browser = opened_desktop.windows.find((window) => window.kind === "browser");
  assert(opened_browser?.phase === "focused", `done terminal command that opens html should hand focus to Navi, got ${opened_browser?.phase}`);
  assert(opened_browser?.payload.target === "gomoku.html", `explicit html open should preserve the workspace target, got ${opened_browser?.payload.target}`);
  assert(opened_browser?.payload.srcdoc === html_content, "explicit html open should keep the latest real file content for immediate rendering");
  const terminal_window = opened_desktop.windows.find((window) => window.kind === "terminal");
  assert(terminal_window?.phase === "background", `terminal should remain as background evidence after opening html, got ${terminal_window?.phase}`);
}
