import { Browser } from "@wailsio/runtime";
import type { Info } from "../lib/api";
import { Dialog } from "./Dialog";
import { Logo } from "./Logo";

export function AboutDialog({ open, info, onClose }: { open: boolean; info: Info | null; onClose: () => void }) {
  const link = (label: string, url: string) => (
    <button
      type="button"
      onClick={() => void Browser.OpenURL(url)}
      className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-left text-sm hover:bg-surface-sunken"
    >
      <span>{label}</span>
      <span className="font-mono text-[11px] text-ink-faint">{url.replace(/^https:\/\//, "")}</span>
    </button>
  );

  return (
    <Dialog open={open} onClose={onClose} title="About Nota">
      <div className="mb-5 flex items-center gap-4">
        <Logo size={52} />
        <div>
          <div className="text-lg font-semibold">Nota</div>
          <div className="font-mono text-xs text-ink-muted">version {info?.version ?? "…"}</div>
          <div className="mt-1 text-xs text-ink-muted">Daily workplans and notes, stored as plain markdown.</div>
        </div>
      </div>
      {info && (
        <div className="grid gap-2">
          {link("Website", info.website)}
          {link("Source on GitHub", info.repository)}
          {link("Releases", info.releases)}
          {link("MIT licence", info.licence)}
        </div>
      )}
      <p className="mt-4 text-[11px] text-ink-faint">Your pages live in <span className="font-mono">{info?.vaultPath}</span></p>
    </Dialog>
  );
}
