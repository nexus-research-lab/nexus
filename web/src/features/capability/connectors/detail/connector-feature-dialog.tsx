/**
 * INPUT: Connector 名称与单项能力详情。
 * OUTPUT: 关联 Connector 身份、正文可换行且技术 scopes 按需展开的本地化能力预览。
 * POS: Connector 详情页的只读能力说明弹窗。
 */
import { useId } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { t } = useI18n();
  const connectorDescriptionId = useId();
  if (!feature) {
    return null;
  }
  return (
    <UiDialogPortal>
      <UiDialogBackdrop describedBy={connectorDescriptionId} layer="dialog" onClose={onClose}>
        <UiDialogShell size="lg" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={feature.name}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <p className={cn("break-words [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "body", tone: "default" }))}>
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
                  {t("capability.connector_feature_includes")}
                </h3>
                <ul className="space-y-2 pl-4">
                  {feature.items.map((item) => (
                    <li
                      className={cn(
                        "list-disc break-words [overflow-wrap:anywhere] marker:text-(--text-muted)",
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
                label={t("capability.connector_feature_scopes")}
                summaryRole="supporting"
                summaryTone="muted"
                variant="section"
              >
                  {feature.scopes.map((scope) => (
                    <code
                      className={cn(
                        "max-w-full break-words [overflow-wrap:anywhere] radius-control-xs bg-(--surface-interactive-hover-background) px-1.5 py-0.5",
                        getUiTypographyClassName({ role: "code", tone: "muted" }),
                      )}
                      key={scope}
                    >
                      {scope}
                    </code>
                  ))}
              </UiDisclosure>
            ) : null}
            <p className="sr-only" id={connectorDescriptionId}>{connectorTitle}</p>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
