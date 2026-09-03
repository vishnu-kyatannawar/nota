import { Dialog } from "./Dialog";

type Props = {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({ open, title, message, confirmLabel = "Confirm", danger, onConfirm, onCancel }: Props) {
  return (
    <Dialog open={open} onClose={onCancel} title={title} width="24rem">
      <p className="mb-5 text-sm text-ink-muted">{message}</p>
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onCancel} className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-surface-sunken">
          Cancel
        </button>
        <button
          type="button"
          onClick={onConfirm}
          autoFocus
          className={`rounded-md px-3 py-1.5 text-sm font-medium text-white ${danger ? "bg-danger" : "bg-accent"}`}
        >
          {confirmLabel}
        </button>
      </div>
    </Dialog>
  );
}
