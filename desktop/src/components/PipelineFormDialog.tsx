import { useEffect, useRef, useCallback } from "react";

import type { PipelineDraft } from "../lib/types";
import type { PipelineTemplate } from "../lib/templates";
import { PipelineForm } from "./PipelineForm";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  templates: PipelineTemplate[];
  onStart: (draft: PipelineDraft) => Promise<void>;
  disabled?: boolean;
  baseCommitSha: string | null;
};

export function PipelineFormDialog({ isOpen, onClose, templates, onStart, disabled, baseCommitSha }: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (isOpen && !dialog.open) {
      dialog.showModal();
    } else if (!isOpen && dialog.open) {
      dialog.close();
    }
  }, [isOpen]);

  const handleClose = useCallback(() => {
    onClose();
  }, [onClose]);

  const handleStart = useCallback(
    async (draft: PipelineDraft) => {
      await onStart(draft);
      onClose();
    },
    [onStart, onClose]
  );

  return (
    <dialog
      ref={dialogRef}
      className="pipeline-form-dialog"
      onClose={handleClose}
    >
      <div className="pipeline-form-dialog__header">
        <h2>Start Pipeline</h2>
        {baseCommitSha && (
          <p className="muted-text">
            Base commit: <code>{baseCommitSha.slice(0, 8)}</code>
          </p>
        )}
        <button type="button" onClick={onClose} className="pipeline-form-dialog__close">
          Close
        </button>
      </div>
      <PipelineForm templates={templates} onStart={handleStart} disabled={disabled} />
    </dialog>
  );
}
