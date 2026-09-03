/**
 * INPUT: Connector 名称与单项能力详情。
 * OUTPUT: 正文优先、技术 scopes 按需展开的 plain 能力预览。
 * POS: Connector 详情页的只读能力说明弹窗。
 */
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ConnectorFeatureDetail } from "@/types/capability/connector";

interface ConnectorFeatureDialogProps {
  connectorTitle: string;
  feature: ConnectorFeatureDetail | null;
  onClose: () => void;
}

export function ConnectorFeatureDialog({
  connectorTitle,
  feature,
  onClose,
}: ConnectorFeatureDialogProps) {
  if (!feature) {
    return null;
  }
  return (
    <UiDialogPortal>
      <UiDialogBackdrop layer="dialog" onClose={onClose}>
        <UiDialogShell size="lg" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={feature.name}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <p className="text-base leading-7 text-(--text-default)">
              {feature.description}
            </p>
            {feature.items?.length ? (
              <section>
                <h3 className="mb-2 text-compact font-semibold text-(--text-strong)">
                  包括
                </h3>
                <ul className="space-y-2 pl-4">
                  {feature.items.map((item) => (
                    <li
                      className="list-disc text-sm leading-6 text-(--text-default) marker:text-(--text-soft)"
                      key={item}
                    >
                      {item}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
            {feature.scopes?.length ? (
              <details className="border-t border-(--divider-subtle-color) pt-3 text-xs">
                <summary className="cursor-pointer select-none font-medium text-(--text-muted) hover:text-(--text-strong)">
                  OAuth scopes
                </summary>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {feature.scopes.map((scope) => (
                    <code
                      className="rounded-[5px] bg-(--surface-interactive-hover-background) px-1.5 py-0.5 text-(--text-muted)"
                      key={scope}
                    >
                      {scope}
                    </code>
                  ))}
                </div>
              </details>
            ) : null}
            <p className="sr-only">{connectorTitle}</p>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
