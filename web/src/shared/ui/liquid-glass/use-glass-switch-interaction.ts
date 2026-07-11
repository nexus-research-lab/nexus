import { useCallback, useEffect, useRef, useState } from "react";

interface GlassSwitchInteractionOptions {
  checked: boolean;
  disabled: boolean;
}

interface GlassSwitchInteraction {
  finishTransition: () => void;
  isPressed: boolean;
  isTransitioning: boolean;
  press: () => void;
  release: () => void;
}

interface GlassSwitchInteractionState {
  isPressed: boolean;
  isTransitioning: boolean;
}

const IDLE_INTERACTION: GlassSwitchInteractionState = {
  isPressed: false,
  isTransitioning: false,
};

/**
 * 交互状态只响应已提交的属性变化，避免组件在 render 阶段修正自身状态。
 */
export function useGlassSwitchInteraction({
  checked,
  disabled,
}: GlassSwitchInteractionOptions): GlassSwitchInteraction {
  const previousCheckedRef = useRef(checked);
  const [interaction, setInteraction] = useState<GlassSwitchInteractionState>(IDLE_INTERACTION);

  useEffect(() => {
    const checkedChanged = previousCheckedRef.current !== checked;
    previousCheckedRef.current = checked;

    setInteraction((current) => {
      if (disabled) {
        return current.isPressed || current.isTransitioning
          ? IDLE_INTERACTION
          : current;
      }
      if (!checkedChanged || current.isTransitioning) {
        return current;
      }
      return { ...current, isTransitioning: true };
    });
  }, [checked, disabled]);

  const press = useCallback(() => {
    if (disabled) {
      return;
    }
    setInteraction((current) => (
      current.isPressed ? current : { ...current, isPressed: true }
    ));
  }, [disabled]);

  const release = useCallback(() => {
    setInteraction((current) => (
      current.isPressed ? { ...current, isPressed: false } : current
    ));
  }, []);

  const finishTransition = useCallback(() => {
    setInteraction((current) => (
      current.isTransitioning ? { ...current, isTransitioning: false } : current
    ));
  }, []);

  return {
    finishTransition,
    isPressed: interaction.isPressed,
    isTransitioning: interaction.isTransitioning,
    press,
    release,
  };
}
