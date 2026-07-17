/**
 * INPUT: Stable Agent browser pages and one Operation Stage session key.
 * OUTPUT: React commands and derived state for the persistent Navi session.
 * POS: Thin controller over browser-navigation-model; no page projection or rendering.
 */
import { useCallback, useEffect, useMemo, useState } from "react";

import type { BrowserPageSnapshot } from "./browser-page-model";
import {
  activeBrowserPage,
  activeBrowserTab,
  closeBrowserTab,
  createBrowserNavigationState,
  moveBrowserHistory,
  navigateBrowserAddress,
  openBrowserTab,
  reloadBrowserPage,
  selectBrowserTab,
  syncBrowserAgentPages,
  toggleBrowserReader,
} from "./browser-navigation-model";

export function useBrowserNavigation({
  agent_pages,
  session_key,
  web_url_builder,
}: {
  agent_pages: BrowserPageSnapshot[];
  session_key: string;
  web_url_builder?: (url: string) => string;
}) {
  const [state, set_state] = useState(() => (
    createBrowserNavigationState(session_key, agent_pages)
  ));
  const active_tab = useMemo(() => activeBrowserTab(state), [state]);
  const active_page = useMemo(() => activeBrowserPage(state), [state]);
  const [address_draft, set_address_draft] = useState(active_page.address);

  useEffect(() => {
    set_state((current) => syncBrowserAgentPages(current, session_key, agent_pages));
  }, [agent_pages, session_key]);

  useEffect(() => {
    set_address_draft(active_page.address);
  }, [active_page.address, active_page.id]);

  const navigate = useCallback((input: string) => {
    set_state((current) => navigateBrowserAddress(current, input, web_url_builder));
  }, [web_url_builder]);
  const close_tab = useCallback((tab_id: string) => {
    set_state((current) => closeBrowserTab(current, tab_id));
  }, []);
  const go_back = useCallback(() => {
    set_state((current) => moveBrowserHistory(current, -1));
  }, []);
  const go_forward = useCallback(() => {
    set_state((current) => moveBrowserHistory(current, 1));
  }, []);
  const new_tab = useCallback(() => {
    set_state(openBrowserTab);
  }, []);
  const reload = useCallback(() => {
    set_state(reloadBrowserPage);
  }, []);
  const select_tab = useCallback((tab_id: string) => {
    set_state((current) => selectBrowserTab(current, tab_id));
  }, []);
  const toggle_reader = useCallback(() => {
    set_state(toggleBrowserReader);
  }, []);

  return {
    active_page,
    active_tab,
    address_draft,
    can_go_back: active_tab.current_index > 0,
    can_go_forward: active_tab.current_index < active_tab.pages.length - 1,
    close_tab,
    go_back,
    go_forward,
    navigate,
    new_tab,
    reload,
    select_tab,
    set_address_draft,
    tabs: state.tabs,
    toggle_reader,
  };
}
