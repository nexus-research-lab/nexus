/**
 * INPUT: Agent-projected pages and direct user navigation actions.
 * OUTPUT: Persistent tabs, per-tab history, and browser navigation state transitions.
 * POS: Pure Navi session model shared by the React controller and semantic tests.
 */
import type {
  BrowserPageSnapshot,
  BrowserReaderSnapshot,
} from "./browser-page-model";
import {
  createBrowserStartPage,
  createBrowserUserPage,
  normalizeBrowserAddress,
} from "./browser-page-model";

const AGENT_TAB_ID = "agent";

export interface BrowserTabState {
  current_index: number;
  id: string;
  pages: BrowserPageSnapshot[];
}

export interface BrowserNavigationState {
  active_tab_id: string;
  next_sequence: number;
  observed_agent_page_ids: string[];
  session_key: string;
  tabs: BrowserTabState[];
}

export function createBrowserNavigationState(
  session_key: string,
  agent_pages: BrowserPageSnapshot[],
): BrowserNavigationState {
  const pages = agent_pages.length
    ? agent_pages
    : [createBrowserStartPage("agent:start", "agent")];
  return {
    active_tab_id: AGENT_TAB_ID,
    next_sequence: 1,
    observed_agent_page_ids: agent_pages.map((page) => page.id),
    session_key,
    tabs: [{ current_index: pages.length - 1, id: AGENT_TAB_ID, pages }],
  };
}

export function syncBrowserAgentPages(
  state: BrowserNavigationState,
  session_key: string,
  agent_pages: BrowserPageSnapshot[],
): BrowserNavigationState {
  if (state.session_key !== session_key) {
    return createBrowserNavigationState(session_key, agent_pages);
  }
  if (!agent_pages.length) {
    return state;
  }

  const observed = new Set(state.observed_agent_page_ids);
  const new_pages = agent_pages.filter((page) => !observed.has(page.id));
  const current_agent_tab = state.tabs.find((tab) => tab.id === AGENT_TAB_ID);
  const base_pages = current_agent_tab?.pages ?? [];
  const incoming_by_id = new Map(agent_pages.map((page) => [page.id, page]));
  const updated_pages = base_pages
    .filter((page) => page.source !== "agent" || incoming_by_id.has(page.id))
    .map((page) => {
      const incoming = incoming_by_id.get(page.id);
      return incoming ? preserve_page_interaction(incoming, page) : page;
    });
  const existing_ids = new Set(updated_pages.map((page) => page.id));
  for (const page of agent_pages) {
    if (!existing_ids.has(page.id)) {
      updated_pages.push(page);
    }
  }

  const agent_tab: BrowserTabState = {
    id: AGENT_TAB_ID,
    pages: updated_pages,
    current_index: new_pages.length
      ? Math.max(0, updated_pages.findIndex((page) => page.id === new_pages.at(-1)?.id))
      : Math.min(current_agent_tab?.current_index ?? updated_pages.length - 1, updated_pages.length - 1),
  };
  const tabs = current_agent_tab
    ? state.tabs.map((tab) => tab.id === AGENT_TAB_ID ? agent_tab : tab)
    : [agent_tab, ...state.tabs];

  return {
    ...state,
    active_tab_id: new_pages.length ? AGENT_TAB_ID : state.active_tab_id,
    observed_agent_page_ids: agent_pages.map((page) => page.id),
    tabs,
  };
}

export function activeBrowserTab(state: BrowserNavigationState): BrowserTabState {
  return state.tabs.find((tab) => tab.id === state.active_tab_id) ?? state.tabs[0];
}

export function activeBrowserPage(state: BrowserNavigationState): BrowserPageSnapshot {
  const tab = activeBrowserTab(state);
  return tab.pages[tab.current_index] ?? createBrowserStartPage(`${tab.id}:start`);
}

export function selectBrowserTab(
  state: BrowserNavigationState,
  tab_id: string,
): BrowserNavigationState {
  if (!state.tabs.some((tab) => tab.id === tab_id) || state.active_tab_id === tab_id) {
    return state;
  }
  return { ...state, active_tab_id: tab_id };
}

export function openBrowserTab(state: BrowserNavigationState): BrowserNavigationState {
  const sequence = state.next_sequence;
  const tab_id = `tab:${sequence}`;
  return {
    ...state,
    active_tab_id: tab_id,
    next_sequence: sequence + 1,
    tabs: [
      ...state.tabs,
      {
        current_index: 0,
        id: tab_id,
        pages: [createBrowserStartPage(`${tab_id}:start`)],
      },
    ],
  };
}

