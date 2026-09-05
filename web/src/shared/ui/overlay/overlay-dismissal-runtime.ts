// INPUT: 已挂载的模态范围、锚点与真实 Portal 浮层元素。
// OUTPUT: 当前模态范围内的最上层浮层身份，以及跨 Portal 的父子命中关系。
// POS: Overlay 关闭仲裁真相源；不持有业务开关、关闭回调或页面滚动锁。

interface ModalScope {
  root: HTMLElement;
  token: symbol;
}

interface AnchoredOverlay {
  anchor: HTMLElement;
  root: HTMLElement;
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
  const overlay = { anchor, root };
  anchoredOverlays.push(overlay);
  return () => {
    const index = anchoredOverlays.indexOf(overlay);
    if (index >= 0) {
      anchoredOverlays.splice(index, 1);
    }
  };
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

export function isAnchoredOverlayOutsidePress(root: HTMLElement | null, target: Node): boolean {
  const overlays = getCurrentScopeOverlays();
  const current = overlays.find((overlay) => overlay.root === root);
  if (!current) {
    return false;
  }
  return !overlays.some((overlay) => (
    (overlay === current || isDescendantOverlay(overlay, current))
    && (overlay.root.contains(target) || overlay.anchor.contains(target))
  ));
}
