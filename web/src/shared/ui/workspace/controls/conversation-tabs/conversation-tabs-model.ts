// INPUT: 标签身份、活动项、固定边缘动作与实测轨道宽度。
// OUTPUT: 稳定溢出判定与标签宽度分配。
// POS: 共享标签几何模型；不认识 Room、会话生命周期或持久偏好。

interface ConversationTabIdentity {
  id: string;
}

// 中文注释：历史与创建入口共用轻量导航带的 32px 边缘占位。
const CONVERSATION_EDGE_CONTROL_SPACE = 32;
const CONVERSATION_TAB_GAP = 2;

export const ACTIVE_TAB_MIN_WIDTH = 156;
export const CONVERSATION_TABS_VIEWPORT_INSET = 4;
export const INACTIVE_TAB_MIN_WIDTH = 104;

const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

export function hasConversationTabsOverflow({
  conversationCount,
  hasCreateButton,
  hasLeadingControl,
  trackWidth,
}: {
  conversationCount: number;
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  trackWidth: number;
}): boolean {
  if (!trackWidth || conversationCount <= 1) {
    return false;
  }
  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasLeadingControl,
    trackWidth,
  });
  const inactiveCount = conversationCount - 1;
  const minimumTabsWidth = ACTIVE_TAB_MIN_WIDTH
    + INACTIVE_TAB_MIN_WIDTH * inactiveCount
    + CONVERSATION_TAB_GAP * inactiveCount;
  return minimumTabsWidth > tabViewportWidth;
}

export function calculateConversationTabWidths({
  activeConversationId,
  hasCreateButton,
  hasLeadingControl,
  hasTabsOverflow,
  tabs,
  trackWidth,
}: {
  activeConversationId: string | null;
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  hasTabsOverflow: boolean;
  tabs: readonly ConversationTabIdentity[];
  trackWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!trackWidth || tabs.length === 0) {
    return widths;
  }

  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasLeadingControl,
    trackWidth,
  });
  const availableWidth = tabViewportWidth
    - CONVERSATION_TAB_GAP * Math.max(0, tabs.length - 1);
  if (tabs.length === 1) {
    widths.set(
      tabs[0].id,
      Math.max(ACTIVE_TAB_MIN_WIDTH, tabViewportWidth),
    );
    return widths;
  }

  const inactiveCount = tabs.length - 1;
  const minimumTotalWidth = ACTIVE_TAB_MIN_WIDTH + INACTIVE_TAB_MIN_WIDTH * inactiveCount;
  if (availableWidth < minimumTotalWidth && hasTabsOverflow) {
    return calculateOverflowConversationTabWidths({
      activeConversationId,
      tabs,
      tabViewportWidth,
    });
  }

  let activeWidth = ACTIVE_TAB_MIN_WIDTH;
  let inactiveWidth = INACTIVE_TAB_MIN_WIDTH;

  if (availableWidth > minimumTotalWidth) {
    const weightedUnitWidth = availableWidth / (inactiveCount + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximumActiveWidth = availableWidth - INACTIVE_TAB_MIN_WIDTH * inactiveCount;
    activeWidth = Math.min(
      maximumActiveWidth,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weightedUnitWidth * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactiveWidth = (availableWidth - activeWidth) / inactiveCount;
  }

  tabs.forEach((conversation) => {
    widths.set(
      conversation.id,
      conversation.id === activeConversationId ? activeWidth : inactiveWidth,
    );
  });
  return widths;
}

function calculateOverflowConversationTabWidths({
  activeConversationId,
  tabs,
  tabViewportWidth,
}: {
  activeConversationId: string | null;
  tabs: readonly ConversationTabIdentity[];
  tabViewportWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  const inactiveCount = tabs.length - 1;
  // 中文注释：以活动标签为锚点，计算一屏能完整容纳的普通标签数。
  const visibleInactiveCount = Math.min(
    inactiveCount,
    Math.max(
      0,
      Math.floor(
        (tabViewportWidth - ACTIVE_TAB_MIN_WIDTH)
        / (INACTIVE_TAB_MIN_WIDTH + CONVERSATION_TAB_GAP),
      ),
    ),
  );
  let activeWidth = ACTIVE_TAB_MIN_WIDTH;
  let inactiveWidth = INACTIVE_TAB_MIN_WIDTH;

  if (visibleInactiveCount > 0) {
    const visibleWidth = tabViewportWidth
      - CONVERSATION_TAB_GAP * visibleInactiveCount;
    const weightedUnitWidth = visibleWidth
      / (visibleInactiveCount + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximumActiveWidth = visibleWidth
      - INACTIVE_TAB_MIN_WIDTH * visibleInactiveCount;
    activeWidth = Math.min(
      maximumActiveWidth,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weightedUnitWidth * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactiveWidth = (visibleWidth - activeWidth) / visibleInactiveCount;
  }

  tabs.forEach((conversation) => {
    widths.set(
      conversation.id,
      conversation.id === activeConversationId ? activeWidth : inactiveWidth,
    );
  });
  return widths;
}

function getAvailableConversationTabWidth({
  hasCreateButton,
  hasLeadingControl,
  trackWidth,
}: {
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  trackWidth: number;
}): number {
  return Math.max(
    0,
    trackWidth - CONVERSATION_TABS_VIEWPORT_INSET * 2 - (
      hasCreateButton ? CONVERSATION_EDGE_CONTROL_SPACE : 0
    ) - (
      hasLeadingControl ? CONVERSATION_EDGE_CONTROL_SPACE : 0
    ),
  );
}
