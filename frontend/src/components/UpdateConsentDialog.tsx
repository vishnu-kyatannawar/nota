import { Dialog } from "./Dialog";
import type { UpdateCheck } from "../lib/api";

/**
 * Asked once, before anything is sent. Nota makes no network requests of any
 * other kind, so this is the whole of the decision — which is why it names
 * exactly what will happen rather than asking for a vague permission.
 */
export function UpdateConsentDialog({ open, onAnswer }: { open: boolean; onAnswer: (c: UpdateCheck) => void }) {
  return (
    <Dialog open={open} onClose={() => onAnswer("never")} title="Check for updates?" width="26rem">
      <p className="text-sm text-ink-muted">
        Nota can ask GitHub whether a newer version has been released — once when it starts, and once a day
        after that. It sends nothing about you or your pages.
      </p>
      <p className="mt-2 text-sm text-ink-muted">
        This is the only time Nota uses the network. You can change your mind in Settings.
      </p>
      <div className="mt-5 flex justify-end gap-2">
        <button
          type="button"
          onClick={() => onAnswer("never")}
          className="rounded-md border border-border px-3 py-1.5 text-sm text-ink hover:bg-surface-sunken"
        >
          No, stay offline
        </button>
        <button
          type="button"
          onClick={() => onAnswer("auto")}
          className="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90"
        >
          Yes, check
        </button>
      </div>
    </Dialog>
  );
}
