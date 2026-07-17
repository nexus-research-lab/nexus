"use client";

/**
 * INPUT: The focused web tool event plus all related events in the active round.
 * OUTPUT: A persistent Navi browser session whose tabs remain usable after tool completion.
 * POS: Operation Stage browser-app orchestrator; page projection and navigation stay in models.
 */
import { useCallback, useMemo, useState } from "react";

import { getWorkspaceHtmlSiteUrl } from "@/lib/api/agent/agent-api";

import type { NexusOperationEvent } from "../operation-types";
import { getOperationBrowserPageUrl } from "../operation-stage-api";
import { BrowserChrome } from "./browser-chrome";
import { buildBrowserAgentPages } from "./browser-page-model";
import { BrowserPageViewport } from "./browser-pages";
import { useBrowserNavigation } from "./use-browser-navigation";

export function BrowserSurface({
  event,
  preview,
  query,
  relatedEvents,
  target,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  query: string;
  relatedEvents: NexusOperationEvent[];
  target?: string | null;
}) {
  const agent_pages = useMemo(() => buildBrowserAgentPages({
    event,
    preview,
    query,
    raw_url_builder: getWorkspaceHtmlSiteUrl,
    related_events: relatedEvents,
    target,
    web_url_builder: getOperationBrowserPageUrl,
  }), [event, preview, query, relatedEvents, target]);
  const browser = useBrowserNavigation({
    agent_pages,
    session_key: `${event.session_key}:${event.round_id}`,
    web_url_builder: getOperationBrowserPageUrl,
  });
  const [frame_state, set_frame_state] = useState<{ loading: boolean; page_id: string } | null>(null);
  const [frame_error_page_id, set_frame_error_page_id] = useState<string | null>(null);
  const set_frame_loading = useCallback((page_id: string, loading: boolean) => {
    set_frame_state((current) => (
      current?.page_id === page_id && current.loading === loading
        ? current
        : { loading, page_id }
    ));
  }, []);
  const set_frame_error = useCallback((page_id: string, failed: boolean) => {
    set_frame_error_page_id((current) => failed ? page_id : (current === page_id ? null : current));
  }, []);
  const is_loading = browser.active_page.event?.phase === "running" || (
    frame_state?.page_id === browser.active_page.id && frame_state.loading
  );
  const chrome_page = frame_error_page_id === browser.active_page.id
    ? { ...browser.active_page, status: { label: "页面无法载入", tone: "error" as const } }
    : frame_state?.page_id === browser.active_page.id
    && !frame_state.loading
    && browser.active_page.source === "user"
    && browser.active_page.status.tone === "loading"
    ? { ...browser.active_page, status: { label: "已载入", tone: "ready" as const } }
    : browser.active_page;

  return (
    <div className="flex min-h-0 min-w-0 max-w-full flex-1 flex-col overflow-hidden bg-white">
      <BrowserChrome
        activePage={chrome_page}
        activeTabId={browser.active_tab.id}
        addressDraft={browser.address_draft}
        canGoBack={browser.can_go_back}
        canGoForward={browser.can_go_forward}
        isLoading={is_loading}
        onAddressChange={browser.set_address_draft}
        onAddressSubmit={browser.navigate}
        onCloseTab={browser.close_tab}
        onGoBack={browser.go_back}
        onGoForward={browser.go_forward}
        onNewTab={browser.new_tab}
        onReload={browser.reload}
        onSelectTab={browser.select_tab}
        onToggleReader={browser.toggle_reader}
        tabs={browser.tabs}
      />
      <BrowserPageViewport
        onFrameError={set_frame_error}
        onFrameLoading={set_frame_loading}
        onNavigate={browser.navigate}
        page={browser.active_page}
      />
    </div>
  );
}
