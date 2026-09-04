type Props = {
  /** "danger" for something that went wrong, "accent" for news. */
  tone: "danger" | "accent";
  children: React.ReactNode;
  /** The one thing to do about it, if there is one. */
  action?: { label: string; onClick: () => void; busy?: boolean };
  onDismiss?: () => void;
};

const TONES = {
  danger: { border: "border-danger/40", text: "text-danger", button: "bg-danger text-white hover:opacity-90" },
  accent: { border: "border-accent/40", text: "text-ink", button: "bg-accent text-accent-ink hover:opacity-90" },
} as const;

/**
 * A message in the corner: an error, or word of a new version. Both live here
 * so the two cannot drift apart in shape or position.
 */
export function Banner({ tone, children, action, onDismiss }: Props) {
  const t = TONES[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={`fixed bottom-4 right-4 z-50 flex max-w-md items-center gap-3 rounded-md border ${t.border} bg-surface-raised p-3 text-xs ${t.text} shadow-lg`}
    >
      <span className="min-w-0 flex-1">{children}</span>
      {action && (
        <button
          type="button"
          onClick={action.onClick}
          disabled={action.busy}
          className={`shrink-0 rounded px-2 py-1 font-medium ${t.button} disabled:opacity-60`}
        >
          {action.label}
        </button>
      )}
      {onDismiss && (
        <button type="button" onClick={onDismiss} className="shrink-0 text-ink-muted underline hover:text-ink">
          dismiss
        </button>
      )}
    </div>
  );
}
