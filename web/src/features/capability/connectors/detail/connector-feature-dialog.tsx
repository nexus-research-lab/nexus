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
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
            <p className={getUiTypographyClassName({ role: "body", tone: "default" })}>
              {feature.description}
            </p>
            {feature.items?.length ? (
              <section>
                <h3 className={cn(
                  "mb-2",
                  getUiTypographyClassName({
                    role: "sectionTitle",
                    tone: "strong",
                    weight: "semibold",
                  }),
                )}>
                  包括
                </h3>
                <ul className="space-y-2 pl-4">
                  {feature.items.map((item) => (
                    <li
                      className={cn(
                        "list-disc marker:text-(--text-soft)",
                        getUiTypographyClassName({ role: "supporting", tone: "default" }),
                      )}
                      key={item}
                    >
                      {item}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
            {feature.scopes?.length ? (
              <UiDisclosure
                contentClassName="flex flex-wrap gap-1.5"
                density="compact"
                label="OAuth scopes"
                summaryRole="caption"
                summaryTone="muted"
                variant="section"
              >
                  {feature.scopes.map((scope) => (
                    <code
                      className={cn(
                        "radius-control-xs bg-(--surface-interactive-hover-background) px-1.5 py-0.5",
                        getUiTypographyClassName({ role: "code", tone: "muted" }),
                      )}
                      key={scope}
                    >
                      {scope}
                    </code>
                  ))}
              </UiDisclosure>
            ) : null}
            <p className="sr-only">{connectorTitle}</p>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
