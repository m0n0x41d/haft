import type { ReactNode } from "react";

import type { IrreversibleActionDialogModel } from "./irreversibleActionDialogModel";

export function IrreversibleActionDialog({
  model,
  reason,
  isSubmitting,
  onReasonChange,
  onCancel,
  onConfirm,
}: {
  model: IrreversibleActionDialogModel;
  reason: string;
  isSubmitting: boolean;
  onReasonChange: (value: string) => void;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const confirmClassName = confirmationButtonClassName(model.tone);
  const isConfirmDisabled = isSubmitting || (model.requiresReason && reason.trim() === "");

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4">
      <div className="w-full max-w-2xl rounded-2xl border border-border bg-surface-1">
        <div className="flex items-start justify-between gap-4 border-b border-border px-6 py-4">
          <div>
            <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">
              Irreversible Action
            </p>
            <h3 className="mt-2 text-lg font-semibold text-text-primary">{model.heading}</h3>
            <p className="mt-1 text-sm text-text-secondary">{model.description}</p>
          </div>

          <button
            onClick={onCancel}
            disabled={isSubmitting}
            className="text-sm text-text-muted transition-colors hover:text-text-primary disabled:opacity-50"
          >
            x
          </button>
        </div>

        <div className="space-y-5 px-6 py-5">
          <DialogSection title="What Will Happen">
            <ul className="space-y-2 text-sm text-text-secondary">
              {model.whatWillHappen.map((step) => (
                <li key={step} className="flex gap-2">
                  <span className="mt-[7px] h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
                  <span>{step}</span>
                </li>
              ))}
            </ul>
          </DialogSection>

          <DialogSection title="Cannot Be Undone">
            <div className="rounded-xl border border-danger/20 bg-danger/5 px-4 py-3 text-sm text-text-secondary">
              {model.irreversibleWarning}
            </div>
          </DialogSection>

          {model.warnings.length > 0 && (
            <DialogSection title="Warnings">
              <div className="space-y-2">
                {model.warnings.map((warning) => (
                  <div
                    key={warning}
                    className="rounded-xl border border-warning/20 bg-warning/10 px-4 py-3 text-sm text-text-secondary"
                  >
                    {warning}
                  </div>
                ))}
              </div>
            </DialogSection>
          )}

          <DialogSection title="Affected Artifacts">
            <div className="space-y-2">
              {model.affectedArtifacts.map((artifact) => (
                <div
                  key={`${artifact.kind}-${artifact.ref}-${artifact.title}`}
                  className="rounded-xl border border-border bg-surface-2/40 px-4 py-3"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full border border-border px-2 py-0.5 text-[11px] text-text-muted">
                      {artifact.kind}
                    </span>
                    {artifact.ref && (
                      <span className="font-mono text-xs text-text-muted">{artifact.ref}</span>
                    )}
                  </div>
                  {artifact.title && (
                    <p className="mt-2 text-sm text-text-primary">{artifact.title}</p>
                  )}
                </div>
              ))}
            </div>
          </DialogSection>

          {model.requiresReason && (
            <DialogSection title={model.reasonLabel}>
              <textarea
                value={reason}
                onChange={(event) => onReasonChange(event.target.value)}
                placeholder={model.reasonPlaceholder}
                rows={4}
                className="w-full rounded-xl border border-border bg-surface-0 px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-faint focus:border-border-bright"
              />
            </DialogSection>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-4">
          <button
            onClick={onCancel}
            disabled={isSubmitting}
            className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-sm text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={isConfirmDisabled}
            className={confirmClassName}
          >
            {isSubmitting ? model.busyLabel : model.confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

function DialogSection({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">{title}</p>
      {children}
    </div>
  );
}

function confirmationButtonClassName(tone: "accent" | "warning" | "danger"): string {
  const toneClassNames: Record<typeof tone, string> = {
    accent:
      "rounded-lg bg-accent px-3 py-1.5 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50",
    warning:
      "rounded-lg border border-warning/20 bg-warning/10 px-3 py-1.5 text-sm text-warning transition-colors hover:bg-warning/20 disabled:cursor-not-allowed disabled:opacity-50",
    danger:
      "rounded-lg border border-danger/20 bg-danger/10 px-3 py-1.5 text-sm text-danger transition-colors hover:bg-danger/20 disabled:cursor-not-allowed disabled:opacity-50",
  };

  return toneClassNames[tone];
}
