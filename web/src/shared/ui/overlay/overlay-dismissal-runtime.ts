// INPUT: 已挂载的模态范围、锚点与真实 Portal 浮层元素。
// OUTPUT: 当前模态范围最上层身份、跨 Portal 父子关系、嵌套层级与源区域命中语义。
// POS: Overlay 父子注册、关闭仲裁及同容器嵌套层级真相源；不持有业务开关、关闭回调或页面滚动锁。

interface ModalScope {
  root: HTMLElement;
  token: symbol;
}

interface AnchoredOverlay {
  anchor: HTMLElement;
  root: HTMLElement;
  originalZIndex: string;
}

const modalScopes: ModalScope[] = [];
const anchoredOverlays: AnchoredOverlay[] = [];
const MODAL_ROOT_SELECTOR = "[data-modal-root='true']";

export function registerModalOverlayScope(token: symbol, root: HTMLElement): void {
  modalScopes.push({ root, token });
}

export function unregisterModalOverlayScope(token: symbol): void {
  const index = modalScopes.findIndex((scope) => scope.token === token);
  if (index >= 0) {
    modalScopes.splice(index, 1);
  }
}

export function registerAnchoredOverlay(anchor: HTMLElement, root: HTMLElement): () => void {
  const overlay = { anchor, root, originalZIndex: root.style.zIndex };
  anchoredOverlays.push(overlay);
  reconcileAnchoredOverlayStacking();
  return () => {
    const index = anchoredOverlays.indexOf(overlay);
    if (index >= 0) {
      anchoredOverlays.splice(index, 1);
      root.style.zIndex = overlay.originalZIndex;
      reconcileAnchoredOverlayStacking();
    }
  };
}

// Portal siblings share a stacking context, so a child's semantic token may be
// lower than its parent. Preserve the token floor while ordering descendants above
// their parent, including child-first layout-effect registration.
function reconcileAnchoredOverlayStacking(): void {
  for (const overlay of anchoredOverlays) overlay.root.style.zIndex = overlay.originalZIndex;
  const resolved = new Set<AnchoredOverlay>();
  const resolving = new Set<AnchoredOverlay>();
  const resolve = (overlay: AnchoredOverlay): void => {
    if (resolved.has(overlay) || resolving.has(overlay)) return;
    resolving.add(overlay);
    const parent = anchoredOverlays.find((candidate) => candidate !== overlay
      && candidate.root.parentElement === overlay.root.parentElement
      && candidate.root.contains(overlay.anchor));
    if (parent) {
      resolve(parent);
      const view = overlay.root.ownerDocument.defaultView;
      const parentLayer = Number.parseInt(view?.getComputedStyle(parent.root).zIndex ?? "", 10);
      const ownLayer = Number.parseInt(view?.getComputedStyle(overlay.root).zIndex ?? "", 10);
      if (Number.isFinite(parentLayer)) {
        overlay.root.style.zIndex = String(Math.max(Number.isFinite(ownLayer) ? ownLayer : 0, parentLayer + 1));
      }
    }
    resolving.delete(overlay);
    resolved.add(overlay);
  };
  for (const overlay of anchoredOverlays) resolve(overlay);
}

function getCurrentScopeOverlays(): AnchoredOverlay[] {
  const modalRoot = modalScopes.findLast((scope) => scope.root.isConnected)?.root ?? null;
  return anchoredOverlays.filter(({ anchor, root }) => (
    root.isConnected
    && anchor.isConnected
    && anchor.closest(MODAL_ROOT_SELECTOR) === modalRoot
  ));
}

function isDescendantOverlay(child: AnchoredOverlay, parent: AnchoredOverlay): boolean {
  const visited = new Set<AnchoredOverlay>();
  let current: AnchoredOverlay | undefined = child;
  while (current && !visited.has(current)) {
    visited.add(current);
    if (parent.root.contains(current.anchor)) {
      return true;
    }
    const anchor: HTMLElement = current.anchor;
    current = anchoredOverlays.find((overlay) => (
      overlay !== current && overlay.root.contains(anchor)
    ));
  }
  return false;
}

export function isTopAnchoredOverlay(root: HTMLElement | null): boolean {
  const overlays = getCurrentScopeOverlays();
  // 子层始终优先于父层，即使 React 在同一提交中先执行子层的 layout effect。
  return overlays.findLast((candidate) => !overlays.some((overlay) => (
    overlay !== candidate && isDescendantOverlay(overlay, candidate)
  )))?.root === root;
}

/** 键盘退出可在关闭前记录父层，避免把即将消失的跨 Portal 菜单当作下一个 Tab 位置。 */
export function getAnchoredOverlayAncestorRoots(root: HTMLElement): HTMLElement[] {
  const current = anchoredOverlays.find((overlay) => overlay.root === root);
  if (!current) return [root];
  return anchoredOverlays.filter((overlay) => overlay === current || isDescendantOverlay(current, overlay))
    .map((overlay) => overlay.root);
}

export function isAnchoredOverlayOutsidePress(
  root: HTMLElement | null,
  target: Node,
  anchorPress: "inside" | "outside" = "inside",
): boolean {
  const overlays = getCurrentScopeOverlays();
  const current = overlays.find((overlay) => overlay.root === root);
  if (!current) {
    return false;
  }
  return !overlays.some((overlay) => (
    (overlay === current || isDescendantOverlay(overlay, current))
    && (overlay.root.contains(target)
      || ((overlay !== current || anchorPress === "inside") && overlay.anchor.contains(target)))
  ));
}
