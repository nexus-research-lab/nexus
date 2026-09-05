// INPUT: Gallery locale and local File fixtures for image, text and ordinary attachments.
// OUTPUT: Real Composer draft previews and exact removal feedback without uploading any data.
// POS: Development-only attachment behavior and responsive geometry fixture.

import { useState } from "react";
import { ComposerAttachmentList } from "@/features/conversation/shared/composer/attachments/composer-local-attachments";
import type { ComposerLocalAttachment } from "@/features/conversation/shared/composer/attachments/composer-local-attachment-model";
import { COMPOSER_SHELL_CLASS_NAME } from "@/features/conversation/shared/composer/composer-styles";
import type { Locale } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { galleryText } from "./ui-gallery-copy";

function makeAttachments(): ComposerLocalAttachment[] {
  return [
    { id: "gallery-image", kind: "image", file: new File([
      '<svg xmlns="http://www.w3.org/2000/svg" width="480" height="320"><rect width="480" height="320" fill="#395c74"/><circle cx="240" cy="160" r="92" fill="#e8bc70"/></svg>',
    ], "preview-sample.svg", { type: "image/svg+xml" }) },
    { id: "gallery-text", kind: "text", file: new File([
      "Draft contents stay local.\n中文长文件名和正文保持可读。\n<script>window.attachmentExecuted = true</script>\n" + "long-content-".repeat(100),
    ], "review-notes-with-a-long-filename.txt", { type: "text/plain" }) },
    { id: "gallery-file", kind: "file", file: new File(["archive fixture"], "project-archive.zip", { type: "application/zip" }) },
  ];
}

export function ComposerAttachmentsGallery({ locale }: { locale: Locale }) {
  const [attachments, setAttachments] = useState(makeAttachments);
  const [removed, setRemoved] = useState<string[]>([]);
  return <section className="min-w-0 space-y-3" data-gallery-composer-attachments>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "Composer 草稿附件", "Composer draft attachments")}
    </h2>
    <div className={COMPOSER_SHELL_CLASS_NAME} data-gallery-composer-shell>
      <ComposerAttachmentList attachments={attachments} onRemove={(id) => {
        setAttachments((current) => current.filter((item) => item.id !== id));
        setRemoved((current) => [...current, id]);
      }} previewResetKey="gallery-draft" removeLabel="Remove draft attachment" />
    </div>
    <UiButton onClick={() => { setAttachments(makeAttachments()); setRemoved([]); }}>Restore attachment sample</UiButton>
    <output data-gallery-removed-attachments>{removed.join(",")}</output>
  </section>;
}
