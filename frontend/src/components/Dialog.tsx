import { useEffect, useRef } from "react";

type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  width?: string;
};

/** A native <dialog>, so Escape, focus trapping and the backdrop come for free. */
export function Dialog({ open, onClose, title, children, width = "28rem" }: Props) {
  const ref = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      onClose={onClose}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
      className="m-auto w-full rounded-lg border border-border bg-surface-raised p-0 text-ink shadow-2xl backdrop:bg-black/40"
      style={{ maxWidth: width }}
    >
      <div className="p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold">{title}</h2>
          <button type="button" onClick={onClose} aria-label="Close" className="rounded px-2 text-ink-muted hover:bg-surface-sunken">
            ✕
          </button>
        </div>
        {children}
      </div>
    </dialog>
  );
}
