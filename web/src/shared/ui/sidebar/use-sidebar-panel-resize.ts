import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

const SIDEBAR_RESIZE_HOTZONE_WIDTH = 8;
const MODAL_ROOT_SELECTOR = "[data-modal-root='true']";

interface UseSidebarPanelResizeOptions {
  set_wide_panel_width: (width: number) => void;
  wide_panel_width: number;
}

export function useSidebarPanelResize({
  set_wide_panel_width,
  wide_panel_width,
}: UseSidebarPanelResizeOptions) {
  const root_ref = useRef<HTMLDivElement | null>(null);
  const [is_resize_hotzone_active, set_is_resize_hotzone_active] = useState(false);
  const is_dragging_ref = useRef(false);
  const start_x_ref = useRef(0);
  const start_width_ref = useRef(0);

  const handle_pointer_down = useCallback(
    (event: ReactPointerEvent) => {
      if (event.target instanceof HTMLElement && event.target.closest(MODAL_ROOT_SELECTOR)) {
        return;
      }
      const root_element = root_ref.current;
      if (!root_element) {
        return;
      }

      const rect = root_element.getBoundingClientRect();
      const distance_to_right_edge = rect.right - event.clientX;
      if (distance_to_right_edge > SIDEBAR_RESIZE_HOTZONE_WIDTH) {
        return;
      }

      event.preventDefault();
      is_dragging_ref.current = true;
      start_x_ref.current = event.clientX;
      start_width_ref.current = wide_panel_width;
      set_is_resize_hotzone_active(true);
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [wide_panel_width],
  );

  const handle_pointer_move = useCallback(
    (event: ReactPointerEvent) => {
      if (event.target instanceof HTMLElement && event.target.closest(MODAL_ROOT_SELECTOR)) {
        if (!is_dragging_ref.current) {
          set_is_resize_hotzone_active(false);
        }
        return;
      }
      const root_element = root_ref.current;
      if (!root_element) {
        return;
      }

      if (!is_dragging_ref.current) {
        const rect = root_element.getBoundingClientRect();
        const distance_to_right_edge = rect.right - event.clientX;
        set_is_resize_hotzone_active(distance_to_right_edge <= SIDEBAR_RESIZE_HOTZONE_WIDTH);
        return;
      }

      const delta = event.clientX - start_x_ref.current;
      const next_width = start_width_ref.current + delta;
      set_wide_panel_width(next_width);
    },
    [set_wide_panel_width],
  );

  const handle_pointer_up = useCallback(() => {
    is_dragging_ref.current = false;
    set_is_resize_hotzone_active(false);
  }, []);

  const handle_pointer_leave = useCallback(() => {
    if (is_dragging_ref.current) {
      return;
    }
    set_is_resize_hotzone_active(false);
  }, []);

  useEffect(() => {
    const handle_select_start = (event: Event) => {
      if (is_dragging_ref.current) {
        event.preventDefault();
      }
    };
    document.addEventListener("selectstart", handle_select_start);
    return () => document.removeEventListener("selectstart", handle_select_start);
  }, []);

  return {
    handle_pointer_down,
    handle_pointer_leave,
    handle_pointer_move,
    handle_pointer_up,
    is_resize_hotzone_active,
    root_ref,
  };
}
