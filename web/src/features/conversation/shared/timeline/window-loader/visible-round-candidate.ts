const UNLOADED_ROUND_SELECTOR =
  '[data-conversation-round-id][data-conversation-round-loaded="false"]';
const LOAD_ROOT_MARGIN_PX = 180;
const LOAD_BOUNDARY_TOLERANCE_PX = 12;

export type ScrollDirection = "down" | "none" | "up";

interface VisibleRoundCandidate {
  centerY: number;
  distance: number;
  roundId: string;
}

type DirectionalCandidateSelector = (
  candidates: VisibleRoundCandidate[],
  focusY: number,
) => VisibleRoundCandidate | null;

const DIRECTIONAL_CANDIDATE_SELECTORS: Record<
  ScrollDirection,
  DirectionalCandidateSelector
> = {
  down: (candidates, focusY) => candidates
    .filter((candidate) => candidate.centerY >= focusY)
    .reduce<VisibleRoundCandidate | null>(
      (best, candidate) => (
        !best || candidate.centerY < best.centerY ? candidate : best
      ),
      null,
    ),
  none: () => null,
  up: (candidates, focusY) => candidates
    .filter((candidate) => candidate.centerY <= focusY)
    .reduce<VisibleRoundCandidate | null>(
      (best, candidate) => (
        !best || candidate.centerY > best.centerY ? candidate : best
      ),
      null,
    ),
};

export function resolveVisibleUnloadedRoundId(
  scrollElement: HTMLDivElement,
  excludedRoundIds: ReadonlySet<string>,
  direction: ScrollDirection,
): string | null {
  const candidates = collectVisibleCandidates(
    scrollElement,
    excludedRoundIds,
  );
  if (candidates.length === 0) {
    return null;
  }

  const containerRect = scrollElement.getBoundingClientRect();
  const focusY =
    containerRect.top + Math.min(scrollElement.clientHeight * 0.38, 260);
  const directionalCandidate = DIRECTIONAL_CANDIDATE_SELECTORS[direction](
    candidates,
    focusY,
  );
  return (directionalCandidate ?? selectNearestCandidate(candidates)).roundId;
}

/**
 * DOM 尚未挂载边界节点时，仍可由索引窗口判定首尾加载目标。
 * 中间稀疏窗口继续交给可见 DOM 候选，避免跨过用户正在阅读的区间。
 */
export function resolveBoundaryUnloadedRoundId({
  clientHeight,
  excludedRoundIds,
  loadedRoundIds,
  roundIds,
  scrollHeight,
  scrollTop,
}: {
  clientHeight: number;
  excludedRoundIds: ReadonlySet<string>;
  loadedRoundIds: readonly string[];
  roundIds: readonly string[];
  scrollHeight: number;
  scrollTop: number;
}): string | null {
  const loaded = new Set(
    loadedRoundIds.map((roundId) => roundId.trim()).filter(Boolean),
  );
  const candidates = roundIds
    .map((roundId, index) => ({ index, roundId: roundId.trim() }))
    .filter(({ roundId }) => (
      roundId
      && !loaded.has(roundId)
      && !excludedRoundIds.has(roundId)
    ));
  if (candidates.length === 0) {
    return null;
  }

  const loadedIndexes = roundIds.flatMap((roundId, index) => (
    loaded.has(roundId.trim()) ? [index] : []
  ));
  if (loadedIndexes.length === 0) {
    return candidates.at(-1)?.roundId ?? null;
  }

  const firstLoadedIndex = loadedIndexes[0];
  const lastLoadedIndex = loadedIndexes.at(-1)!;
  if (scrollTop <= LOAD_BOUNDARY_TOLERANCE_PX) {
    return candidates
      .filter(({ index }) => index < firstLoadedIndex)
      .at(-1)?.roundId ?? null;
  }

  const maxScrollTop = Math.max(0, scrollHeight - clientHeight);
  if (scrollTop >= maxScrollTop - LOAD_BOUNDARY_TOLERANCE_PX) {
    return candidates.find(({ index }) => index > lastLoadedIndex)?.roundId
      ?? null;
  }
  return null;
}

function collectVisibleCandidates(
  scrollElement: HTMLDivElement,
  excludedRoundIds: ReadonlySet<string>,
): VisibleRoundCandidate[] {
  const containerRect = scrollElement.getBoundingClientRect();
  const minY = containerRect.top - LOAD_ROOT_MARGIN_PX;
  const maxY = containerRect.bottom + LOAD_ROOT_MARGIN_PX;
  const focusY =
    containerRect.top + Math.min(scrollElement.clientHeight * 0.38, 260);
  const candidates: VisibleRoundCandidate[] = [];

  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(UNLOADED_ROUND_SELECTOR),
  );
  for (const element of elements) {
    const roundId = element.dataset.conversationRoundId?.trim();
    if (!roundId || excludedRoundIds.has(roundId)) {
      continue;
    }
    const rect = element.getBoundingClientRect();
    if (rect.bottom < minY || rect.top > maxY) {
      continue;
    }
    const centerY = rect.top + Math.min(rect.height, 120) / 2;
    candidates.push({
      centerY,
      distance: Math.abs(centerY - focusY),
      roundId,
    });
  }
  return candidates;
}

function selectNearestCandidate(
  candidates: VisibleRoundCandidate[],
): VisibleRoundCandidate {
  return candidates.reduce((best, candidate) => (
    candidate.distance < best.distance ? candidate : best
  ));
}
