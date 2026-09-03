import { useEffect, useState } from "react";
import { AppService } from "../bindings/github.com/vishnu-kyatannawar/nota/services";

type Info = {
  version: string;
  vaultPath: string;
  workplanDir: string;
};

/**
 * M0 shell. The layout is the one the app keeps — a folder sidebar beside a
 * single note pane — but both sides are placeholders until the vault lands in M1.
 * What it does prove is the whole pipeline: React renders, Tailwind styles, and
 * the Go service answers over the Wails bindings.
 */
function App() {
  const [info, setInfo] = useState<Info | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    AppService.GetInfo()
      .then((result: Info) => setInfo(result))
      .catch((err: unknown) => setError(String(err)));
  }, []);

  return (
    <div className="flex h-full">
      <aside className="w-56 shrink-0 border-r border-surface-border bg-surface-raised p-4">
        <div className="mb-4 text-[10px] font-bold tracking-widest text-ink-muted">VAULT</div>
        <div className="rounded px-2 py-1.5 text-sm text-accent">Workplans</div>
        <div className="rounded px-2 py-1.5 text-sm text-ink-muted">Projects</div>
      </aside>

      <main className="min-w-0 flex-1 p-8">
        <h1 className="text-2xl font-bold">Nota</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Daily workplans and notes, stored as plain markdown.
        </p>

        {error && (
          <p className="mt-6 rounded border border-red-900 bg-red-950/40 p-3 text-sm text-red-300">
            Could not reach the Go service: {error}
          </p>
        )}

        {info && (
          <dl className="mt-6 grid max-w-xl grid-cols-[8rem_minmax(0,1fr)] gap-y-2 text-sm">
            <dt className="text-ink-muted">Version</dt>
            <dd className="font-mono">{info.version}</dd>
            <dt className="text-ink-muted">Vault</dt>
            <dd className="truncate font-mono">{info.vaultPath}</dd>
            <dt className="text-ink-muted">Workplans</dt>
            <dd className="truncate font-mono">{info.workplanDir}</dd>
          </dl>
        )}
      </main>
    </div>
  );
}

export default App;
