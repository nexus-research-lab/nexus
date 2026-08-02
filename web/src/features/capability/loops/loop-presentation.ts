import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Locale, TranslationKey } from "@/shared/i18n/messages";
import type { LoopCatalogItem } from "@/types/capability/loop";

type Translate = I18nContextValue["t"];

interface LoopMetadataPresentation {
  installsLabel: string;
  triggerLabel: string;
  viewsLabel: string;
}

const LOOP_TRIGGER_MESSAGE_KEYS: Readonly<
  Partial<Record<string, TranslationKey>>
> = {
  event: "capability.loops_trigger_event",
  interval: "capability.loops_trigger_interval",
  manual: "capability.loops_trigger_manual",
};

export function getLoopTriggerLabel(
  triggerType: string,
  t: Translate,
): string {
  const messageKey = LOOP_TRIGGER_MESSAGE_KEYS[triggerType];
  return messageKey ? t(messageKey) : triggerType;
}

export function buildLoopMetadataPresentation(
  loop: Pick<LoopCatalogItem, "installs" | "trigger_type" | "views">,
  locale: Locale,
  t: Translate,
): LoopMetadataPresentation {
  const numberFormatter = new Intl.NumberFormat(locale);
  return {
    installsLabel: t("capability.loops_installs", {
      count: numberFormatter.format(loop.installs),
    }),
    triggerLabel: getLoopTriggerLabel(loop.trigger_type, t),
    viewsLabel: t("capability.loops_views", {
      count: numberFormatter.format(loop.views),
    }),
  };
}
