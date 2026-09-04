import { useState } from "react";
import { Browser } from "@wailsio/runtime";
import type { Info } from "../lib/api";
import { api } from "../lib/api";
import { Dialog } from "./Dialog";
import { Logo } from "./Logo";

export function AboutDialog({ open, info, onClose }: { open: boolean; info: Info | null; onClose: () => void }) {
  const [checking, setChecking] = useState(false);
  const [result, setResult] = useState<string | null>(null);

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

  const check = async () => {
    setChecking(true);
    setResult(null);
    try {
      const state = await api.checkUpdate(true);
      setResult(
        state.phase === "available" ? `Nota ${state.version} is available — see Releases.`
        : state.phase === "current" ? "You are on the latest version."
        : state.phase === "failed" ? `Could not check: ${state.message}`
        : null,
      );
    } catch (e) {
      setResult(`Could not check: ${String(e)}`);
    } finally {
      setChecking(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} title="About Nota">
      <div className="mb-5 flex items-center gap-4">
        <Logo size={52} />
        <div>
          <div className="text-lg font-semibold">Nota</div>
          <div className="flex items-center gap-2">
            <span className="font-mono text-xs text-ink-muted">version {info?.version ?? "…"}</span>
            <button
              type="button"
              onClick={() => void check()}
              disabled={checking}
              className="rounded border border-border px-1.5 py-0.5 text-[11px] text-ink-muted hover:bg-surface-sunken hover:text-ink disabled:opacity-60"
            >
              {checking ? "Checking…" : "Check for updates"}
            </button>
          </div>
          {result && <div className="mt-1 text-[11px] text-ink-muted">{result}</div>}
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