export function closeBrowserTab(
  state: BrowserNavigationState,
  tab_id: string,
): BrowserNavigationState {
  const closing_index = state.tabs.findIndex((tab) => tab.id === tab_id);
  if (closing_index < 0) {
    return state;
  }
  if (state.tabs.length === 1) {
    const replacement = createBrowserStartPage(`${tab_id}:start:${state.next_sequence}`);
    return {
      ...state,
      next_sequence: state.next_sequence + 1,
      tabs: [{ current_index: 0, id: tab_id, pages: [replacement] }],
    };
  }
  const tabs = state.tabs.filter((tab) => tab.id !== tab_id);
  if (state.active_tab_id !== tab_id) {
    return { ...state, tabs };
  }
  return {
    ...state,
    active_tab_id: tabs[Math.min(closing_index, tabs.length - 1)].id,
    tabs,
  };
}

export function navigateBrowserTab(
  state: BrowserNavigationState,
  page: BrowserPageSnapshot,
): BrowserNavigationState {
  return update_active_tab(state, (tab) => ({
    ...tab,
    current_index: tab.current_index + 1,
    pages: [...tab.pages.slice(0, tab.current_index + 1), page],
  }));
}

export function navigateBrowserAddress(
  state: BrowserNavigationState,
  input: string,
  web_url_builder?: (url: string) => string,
): BrowserNavigationState {
  const normalized = normalizeBrowserAddress(input);
  const known_page = find_known_page(state, normalized ?? "about:blank");
  const sequence = state.next_sequence;
  const page = known_page
    ? clone_known_page(known_page, `user:${sequence}`)
    : createBrowserUserPage({
      id: `user:${sequence}`,
      input,
      reader: find_reader_for_url(state, normalized),
      web_url_builder,
    });
  return {
    ...navigateBrowserTab(state, page),
    next_sequence: sequence + 1,
  };
}

export function moveBrowserHistory(
  state: BrowserNavigationState,
  delta: -1 | 1,
): BrowserNavigationState {
  return update_active_tab(state, (tab) => ({
    ...tab,
    current_index: Math.max(0, Math.min(tab.current_index + delta, tab.pages.length - 1)),
  }));
}

export function reloadBrowserPage(state: BrowserNavigationState): BrowserNavigationState {
  return update_active_page(state, (page) => ({
    ...page,
    reload_key: page.reload_key + 1,
    srcdoc: page.kind === "workspace" && page.iframe_url ? null : page.srcdoc,
  }));
}

export function toggleBrowserReader(state: BrowserNavigationState): BrowserNavigationState {
  const page = activeBrowserPage(state);
  if (!page.reader) {
    return state;
  }
  return update_active_page(state, (current) => ({
    ...current,
    presentation: current.presentation === "reader" ? "live" : "reader",
  }));
}

function update_active_tab(
  state: BrowserNavigationState,
  update: (tab: BrowserTabState) => BrowserTabState,
): BrowserNavigationState {
  return {
    ...state,
    tabs: state.tabs.map((tab) => tab.id === state.active_tab_id ? update(tab) : tab),
  };
}

function update_active_page(
  state: BrowserNavigationState,
  update: (page: BrowserPageSnapshot) => BrowserPageSnapshot,
): BrowserNavigationState {
  return update_active_tab(state, (tab) => ({
    ...tab,
    pages: tab.pages.map((page, index) => index === tab.current_index ? update(page) : page),
  }));
}

function find_known_page(
  state: BrowserNavigationState,
  address: string,
): BrowserPageSnapshot | null {
  for (const tab of state.tabs) {
    const page = [...tab.pages].reverse().find((item) => (
      item.address === address || item.query === address || item.target === address
    ));
    if (page) {
      return page;
    }
  }
  return null;
}

function find_reader_for_url(
  state: BrowserNavigationState,
  url: string | null,
): BrowserReaderSnapshot | null {
  if (!url) {
    return null;
  }
  for (const tab of state.tabs) {
    const reader = [...tab.pages].reverse().find((page) => page.reader?.url === url)?.reader;
    if (reader) {
      return reader;
    }
  }
  return null;
}

function clone_known_page(page: BrowserPageSnapshot, id: string): BrowserPageSnapshot {
  return {
    ...page,
    id,
    reload_key: 0,
    source: "user",
  };
}

function preserve_page_interaction(
  incoming: BrowserPageSnapshot,
  existing: BrowserPageSnapshot,
): BrowserPageSnapshot {
  return {
    ...incoming,
    presentation: existing.reader ? existing.presentation : incoming.presentation,
    reload_key: existing.reload_key,
  };
}
